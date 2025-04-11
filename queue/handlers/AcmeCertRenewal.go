package handlers

import (
	"context"
	"crypto" // Add standard crypto import
	"crypto/x509"
	"encoding/json" // Added for domains JSON
	"encoding/pem"
	"fmt"
	"log/slog"
	// "os" // No longer needed as we check CertData directly
	"time"

	"github.com/caasmo/restinpieces/config"
	"github.com/caasmo/restinpieces/db" // Added for db types
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
	dbAcme         db.DbAcme        // Database access for ACME certs
	logger         *slog.Logger
}

// NewTLSCertRenewalHandler creates a new handler instance.
func NewTLSCertRenewalHandler(provider *config.Provider, dbAcme db.DbAcme, logger *slog.Logger) *TLSCertRenewalHandler {
	return &TLSCertRenewalHandler{
		configProvider: provider,
		dbAcme:         dbAcme,                                         // Store dbAcme
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

	// --- Check Expiry ---
	// Check expiry using CertData directly from config
	needsRenewal, err := h.certificateNeedsRenewal(cfg.Server.CertData, cfg.Acme.RenewalDaysBeforeExpiry)
	if err != nil {
		// This indicates a parsing error or other logic error within certificateNeedsRenewal
		h.logger.Error("Failed to check certificate expiry using CertData", "error", err)
		return err
	}

	if !needsRenewal {
		h.logger.Info("Certificate renewal not required.")
		return nil // Nothing to do
	}

	// --- Configure Lego ---
	h.logger.Info("Starting ACME certificate renewal process", "domains", cfg.Acme.Domains)

	// --- Load and Parse ACME Account Private Key ---
	acmePrivateKeyPEM := cfg.Acme.AcmePrivateKey
	if acmePrivateKeyPEM == "" {
		// Use the renamed environment variable constant
		err := fmt.Errorf("ACME account private key is missing. Set %s environment variable", config.EnvAcmeLetsencryptPrivateKey)
		h.logger.Error(err.Error())
		// Use the renamed environment variable constant
		err = fmt.Errorf("ACME account private key is missing. Set %s environment variable", config.EnvAcmeLetsencryptPrivateKey)
		h.logger.Error(err.Error())
		return err // Configuration error
	}

	// Parse the PEM encoded key. We enforce ECDSA P-256.
	acmePrivateKey, err := certcrypto.ParsePEMPrivateKey([]byte(acmePrivateKeyPEM))
	if err != nil {
		h.logger.Error("Failed to parse ACME account private key. Ensure it is a valid ECDSA P-256 key in PEM format.", "env_var", config.EnvAcmeLetsencryptPrivateKey, "error", err)
		return fmt.Errorf("failed to parse ACME account private key (expecting ECDSA P-256 PEM): %w", err)
	}
	// We don't need to check the type here anymore, but Lego might internally.
	// The KeyType in the config below tells Lego what *certificate* key type to request.

	acmeUser := AcmeUser{
		Email:      cfg.Acme.Email,
		PrivateKey: acmePrivateKey,
		// Registration will be filled by lego if needed
	}

	legoConfig := lego.NewConfig(&acmeUser)
	legoConfig.CADirURL = cfg.Acme.CADirectoryURL     // Use configured directory (staging/prod)
	legoConfig.Certificate.KeyType = certcrypto.EC256 // Enforce ECDSA P-256 for the *certificate* key type as well

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

	// --- Save Certificate to Database ---
	if err := h.saveCertificateResource(resource, h.logger); err != nil {
		// Error already logged by saveCertificateResource
		return err // Propagate error
	}

	// Note: We no longer save to files directly here, assuming DB is the source of truth
	// or another mechanism handles file writing from DB if needed.
	h.logger.Info("Successfully processed ACME certificate renewal.", "domains", request.Domains)
	return nil
}

// saveCertificateResource saves the obtained certificate resource to the database.
func (h *TLSCertRenewalHandler) saveCertificateResource(resource *certificate.Resource, logger *slog.Logger) error {
	// 1. Parse the certificate to get expiry and issue dates
	block, _ := pem.Decode(resource.Certificate)
	if block == nil {
		err := fmt.Errorf("failed to decode PEM block from obtained certificate")
		logger.Error(err.Error(), "domain", resource.Domain)
		return err
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		err = fmt.Errorf("failed to parse obtained certificate: %w", err)
		logger.Error(err.Error(), "domain", resource.Domain)
		return err
	}

	// 2. Prepare domains list as JSON string
	// Use the domains from the original request stored in the handler's config snapshot
	cfg := h.configProvider.Get() // Get current config
	domainsJSON, err := json.Marshal(cfg.Acme.Domains)
	if err != nil {
		err = fmt.Errorf("failed to marshal domains to JSON: %w", err)
		logger.Error(err.Error(), "domains", cfg.Acme.Domains)
		return err
	}

	// 3. Create the db.AcmeCert struct
	// Assuming db.AcmeCert struct fields match the 'acme_certificates' table schema.
	// Using resource.Domain as the identifier. Consider making this configurable if needed.
	dbCert := db.AcmeCert{
		Identifier:       resource.Domain, // Use primary domain as identifier
		Domains:          string(domainsJSON),
		CertificateChain: string(resource.Certificate), // Assumes resource.Certificate is PEM encoded chain
		PrivateKey:       string(resource.PrivateKey),  // Assumes resource.PrivateKey is PEM encoded key
		IssuedAt:         cert.NotBefore.UTC(),         // Assign time.Time directly
		ExpiresAt:        cert.NotAfter.UTC(),          // Assign time.Time directly
		// CreatedAt/UpdatedAt are typically handled by the database or Save method.
		// LastRenewalAttemptAt is zero time initially, could be updated elsewhere.
	}

	// 4. Call Save method via the dbAcme interface
	err = h.dbAcme.Save(dbCert) // Assuming Save takes db.AcmeCert by value
	if err != nil {
		logger.Error("Failed to save ACME certificate to database", "identifier", dbCert.Identifier, "error", err)
		return fmt.Errorf("failed to save ACME certificate for %s: %w", dbCert.Identifier, err)
	}

	logger.Info("Successfully saved renewed certificate to database", "identifier", dbCert.Identifier, "expiry", dbCert.ExpiresAt)
	return nil
}

// certificateNeedsRenewal checks if the certificate PEM data needs renewal.
// It returns true if the certificate data is empty, fails to parse, or expires within the threshold.
// It returns an error only for parsing errors.
func (h *TLSCertRenewalHandler) certificateNeedsRenewal(certPEMData string, renewalDaysThreshold int) (bool, error) {
	if certPEMData == "" {
		h.logger.Info("Certificate data is empty in config, renewal required.")
		return true, nil // Needs renewal
	}

	block, _ := pem.Decode([]byte(certPEMData))
	if block == nil {
		h.logger.Warn("Failed to decode PEM block from certificate data in config, assuming renewal needed.")
		// Return an error here? Or just assume renewal? Let's assume renewal for now.
		// Consider returning an error if strict validation is needed:
		// return false, fmt.Errorf("failed to decode PEM block from certificate data")
		return true, nil // Treat as needing renewal
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		h.logger.Warn("Failed to parse certificate from config data, assuming renewal needed.", "error", err)
		// Return an error here? Or just assume renewal? Let's assume renewal for now.
		// Consider returning an error if strict validation is needed:
		// return false, fmt.Errorf("failed to parse certificate from config data: %w", err)
		return true, nil // Treat as needing renewal
	}

	daysLeft := time.Until(cert.NotAfter).Hours() / 24
	h.logger.Info("Checking certificate expiry from config data",
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

// Removed parseAcmePrivateKeyAndGetType function as we now enforce ECDSA P-256
