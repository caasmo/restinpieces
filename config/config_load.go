package config

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"os"

	"filippo.io/age"
	"github.com/pelletier/go-toml/v2" // TOML v2 parser

	"github.com/caasmo/restinpieces/db" // Adjust import path if necessary
)

const (
	EnvGoogleClientID             = "OAUTH2_GOOGLE_CLIENT_ID"
	EnvGoogleClientSecret         = "OAUTH2_GOOGLE_CLIENT_SECRET"
	EnvGithubClientID             = "OAUTH2_GITHUB_CLIENT_ID"
	EnvGithubClientSecret         = "OAUTH2_GITHUB_CLIENT_SECRET"
	EnvSmtpUsername               = "SMTP_USERNAME"
	EnvSmtpPassword               = "SMTP_PASSWORD"
	EnvJwtAuthSecret              = "JWT_AUTH_SECRET"
	EnvJwtVerificationEmailSecret = "JWT_VERIFICATION_EMAIL_SECRET"
	EnvJwtPasswordResetSecret     = "JWT_PASSWORD_RESET_SECRET"
	EnvJwtEmailChangeSecret       = "JWT_EMAIL_CHANGE_SECRET"
	EnvAcmeCloudflareApiToken     = "ACME_CLOUDFLARE_API_TOKEN"
	EnvAcmeLetsencryptPrivateKey  = "ACME_LETSENCRYPT_PRIVATE_KEY" // ACME account private key (PEM format)
)
// LoadFromDb loads configuration from the database using the provided DbConfig.
func LoadFromDb(db db.DbConfig, logger *slog.Logger) (*Config, error) {
	logger.Info("loading configuration from database")
	encryptedData, err := db.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("config: failed to get from db: %w", err)
	}

	// Check if config is empty
	if len(encryptedData) == 0 {
		logger.Warn("no configuration found in database")
		return nil, fmt.Errorf("config: no configuration found in database")
	}

	// --- Decrypt Config ---
	keyFile := "age_key.txt" // TODO: Make this configurable
	keyContent, err := os.ReadFile(keyFile)
	if err != nil {
		logger.Error("failed to read age key file", "path", keyFile, "error", err)
		return nil, fmt.Errorf("failed to read age key file '%s': %w", keyFile, err)
	}

	identities, err := age.ParseIdentities(bytes.NewReader(keyContent))
	if err != nil {
		logger.Error("failed to parse age identities", "path", keyFile, "error", err)
		return nil, fmt.Errorf("failed to parse age identities from key file '%s': %w", keyFile, err)
	}
	if len(identities) == 0 {
		logger.Error("no age identities found in key file", "path", keyFile)
		return nil, fmt.Errorf("no age identities found in key file '%s'", keyFile)
	}

	encryptedDataReader := bytes.NewReader(encryptedData) // Use the byte slice directly
	decryptedDataReader, err := age.Decrypt(encryptedDataReader, identities...)
	if err != nil {
		logger.Error("failed to decrypt configuration data", "error", err)
		return nil, fmt.Errorf("failed to decrypt configuration data: %w", err)
	}

	decryptedBytes, err := io.ReadAll(decryptedDataReader)
	if err != nil {
		logger.Error("failed to read decrypted data stream", "error", err)
		return nil, fmt.Errorf("failed to read decrypted data stream: %w", err)
	}

	// --- Unmarshal TOML ---
	cfg := &Config{}
	err = toml.Unmarshal(decryptedBytes, cfg)
	if err != nil {
		logger.Error("failed to unmarshal TOML from database", "error", err)
		// Log the decrypted content only if unmarshalling fails, for debugging
		logger.Debug("decrypted content on unmarshal failure", "content", string(decryptedBytes))
		return nil, fmt.Errorf("config: failed to unmarshal TOML: %w", err)
	}

	// Validate the loaded configuration
	if err := Validate(cfg); err != nil {
		logger.Error("configuration validation failed after loading from DB", "error", err)
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	// Load secrets after initial config load
	if err := loadSecrets(cfg, logger); err != nil {
		// Error already logged within loadSecrets
		return nil, fmt.Errorf("failed to load secrets: %w", err)
	}

	logger.Info("successfully loaded configuration from database", "cfg", cfg)
	return cfg, nil
}

