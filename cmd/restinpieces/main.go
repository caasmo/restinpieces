package main

import (
	"flag"
	"log/slog"
	"os"
	"time"
	"strings"

	"github.com/caasmo/restinpieces/config"
	"github.com/caasmo/restinpieces/server"
)

func main() {

	dbfile := flag.String("dbfile", "bench.db", "SQLite database file path")
	flag.Parse()


	cfg := &config.Config{
		JwtSecret:         []byte("test_secret_32_bytes_long_xxxxxx"), // 32-byte secret
		TokenDuration:     15 * time.Minute,
		DBFile:            *dbfile,
		OAuth2Providers: make(map[string]config.OAuth2ProviderConfig),
		CallbackURL: "http://localhost:8080",
	}

	// Load environment variables
	env := config.env{
		GoogleClientID:     config.envOauth2{Name: config.EnvGoogleClientID, Value: os.Getenv(config.EnvGoogleClientID)},
		GoogleClientSecret: config.envOauth2{Name: config.EnvGoogleClientSecret, Value: os.Getenv(config.EnvGoogleClientSecret)},
		GithubClientID:     config.envOauth2{Name: config.EnvGithubClientID, Value: os.Getenv(config.EnvGithubClientID)},
		GithubClientSecret: config.envOauth2{Name: config.EnvGithubClientSecret, Value: os.Getenv(config.EnvGithubClientSecret)},
	}

	// Configure OAuth2 providers only if both client ID and secret are present
	if env.GoogleClientID.Value != "" && env.GoogleClientSecret.Value != "" {
		cfg.OAuth2Providers[config.OAuth2ProviderGoogle] = config.OAuth2ProviderConfig{
			Name:         config.OAuth2ProviderGoogle,
			ClientID:     env.GoogleClientID.Value,
			ClientSecret: env.GoogleClientSecret.Value,
			DisplayName:  "Google",
			RedirectURL:  "http://localhost:8080/callback/google",
			AuthURL:      "https://accounts.google.com/o/oauth2/auth",
			TokenURL:     "https://oauth2.googleapis.com/token",
			UserInfoURL:  "https://www.googleapis.com/oauth2/v3/userinfo",
			Scopes:       []string{"https://www.googleapis.com/auth/userinfo.email"},
			PKCE:         true,
		}
	}

	if env.GithubClientID.Value != "" && env.GithubClientSecret.Value != "" {
		cfg.OAuth2Providers[config.OAuth2ProviderGitHub] = config.OAuth2ProviderConfig{
			Name:         config.OAuth2ProviderGitHub,
			ClientID:     env.GithubClientID.Value,
			ClientSecret: env.GithubClientSecret.Value,
			DisplayName:  "GitHub",
			RedirectURL:  "http://localhost:8080/callback/github",
			AuthURL:      "https://github.com/login/oauth/authorize",
			TokenURL:     "https://github.com/login/oauth/access_token",
			UserInfoURL:  "https://api.github.com/user",
			Scopes:       []string{"user:email"},
			PKCE:         true,
		}
	}
	}

	ap, err := initApp(cfg)
	if err != nil {
		slog.Error("failed to initialize app", "error", err)
		os.Exit(1)
	}

	defer ap.Close()

	route(ap)

	server.Run(":8080", ap.Router())
}
