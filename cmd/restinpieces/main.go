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
		JwtSecret:       []byte("test_secret_32_bytes_long_xxxxxx"), // 32-byte secret
		TokenDuration:   15 * time.Minute,
		DBFile:          *dbfile,
		OAuth2Providers: make(map[string]config.OAuth2Provider),
	}

	// Configure Google OAuth2 provider
	googleConfig := config.OAuth2Provider{
		Name:         config.OAuth2ProviderGoogle,
		ClientID:     config.EnvVar{Name: config.EnvGoogleClientID},
		ClientSecret: config.EnvVar{Name: config.EnvGoogleClientSecret},
		DisplayName:  "Google",
		RedirectURL:  "http://localhost:8080/oauth2/callback/",
		AuthURL:      "https://accounts.google.com/o/oauth2/v2/auth",
		TokenURL:     "https://oauth2.googleapis.com/token",
		UserInfoURL:  "https://www.googleapis.com/oauth2/v3/userinfo",
		Scopes: []string{
			"https://www.googleapis.com/auth/userinfo.profile",
			"https://www.googleapis.com/auth/userinfo.email",
		},
		PKCE: true,
		UserInfoFields: map[string]string{
			"id":             "sub",
			"email":          "email",
			"name":           "name",
			"avatar":         "picture",
			"email_verified": "email_verified",
		},
	}
	if err := googleConfig.FillEnvVars(); err != nil {
		slog.Warn("skipping Google OAuth2 provider", "error", err)
	} else {
		cfg.OAuth2Providers[config.OAuth2ProviderGoogle] = googleConfig
	}

	// Configure GitHub OAuth2 provider
	githubConfig := config.OAuth2Provider{
		Name:         config.OAuth2ProviderGitHub,
		ClientID:     config.EnvVar{Name: config.EnvGithubClientID},
		ClientSecret: config.EnvVar{Name: config.EnvGithubClientSecret},
		DisplayName:  "GitHub",
		RedirectURL:  "http://localhost:8080/oauth2/callback/",
		AuthURL:      "https://github.com/login/oauth/authorize",
		TokenURL:     "https://github.com/login/oauth/access_token",
		UserInfoURL:  "https://api.github.com/user",
		Scopes:       []string{"read:user", "user:email"},
		PKCE:         true,
	}
	if err := githubConfig.FillEnvVars(); err != nil {
		slog.Warn("skipping GitHub OAuth2 provider", "error", err)
	} else {
		cfg.OAuth2Providers[config.OAuth2ProviderGitHub] = githubConfig
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
