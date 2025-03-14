package main

import (
	"flag"
	"log/slog"
	"os"
	"time"

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
		OAuth2Providers: map[string]config.OAuth2ProviderConfig{
			config.OAuth2ProviderGoogle: {
				Name:         config.OAuth2ProviderGoogle,
				ClientID:     "google_client_id_example",
				ClientSecret: "google_client_secret_example",
				DisplayName:  "Google",
				RedirectURL:  "http://localhost:8080/callback/google",
				AuthURL:      "https://accounts.google.com/o/oauth2/auth",
				TokenURL:     "https://oauth2.googleapis.com/token",
				UserInfoURL:  "https://www.googleapis.com/oauth2/v3/userinfo",
				Scopes:       []string{"https://www.googleapis.com/auth/userinfo.email"},
				PKCE:         true,
			},
			config.OAuth2ProviderGitHub: {
				Name:         config.OAuth2ProviderGitHub,
				ClientID:     "github_client_id_example",
				ClientSecret: "github_client_secret_example",
				DisplayName:  "GitHub",
				RedirectURL:  "http://localhost:8080/callback/github",
				AuthURL:      "https://github.com/login/oauth/authorize",
				TokenURL:     "https://github.com/login/oauth/access_token",
				UserInfoURL:  "https://api.github.com/user",
				Scopes:       []string{"user:email"},
				PKCE:         true,
			},
		},
		CallbackURL: "http://localhost:8080",
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