// loadSecrets handles loading all secrets from environment variables
func loadSecrets(cfg *Config, logger *slog.Logger) error {
	if cfg.OAuth2Providers == nil {
		cfg.OAuth2Providers = make(map[string]OAuth2Provider)
	}

	if err := LoadJwt(cfg, logger); err != nil {
		return err
	}

	if err := LoadSmtp(cfg, logger); err != nil {
		return err
	}

	if err := LoadOAuth2(cfg, logger); err != nil {
		return err
	}

	if err := LoadAcme(cfg, logger); err != nil {
		return err
	}

	return nil
}

// LoadAcme loads ACME provider secrets (like Cloudflare API token)
func LoadAcme(cfg *Config, logger *slog.Logger) error {
	// Only load secrets if ACME is enabled and the provider requires them
	if !cfg.Acme.Enabled || cfg.Acme.DNSProvider != "cloudflare" {
		logger.Debug("Skipping ACME secret loading (disabled or provider != cloudflare)")
		return nil
	}

	var err error
	var source string

	cfg.Acme.CloudflareApiToken, source, err = LoadEnvSecret(EnvAcmeCloudflareApiToken, cfg.Acme.CloudflareApiToken) // Token MUST come from env
	if err != nil {
		logger.Error("failed to load ACME Cloudflare API token", "env_var", EnvAcmeCloudflareApiToken, "error", err)
		return fmt.Errorf("failed to load ACME Cloudflare API token: %w", err)
	}
	logger.Debug("Load Envar:", "envvar", EnvAcmeCloudflareApiToken, "source", source)

	// Load ACME Account Private Key
	cfg.Acme.AcmePrivateKey, source, err = LoadEnvSecret(EnvAcmeLetsencryptPrivateKey, cfg.Acme.AcmePrivateKey) // Key MUST come from env
	if err != nil {
		logger.Error("failed to load ACME account private key", "env_var", EnvAcmeLetsencryptPrivateKey, "error", err)
		return fmt.Errorf("failed to load ACME account private key: %w", err)
	}
	logger.Debug("Load Envar:", "envvar", EnvAcmeLetsencryptPrivateKey, "source", source)

	return nil
}

// LoadEnvSecret loads a secret from an environment variable or config.
// Returns: (value, source, error) where source is either "environment" or "config"
func LoadEnvSecret(envVar string, defaultValue string) (string, string, error) {
	if value := os.Getenv(envVar); value != "" {
		return value, "environment", nil
	}
	if defaultValue != "" {
		return defaultValue, "config", nil
	}
	return "", "", fmt.Errorf("secret required: set %s in environment variables or in config", envVar)
}

// LoadJwt loads JWT secrets from environment variables or the config file.
func LoadJwt(cfg *Config, logger *slog.Logger) error {
	var err error
	var source string

	cfg.Jwt.AuthSecret, source, err = LoadEnvSecret(EnvJwtAuthSecret, cfg.Jwt.AuthSecret)
	if err != nil {
		logger.Error("failed to load JWT auth secret", "env_var", EnvJwtAuthSecret, "error", err)
		return fmt.Errorf("failed to load auth secret: %w", err)
	}
	logger.Debug("Load Envar:", "envvar", EnvJwtAuthSecret, "source", source)

	cfg.Jwt.VerificationEmailSecret, source, err = LoadEnvSecret(EnvJwtVerificationEmailSecret, cfg.Jwt.VerificationEmailSecret)
	if err != nil {
		logger.Error("failed to load JWT verification email secret", "env_var", EnvJwtVerificationEmailSecret, "error", err)
		return fmt.Errorf("failed to load verification email secret: %w", err)
	}
	logger.Debug("Load Envar:", "envvar", EnvJwtVerificationEmailSecret, "source", source)

	cfg.Jwt.PasswordResetSecret, source, err = LoadEnvSecret(EnvJwtPasswordResetSecret, cfg.Jwt.PasswordResetSecret)
	if err != nil {
		logger.Error("failed to load JWT password reset secret", "env_var", EnvJwtPasswordResetSecret, "error", err)
		return fmt.Errorf("failed to load password reset secret: %w", err)
	}
	logger.Debug("Load Envar:", "envvar", EnvJwtPasswordResetSecret, "source", source)

	cfg.Jwt.EmailChangeSecret, source, err = LoadEnvSecret(EnvJwtEmailChangeSecret, cfg.Jwt.EmailChangeSecret)
	if err != nil {
		logger.Error("failed to load JWT email change secret", "env_var", EnvJwtEmailChangeSecret, "error", err)
		return fmt.Errorf("failed to load email change secret: %w", err)
	}
	logger.Debug("Load Envar:", "envvar", EnvJwtEmailChangeSecret, "source", source)

	return nil
}

