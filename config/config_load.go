package config

import (
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
)

// Load decodes the default embedded TOML config, sets the DB file path,
// initializes the OAuth2 provider map if needed, and then loads secrets
// (JWT, SMTP, OAuth2) from environment variables or the config file.
func Load(dbfile string) (*Config, error) {
	cfg := &Config{}

	if _, err := toml.Decode(string(DefaultConfigToml), cfg); err != nil {
		return nil, fmt.Errorf("failed to decode embedded default config: %w", err)
	}

	cfg.DBFile = dbfile

	if cfg.OAuth2Providers == nil {
		cfg.OAuth2Providers = make(map[string]OAuth2Provider)
	}

	if err := LoadJwt(cfg); err != nil {
		return nil, err
	}

	if err := LoadSmtp(cfg); err != nil {
		return nil, err
	}

	if err := LoadOAuth2(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
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
func LoadJwt(cfg *Config) error {
	var err error
	cfg.Jwt.AuthSecret, err = LoadEnvSecret("JWT_AUTH_SECRET", cfg.Jwt.AuthSecret)
	if err != nil {
		return fmt.Errorf("failed to load auth secret: %w", err)
	}

	cfg.Jwt.VerificationEmailSecret, err = LoadEnvSecret("JWT_VERIFICATION_EMAIL_SECRET", cfg.Jwt.VerificationEmailSecret)
	if err != nil {
		return fmt.Errorf("failed to load verification email secret: %w", err)
	}

	cfg.Jwt.PasswordResetSecret, err = LoadEnvSecret("JWT_PASSWORD_RESET_SECRET", cfg.Jwt.PasswordResetSecret)
	if err != nil {
		return fmt.Errorf("failed to load password reset secret: %w", err)
	}

	cfg.Jwt.EmailChangeSecret, err = LoadEnvSecret("JWT_EMAIL_CHANGE_SECRET", cfg.Jwt.EmailChangeSecret)
	if err != nil {
		return fmt.Errorf("failed to load email change secret: %w", err)
	}

	return nil
}

// LoadSmtp loads SMTP credentials from environment variables or the config file.
func LoadSmtp(cfg *Config) error {
	cfg.Smtp.Username = os.Getenv(EnvSmtpUsername)

	var err error
	cfg.Smtp.Password, err = LoadEnvSecret(EnvSmtpPassword, cfg.Smtp.Password)
	if err != nil {
		return fmt.Errorf("failed to load SMTP password: %w", err)
	}

	if fromAddr := os.Getenv("SMTP_FROM_ADDRESS"); fromAddr != "" {
		cfg.Smtp.FromAddress = fromAddr
	}

	return nil
}

// LoadOAuth2 loads OAuth2 client credentials from environment variables or the config file.
// It also constructs the RedirectURL based on the server's BaseURL.
// Providers without both ClientID and ClientSecret are removed.
func LoadOAuth2(cfg *Config) error {
	baseURL := cfg.Server.BaseURL()

	// Google OAuth2
	if googleCfg, ok := cfg.OAuth2Providers[OAuth2ProviderGoogle]; ok {
		var errID, errSecret error
		googleCfg.ClientID, errID = LoadEnvSecret(EnvGoogleClientID, googleCfg.ClientID)
		googleCfg.ClientSecret, errSecret = LoadEnvSecret(EnvGoogleClientSecret, googleCfg.ClientSecret)
		googleCfg.RedirectURL = fmt.Sprintf("%s/oauth2/callback/", baseURL)

		if errID != nil || errSecret != nil {
			delete(cfg.OAuth2Providers, OAuth2ProviderGoogle)
		} else {
			cfg.OAuth2Providers[OAuth2ProviderGoogle] = googleCfg
		}
	}

	// GitHub OAuth2
	if githubCfg, ok := cfg.OAuth2Providers[OAuth2ProviderGitHub]; ok {
		var errID, errSecret error
		githubCfg.ClientID, errID = LoadEnvSecret(EnvGithubClientID, githubCfg.ClientID)
		githubCfg.ClientSecret, errSecret = LoadEnvSecret(EnvGithubClientSecret, githubCfg.ClientSecret)
		githubCfg.RedirectURL = fmt.Sprintf("%s/oauth2/callback/", baseURL)

		if errID != nil || errSecret != nil {
			delete(cfg.OAuth2Providers, OAuth2ProviderGitHub)
		} else {
			cfg.OAuth2Providers[OAuth2ProviderGitHub] = githubCfg
		}
	}

	return nil
}

