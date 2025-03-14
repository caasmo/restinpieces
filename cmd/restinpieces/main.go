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

	// Configure OAuth2 providers only if both client ID and secret are present
	if googleClientID, googleClientSecret := os.Getenv("OAUTH2_GOOGLE_CLIENT_ID"), os.Getenv("OAUTH2_GOOGLE_CLIENT_SECRET"); googleClientID != "" && googleClientSecret != "" {
		cfg.OAuth2Providers[config.OAuth2ProviderGoogle] = config.OAuth2ProviderConfig{
			Name:         config.OAuth2ProviderGoogle,
			ClientID:     googleClientID,
			ClientSecret: googleClientSecret,
			DisplayName:  "Google",
			RedirectURL:  "http://localhost:8080/callback/google",
			AuthURL:      "https://accounts.google.com/o/oauth2/auth",
			TokenURL:     "https://oauth2.googleapis.com/token",
			UserInfoURL:  "https://www.googleapis.com/oauth2/v3/userinfo",
			Scopes:       []string{"https://www.googleapis.com/auth/userinfo.email"},
			PKCE:         true,
		}
	}

	if githubClientID, githubClientSecret := os.Getenv("OAUTH2_GITHUB_CLIENT_ID"), os.Getenv("OAUTH2_GITHUB_CLIENT_SECRET"); githubClientID != "" && githubClientSecret != "" {
		cfg.OAuth2Providers[config.OAuth2ProviderGitHub] = config.OAuth2ProviderConfig{
			Name:         config.OAuth2ProviderGitHub,
			ClientID:     githubClientID,
			ClientSecret: githubClientSecret,
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
