package handlers

import (
	"context"
	"crypto" // Add standard crypto import
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/caasmo/restinpieces/config"
	"github.com/caasmo/restinpieces/queue"
	"github.com/go-acme/lego/v4/certcrypto"
	"github.com/go-acme/lego/v4/certificate"
	"github.com/go-acme/lego/v4/challenge/dns01"
	"github.com/go-acme/lego/v4/lego"
	"github.com/go-acme/lego/v4/providers/dns/cloudflare"
	"github.com/go-acme/lego/v4/registration"
)

// TLSCertRenewalHandler handles the job for renewing TLS certificates via ACME.
type TLSCertRenewalHandler struct {
	configProvider *config.Provider // Access to config
	logger         *slog.Logger
}

// NewTLSCertRenewalHandler creates a new handler instance.
func NewTLSCertRenewalHandler(provider *config.Provider, logger *slog.Logger) *TLSCertRenewalHandler {
	return &TLSCertRenewalHandler{
		configProvider: provider,
		logger:         logger.With("job_handler", "tls_cert_renewal"), // Add context to logger
	}
}

// AcmeUser implements lego's registration.User interface
type AcmeUser struct {
	Email        string
	Registration *registration.Resource
	PrivateKey   crypto.PrivateKey // Use standard crypto.PrivateKey interface type
}

func (u *AcmeUser) GetEmail() string {
	return u.Email
}
func (u *AcmeUser) GetRegistration() *registration.Resource {
	return u.Registration
}
func (u *AcmeUser) GetPrivateKey() crypto.PrivateKey { // Return type matches interface
	return u.PrivateKey
}

