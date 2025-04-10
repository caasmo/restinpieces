package main

import (
	"context"
	"log/slog"
	"os"
	"flag"
	"time"

	"github.com/caasmo/restinpieces/config" // Adjust import path if needed
	"github.com/caasmo/restinpieces/db"     // Added for DB interface
	"github.com/caasmo/restinpieces/db/zombiezen" // Changed to zombiezen implementation
	"github.com/caasmo/restinpieces/queue"        // Adjust import path if needed
	"github.com/caasmo/restinpieces/queue/handlers" // Adjust import path if needed
	"zombiezen.com/go/sqlite" // Added for zombiezen connection
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
	var forceIssue bool
	flag.StringVar(&configPath, "config", "config.toml", "path to config TOML file")
	flag.StringVar(&dbPath, "dbfile", "restinpieces.db", "path to SQLite database file")
	flag.BoolVar(&forceIssue, "force", false, "force certificate issuance even if valid cert exists")
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
	logger.Info("Connecting to database...", "path", dbPath)
	// Use OpenConn similar to create-app, no pool needed for this command
	conn, err := sqlite.OpenConn(dbPath, sqlite.OpenReadWrite) // Open existing DB ReadWrite
	if err != nil {
		logger.Error("Failed to open database connection", "path", dbPath, "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := conn.Close(); err != nil {
			logger.Error("Failed to close database connection", "error", err)
		} else {
			logger.Info("Database connection closed.")
		}
	}()
	// Create Db instance satisfying interfaces using the zombiezen implementation
	dbConn := zombiezen.NewDb(conn)

	// --- Handler Instantiation ---
	cfgProvider := config.NewProvider(cfg)
	// Pass the database connection to the handler
	renewalHandler := handlers.NewTLSCertRenewalHandler(cfgProvider, dbConn, logger)

	// --- Force Issuance Logic ---
	if forceIssue {
		certPath := cfg.Server.CertFile
		logger.Info("Force flag is set. Checking for existing certificate file to remove.", "path", certPath)
		if _, err := os.Stat(certPath); err == nil {
			logger.Warn("Removing existing certificate file due to --force flag.", "path", certPath)
			if err := os.Remove(certPath); err != nil {
				logger.Error("Failed to remove existing certificate file. Proceeding anyway.", "path", certPath, "error", err)
				// Decide if this should be a fatal error? For now, we proceed.
			}
		} else if !os.IsNotExist(err) {
			// Error stating the file other than not existing
			logger.Error("Error checking existing certificate file status.", "path", certPath, "error", err)
			// Decide if this should be fatal? For now, we proceed.
		} else {
			logger.Info("Certificate file does not exist, no removal needed.", "path", certPath)
		}
		// Also remove key file if it exists? Let's assume cert removal is enough for the handler's check.
	}

	// --- Job Execution ---
	// Create a context (e.g., with a timeout)
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