// LoadSmtp loads SMTP credentials from environment variables or the config file.
// Only loads credentials if SMTP is enabled in config.
func LoadSmtp(cfg *Config, logger *slog.Logger) error {
	if !cfg.Smtp.Enabled {
		logger.Debug("SMTP is disabled in config, skipping credential loading")
		return nil
	}

	var err error
	var source string

	// Require username when SMTP is enabled
	cfg.Smtp.Username, source, err = LoadEnvSecret(EnvSmtpUsername, cfg.Smtp.Username)
	if err != nil {
		logger.Error("SMTP username required when SMTP is enabled", "env_var", EnvSmtpUsername, "error", err)
		return fmt.Errorf("SMTP username required when SMTP is enabled: %w", err)
	}
	logger.Debug("Loaded SMTP username", "source", source)

	// Require password when SMTP is enabled
	cfg.Smtp.Password, source, err = LoadEnvSecret(EnvSmtpPassword, cfg.Smtp.Password)
	if err != nil {
		logger.Error("SMTP password required when SMTP is enabled", "env_var", EnvSmtpPassword, "error", err)
		return fmt.Errorf("SMTP password required when SMTP is enabled: %w", err)
	}
	logger.Debug("Loaded SMTP password", "source", source)

	// Validate other required fields
	if cfg.Smtp.Host == "" {
		logger.Warn("SMTP host not configured but SMTP is enabled")
	}
	if cfg.Smtp.Port == 0 {
		logger.Warn("SMTP port not configured but SMTP is enabled")
	}
	if cfg.Smtp.FromAddress == "" {
		logger.Warn("SMTP from address not configured but SMTP is enabled")
	}

	return nil
}

// LoadOAuth2 loads OAuth2 client credentials from environment variables or the config file.
// It also constructs the RedirectURL based on the server's BaseURL.
// Providers without both ClientID and ClientSecret are removed.
func LoadOAuth2(cfg *Config, logger *slog.Logger) error {
	baseURL := cfg.Server.BaseURL()

	// Google OAuth2
	if googleCfg, ok := cfg.OAuth2Providers[OAuth2ProviderGoogle]; ok {
		var errID, errSecret error
		var sourceID, sourceSecret string

		googleCfg.ClientID, sourceID, errID = LoadEnvSecret(EnvGoogleClientID, googleCfg.ClientID)
		googleCfg.ClientSecret, sourceSecret, errSecret = LoadEnvSecret(EnvGoogleClientSecret, googleCfg.ClientSecret)
		// TODO path is in conf
		googleCfg.RedirectURL = fmt.Sprintf("%s/oauth2/callback/", baseURL)

		if errID != nil || errSecret != nil {
			logger.Warn("disabling Google OAuth2 provider due to missing secrets",
				"client_id_error", errID,
				"client_secret_error", errSecret)
			delete(cfg.OAuth2Providers, OAuth2ProviderGoogle)
		} else {
			cfg.OAuth2Providers[OAuth2ProviderGoogle] = googleCfg
			logger.Debug("Load Envar:", "envvar", EnvGoogleClientID, "source", sourceID)
			logger.Debug("Load Envar:", "envvar", EnvGoogleClientSecret, "source", sourceSecret)
		}
	}

	// GitHub OAuth2
	if githubCfg, ok := cfg.OAuth2Providers[OAuth2ProviderGitHub]; ok {
		var errID, errSecret error
		var sourceID, sourceSecret string

		githubCfg.ClientID, sourceID, errID = LoadEnvSecret(EnvGithubClientID, githubCfg.ClientID)
		githubCfg.ClientSecret, sourceSecret, errSecret = LoadEnvSecret(EnvGithubClientSecret, githubCfg.ClientSecret)
		githubCfg.RedirectURL = fmt.Sprintf("%s/oauth2/callback/", baseURL)

		if errID != nil || errSecret != nil {
			logger.Warn("disabling GitHub OAuth2 provider due to missing secrets",
				"client_id_error", errID,
				"client_secret_error", errSecret)
			delete(cfg.OAuth2Providers, OAuth2ProviderGitHub)
		} else {
			cfg.OAuth2Providers[OAuth2ProviderGitHub] = githubCfg
			logger.Debug("Load Envar:", "envvar", EnvGithubClientID, "source", sourceID)
			logger.Debug("Load Envar:", "envvar", EnvGithubClientSecret, "source", sourceSecret)
		}
	}

	return nil
}