// Handle executes the certificate renewal logic.
func (h *TLSCertRenewalHandler) Handle(ctx context.Context, job queue.Job) error {
	// Get current config snapshot directly from the provider
	cfg := h.configProvider.Get()

	if !cfg.Acme.Enabled {
		h.logger.Info("ACME certificate renewal is disabled in config, skipping job.")
		return nil // Not an error, just disabled
	}

	if cfg.Acme.DNSProvider != "cloudflare" {
		err := fmt.Errorf("unsupported DNS provider configured: %s. Only 'cloudflare' is supported", cfg.Acme.DNSProvider)
		h.logger.Error(err.Error())
		return err // Configuration error
	}

	if cfg.Acme.CloudflareApiToken == "" {
		err := fmt.Errorf("cloudflare API token is missing. Set %s environment variable", config.EnvAcmeCloudflareApiToken)
		h.logger.Error(err.Error())
		return err // Configuration error
	}

	if len(cfg.Acme.Domains) == 0 {
		err := fmt.Errorf("no domains configured for ACME renewal")
		h.logger.Error(err.Error())
		return err // Configuration error
	}

	if cfg.Server.CertFile == "" || cfg.Server.KeyFile == "" {
		err := fmt.Errorf("server CertFile or KeyFile path not configured")
		h.logger.Error(err.Error())
		return err // Configuration error
	}

	// --- Check Expiry ---
	certPath := cfg.Server.CertFile
	keyPath := cfg.Server.KeyFile // Keep keyPath definition here for later use

	needsRenewal, err := h.certificateNeedsRenewal(certPath, cfg.Acme.RenewalDaysBeforeExpiry)
	if err != nil {
		// This indicates a file read error other than NotExist
		h.logger.Error("Failed to check certificate expiry", "path", certPath, "error", err)
		return err
	}

	if !needsRenewal {
		h.logger.Info("Certificate renewal not required.")
		return nil // Nothing to do
	}

	// --- Configure Lego ---
	h.logger.Info("Starting ACME certificate renewal process", "domains", cfg.Acme.Domains)

	// --- Load and Parse ACME Private Key ---
	acmePrivateKeyPEM := cfg.Acme.AcmePrivateKey
	if acmePrivateKeyPEM == "" {
		err := fmt.Errorf("ACME private key is missing. Set %s environment variable", config.EnvAcmePrivateKey)
		h.logger.Error(err.Error())
		return err // Configuration error
	}

	acmePrivateKey, keyType, err := parseAcmePrivateKeyAndGetType(acmePrivateKeyPEM, h.logger)
	if err != nil {
		// Error already logged by parseAcmePrivateKeyAndGetType
		return err // Return the wrapped error
	}
	h.logger.Debug("Successfully parsed ACME private key and determined type", "key_type", keyType)

	acmeUser := AcmeUser{
		Email:      cfg.Acme.Email,
		PrivateKey: acmePrivateKey,
		// Registration will be filled by lego if needed
	}

	legoConfig := lego.NewConfig(&acmeUser)
	legoConfig.CADirURL = cfg.Acme.CADirectoryURL // Use configured directory (staging/prod)
	legoConfig.Certificate.KeyType = keyType      // Use the type determined from the loaded key

	legoClient, err := lego.NewClient(legoConfig)
	if err != nil {
		h.logger.Error("Failed to create ACME client", "error", err)
		return fmt.Errorf("failed to create ACME client: %w", err)
	}

	// Configure Cloudflare DNS Provider
	cfConfig := cloudflare.NewDefaultConfig()
	cfConfig.AuthToken = cfg.Acme.CloudflareApiToken
	// Add other CF config if needed (AuthEmail, AuthKey, ZoneToken etc.)

	cfProvider, err := cloudflare.NewDNSProviderConfig(cfConfig)
	if err != nil {
		h.logger.Error("Failed to create Cloudflare DNS provider", "error", err)
		return fmt.Errorf("failed to create Cloudflare provider: %w", err)
	}

	// Configure the lego client to use the Cloudflare provider for DNS-01 challenges.
	// This call is NON-BLOCKING and performs NO network communication.
	// It simply registers the provider internally within the lego client instance.
	// The provider's methods (`Present` and `CleanUp`) will be called later during the `Obtain` process.
	// The timeout option configures how long the provider should wait for DNS propagation during `Present`.
	err = legoClient.Challenge.SetDNS01Provider(cfProvider, dns01.AddDNSTimeout(10*time.Minute)) // Increase timeout
	if err != nil {
		h.logger.Error("Failed to set DNS01 provider", "error", err)
		return fmt.Errorf("failed to set DNS01 provider: %w", err)
	}

	// --- Obtain Certificate ---
	// Register the ACME account with the CA (e.g., Let's Encrypt).
	// This call IS BLOCKING and involves network communication.
	// Steps:
	// 1. Sends a signed request to the ACME server's newAccount endpoint using the user's private key.
	// 2. Includes the user's email and agreement to Terms of Service.
	// 3. ACME server creates/retrieves the account linked to the key and responds with account details.
	// 4. Lego populates the user's Registration field with the response.
	// Note: Lego typically handles checking if an account already exists for the key.
	if acmeUser.Registration == nil {
		reg, err := legoClient.Registration.Register(registration.RegisterOptions{TermsOfServiceAgreed: true})
		if err != nil {
			h.logger.Error("ACME registration failed", "email", acmeUser.Email, "error", err)
			return fmt.Errorf("ACME registration failed: %w", err)
		}
		acmeUser.Registration = reg
		h.logger.Info("ACME account registered successfully", "email", acmeUser.Email)
		// Persist acmeUser.Registration and acmeUser.PrivateKey securely here if needed for reuse.
	}


	request := certificate.ObtainRequest{
		Domains: cfg.Acme.Domains,
		Bundle:  true, // Get the full chain
	}

	// Obtain the certificate from the ACME server.
	// This call IS BLOCKING and involves significant network communication and potential waiting.
	// Steps:
	// 1. Create Order: Request a new certificate order from the ACME server for the domains.
	// 2. Receive Challenges: Get challenge details (DNS-01 in this case) from the server.
	// 3. Trigger Challenge Setup: Call `Present` on the registered DNS provider (Cloudflare).
	//    - Provider uses API to create the TXT record.
	//    - Provider waits (blocks) for DNS propagation (respecting timeout).
	// 4. Notify ACME Server: Tell the server to verify the challenge.
	// 5. ACME Server Verification: Server performs DNS lookups to check the TXT record.
	// 6. Poll Order Status: Lego polls (blocks/waits) the ACME server until the order is ready or failed.
	// 7. Finalize Order: Generate a CSR and send it to the ACME server.
	// 8. Download Certificate: Download the issued certificate chain.
	// 9. Trigger Challenge Cleanup: Call `CleanUp` on the DNS provider to delete the TXT record.
	// 10. Return Result: Return the certificate and its corresponding private key.
	resource, err := legoClient.Certificate.Obtain(request)
	if err != nil {
		h.logger.Error("Failed to obtain ACME certificate", "domains", request.Domains, "error", err)
		return fmt.Errorf("failed to obtain ACME certificate: %w", err)
	}

	h.logger.Info("Successfully obtained ACME certificate", "domains", request.Domains, "certificate_url", resource.CertURL)

	// --- Save Files ---
	if err := saveCertificateResource(certPath, keyPath, resource, h.logger); err != nil {
		// Error already logged by saveCertificateResource
		return err
	}

	h.logger.Info("Successfully saved renewed certificate and key.", "cert_path", certPath, "key_path", keyPath)
	return nil
}

