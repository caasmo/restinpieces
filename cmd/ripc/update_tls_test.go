package main

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net"
	"strings"
	"testing"
	"time"
)

func newTlsTestPair(t *testing.T, notBefore, notAfter time.Time) (string, string) {
	t.Helper()

	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate private key: %v", err)
	}

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		t.Fatalf("failed to generate serial number: %v", err)
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Test Co"},
		},
		NotBefore: notBefore,
		NotAfter:  notAfter,

		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("failed to create certificate: %v", err)
	}

	certOut := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})

	privBytes, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		t.Fatalf("failed to marshal private key: %v", err)
	}
	keyOut := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: privBytes})

	return string(certOut), string(keyOut)
}

func updateTlsTestConf(certificatePEM string, privateKeyPEM string) string {
	return updateTlsTestConfWithLive(certificatePEM, privateKeyPEM, "old", "old")
}

// updateTlsTestConfWithLive builds a config with the staged pair under acme and
// the given live pair under server.tls. The staged pair is written the way the
// acme daemon stores it, as a multi-line string.
func updateTlsTestConfWithLive(certificatePEM string, privateKeyPEM string, liveCertificate string, livePrivateKey string) string {
	return fmt.Sprintf(`# Staged certificate and key, written by the acme daemon
[acme]
  certificate = """
%s"""
  private_key = """
%s"""

[server]
  # Address the server listens on
  addr = ":8080"

  [server.tls]
    # Certificate the server serves to clients
    certificate = %q
    # Key the server serves to clients
    private_key = %q
`, certificatePEM, privateKeyPEM, liveCertificate, livePrivateKey)
}

func TestUpdate_TlsHappyPath(t *testing.T) {
	now := time.Now()
	certificatePEM, privateKeyPEM := newTlsTestPair(t, now.Add(-time.Hour), now.Add(24*time.Hour))
	scope := "app"
	mockStore := NewMockUpdateSecureStore(map[string][]byte{scope: []byte(updateTlsTestConf(certificatePEM, privateKeyPEM))})
	var stdout, stderr bytes.Buffer
	ui := UI{Out: &stdout, Err: &stderr}

	err := updateValues(ui, mockStore, scope, "", "tls")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(mockStore.saveHistory) != 1 {
		t.Fatalf("expected 1 save, got %d", len(mockStore.saveHistory))
	}

	tree := getUpdateTreeFromStore(t, mockStore, scope)
	if got := tree.Get("server.tls.certificate"); got != certificatePEM {
		t.Errorf("expected server.tls.certificate to match staged certificate")
	}
	if got := tree.Get("server.tls.private_key"); got != privateKeyPEM {
		t.Errorf("expected server.tls.private_key to match staged private key")
	}

	saved := getUpdateTreeWithComments(t, mockStore, scope)
	commentTests := []struct {
		path string
		want string
	}{
		{path: "server.tls.certificate", want: "Certificate the server serves to clients"},
		{path: "server.addr", want: "Address the server listens on"},
	}
	for _, tt := range commentTests {
		if got := commentAt(t, saved, tt.path); got != tt.want {
			t.Errorf("comment at %q = %q, want %q", tt.path, got, tt.want)
		}
	}

	if raw := getUpdateTomlFromStore(t, mockStore, scope); !strings.Contains(raw, "certificate = \"\"\"") {
		t.Errorf("staged certificate lost its multiline form:\n%s", raw)
	}

	output := stderr.String()
	if !strings.Contains(output, "SHA256") {
		t.Errorf("expected fingerprints in output, got %q", output)
	}
	if strings.Contains(output, "BEGIN CERTIFICATE") || strings.Contains(output, "BEGIN EC PRIVATE KEY") || strings.Contains(output, "BEGIN PRIVATE KEY") {
		t.Errorf("expected no PEM text in output, got %q", output)
	}
}

func TestUpdate_TlsSamePairWritesNothing(t *testing.T) {
	now := time.Now()
	certificatePEM, privateKeyPEM := newTlsTestPair(t, now.Add(-time.Hour), now.Add(24*time.Hour))
	scope := "app"
	conf := updateTlsTestConfWithLive(certificatePEM, privateKeyPEM, certificatePEM, privateKeyPEM)
	mockStore := NewMockUpdateSecureStore(map[string][]byte{scope: []byte(conf)})
	var stdout, stderr bytes.Buffer
	ui := UI{Out: &stdout, Err: &stderr}

	err := updateValues(ui, mockStore, scope, "", "tls")
	if !errors.Is(err, ErrTLSSamePair) {
		t.Fatalf("expected error to wrap ErrTLSSamePair, got %v", err)
	}

	if len(mockStore.saveHistory) != 0 {
		t.Fatalf("expected 0 saves when the pair is already live, got %d", len(mockStore.saveHistory))
	}
}

func TestUpdate_TlsEmptyStagedWritesNothing(t *testing.T) {
	scope := "app"
	mockStore := NewMockUpdateSecureStore(map[string][]byte{scope: []byte(updateTlsTestConf("", ""))})
	var stdout, stderr bytes.Buffer
	ui := UI{Out: &stdout, Err: &stderr}

	err := updateValues(ui, mockStore, scope, "", "tls")
	if !errors.Is(err, ErrTLSEmptyStaged) {
		t.Fatalf("expected error to wrap ErrTLSEmptyStaged, got %v", err)
	}

	if len(mockStore.saveHistory) != 0 {
		t.Fatalf("expected 0 saves, got %d", len(mockStore.saveHistory))
	}
}

