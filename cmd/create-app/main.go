package main

import (
	"context"
	"flag"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/pelletier/go-toml/v2"

	"github.com/caasmo/restinpieces"
	"github.com/caasmo/restinpieces/config"
	"github.com/caasmo/restinpieces/crypto"
	zdb "github.com/caasmo/restinpieces/db/zombiezen"
	"github.com/caasmo/restinpieces/migrations"
	"zombiezen.com/go/sqlite/sqlitex"
)

type AppCreator struct {
	logger       *slog.Logger
	pool         *sqlitex.Pool
	secureConfig config.SecureConfig
}

func NewAppCreator() *AppCreator {
	return &AppCreator{
		logger: slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		})),
	}
}

// CreateDatabasePool initializes the database pool.
func (ac *AppCreator) CreateDatabasePool(dbPath string) error {
	if _, err := os.Stat(dbPath); err == nil {
		ac.logger.Error("database file already exists", "file", dbPath)
		return os.ErrExist
	}

	// Use the library helper to create the pool, ensuring consistency
	pool, err := restinpieces.NewZombiezenPool(dbPath)
	if err != nil {
		ac.logger.Error("failed to create database pool", "error", err)
		return err
	}
	ac.pool = pool
	return nil
}

func (ac *AppCreator) RunMigrations() error {
	conn, err := ac.pool.Take(context.Background())
	if err != nil {
		ac.logger.Error("failed to get connection from pool for migrations", "error", err)
		return err
	}
	defer ac.pool.Put(conn)

	schemaFS := migrations.Schema()
	migrationFiles, err := fs.ReadDir(schemaFS, ".")
	if err != nil {
		ac.logger.Error("failed to read embedded migrations", "error", err)
		return err
	}

	for _, migration := range migrationFiles {
		if filepath.Ext(migration.Name()) != ".sql" {
			continue
		}

		sqlBytes, err := fs.ReadFile(schemaFS, migration.Name())
		if err != nil {
			ac.logger.Error("failed to read embedded migration",
				"file", migration.Name(),
				"error", err)
			return err
		}

		ac.logger.Info("applying migration", "file", migration.Name())
		if err := sqlitex.ExecuteScript(conn, string(sqlBytes), nil); err != nil {
			ac.logger.Error("failed to execute migration",
				"file", migration.Name(),
				"error", err)
			return err
		}
	}
	return nil
}

func (ac *AppCreator) generateDefaultConfig() (*config.Config, error) {
	// Values based on config.toml.example
	cfg := &config.Config{
		DBPath:    "app.db",
		PublicDir: "static/dist",
		Jwt: config.Jwt{
			AuthSecret:                     crypto.RandomString(32, crypto.AlphanumericAlphabet), // Generated
			AuthTokenDuration:              config.Duration{Duration: 45 * time.Minute},
			VerificationEmailSecret:        crypto.RandomString(32, crypto.AlphanumericAlphabet), // Generated
			VerificationEmailTokenDuration: config.Duration{Duration: 24 * time.Hour},
			PasswordResetSecret:            crypto.RandomString(32, crypto.AlphanumericAlphabet), // Generated
			PasswordResetTokenDuration:     config.Duration{Duration: 1 * time.Hour},
			EmailChangeSecret:              crypto.RandomString(32, crypto.AlphanumericAlphabet), // Generated
			EmailChangeTokenDuration:       config.Duration{Duration: 1 * time.Hour},
		},
		Scheduler: config.Scheduler{
			Interval:              config.Duration{Duration: 60 * time.Second},
			MaxJobsPerTick:        10,
			ConcurrencyMultiplier: 2,
		},
		Server: config.Server{
			Addr:                    ":8080",
			ShutdownGracefulTimeout: config.Duration{Duration: 15 * time.Second},
			ReadTimeout:             config.Duration{Duration: 2 * time.Second},
			ReadHeaderTimeout:       config.Duration{Duration: 2 * time.Second},
			WriteTimeout:            config.Duration{Duration: 3 * time.Second},
			IdleTimeout:             config.Duration{Duration: 1 * time.Minute},
			ClientIpProxyHeader:     "",
			EnableTLS:               false,
			CertData:                "",
			KeyData:                 "",
			RedirectAddr:            "",
		},
		RateLimits: config.RateLimits{
			PasswordResetCooldown:     config.Duration{Duration: 2 * time.Hour},
			EmailVerificationCooldown: config.Duration{Duration: 1 * time.Hour},
			EmailChangeCooldown:       config.Duration{Duration: 1 * time.Hour},
		},
		OAuth2Providers: map[string]config.OAuth2Provider{
			"google": {
				Name:         "google",
				DisplayName:  "Google",
				RedirectURL:  "", // Dynamic
				AuthURL:      "https://accounts.google.com/o/oauth2/v2/auth",
				TokenURL:     "https://oauth2.googleapis.com/token",
				UserInfoURL:  "https://www.googleapis.com/oauth2/v3/userinfo",
				Scopes:       []string{"https://www.googleapis.com/auth/userinfo.profile", "https://www.googleapis.com/auth/userinfo.email"},
				PKCE:         true,
				ClientID:     "", // Must be set via env
				ClientSecret: "", // Must be set via env
			},
			"github": {
				Name:         "github",
				DisplayName:  "GitHub",
				RedirectURL:  "", // Dynamic
				AuthURL:      "https://github.com/login/oauth/authorize",
				TokenURL:     "https://github.com/login/oauth/access_token",
				UserInfoURL:  "https://api.github.com/user",
				Scopes:       []string{"read:user", "user:email"},
				PKCE:         true,
				ClientID:     "", // Must be set via env
				ClientSecret: "", // Must be set via env
			},
		},
		Smtp: config.Smtp{
			Enabled:     false,
			Host:        "smtp.gmail.com", // Example
			Port:        587,              // Example
			FromName:    "My App",
			FromAddress: "", // Must be set via env
			LocalName:   "", // Default to localhost if empty
			AuthMethod:  "plain",
			UseTLS:      false,
			UseStartTLS: true,
			Username:    "", // Must be set via env
			Password:    "", // Must be set via env
		},
		Endpoints: config.Endpoints{
			RefreshAuth:              "POST /api/refresh-auth",
			RequestEmailVerification: "POST /api/request-email-verification",
			ConfirmEmailVerification: "POST /api/confirm-email-verification", // Corrected based on example
			ListEndpoints:            "GET /api/list-endpoints",
			AuthWithPassword:         "POST /api/auth-with-password",
			AuthWithOAuth2:           "POST /api/auth-with-oauth2", // Corrected based on example
			RegisterWithPassword:     "POST /api/register-with-password",
			ListOAuth2Providers:      "GET /api/list-oauth2-providers",
			RequestPasswordReset:     "POST /api/request-password-reset",
			ConfirmPasswordReset:     "POST /api/confirm-password-reset",
			RequestEmailChange:       "POST /api/request-email-change",
			ConfirmEmailChange:       "POST /api/confirm-email-change",
		},
		BlockIp: config.BlockIp{
			Enabled: true, // Default from example
		},
		Maintenance: config.Maintenance{
			Enabled:   true, // Default from example
			Activated: false,
		},
		BlockUa: config.BlockUa{
			Activated: true, // Default to activated
			List: config.Regexp{
				// Example list demonstrating escaping:
				// - \. is required to match a literal dot.
				// - \- and \  are tolerated but unnecessary escapes for hyphen and space.
				// Replace with your actual blocklist.
				Regexp: regexp.MustCompile(`(BotName\.v1|Super\-Bot|My\ Bot|AnotherBot)`),
			},
		},
	}
	return cfg, nil
}