// saveCertificateResource saves the certificate and private key using atomic writes.
func saveCertificateResource(certFile, keyFile string, resource *certificate.Resource, logger *slog.Logger) error {
	// Create directories if they don't exist
	certDir := filepath.Dir(certFile)
	keyDir := filepath.Dir(keyFile)
	if err := os.MkdirAll(certDir, 0755); err != nil {
		logger.Error("Failed to create directory for certificate", "dir", certDir, "error", err)
		return fmt.Errorf("failed to create cert dir %s: %w", certDir, err)
	}
	// Check if keyDir is different before creating, avoid redundant MkdirAll
	if certDir != keyDir {
		if err := os.MkdirAll(keyDir, 0755); err != nil {
			logger.Error("Failed to create directory for key", "dir", keyDir, "error", err)
			return fmt.Errorf("failed to create key dir %s: %w", keyDir, err)
		}
	}


	// Write to temporary files first
	certTmpFile, err := os.CreateTemp(certDir, filepath.Base(certFile)+".tmp-*")
	if err != nil {
		logger.Error("Failed to create temporary certificate file", "dir", certDir, "error", err)
		return fmt.Errorf("failed to create temp cert file: %w", err)
	}
	// Ensure temp file is cleaned up on error *before* rename.
	// If Rename succeeds, this defer will run but os.Remove will fail harmlessly as the file no longer exists at the temp path.
	defer os.Remove(certTmpFile.Name())

	keyTmpFile, err := os.CreateTemp(keyDir, filepath.Base(keyFile)+".tmp-*")
	if err != nil {
		logger.Error("Failed to create temporary key file", "dir", keyDir, "error", err)
		return fmt.Errorf("failed to create temp key file: %w", err)
	}
	// Ensure temp file is cleaned up on error *before* rename.
	// If Rename succeeds, this defer will run but os.Remove will fail harmlessly as the file no longer exists at the temp path.
	defer os.Remove(keyTmpFile.Name())

	// Write content
	if _, err := certTmpFile.Write(resource.Certificate); err != nil {
		certTmpFile.Close() // Close even on write error
		logger.Error("Failed to write to temporary certificate file", "path", certTmpFile.Name(), "error", err)
		return fmt.Errorf("failed to write temp cert: %w", err)
	}
	if err := certTmpFile.Close(); err != nil {
		logger.Error("Failed to close temporary certificate file", "path", certTmpFile.Name(), "error", err)
		return fmt.Errorf("failed to close temp cert: %w", err)
	}

	if _, err := keyTmpFile.Write(resource.PrivateKey); err != nil {
		keyTmpFile.Close() // Close even on write error
		logger.Error("Failed to write to temporary key file", "path", keyTmpFile.Name(), "error", err)
		return fmt.Errorf("failed to write temp key: %w", err)
	}
	// Set strict permissions for the private key *before* closing and renaming
	if err := keyTmpFile.Chmod(0600); err != nil {
		keyTmpFile.Close()
		logger.Error("Failed to set permissions on temporary key file", "path", keyTmpFile.Name(), "error", err)
		return fmt.Errorf("failed to chmod temp key: %w", err)
	}
	if err := keyTmpFile.Close(); err != nil {
		logger.Error("Failed to close temporary key file", "path", keyTmpFile.Name(), "error", err)
		return fmt.Errorf("failed to close temp key: %w", err)
	}


	// Atomically replace old files with new ones
	if err := os.Rename(certTmpFile.Name(), certFile); err != nil {
		logger.Error("Failed to rename temporary certificate file", "from", certTmpFile.Name(), "to", certFile, "error", err)
		return fmt.Errorf("failed to rename cert file: %w", err)
	}
	// If cert rename succeeded, the defer os.Remove(certTmpFile.Name()) won't run on the original temp name

	if err := os.Rename(keyTmpFile.Name(), keyFile); err != nil {
		logger.Error("Failed to rename temporary key file", "from", keyTmpFile.Name(), "to", keyFile, "error", err)
		// Attempt to rollback cert rename? Or leave inconsistent state? Log clearly.
		logger.Error("CRITICAL: Key file rename failed after cert file rename succeeded. State might be inconsistent.", "cert_file", certFile, "key_file_failed_rename", keyFile)
		return fmt.Errorf("failed to rename key file: %w", err)
	}
	// If key rename succeeded, the defer os.Remove(keyTmpFile.Name()) won't run on the original temp name

	return nil
}


