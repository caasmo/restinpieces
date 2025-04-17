package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"filippo.io/age"
	"github.com/pelletier/go-toml/v2"

	"github.com/caasmo/restinpieces/config"
	"github.com/caasmo/restinpieces/crypto"
	"github.com/caasmo/restinpieces/migrations"
	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"
)

type AppCreator struct {
	logger *slog.Logger
	conn   *sqlite.Conn
}

func NewAppCreator() *AppCreator {
	return &AppCreator{
		logger: slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		})),
	}
}

func (ac *AppCreator) CreateDatabase(dbPath string) error {
	if _, err := os.Stat(dbPath); err == nil {
		ac.logger.Error("database file already exists", "file", dbPath)
		return os.ErrExist
	}

	conn, err := sqlite.OpenConn(dbPath, sqlite.OpenReadWrite|sqlite.OpenCreate)
	if err != nil {
		ac.logger.Error("failed to open database", "error", err)
		return err
	}
	ac.conn = conn
	return nil
}

func (ac *AppCreator) RunMigrations() error {
	schemaFS := migrations.Schema()
	migrations, err := fs.ReadDir(schemaFS, ".")
	if err != nil {
		ac.logger.Error("failed to read embedded migrations", "error", err)
		return err
	}

	for _, migration := range migrations {
		if filepath.Ext(migration.Name()) != ".sql" {
			continue
		}

		sql, err := fs.ReadFile(schemaFS, migration.Name())
		if err != nil {
			ac.logger.Error("failed to read embedded migration",
				"file", migration.Name(),
				"error", err)
			return err
		}

		ac.logger.Info("applying migration", "file", migration.Name())
		if err := sqlitex.ExecuteScript(ac.conn, string(sql), &sqlitex.ExecOptions{
			Args: nil,
		}); err != nil {
			ac.logger.Error("failed to execute migration",
				"file", migration.Name(),
				"error", err)
			return err
		}
	}
	return nil
}

func (ac *AppCreator) generateDefaultConfig() (*config.Config, error) {
	cfg := &config.Config{
		DBPath:    "app.db", // Default, but will be overridden by flag in main app
		PublicDir: "public",
		Jwt: config.Jwt{
			AuthSecret:                     crypto.RandomString(32, crypto.AlphanumericAlphabet),
			AuthTokenDuration:              config.Duration{Duration: 24 * time.Hour},
			VerificationEmailSecret:        crypto.RandomString(32, crypto.AlphanumericAlphabet),
			VerificationEmailTokenDuration: config.Duration{Duration: 1 * time.Hour},
			PasswordResetSecret:            crypto.RandomString(32, crypto.AlphanumericAlphabet),
			PasswordResetTokenDuration:     config.Duration{Duration: 1 * time.Hour},
			EmailChangeSecret:              crypto.RandomString(32, crypto.AlphanumericAlphabet),
			EmailChangeTokenDuration:       config.Duration{Duration: 1 * time.Hour},
		},
		Scheduler: config.Scheduler{
			Interval:              config.Duration{Duration: 10 * time.Second},
			MaxJobsPerTick:        10,
			ConcurrencyMultiplier: 4,
		},
		Server: config.Server{
			Addr:                    ":8080",
			ShutdownGracefulTimeout: config.Duration{Duration: 30 * time.Second},
			ReadTimeout:             config.Duration{Duration: 10 * time.Second},
			ReadHeaderTimeout:       config.Duration{Duration: 5 * time.Second},
			WriteTimeout:            config.Duration{Duration: 10 * time.Second},
			IdleTimeout:             config.Duration{Duration: 120 * time.Second},
			ClientIpProxyHeader:     "", // Default: trust direct connection IP
			EnableTLS:               false,
			RedirectAddr:            "", // No redirect by default
		},
		RateLimits: config.RateLimits{
			PasswordResetCooldown:      config.Duration{Duration: 5 * time.Minute},
			EmailVerificationCooldown: config.Duration{Duration: 5 * time.Minute},
			EmailChangeCooldown:       config.Duration{Duration: 5 * time.Minute},
		},
		OAuth2Providers: map[string]config.OAuth2Provider{
			// Add default provider structures if needed, secrets will be loaded later
		},
		Smtp: config.Smtp{
			Enabled:     false, // Disabled by default
			Host:        "smtp.example.com",
			Port:        587,
			FromName:    "My App",
			FromAddress: "noreply@example.com",
			LocalName:   "localhost",
			AuthMethod:  "plain",
			UseStartTLS: true,
		},
		Endpoints: config.Endpoints{
			RefreshAuth:              "POST /api/auth/refresh",
			RequestEmailVerification: "POST /api/auth/request-verification",
			ConfirmEmailVerification: "GET /api/auth/confirm-verification",
			ListEndpoints:            "GET /api/endpoints",
			AuthWithPassword:         "POST /api/auth/password",
			AuthWithOAuth2:           "GET /api/auth/oauth2/:provider", // Note: :provider is handled by router
			RegisterWithPassword:     "POST /api/auth/register",
			ListOAuth2Providers:      "GET /api/auth/oauth2/providers",
			RequestPasswordReset:     "POST /api/auth/request-reset",
			ConfirmPasswordReset:     "POST /api/auth/confirm-reset",
			RequestEmailChange:       "POST /api/auth/request-email-change",
			ConfirmEmailChange:       "POST /api/auth/confirm-email-change",
		},
		Acme: config.Acme{
			Enabled:                 false, // Disabled by default
			Email:                   "admin@example.com",
			Domains:                 []string{"example.com"},
			DNSProvider:             "cloudflare",
			RenewalDaysBeforeExpiry: 30,
			CADirectoryURL:          "https://acme-v02.api.letsencrypt.org/directory", // Production LE
			// CADirectoryURL: "https://acme-staging-v02.api.letsencrypt.org/directory", // Staging LE
		},
		BlockIp: config.BlockIp{
			Enabled: false, // Disabled by default
		},
		Maintenance: config.Maintenance{
			Enabled:   false,
			Activated: false,
		},
	}
	return cfg, nil
}

