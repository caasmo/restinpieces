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
	
	cfg.Jwt.AuthSecret, source, err = LoadEnvSecret("JWT_AUTH_SECRET", cfg.Jwt.AuthSecret)
	if err != nil {
		logger.Error("failed to load JWT auth secret", "env_var", "JWT_AUTH_SECRET", "error", err)
		return fmt.Errorf("failed to load auth secret: %w", err)
	}
	logger.Debug("loaded JWT_AUTH_SECRET", "source", source)

	cfg.Jwt.VerificationEmailSecret, source, err = LoadEnvSecret("JWT_VERIFICATION_EMAIL_SECRET", cfg.Jwt.VerificationEmailSecret)
	if err != nil {
		logger.Error("failed to load JWT verification email secret", "env_var", "JWT_VERIFICATION_EMAIL_SECRET", "error", err)
		return fmt.Errorf("failed to load verification email secret: %w", err)
	}
	logger.Debug("loaded JWT_VERIFICATION_EMAIL_SECRET", "source", source)

	cfg.Jwt.PasswordResetSecret, source, err = LoadEnvSecret("JWT_PASSWORD_RESET_SECRET", cfg.Jwt.PasswordResetSecret)
	if err != nil {
		logger.Error("failed to load JWT password reset secret", "env_var", "JWT_PASSWORD_RESET_SECRET", "error", err)
		return fmt.Errorf("failed to load password reset secret: %w", err)
	}
	logger.Debug("loaded JWT_PASSWORD_RESET_SECRET", "source", source)

	cfg.Jwt.EmailChangeSecret, source, err = LoadEnvSecret("JWT_EMAIL_CHANGE_SECRET", cfg.Jwt.EmailChangeSecret)
	if err != nil {
		logger.Error("failed to load JWT email change secret", "env_var", "JWT_EMAIL_CHANGE_SECRET", "error", err)
		return fmt.Errorf("failed to load email change secret: %w", err)
	}
	logger.Debug("loaded JWT_EMAIL_CHANGE_SECRET", "source", source)

	return nil
}

// LoadSmtp loads SMTP credentials from environment variables or the config file.
func LoadSmtp(cfg *Config, logger *slog.Logger) error {
	var err error
	var source string
	
	cfg.Smtp.Username, source, err = LoadEnvSecret(EnvSmtpUsername, cfg.Smtp.Username)
	if err != nil {
		logger.Error("failed to load SMTP username", "env_var", EnvSmtpUsername, "error", err)
		return fmt.Errorf("failed to load SMTP username: %w", err)
	}
	logger.Debug("SMTP username loaded", "source", source)

	cfg.Smtp.Password, source, err = LoadEnvSecret(EnvSmtpPassword, cfg.Smtp.Password)
	if err != nil {
		logger.Error("failed to load SMTP password", "env_var", EnvSmtpPassword, "error", err)
		return fmt.Errorf("failed to load SMTP password: %w", err)
	}
	logger.Debug("SMTP password loaded", "source", source)

	cfg.Smtp.FromAddress, source, err = LoadEnvSecret("SMTP_FROM_ADDRESS", cfg.Smtp.FromAddress)
	if err != nil {
		logger.Error("failed to load SMTP from address", "env_var", "SMTP_FROM_ADDRESS", "error", err)
		return fmt.Errorf("failed to load SMTP from address: %w", err)
	}
	logger.Debug("SMTP from address loaded", "source", source)

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
		googleCfg.RedirectURL = fmt.Sprintf("%s/oauth2/callback/", baseURL)

		if errID != nil || errSecret != nil {
			logger.Warn("disabling Google OAuth2 provider due to missing secrets", 
				"client_id_error", errID, 
				"client_secret_error", errSecret)
			delete(cfg.OAuth2Providers, OAuth2ProviderGoogle)
		} else {
			cfg.OAuth2Providers[OAuth2ProviderGoogle] = googleCfg
			logger.Debug("Google OAuth2 client ID loaded", "source", sourceID)
			logger.Debug("Google OAuth2 client secret loaded", "source", sourceSecret)
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
			logger.Debug("GitHub OAuth2 client ID loaded", "source", sourceID)
			logger.Debug("GitHub OAuth2 client secret loaded", "source", sourceSecret)
		}
	}

	return nil
}
