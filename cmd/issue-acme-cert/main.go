package main

import (
	"context"
	"log/slog"
	"os"
	"flag"
	"time"

	"github.com/caasmo/restinpieces/config" // Adjust import path if needed
	"github.com/caasmo/restinpieces/db/zombiezen" // Changed to zombiezen implementation
	"github.com/caasmo/restinpieces/queue"        // Adjust import path if needed
	"github.com/caasmo/restinpieces/queue/handlers" // Adjust import path if needed
	"zombiezen.com/go/sqlite"                       // Keep for OpenFlags
	"zombiezen.com/go/sqlite/sqlitex"               // Added for pool
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

	// --- Flags ---
	var configPath string
	var dbPath string
	// var forceIssue bool // Removed force flag
	flag.StringVar(&configPath, "config", "config.toml", "path to config TOML file")
	flag.StringVar(&dbPath, "dbfile", "app.db", "path to SQLite database file")
	// flag.BoolVar(&forceIssue, "force", false, "force certificate issuance even if valid cert exists") // Removed force flag
	flag.Parse()

	// --- Configuration Loading ---
	logger.Info("Loading configuration...", "path", configPath)
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
		"ACME Key Set", cfg.Acme.AcmePrivateKey != "", // Check if key is present
	)

	// --- Database Connection ---
	logger.Info("Connecting to database pool...", "path", dbPath)
	// Use a pool, similar to the main application, for consistency
	pool, err := sqlitex.NewPool(dbPath, sqlitex.PoolOptions{
		Flags: sqlite.OpenReadWrite, // Ensure DB exists, open read-write
		// PoolSize can be small for this command, 1 is likely sufficient
		PoolSize: 1,
	})
	if err != nil {
		logger.Error("Failed to open database pool", "path", dbPath, "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := pool.Close(); err != nil {
			logger.Error("Failed to close database pool", "error", err)
		} else {
			logger.Info("Database pool closed.")
		}
	}()
	// Create Db instance satisfying interfaces using the zombiezen implementation with the pool
	dbConn, err := zombiezen.New(pool) // Use the New constructor which takes a pool
	if err != nil {
		// This error check was missing in the previous pool refactor, adding it now.
		logger.Error("Failed to create zombiezen DB instance from pool", "error", err)
		// This error check was missing in the previous pool refactor, adding it now.
		logger.Error("Failed to create zombiezen DB instance from pool", "error", err)
		os.Exit(1)
	}

	// --- Load Existing Cert from DB into Config ---
	logger.Info("Attempting to load existing certificate from database...")
	existingCert, err := dbConn.Get()
	if err != nil {
		// Log the error, but proceed if it's just "not found"
		// Assuming db.Get() returns nil, nil or a specific error for not found
		// Adjust this check if db.Get() has different error semantics for "not found"
		if err.Error() == "acme: no certificate found" { // Example check, adjust as needed
			logger.Info("No existing certificate found in the database. Will attempt issuance if needed.")
		} else {
			logger.Warn("Failed to get existing certificate from database. Proceeding, may force issuance.", "error", err)
			// Proceed, CertData/KeyData will be empty in cfg
		}
	} else if existingCert != nil {
		logger.Info("Existing certificate loaded from database.", "identifier", existingCert.Identifier, "expires", existingCert.ExpiresAt)
		cfg.Server.CertData = existingCert.CertificateChain
		cfg.Server.KeyData = existingCert.PrivateKey
		// Log snippet for verification?
		// logger.Debug("Loaded CertData (snippet)", "data", cfg.Server.CertData[:min(100, len(cfg.Server.CertData))]+"...")
	} else {
		// This case (nil, nil) might indicate "not found" depending on Get() implementation
		logger.Info("No existing certificate found in the database (Get returned nil, nil). Will attempt issuance if needed.")
	}


	// --- Handler Instantiation ---
	// Create provider *after* potentially loading cert data into cfg
	cfgProvider := config.NewProvider(cfg)
	// Pass the database connection to the handler
	renewalHandler := handlers.NewTLSCertRenewalHandler(cfgProvider, dbConn, logger)

	// --- Job Execution ---
	// Create a context (e.g., with a timeout)
	// Force issuance logic removed as handler now checks CertData from config provider.
	// This command loads config from file, where CertData is typically empty,
	// thus triggering the handler's issuance logic if ACME is enabled.
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute) // Generous timeout for ACME+DNS
	defer cancel()

	// Create a dummy job (payload is not used by your current handler)
	dummyJob := queue.Job{ID: 1}

	logger.Info("Executing Handle method...")
	err = renewalHandler.Handle(ctx, dummyJob)

	// --- Result ---
	if err != nil {
		logger.Error("Handler execution failed", "error", err)
		os.Exit(1) // Indicate failure
	}

	logger.Info("Handler execution completed successfully.")

	// --- Verification Hint ---
	logger.Info("Certificate should now be saved in the database.", "db_file", dbPath)
	logger.Info("If Server.CertFile/KeyFile are configured, the application *might* also write them there upon loading from DB, depending on its startup logic.")
	logger.Info("You can check the database content using sqlite tools or potentially a dump-config command if available.")
	// Keep the openssl command hint as it's still useful if the file *is* written.
	if cfg.Server.CertFile != "" {
		logger.Info("If file was written, inspect it with:", "command", "openssl x509 -in "+cfg.Server.CertFile+" -text -noout")
	}
}