// SaveConfig uses the configured SecureConfig implementation to save the config.
func (ac *AppCreator) SaveConfig(configData []byte) error {
	ac.logger.Info("saving initial configuration using SecureConfig")
	err := ac.secureConfig.Save(
		config.ScopeApplication,
		configData,
		"toml",
		"Initial default configuration",
	)
	if err != nil {
		ac.logger.Error("failed to save initial config via SecureConfig", "error", err)
		return fmt.Errorf("failed to save initial config: %w", err)
	}
	return nil
}

func main() {
	dbPathFlag := flag.String("db", "", "Path to the SQLite database file to create (required)")
	ageKeyPathFlag := flag.String("age-key", "", "Path to the age identity (private key) file for encryption (required)")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s -db <database-path> -age-key <identity-file-path>\n", os.Args[0])
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

	// 1. Create Database Pool
	creator.logger.Info("creating sqlite database pool", "path", *dbPathFlag)
	if err := creator.CreateDatabasePool(*dbPathFlag); err != nil {
		os.Exit(1) // Error logged in CreateDatabasePool
	}
	defer func() {
		if creator.pool != nil {
			creator.logger.Info("closing database pool")
			if err := creator.pool.Close(); err != nil {
				creator.logger.Error("error closing database pool", "error", err)
			}
		}
	}()

	// 2. Instantiate DB implementation
	dbImpl, err := zdb.New(creator.pool)
	if err != nil {
		creator.logger.Error("failed to instantiate zombiezen db", "error", err)
		os.Exit(1)
	}

	// 3. Instantiate SecureConfig
	secureCfg, err := config.NewSecureConfigAge(dbImpl, *ageKeyPathFlag, creator.logger)
	if err != nil {
		creator.logger.Error("failed to instantiate secure config (age)", "error", err)
		os.Exit(1)
	}
	creator.secureConfig = secureCfg // Assign to creator

	// 4. Run Migrations (Apply Schema)
	if err := creator.RunMigrations(); err != nil {
		os.Exit(1) // Error logged in RunMigrations
	}

	// 5. Generate Default Config Struct
	defaultCfg, err := creator.generateDefaultConfig()
	if err != nil {
		creator.logger.Error("failed to generate default config struct", "error", err)
		os.Exit(1)
	}

	// 6. Marshal Config to TOML
	tomlBytes, err := toml.Marshal(defaultCfg)
	if err != nil {
		creator.logger.Error("failed to marshal default config to TOML", "error", err)
		os.Exit(1)
	}

	// 7. Save Encrypted Config into DB via SecureConfig
	if err := creator.SaveConfig(tomlBytes); err != nil {
		// Error logged in SaveConfig
		os.Exit(1)
	}

	creator.logger.Info("application database created and configured successfully", "db_file", *dbPathFlag)
}