func (ac *AppCreator) encryptData(data []byte, agePublicKeyPath string) ([]byte, error) {
	keyContent, err := os.ReadFile(agePublicKeyPath)
	if err != nil {
		ac.logger.Error("failed to read age public key file", "path", agePublicKeyPath, "error", err)
		return nil, fmt.Errorf("failed to read age public key file '%s': %w", agePublicKeyPath, err)
	}

	recipients, err := age.ParseRecipients(bytes.NewReader(keyContent))
	if err != nil {
		ac.logger.Error("failed to parse age recipients (public key)", "path", agePublicKeyPath, "error", err)
		return nil, fmt.Errorf("failed to parse age recipients from '%s': %w", agePublicKeyPath, err)
	}
	if len(recipients) == 0 {
		return nil, fmt.Errorf("no age recipients found in file '%s'", agePublicKeyPath)
	}

	encryptedOutput := &bytes.Buffer{}
	encryptWriter, err := age.Encrypt(encryptedOutput, recipients...)
	if err != nil {
		ac.logger.Error("failed to create age encryption writer", "error", err)
		return nil, fmt.Errorf("failed to create age encryption writer: %w", err)
	}
	if _, err := io.Copy(encryptWriter, bytes.NewReader(data)); err != nil {
		ac.logger.Error("failed to write data to age encryption writer", "error", err)
		return nil, fmt.Errorf("failed to write data to age encryption writer: %w", err)
	}
	if err := encryptWriter.Close(); err != nil {
		ac.logger.Error("failed to close age encryption writer", "error", err)
		return nil, fmt.Errorf("failed to close age encryption writer: %w", err)
	}
	return encryptedOutput.Bytes(), nil
}

func (ac *AppCreator) InsertConfig(encryptedConfig []byte) error {
	ac.logger.Info("inserting initial encrypted configuration")
	err := sqlitex.Execute(ac.conn,
		`INSERT INTO app_config (content, format, description, created_at)
		VALUES (?, ?, ?, ?)`,
		&sqlitex.ExecOptions{
			Args: []interface{}{
				encryptedConfig,
				"toml",
				"Initial default configuration",
				time.Now().UTC().Format(time.RFC3339),
			},
		})
	if err != nil {
		ac.logger.Error("failed to insert initial config", "error", err)
		return fmt.Errorf("failed to insert initial config: %w", err)
	}
	return nil
}

func main() {
	dbPathFlag := flag.String("db", "", "Path to the SQLite database file to create (required)")
	ageKeyPathFlag := flag.String("age-key", "", "Path to the age public key file (recipient) for encryption (required)")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s -db <database-path> -age-key <public-key-path>\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Creates a new SQLite database with an initial, encrypted configuration.\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	if *dbPathFlag == "" || *ageKeyPathFlag == "" {
		flag.Usage()
		os.Exit(1)
	}

	creator := NewAppCreator()
	defer func() {
		if creator.conn != nil {
			creator.conn.Close()
		}
	}()

	// 1. Create Database File
	creator.logger.Info("creating sqlite file", "path", *dbPathFlag)
	if err := creator.CreateDatabase(*dbPathFlag); err != nil {
		os.Exit(1) // Error logged in CreateDatabase
	}

	// 2. Run Migrations (Apply Schema)
	if err := creator.RunMigrations(); err != nil {
		os.Exit(1) // Error logged in RunMigrations
	}

	// 3. Generate Default Config Struct
	defaultCfg, err := creator.generateDefaultConfig()
	if err != nil {
		creator.logger.Error("failed to generate default config struct", "error", err)
		os.Exit(1)
	}

	// 4. Marshal Config to TOML
	tomlBytes, err := toml.Marshal(defaultCfg)
	if err != nil {
		creator.logger.Error("failed to marshal default config to TOML", "error", err)
		os.Exit(1)
	}

	// 5. Encrypt TOML Data
	encryptedConfig, err := creator.encryptData(tomlBytes, *ageKeyPathFlag)
	if err != nil {
		// Error logged in encryptData
		os.Exit(1)
	}

	// 6. Insert Encrypted Config into DB
	if err := creator.InsertConfig(encryptedConfig); err != nil {
		// Error logged in InsertConfig
		os.Exit(1)
	}

	creator.logger.Info("application database created successfully", "db_file", *dbPathFlag)
}
