//go:build ignore

// Command smtp_probe sends one OTP mail with the latest config from the
// application database and exits. It reads what the app reads, so a
// provider switch is verified before users hit it.
//
// Runs in the deploy directory: the database is data/app.db and the age
// key is age.key. Prod only.
//
// Usage:
//
//	go run scripts/smtp_probe.go you@example.com
//	go build -o /tmp/smtp-probe scripts/smtp_probe.go
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/caasmo/restinpieces"
	"github.com/caasmo/restinpieces/config"
	"github.com/caasmo/restinpieces/db/databasesql"
	"github.com/caasmo/restinpieces/mail"
	"github.com/pelletier/go-toml/v2"
)

const (
	dbPath     = "data/app.db"
	ageKeyPath = "age.key"
)

func main() {
	if len(os.Args) != 2 {
		_, _ = fmt.Fprintf(os.Stderr, "usage: %s you@example.com\n", os.Args[0])
		os.Exit(1)
	}
	recipient := os.Args[1]

	pool, err := restinpieces.NewModerncPool(dbPath)
	if err != nil {
		slog.Error("open db", "path", dbPath, "error", err)
		os.Exit(1)
	}
	defer func() {
		_ = pool.Close()
	}()

	dbInstance, err := databasesql.New(pool)
	if err != nil {
		slog.Error("init db", "error", err)
		os.Exit(1)
	}

	secureStore, err := config.NewSecureStoreAge(dbInstance, ageKeyPath)
	if err != nil {
		slog.Error("init config store", "error", err)
		os.Exit(1)
	}

	encrypted, _, err := secureStore.Get(config.ScopeApplication, 0)
	if err != nil {
		slog.Error("load latest config", "error", err)
		os.Exit(1)
	}

	cfg := config.NewDefaultConfig()
	if err := toml.Unmarshal(encrypted, cfg); err != nil {
		slog.Error("parse config", "error", err)
		os.Exit(1)
	}
	if err := config.Validate(cfg); err != nil {
		slog.Error("invalid config", "error", err)
		os.Exit(1)
	}

	provider := config.NewProvider(cfg)

	mailer, err := mail.New(provider)
	if err != nil {
		slog.Error("init mailer", "error", err)
		os.Exit(1)
	}

	smtpCfg := provider.Get().Smtp
	slog.Info("sending probe", "smtp", fmt.Sprintf("%s:%d", smtpCfg.Host, smtpCfg.Port), "from", smtpCfg.FromAddress, "to", recipient)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := mailer.SendOtpEmail(ctx, recipient, "123456"); err != nil {
		slog.Error("send failed", "error", err)
		os.Exit(1)
	}

	slog.Info("sent")
}
