package config

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/BurntSushi/toml"
	"github.com/caasmo/restinpieces/db"
)

// LoadFromToml loads configuration from a TOML file at the given path.
// Returns error if file doesn't exist or can't be decoded.
func LoadFromToml(path string, logger *slog.Logger) (*Config, error) {
	logger.Info("loading configuration from TOML file", "path", path)
	cfg := &Config{}

	if _, err := toml.DecodeFile(path, cfg); err != nil {
		logger.Error("failed to decode config file", "path", path, "error", err)
		return nil, fmt.Errorf("failed to decode config file: %w", err)
	}

	// Load secrets after initial config load
	if err := loadSecrets(cfg, logger); err != nil {
		// Error already logged within loadSecrets
		return nil, fmt.Errorf("failed to load secrets: %w", err)
	}

	logger.Info("successfully loaded configuration from TOML file", "path", path)
	return cfg, nil
}

// LoadFromDb loads configuration from the database using the provided DbConfig.
// Falls back to embedded defaults if no config exists in database.
func LoadFromDb(db db.DbConfig, logger *slog.Logger) (*Config, error) {
	logger.Info("loading configuration from database")

	// Get config TOML from database
	configToml, err := db.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("config: failed to get from db: %w", err)
	}
	
	// Check if config is empty
	if configToml == "" {
		logger.Warn("no configuration found in database")
		return nil, fmt.Errorf("config: no configuration found in database")
	}

	// Decode TOML into Config struct
	cfg := &Config{}
	if _, err := toml.Decode(configToml, cfg); err != nil {
		logger.Error("failed to decode configuration from database", "error", err)
		return nil, fmt.Errorf("config: failed to decode: %w", err)
	}

	// Load secrets after initial config load
	if err := loadSecrets(cfg, logger); err != nil {
		// Error already logged within loadSecrets
		return nil, fmt.Errorf("failed to load secrets: %w", err)
	}

	logger.Info("successfully loaded configuration from database")
	return cfg, nil
}

// loadSecrets handles loading all secrets from environment variables
func loadSecrets(cfg *Config, logger *slog.Logger) error {
	logger.Debug("loading secrets from environment variables and config")
	if cfg.OAuth2Providers == nil {
		cfg.OAuth2Providers = make(map[string]OAuth2Provider)
	}

	if err := LoadJwt(cfg, logger); err != nil {
		logger.Error("failed to load JWT secrets", "error", err)
		return err
	}

	if err := LoadSmtp(cfg, logger); err != nil {
		logger.Error("failed to load SMTP secrets", "error", err)
		return err
	}

	if err := LoadOAuth2(cfg, logger); err != nil {
		logger.Error("failed to load OAuth2 secrets", "error", err)
		return err
	}

	logger.Debug("finished loading secrets")
	return nil
}

// LoadEnvSecret loads a secret from an environment variable.
// If the env var is empty, it returns the defaultValue.
// Returns an error if both are empty.
func LoadEnvSecret(envVar string, defaultValue string) (string, error) {
	if value := os.Getenv(envVar); value != "" {
		return value, nil
	}

	if defaultValue != "" {
		return defaultValue, nil
	}
	return "", fmt.Errorf("secret required: set %s in environment variables or in config", envVar)
}

// LoadJwt loads JWT secrets from environment variables or the config file.
func LoadJwt(cfg *Config, logger *slog.Logger) error {
	logger.Debug("loading JWT secrets")
	var err error
	cfg.Jwt.AuthSecret, err = LoadEnvSecret("JWT_AUTH_SECRET", cfg.Jwt.AuthSecret)
	if err != nil {
		logger.Error("failed to load JWT auth secret", "env_var", "JWT_AUTH_SECRET", "error", err)
		return fmt.Errorf("failed to load auth secret: %w", err)
	}

	cfg.Jwt.VerificationEmailSecret, err = LoadEnvSecret("JWT_VERIFICATION_EMAIL_SECRET", cfg.Jwt.VerificationEmailSecret)
	if err != nil {
		logger.Error("failed to load JWT verification email secret", "env_var", "JWT_VERIFICATION_EMAIL_SECRET", "error", err)
		return fmt.Errorf("failed to load verification email secret: %w", err)
	}

	cfg.Jwt.PasswordResetSecret, err = LoadEnvSecret("JWT_PASSWORD_RESET_SECRET", cfg.Jwt.PasswordResetSecret)
	if err != nil {
		logger.Error("failed to load JWT password reset secret", "env_var", "JWT_PASSWORD_RESET_SECRET", "error", err)
		return fmt.Errorf("failed to load password reset secret: %w", err)
	}

	cfg.Jwt.EmailChangeSecret, err = LoadEnvSecret("JWT_EMAIL_CHANGE_SECRET", cfg.Jwt.EmailChangeSecret)
	if err != nil {
		logger.Error("failed to load JWT email change secret", "env_var", "JWT_EMAIL_CHANGE_SECRET", "error", err)
		return fmt.Errorf("failed to load email change secret: %w", err)
	}

	logger.Debug("finished loading JWT secrets")
	return nil
}

