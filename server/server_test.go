package server

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"syscall"
	"testing"
	"time"

	"github.com/caasmo/restinpieces/config"
)

// --- Test Fakes and Mocks ---

type fakeDaemon struct {
	name             string
	startShouldError error
	stopShouldError  error
	startCalledChan  chan bool
	stopCalledChan   chan bool
}

func newFakeDaemon(name string) *fakeDaemon {
	return &fakeDaemon{
		name:            name,
		startCalledChan: make(chan bool, 1),
		stopCalledChan:  make(chan bool, 1),
	}
}

func (fd *fakeDaemon) Name() string { return fd.name }

func (fd *fakeDaemon) Start() error {
	fd.startCalledChan <- true
	return fd.startShouldError
}

func (fd *fakeDaemon) Stop(ctx context.Context) error {
	fd.stopCalledChan <- true
	return fd.stopShouldError
}

// --- Test Helper Functions ---

func newTestServer(t *testing.T, reloadFunc func() error) (*Server, *config.Provider) {
	t.Helper()
	cfg := config.NewDefaultConfig()
	cfg.Server.Addr = ":0" // Use random free port
	cfg.Server.ShutdownGracefulTimeout.Duration = 200 * time.Millisecond
	provider := config.NewProvider(cfg)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	if reloadFunc == nil {
		reloadFunc = func() error { return nil }
	}
	return NewServer(provider, handler, logger, reloadFunc), provider
}

// --- Test Cases ---

func TestServer_Run_FullLifecycle(t *testing.T) {
	// 1. Setup
	server, _ := newTestServer(t, nil)
	d := newFakeDaemon("test-daemon")
	server.AddDaemon(d)

	exitCalledChan := make(chan int, 1)
	server.exitFunc = func(code int) {
		exitCalledChan <- code
	}

	// 2. Action
	go server.Run()

	// 3. Verification (Startup)
	select {
	case <-d.startCalledChan:
		// Daemon started, good.
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timed out waiting for daemon to start")
	}

	// 4. Action (Shutdown)
	// Send signal to trigger shutdown
	if err := syscall.Kill(syscall.Getpid(), syscall.SIGINT); err != nil {
		t.Fatalf("Failed to send SIGINT: %v", err)
	}

	// 5. Verification (Shutdown)
	select {
	case <-d.stopCalledChan:
		// Daemon stopped, good.
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timed out waiting for daemon to stop")
	}

	select {
	case code := <-exitCalledChan:
		if code != 0 {
			t.Errorf("expected exit code 0 for graceful shutdown, got %d", code)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timed out waiting for server to exit")
	}
}

func TestServer_Run_DaemonStartFailure(t *testing.T) {
	// 1. Setup
	server, _ := newTestServer(t, nil)
	d1 := newFakeDaemon("daemon1-ok")
	d2 := newFakeDaemon("daemon2-fail")
	d2.startShouldError = errors.New("startup failed")
	server.AddDaemon(d1)
	server.AddDaemon(d2)

	exitCalledChan := make(chan int, 1)
	server.exitFunc = func(code int) {
		exitCalledChan <- code
	}

	// 2. Action
	go server.Run()

	// 3. Verification
	// d1 should start successfully.
	select {
	case <-d1.startCalledChan:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timed out waiting for daemon1 to start")
	}

	// d2 start should be attempted.
	select {
	case <-d2.startCalledChan:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timed out waiting for daemon2 start to be attempted")
	}

	// Because d2 failed, d1 should be stopped as part of cleanup.
	select {
	case <-d1.stopCalledChan:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timed out waiting for daemon1 to be stopped during cleanup")
	}

	// The server should exit with a non-zero code.
	select {
	case code := <-exitCalledChan:
		if code == 0 {
			t.Error("expected non-zero exit code for startup failure, got 0")
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timed out waiting for server to exit after daemon failure")
	}
}

func TestServer_Run_HandlesSIGHUP(t *testing.T) {
	// 1. Setup
	reloadCalledChan := make(chan bool, 1)
	reloader := func() error {
		reloadCalledChan <- true
		return nil
	}
	server, _ := newTestServer(t, reloader)

	exitCalledChan := make(chan int, 1)
	server.exitFunc = func(code int) {
		exitCalledChan <- code
	}

	// 2. Action
	go server.Run()

	// Give server time to start listening for signals
	time.Sleep(20 * time.Millisecond)

	if err := syscall.Kill(syscall.Getpid(), syscall.SIGHUP); err != nil {
		t.Fatalf("Failed to send SIGHUP: %v", err)
	}

	// 3. Verification
	select {
	case <-reloadCalledChan:
		// Success
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timed out waiting for reload func to be called")
	}

	// Ensure the server did NOT exit
	select {
	case code := <-exitCalledChan:
		t.Fatalf("server exited with code %d after SIGHUP, but should have continued running", code)
	default:
		// Good, server is still running
	}

	// 4. Cleanup
	if err := syscall.Kill(syscall.Getpid(), syscall.SIGINT); err != nil {
		t.Fatalf("Failed to send SIGINT for cleanup: %v", err)
	}
	select {
	case <-exitCalledChan:
		// Final shutdown successful
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timed out waiting for server to exit during cleanup")
	}
}