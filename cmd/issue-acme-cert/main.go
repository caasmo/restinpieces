package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/caasmo/restinpieces/config" // Adjust import path if needed
	"github.com/caasmo/restinpieces/queue/handlers" // Adjust import path if needed
	"github.com/caasmo/restinpieces/queue"    // Adjust import path if needed
)

func main() {
	// Basic Logger Setup
	logLevel := slog.LevelInfo // Default
	if os.Getenv("LOG_LEVEL") == "debug" {
		logLevel = slog.LevelDebug
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel}))
	slog.SetDefault(logger) // Set globally for libraries that might use slog's default

	logger.Info("Starting local TLS Cert Renewal test runner...")

	// --- Configuration ---
	// Load config from TOML file
	configPath := os.Getenv("CONFIG_FILE")
	if configPath == "" {
		configPath = "config.toml"
	}

	cfg, err := config.LoadFromToml(configPath, logger)
	if err != nil {
		logger.Error("Failed to load config file", "path", configPath, "error", err)
		os.Exit(1)
	}

	logger.Info("Config loaded from file",
		"path", configPath,
		"ACME Enabled", cfg.Acme.Enabled,
		"ACME Email", cfg.Acme.Email,
		"ACME Domains", cfg.Acme.Domains,
		"ACME Provider", cfg.Acme.DNSProvider,
		"ACME CA URL", cfg.Acme.CADirectoryURL,
		"Cert Path", cfg.Server.CertFile,
		"Key Path", cfg.Server.KeyFile,
		"Cloudflare Token Set", cfg.Acme.CloudflareApiToken != "", // Check if token is present
		"ACME Key Set", cfg.Acme.AcmePrivateKey != "",           // Check if key is present
	)

	// --- Handler Instantiation ---
	cfgProvider := config.NewProvider(cfg)
	renewalHandler := handlers.NewTLSCertRenewalHandler(cfgProvider, logger)

	// --- Job Execution ---
	// Create a context (e.g., with a timeout)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute) // Generous timeout for ACME+DNS
	defer cancel()

	// Create a dummy job (payload is not used by your current handler)
	dummyJob := queue.Job{ID: "local-test-run-1"}

	logger.Info("Executing Handle method...")
	err = renewalHandler.Handle(ctx, dummyJob)

	// --- Result ---
	if err != nil {
		logger.Error("Handler execution failed", "error", err)
		os.Exit(1) // Indicate failure
	}

	logger.Info("Handler execution completed successfully.")

	// --- Verification Hint ---
	logger.Info("Check for certificate and key files:",
		"cert_file", cfg.Server.CertFile,
		"key_file", cfg.Server.KeyFile)
	logger.Info("You can inspect the cert with:", "command", "openssl x509 -in "+cfg.Server.CertFile+" -text -noout")
}