// certificateNeedsRenewal checks if the certificate at the given path needs renewal.
// It returns true if the certificate doesn't exist, fails to parse, or expires within the threshold.
// It returns an error only for file system read errors (excluding os.IsNotExist).
func (h *TLSCertRenewalHandler) certificateNeedsRenewal(certPath string, renewalDaysThreshold int) (bool, error) {
	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		if os.IsNotExist(err) {
			h.logger.Info("Certificate file not found, renewal required.", "path", certPath)
			return true, nil // Needs renewal, not a file system error for the caller
		}
		// Other read error (permissions, etc.)
		return false, fmt.Errorf("failed to read certificate file %s: %w", certPath, err)
	}

	block, _ := pem.Decode(certPEM)
	if block == nil {
		h.logger.Warn("Failed to decode PEM block from certificate file, assuming renewal needed.", "path", certPath)
		return true, nil // Treat as needing renewal
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		h.logger.Warn("Failed to parse certificate from file, assuming renewal needed.", "path", certPath, "error", err)
		return true, nil // Treat as needing renewal
	}

	daysLeft := time.Until(cert.NotAfter).Hours() / 24
	h.logger.Info("Checking certificate expiry",
		"path", certPath,
		"subject", cert.Subject.CommonName,
		"expiry", cert.NotAfter.Format(time.RFC3339),
		"days_left", int(daysLeft))

	if daysLeft < float64(renewalDaysThreshold) {
		h.logger.Info("Certificate is expiring soon, renewal required.", "days_left", int(daysLeft), "threshold_days", renewalDaysThreshold)
		return true, nil
	}

	// Certificate exists, is valid, and is not expiring soon.
	return false, nil
}

// parseAcmePrivateKeyAndGetType parses the PEM encoded private key string,
// determines its type for Lego, and returns the crypto.PrivateKey and certcrypto.KeyType.
func parseAcmePrivateKeyAndGetType(privateKeyPEM string, logger *slog.Logger) (crypto.PrivateKey, certcrypto.KeyType, error) {
	acmePrivateKey, err := certcrypto.ParsePEMPrivateKey([]byte(privateKeyPEM))
	if err != nil {
		logger.Error("Failed to parse ACME private key from environment variable", "env_var", config.EnvAcmePrivateKey, "error", err)
		return nil, "", fmt.Errorf("failed to parse ACME private key: %w", err)
	}

	var keyType certcrypto.KeyType
	switch pk := acmePrivateKey.(type) {
	case *rsa.PrivateKey:
		switch pk.N.BitLen() {
		case 2048:
			keyType = certcrypto.RSA2048
		case 3072:
			keyType = certcrypto.RSA3072
		case 4096:
			keyType = certcrypto.RSA4096
		default:
			logger.Warn("Using default RSA2048 key type for unknown RSA key size", "bits", pk.N.BitLen())
			keyType = certcrypto.RSA2048 // Default assumption
		}
	case *ecdsa.PrivateKey:
		switch pk.Curve.Params().Name {
		case "P-256":
			keyType = certcrypto.EC256
		case "P-384":
			keyType = certcrypto.EC384
		default:
			err := fmt.Errorf("unsupported ECDSA curve: %s", pk.Curve.Params().Name)
			logger.Error(err.Error())
			return nil, "", err
		}
	default:
		err := fmt.Errorf("unsupported ACME private key type: %T", acmePrivateKey)
		logger.Error(err.Error())
		return nil, "", err
	}

	return acmePrivateKey, keyType, nil
}
