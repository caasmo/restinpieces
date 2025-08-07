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
	name             string        // name is the identifier for the daemon.
	startShouldError error         // The error to return when Start() is called. If nil, Start succeeds.
	stopShouldError  error         // The error to return when Stop() is called. If nil, Stop succeeds.
	startCalledChan  chan bool     // A channel that receives a value when Start() is called.
	stopCalledChan   chan bool     // A channel that receives a value when Stop() is called.
	startDelay       time.Duration // A delay to wait before the Start() method returns.
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
	if fd.startDelay > 0 {
		time.Sleep(fd.startDelay)
	}
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

// TestServer_Run_HttpAndDaemonStartFailure is a regression test to ensure the
// server does not deadlock during startup if both the HTTP server and a daemon
// fail concurrently.
//
// Original Problem:
// The `serverError` channel was buffered to 1. If the HTTP server goroutine
// failed and sent an error, the channel would be full. If a daemon subsequently
// failed in the main goroutine, its attempt to send a second error would block
// indefinitely, causing a deadlock.
//
// Fix:
// The `serverError` channel buffer was increased to 2.
//
// How this test works:
// 1. It configures the HTTP server to fail immediately (by enabling TLS without certs).
// 2. It uses a `fakeDaemon` with a small `startDelay` to ensure the HTTP server
//    error is sent first. The daemon is also configured to fail.
// 3. The test verifies that the server can receive both errors without
//    deadlocking and proceeds to a graceful shutdown, exiting with a non-zero
//    code. A timeout in this test would indicate a regression of the deadlock bug.
func TestServer_Run_HttpAndDaemonStartFailure(t *testing.T) {
	// 1. Setup
	// Create a server config that will cause an error during HTTP server setup
	// (e.g., by enabling TLS without providing certificates).
	server, provider := newTestServer(t, nil)
	cfg := provider.Get()
	cfg.Server.EnableTLS = true // Enable TLS
	cfg.Server.CertData = ""    // but provide no cert
	cfg.Server.KeyData = ""     // which will cause createTLSConfig to fail
	provider.Update(cfg)

	// Add a daemon that is also configured to fail on start.
	d := newFakeDaemon("daemon-fail")
	d.startShouldError = errors.New("daemon startup failed")
	// Crucially, add a small delay. This makes it highly probable that the
	// HTTP server goroutine fails and sends its error to the buffered channel
	// *before* this daemon's Start() method is even called.
	d.startDelay = 50 * time.Millisecond
	server.AddDaemon(d)

	exitCalledChan := make(chan int, 1)
	server.exitFunc = func(code int) {
		exitCalledChan <- code
	}

	// 2. Action
	go server.Run()

	// 3. Verification
	// The server should detect one of the errors and shut down, exiting with a
	// non-zero status code. The crucial part is that it should not deadlock.
	// We use a timeout to detect a potential deadlock.
	select {
	case code := <-exitCalledChan:
		if code == 0 {
			t.Error("expected non-zero exit code for startup failure, got 0")
		}
		// If we receive an exit code, it means the server did not deadlock.
	case <-time.After(500 * time.Millisecond): // Timeout longer than shutdown timeout
		t.Fatal("timed out waiting for server to exit, potential deadlock detected")
	}
}