// LoadSmtp loads SMTP credentials from environment variables or the config file.
func LoadSmtp(cfg *Config, logger *slog.Logger) error {
	logger.Debug("loading SMTP secrets")
	cfg.Smtp.Username = os.Getenv(EnvSmtpUsername)
	if cfg.Smtp.Username != "" {
		logger.Debug("loaded SMTP username from environment variable", "env_var", EnvSmtpUsername)
	}

	var err error
	cfg.Smtp.Password, err = LoadEnvSecret(EnvSmtpPassword, cfg.Smtp.Password)
	if err != nil {
		logger.Error("failed to load SMTP password", "env_var", EnvSmtpPassword, "error", err)
		return fmt.Errorf("failed to load SMTP password: %w", err)
	}
	// Avoid logging the password itself, just confirm it was loaded if not default
	if os.Getenv(EnvSmtpPassword) != "" {
		logger.Debug("loaded SMTP password from environment variable", "env_var", EnvSmtpPassword)
	} else if cfg.Smtp.Password != "" {
		logger.Debug("using SMTP password from config file")
	}


	if fromAddr := os.Getenv("SMTP_FROM_ADDRESS"); fromAddr != "" {
		cfg.Smtp.FromAddress = fromAddr
		logger.Debug("loaded SMTP FromAddress from environment variable", "env_var", "SMTP_FROM_ADDRESS")
	}

	logger.Debug("finished loading SMTP secrets")
	return nil
}

// LoadOAuth2 loads OAuth2 client credentials from environment variables or the config file.
// It also constructs the RedirectURL based on the server's BaseURL.
// Providers without both ClientID and ClientSecret are removed.
func LoadOAuth2(cfg *Config, logger *slog.Logger) error {
	logger.Debug("loading OAuth2 secrets")
	baseURL := cfg.Server.BaseURL()

	// Google OAuth2
	if googleCfg, ok := cfg.OAuth2Providers[OAuth2ProviderGoogle]; ok {
		logger.Debug("loading Google OAuth2 secrets")
		var errID, errSecret error
		googleCfg.ClientID, errID = LoadEnvSecret(EnvGoogleClientID, googleCfg.ClientID)
		googleCfg.ClientSecret, errSecret = LoadEnvSecret(EnvGoogleClientSecret, googleCfg.ClientSecret)
		googleCfg.RedirectURL = fmt.Sprintf("%s/oauth2/callback/", baseURL) // Assuming this is correct, adjust if needed

		if errID != nil || errSecret != nil {
			logger.Warn("disabling Google OAuth2 provider due to missing secrets", "client_id_error", errID, "client_secret_error", errSecret)
			delete(cfg.OAuth2Providers, OAuth2ProviderGoogle)
		} else {
			logger.Debug("successfully loaded Google OAuth2 secrets")
			cfg.OAuth2Providers[OAuth2ProviderGoogle] = googleCfg
		}
	}

	// GitHub OAuth2
	if githubCfg, ok := cfg.OAuth2Providers[OAuth2ProviderGitHub]; ok {
		logger.Debug("loading GitHub OAuth2 secrets")
		var errID, errSecret error
		githubCfg.ClientID, errID = LoadEnvSecret(EnvGithubClientID, githubCfg.ClientID)
		githubCfg.ClientSecret, errSecret = LoadEnvSecret(EnvGithubClientSecret, githubCfg.ClientSecret)
		githubCfg.RedirectURL = fmt.Sprintf("%s/oauth2/callback/", baseURL) // Assuming this is correct, adjust if needed

		if errID != nil || errSecret != nil {
			logger.Warn("disabling GitHub OAuth2 provider due to missing secrets", "client_id_error", errID, "client_secret_error", errSecret)
			delete(cfg.OAuth2Providers, OAuth2ProviderGitHub)
		} else {
			logger.Debug("successfully loaded GitHub OAuth2 secrets")
			cfg.OAuth2Providers[OAuth2ProviderGitHub] = githubCfg
		}
	}

	logger.Debug("finished loading OAuth2 secrets")
	return nil
}
