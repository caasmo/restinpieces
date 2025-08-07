package server

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"io"
	"log/slog"
	"math/big"
	"net/http"
	"net/http/httptest"
	"syscall"
	"testing"
	"time"

	"github.com/caasmo/restinpieces/config"
	"github.com/caasmo/restinpieces/db"
	"github.com/caasmo/restinpieces/queue/executor"
	scl "github.com/caasmo/restinpieces/queue/scheduler"
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

// mockJobExecutor is a spy for the JobExecutor interface.
type mockJobExecutor struct {
	registeredJobType string
	registeredHandler executor.JobHandler
}

func (m *mockJobExecutor) Register(jobType string, handler executor.JobHandler) {
	m.registeredJobType = jobType
	m.registeredHandler = handler
}

func (m *mockJobExecutor) Execute(ctx context.Context, job db.Job) error {
	return nil // Not used in these tests
}

// mockJobHandler is a placeholder JobHandler.
type mockJobHandler struct{}

func (m *mockJobHandler) Handle(ctx context.Context, job db.Job) error {
	return nil // Not used in these tests
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

// generateTestCert creates a self-signed certificate and key for testing.
func generateTestCert(t *testing.T) (certPEM, keyPEM []byte) {
	t.Helper()

	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate private key: %v", err)
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Test Co"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		t.Fatalf("Failed to create certificate: %v", err)
	}

	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})

	keyBytes, err := x509.MarshalECPrivateKey(privateKey)
	if err != nil {
		t.Fatalf("Failed to marshal private key: %v", err)
	}
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyBytes})

	return certPEM, keyPEM
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

func TestAddDaemon_Nil(t *testing.T) {
	server, _ := newTestServer(t, nil)
	// Adding a nil daemon should not panic and should be logged.
	// Since we discard logs, we just check that it doesn't panic.
	server.AddDaemon(nil)
	if len(server.daemons) != 0 {
		t.Error("expected daemon list to be empty after adding nil")
	}
}

func TestRedirectToHTTPS(t *testing.T) {
	// 1. Setup
	server, provider := newTestServer(t, nil)
	cfg := provider.Get()
	// Configure the server's BaseURL by setting the relevant fields in the config.
	cfg.Server.EnableTLS = true
	cfg.Server.Addr = "secure.example.com:8443" // This will be used by BaseURL()
	provider.Update(cfg)

	handler := server.redirectToHTTPS()

	// Create a test request with a path and query string.
	req, err := http.NewRequest("GET", "/test/path?query=val", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	// The redirectToHTTPS handler uses `r.URL.RequestURI()` which is derived
	// from the original request line.
	req.RequestURI = "/test/path?query=val"

	rr := httptest.NewRecorder()

	// 2. Action
	handler.ServeHTTP(rr, req)

	// 3. Verification
	if status := rr.Code; status != http.StatusMovedPermanently {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusMovedPermanently)
	}

	// The BaseURL() function will construct this from the config.
	// Note: We assume sanitizeAddrEmptyHost in config.go correctly handles this.
	expectedURL := "https://secure.example.com:8443/test/path?query=val"
	if location := rr.Header().Get("Location"); location != expectedURL {
		t.Errorf("handler returned wrong redirect location: got %q want %q",
			location, expectedURL)
	}
}

func TestAddJobHandler_Success(t *testing.T) {
	// 1. Setup
	server, provider := newTestServer(t, nil)
	mockExec := &mockJobExecutor{}
	// Create a real scheduler, but inject our mock executor.
	// This satisfies the type assertion while letting us spy on the Register call.
	scheduler := scl.NewScheduler(provider, nil, mockExec, slog.New(slog.NewTextHandler(io.Discard, nil)))
	server.AddDaemon(scheduler)

	handler := &mockJobHandler{}
	jobType := "test-job"

	// 2. Action
	err := server.AddJobHandler(jobType, handler)

	// 3. Verification
	if err != nil {
		t.Fatalf("AddJobHandler returned an unexpected error: %v", err)
	}
	if mockExec.registeredJobType != jobType {
		t.Errorf("handler registered with wrong job type: got %q want %q", mockExec.registeredJobType, jobType)
	}
	if mockExec.registeredHandler != handler {
		t.Error("handler registered was not the one provided")
	}
}

func TestAddJobHandler_SchedulerNotFound(t *testing.T) {
	// 1. Setup
	server, _ := newTestServer(t, nil) // No daemons added
	handler := &mockJobHandler{}

	// 2. Action
	err := server.AddJobHandler("any-job", handler)

	// 3. Verification
	if err == nil {
		t.Fatal("AddJobHandler should have returned an error, but did not")
	}
}

func TestAddJobHandler_IncorrectDaemonType(t *testing.T) {
	// 1. Setup
	server, _ := newTestServer(t, nil)
	// Add a daemon with the correct name, but wrong type.
	wrongDaemon := newFakeDaemon("Scheduler")
	server.AddDaemon(wrongDaemon)
	handler := &mockJobHandler{}

	// 2. Action
	err := server.AddJobHandler("any-job", handler)

	// 3. Verification
	if err == nil {
		t.Fatal("AddJobHandler should have returned an error, but did not")
	}
}

func TestAddJobHandler_NilExecutor(t *testing.T) {
	// 1. Setup
	server, provider := newTestServer(t, nil)
	// Create a real scheduler, but with a nil executor.
	scheduler := scl.NewScheduler(provider, nil, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	server.AddDaemon(scheduler)
	handler := &mockJobHandler{}

	// 2. Action
	err := server.AddJobHandler("any-job", handler)

	// 3. Verification
	if err == nil {
		t.Fatal("AddJobHandler should have returned an error, but did not")
	}
}

func TestCreateTLSConfig_Success(t *testing.T) {
	certPEM, keyPEM := generateTestCert(t)
	cfg := &config.Server{
		CertData: string(certPEM),
		KeyData:  string(keyPEM),
	}

	tlsConfig, err := createTLSConfig(cfg)

	if err != nil {
		t.Fatalf("createTLSConfig returned an unexpected error: %v", err)
	}
	if tlsConfig == nil {
		t.Fatal("createTLSConfig returned a nil config")
	}
	if len(tlsConfig.Certificates) != 1 {
		t.Errorf("expected 1 certificate, got %d", len(tlsConfig.Certificates))
	}
	if tlsConfig.MinVersion != tls.VersionTLS13 {
		t.Errorf("expected MinVersion to be TLS 1.3, got %d", tlsConfig.MinVersion)
	}
}

func TestCreateTLSConfig_InvalidKeyPair(t *testing.T) {
	certPEM, _ := generateTestCert(t)
	_, keyPEM2 := generateTestCert(t) // Mismatched key
	cfg := &config.Server{
		CertData: string(certPEM),
		KeyData:  string(keyPEM2),
	}

	_, err := createTLSConfig(cfg)

	if err == nil {
		t.Fatal("createTLSConfig should have returned an error for mismatched key pair, but did not")
	}
}

func TestCreateTLSConfig_MissingData(t *testing.T) {
	certPEM, keyPEM := generateTestCert(t)

	testCases := []struct {
		name     string
		cfg      *config.Server
		expected bool
	}{
		{
			name: "Missing CertData",
			cfg: &config.Server{
				KeyData: string(keyPEM),
			},
		},
		{
			name: "Missing KeyData",
			cfg: &config.Server{
				CertData: string(certPEM),
			},
		},
		{
			name: "Missing Both",
			cfg:  &config.Server{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := createTLSConfig(tc.cfg)
			if err == nil {
				t.Errorf("createTLSConfig should have returned an error but did not")
			}
		})
	}
}