func TestUpdate_TlsBadCertificateWritesNothing(t *testing.T) {
	now := time.Now()
	_, privateKeyPEM := newTlsTestPair(t, now.Add(-time.Hour), now.Add(24*time.Hour))
	scope := "app"
	mockStore := NewMockUpdateSecureStore(map[string][]byte{scope: []byte(updateTlsTestConf("not a cert", privateKeyPEM))})
	var stdout, stderr bytes.Buffer
	ui := UI{Out: &stdout, Err: &stderr}

	err := updateValues(ui, mockStore, scope, "", "tls")
	if !errors.Is(err, ErrTLSBadCertificate) {
		t.Fatalf("expected error to wrap ErrTLSBadCertificate, got %v", err)
	}

	if len(mockStore.saveHistory) != 0 {
		t.Fatalf("expected 0 saves, got %d", len(mockStore.saveHistory))
	}
}

func TestUpdate_TlsBadPrivateKeyWritesNothing(t *testing.T) {
	now := time.Now()
	certificatePEM, _ := newTlsTestPair(t, now.Add(-time.Hour), now.Add(24*time.Hour))
	scope := "app"
	mockStore := NewMockUpdateSecureStore(map[string][]byte{scope: []byte(updateTlsTestConf(certificatePEM, "not a key"))})
	var stdout, stderr bytes.Buffer
	ui := UI{Out: &stdout, Err: &stderr}

	err := updateValues(ui, mockStore, scope, "", "tls")
	if !errors.Is(err, ErrTLSPairUnusable) {
		t.Fatalf("expected error to wrap ErrTLSPairUnusable, got %v", err)
	}

	if len(mockStore.saveHistory) != 0 {
		t.Fatalf("expected 0 saves, got %d", len(mockStore.saveHistory))
	}
}

func TestUpdate_TlsMismatchedPairWritesNothing(t *testing.T) {
	now := time.Now()
	certificatePEM, _ := newTlsTestPair(t, now.Add(-time.Hour), now.Add(24*time.Hour))
	_, otherPrivateKeyPEM := newTlsTestPair(t, now.Add(-time.Hour), now.Add(24*time.Hour))
	scope := "app"
	mockStore := NewMockUpdateSecureStore(map[string][]byte{scope: []byte(updateTlsTestConf(certificatePEM, otherPrivateKeyPEM))})
	var stdout, stderr bytes.Buffer
	ui := UI{Out: &stdout, Err: &stderr}

	err := updateValues(ui, mockStore, scope, "", "tls")
	if !errors.Is(err, ErrTLSPairUnusable) {
		t.Fatalf("expected error to wrap ErrTLSPairUnusable, got %v", err)
	}

	if len(mockStore.saveHistory) != 0 {
		t.Fatalf("expected 0 saves, got %d", len(mockStore.saveHistory))
	}
}

func TestUpdate_TlsExpiredWritesNothing(t *testing.T) {
	now := time.Now()
	certificatePEM, privateKeyPEM := newTlsTestPair(t, now.Add(-48*time.Hour), now.Add(-24*time.Hour))
	scope := "app"
	mockStore := NewMockUpdateSecureStore(map[string][]byte{scope: []byte(updateTlsTestConf(certificatePEM, privateKeyPEM))})
	var stdout, stderr bytes.Buffer
	ui := UI{Out: &stdout, Err: &stderr}

	err := updateValues(ui, mockStore, scope, "", "tls")
	if !errors.Is(err, ErrTLSExpired) {
		t.Fatalf("expected error to wrap ErrTLSExpired, got %v", err)
	}

	if len(mockStore.saveHistory) != 0 {
		t.Fatalf("expected 0 saves, got %d", len(mockStore.saveHistory))
	}
}

func TestUpdate_MissingLabel(t *testing.T) {
	mockStore := NewMockUpdateSecureStore(nil)
	var stdout, stderr bytes.Buffer
	ui := UI{Out: &stdout, Err: &stderr}

	err := handleUpdateCommand(mockStore, []string{}, ui)
	if !errors.Is(err, ErrMissingArgument) {
		t.Fatalf("expected error to wrap ErrMissingArgument, got %v", err)
	}
}

func TestUpdate_TlsFieldPathUnknown(t *testing.T) {
	now := time.Now()
	certificatePEM, privateKeyPEM := newTlsTestPair(t, now.Add(-time.Hour), now.Add(24*time.Hour))
	scope := "app"
	mockStore := NewMockUpdateSecureStore(map[string][]byte{scope: []byte(updateTlsTestConf(certificatePEM, privateKeyPEM))})
	var stdout, stderr bytes.Buffer
	ui := UI{Out: &stdout, Err: &stderr}

	err := updateValues(ui, mockStore, scope, "", "server.tls.certificate")
	if !errors.Is(err, ErrUnknownUpdateLabel) {
		t.Fatalf("expected error to wrap ErrUnknownUpdateLabel, got %v", err)
	}

	if len(mockStore.saveHistory) != 0 {
		t.Fatalf("expected 0 saves, got %d", len(mockStore.saveHistory))
	}
}
