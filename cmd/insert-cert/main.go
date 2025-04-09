package main

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"
)

type CertInserter struct {
	dbfile string
	logger *slog.Logger
	pool   *sqlitex.Pool
}

func NewCertInserter(dbfile string) *CertInserter {
	return &CertInserter{
		dbfile: dbfile,
		logger: slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		})),
	}
}

func (ci *CertInserter) OpenDatabase() error {
	pool, err := sqlitex.NewPool(ci.dbfile, sqlitex.PoolOptions{
		Flags:    sqlite.OpenReadWrite,
		PoolSize: runtime.NumCPU(),
	})
	if err != nil {
		ci.logger.Error("failed to open database", "error", err)
		return err
	}
	ci.pool = pool
	return nil
}

func (ci *CertInserter) InsertCert(keyPath, certPath string) error {
	// Read key and cert files
	keyData, err := os.ReadFile(keyPath)
	if err != nil {
		ci.logger.Error("failed to read key file", "path", keyPath, "error", err)
		return err
	}

	certData, err := os.ReadFile(certPath)
	if err != nil {
		ci.logger.Error("failed to read cert file", "path", certPath, "error", err)
		return err
	}

	conn, err := ci.pool.Take(context.Background())
	if err != nil {
		return err
	}
	defer ci.pool.Put(conn)

	now := time.Now().UTC().Format(time.RFC3339)
	expires := time.Now().Add(90 * 24 * time.Hour).UTC().Format(time.RFC3339)

	err = sqlitex.Execute(conn,
		`INSERT INTO acme_certificates (
			identifier, 
			domains,
			private_key,
			certificate_chain,
			issued_at,
			expires_at
		) VALUES (?, ?, ?, ?, ?, ?)`,
		&sqlitex.ExecOptions{
			Args: []interface{}{
				"default",               // identifier
				"[]",                    // domains (empty JSON array)
				string(keyData),          // private_key
				string(certData),        // certificate_chain
				now,                     // issued_at
				expires,                 // expires_at
			},
		})

	if err != nil {
		ci.logger.Error("failed to insert certificate", "error", err)
		return err
	}

	ci.logger.Info("successfully inserted ACME certificate")
	return nil
}

func main() {
	if len(os.Args) != 3 {
		slog.Error("usage: insert-cert <key-file> <cert-file>")
		os.Exit(1)
	}

	keyPath := os.Args[1]
	certPath := os.Args[2]

	// Use same default DB path as create-app
	dbPath := "app.db"
	if envDb := os.Getenv("DB_FILE"); envDb != "" {
		dbPath = envDb
	}

	inserter := NewCertInserter(dbPath)
	if err := inserter.OpenDatabase(); err != nil {
		os.Exit(1)
	}
	defer inserter.pool.Close()

	if err := inserter.InsertCert(keyPath, certPath); err != nil {
		os.Exit(1)
	}
}
