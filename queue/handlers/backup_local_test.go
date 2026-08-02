package handlers

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/caasmo/restinpieces/backup"
	"github.com/caasmo/restinpieces/config"
	"github.com/caasmo/restinpieces/db"
	"github.com/caasmo/restinpieces/db/zombiezen"
	"github.com/caasmo/restinpieces/migrations"
	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"
)

// setupTest creates a temporary directory, a source database with the full schema
// and some data, and a config with the backup directory pointing to the temporary path.
func setupTest(t *testing.T, withData bool) (cfg *config.Config, sourceDbPath, backupDir string) {
	t.Helper()

	tempDir := t.TempDir()
	sourceDbPath = filepath.Join(tempDir, "source.db")
	backupDir = filepath.Join(tempDir, "backups")

	if err := os.Mkdir(backupDir, 0755); err != nil {
		t.Fatalf("Failed to create backup dir: %v", err)
	}

	// Create and populate the source database using the project's migrations
	conn, err := zombiezen.NewConn(sourceDbPath)
	if err != nil {
		t.Fatalf("Failed to open source db connection: %v", err)
	}
	defer func() {
		if err := conn.Close(); err != nil {
			t.Logf("Failed to close source db connection: %v", err)
		}
	}()

	// Apply all schemas
	schemaFS := migrations.Schema()
	err = fs.WalkDir(schemaFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil // Skip directories
		}
		sqlBytes, err := fs.ReadFile(schemaFS, path)
		if err != nil {
			return fmt.Errorf("failed to read migration %s: %w", path, err)
		}
		if err := sqlitex.ExecuteScript(conn, string(sqlBytes), nil); err != nil {
			return fmt.Errorf("failed to execute migration %s: %w", path, err)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Failed to apply migrations: %v", err)
	}

	if withData {
		// Insert some test data to ensure the backup is not empty
		err = sqlitex.Execute(conn, "INSERT INTO users (name, email) VALUES ('test-user', 'test@example.com');", nil)
		if err != nil {
			t.Fatalf("Failed to insert test data: %v", err)
		}
	}

	// Create a config for the test
	cfg = config.NewDefaultConfig()
	cfg.BackupLocal.BackupDir = backupDir

	return cfg, sourceDbPath, backupDir
}

// addDatabase adds a database file entry to the BackupLocal files map.
func addDatabase(cfg *config.Config, key, sourcePath string, compress bool, strategy, frequency string) {
	freq, err := time.ParseDuration(frequency)
	if err != nil {
		panic(fmt.Sprintf("invalid test frequency %q: %v", frequency, err))
	}
	if cfg.BackupLocal.Files == nil {
		cfg.BackupLocal.Files = make(map[string]config.BackupLocalDbFile)
	}
	cfg.BackupLocal.Files[key] = config.BackupLocalDbFile{
		SourcePath:  sourcePath,
		Compression: compress,
		Strategy:    strategy,
		Frequency:   config.Duration{Duration: freq},
	}
}

// verifyBackup checks if a backup file is a valid, non-empty SQLite database.
// It handles both compressed (.bck.gz) and uncompressed (.db) backup files.
func verifyBackup(t *testing.T, backupPath string, expectData bool, isCompressed bool) {
	t.Helper()

	dbPath := backupPath
	if isCompressed {
		// Decompress the backup file
		gzFile, err := os.Open(backupPath)
		if err != nil {
			t.Fatalf("Failed to open gzipped backup file: %v", err)
		}
		defer func() {
			if err := gzFile.Close(); err != nil {
				t.Logf("Failed to close gzipped backup file: %v", err)
			}
		}()

		gzReader, err := gzip.NewReader(gzFile)
		if err != nil {
			t.Fatalf("Failed to create gzip reader: %v", err)
		}
		defer func() {
			if err := gzReader.Close(); err != nil {
				t.Logf("Failed to close gzip reader: %v", err)
			}
		}()

		dbPath = backupPath + ".db"
		destFile, err := os.Create(dbPath)
		if err != nil {
			t.Fatalf("Failed to create decompressed destination file: %v", err)
		}
		defer func() {
			if err := destFile.Close(); err != nil {
				t.Logf("Failed to close decompressed destination file: %v", err)
			}
		}()

		if _, err := io.Copy(destFile, gzReader); err != nil {
			t.Fatalf("Failed to decompress file: %v", err)
		}
	}

	// Verify the contents of the database
	conn, err := zombiezen.NewConn(dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer func() {
		if err := conn.Close(); err != nil {
			t.Logf("Failed to close database connection: %v", err)
		}
	}()

	var count int
	err = sqlitex.Execute(conn, "SELECT count(*) FROM users", &sqlitex.ExecOptions{
		ResultFunc: func(stmt *sqlite.Stmt) error {
			count = stmt.ColumnInt(0)
			return nil
		},
	})
	if err != nil {
		t.Fatalf("Failed to query database: %v", err)
	}

	if expectData && count == 0 {
		t.Error("Expected data in backup, but users table is empty")
	}
	if !expectData && count > 0 {
		t.Errorf("Expected empty backup, but found %d users", count)
	}
}

func TestBackupHandler_Handle_SingleDB(t *testing.T) {
	mockTime := time.Date(2025, 8, 1, 10, 30, 0, 0, time.UTC)

	testCases := []struct {
		name        string
		strategy    string
		compression bool
	}{
		{"OnlineCompressed", config.BackupStrategyOnline, true},
		{"OnlineUncompressed", config.BackupStrategyOnline, false},
		{"VacuumCompressed", config.BackupStrategyVacuum, true},
		{"VacuumUncompressed", config.BackupStrategyVacuum, false},
		{"DefaultStrategyCompressed", "", true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg, sourcePath, backupDir := setupTest(t, true)
			addDatabase(cfg, "source", sourcePath, tc.compression, tc.strategy, "24h")

			provider := config.NewProvider(cfg)
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			handler := NewHandler(provider, logger)

			err := handler.handle(context.Background(), mockTime)
			if err != nil {
				t.Fatalf("handle() error = %v, want nil", err)
			}

			// Verify the backup file exists
			dbName := "source-source.db"
			timestamp := mockTime.UTC().Format(timestampFormat)
			var expectedPath string
			isCompressed := tc.compression
			if isCompressed {
				expectedPath = filepath.Join(backupDir, fmt.Sprintf(backupCompressedFmt, dbName, timestamp))
			} else {
				expectedPath = filepath.Join(backupDir, fmt.Sprintf(backupFmt, dbName, timestamp))
			}
			if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
				t.Fatalf("Expected backup file not found at %s", expectedPath)
			}
			verifyBackup(t, expectedPath, true, isCompressed)

			// Check latest link presence
			latestPath := filepath.Join(backupDir, fmt.Sprintf(backup.LatestFmt, dbName))
			if isCompressed {
				if _, err := os.Stat(latestPath); !os.IsNotExist(err) {
					t.Fatalf("Unexpected latest link for compressed backup at %s", latestPath)
				}
			} else {
				if _, err := os.Stat(latestPath); os.IsNotExist(err) {
					t.Fatalf("Expected latest link not found at %s", latestPath)
				}
			}
		})
	}
}

func TestBackupHandler_Handle_MultiDB(t *testing.T) {
	mockTime := time.Date(2025, 8, 1, 10, 30, 0, 0, time.UTC)

	// Create first database (source.db) via setupTest
	cfg, sourcePath, backupDir := setupTest(t, true)

	// Create a second database (second.db) in the same temp directory
	secondDbPath := filepath.Join(filepath.Dir(sourcePath), "second.db")
	conn, err := zombiezen.NewConn(secondDbPath)
	if err != nil {
		t.Fatalf("Failed to open second db connection: %v", err)
	}
	schemaFS := migrations.Schema()
	err = fs.WalkDir(schemaFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		sqlBytes, err := fs.ReadFile(schemaFS, path)
		if err != nil {
			return fmt.Errorf("failed to read migration %s: %w", path, err)
		}
		if err := sqlitex.ExecuteScript(conn, string(sqlBytes), nil); err != nil {
			return fmt.Errorf("failed to execute migration %s: %w", path, err)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Failed to apply migrations to second db: %v", err)
	}
	// Insert test data into second database
	err = sqlitex.Execute(conn, "INSERT INTO users (name, email) VALUES ('second-user', 'second@example.com');", nil)
	if err != nil {
		t.Fatalf("Failed to insert test data into second db: %v", err)
	}
	if err := conn.Close(); err != nil {
		t.Logf("Failed to close second db connection: %v", err)
	}

	// Add both databases: first uncompressed (online), second compressed (vacuum)
	addDatabase(cfg, "first", sourcePath, false, config.BackupStrategyOnline, "24h")
	addDatabase(cfg, "second", secondDbPath, true, config.BackupStrategyVacuum, "24h")

	provider := config.NewProvider(cfg)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := NewHandler(provider, logger)

	err = handler.handle(context.Background(), mockTime)
	if err != nil {
		t.Fatalf("handle() error = %v, want nil", err)
	}

	// Verify the uncompressed backup file exists and has a latest link
	timestamp := mockTime.UTC().Format(timestampFormat)
	uncompressedPath := filepath.Join(backupDir, fmt.Sprintf(backupFmt, "first-source.db", timestamp))
	if _, err := os.Stat(uncompressedPath); os.IsNotExist(err) {
		t.Fatalf("Expected uncompressed backup not found at %s", uncompressedPath)
	}
	verifyBackup(t, uncompressedPath, true, false)

	latestPath := filepath.Join(backupDir, fmt.Sprintf(backup.LatestFmt, "first-source.db"))
	if _, err := os.Stat(latestPath); os.IsNotExist(err) {
		t.Fatalf("Expected latest link not found at %s", latestPath)
	}

	// Verify the compressed backup file exists but has no latest link
	compressedPath := filepath.Join(backupDir, fmt.Sprintf(backupCompressedFmt, "second-second.db", timestamp))
	if _, err := os.Stat(compressedPath); os.IsNotExist(err) {
		t.Fatalf("Expected compressed backup not found at %s", compressedPath)
	}
	verifyBackup(t, compressedPath, true, true)

	compressedLatest := filepath.Join(backupDir, fmt.Sprintf(backup.LatestFmt, "second-second.db"))
	if _, err := os.Stat(compressedLatest); !os.IsNotExist(err) {
		t.Fatalf("Unexpected latest link for compressed backup at %s", compressedLatest)
	}
}

func TestBackupHandler_Handle_NoFiles(t *testing.T) {
	cfg, _, _ := setupTest(t, true) // backupDir is set, but no files added
	provider := config.NewProvider(cfg)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := NewHandler(provider, logger)
	err := handler.handle(context.Background(), time.Date(2025, 8, 1, 10, 30, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("handle() with no files should not error, got: %v", err)
	}
}

func TestBackupHandler_Handle_Deactivated(t *testing.T) {
	cfg, sourcePath, _ := setupTest(t, true)
	cfg.BackupLocal.BackupDir = "" // zero value deactivates the feature
	addDatabase(cfg, "source", sourcePath, false, config.BackupStrategyOnline, "24h")

	provider := config.NewProvider(cfg)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := NewHandler(provider, logger)

	err := handler.handle(context.Background(), time.Date(2025, 8, 1, 10, 30, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("handle() with empty backup_dir should not error, got: %v", err)
	}
}

func TestBackupHandler_Handle_EmptySourcePathSkipped(t *testing.T) {
	mockTime := time.Date(2025, 8, 1, 10, 30, 0, 0, time.UTC)
	cfg, sourcePath, backupDir := setupTest(t, true)
	addDatabase(cfg, "active", sourcePath, false, config.BackupStrategyOnline, "24h")
	cfg.BackupLocal.Files["deactivated"] = config.BackupLocalDbFile{
		SourcePath: "",
		Frequency:  config.Duration{Duration: 24 * time.Hour},
	}

	provider := config.NewProvider(cfg)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := NewHandler(provider, logger)

	err := handler.handle(context.Background(), mockTime)
	if err != nil {
		t.Fatalf("handle() with a deactivated entry should not error, got: %v", err)
	}

	// The active entry is backed up; the deactivated entry produces nothing.
	timestamp := mockTime.UTC().Format(timestampFormat)
	activePath := filepath.Join(backupDir, fmt.Sprintf(backupFmt, "active-source.db", timestamp))
	if _, err := os.Stat(activePath); os.IsNotExist(err) {
		t.Fatalf("expected active backup not found at %s", activePath)
	}
	entries, readErr := os.ReadDir(backupDir)
	if readErr != nil {
		t.Fatalf("failed to read backup dir: %v", readErr)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "deactivated-") {
			t.Fatalf("unexpected artifact for deactivated entry: %s", e.Name())
		}
	}
}

func TestBackupHandler_Handle_BackupDirNotADirectory(t *testing.T) {
	mockTime := time.Date(2025, 8, 1, 10, 30, 0, 0, time.UTC)
	cfg, sourcePath, _ := setupTest(t, true)
	// Point backup_dir at an existing file instead of a directory.
	notADir := filepath.Join(filepath.Dir(sourcePath), "not-a-dir")
	if err := os.WriteFile(notADir, []byte("x"), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}
	cfg.BackupLocal.BackupDir = notADir
	addDatabase(cfg, "source", sourcePath, false, config.BackupStrategyOnline, "24h")

	provider := config.NewProvider(cfg)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := NewHandler(provider, logger)

	err := handler.handle(context.Background(), mockTime)
	if err == nil {
		t.Fatal("handle() expected an error for backup_dir being a file, got nil")
	}
}

func TestBackupHandler_Handle_FrequencyRespected(t *testing.T) {
	cfg, sourcePath, backupDir := setupTest(t, true)
	addDatabase(cfg, "source", sourcePath, false, config.BackupStrategyOnline, "2h") // frequency: 2 hours

	provider := config.NewProvider(cfg)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := NewHandler(provider, logger)

	// First backup at T=0
	t0 := time.Date(2025, 8, 1, 10, 0, 0, 0, time.UTC)
	if err := handler.handle(context.Background(), t0); err != nil {
		t.Fatalf("first backup failed: %v", err)
	}

	// Second attempt at T+30min — should be skipped (not due yet)
	t1 := t0.Add(30 * time.Minute)
	if err := handler.handle(context.Background(), t1); err != nil {
		t.Fatalf("second backup (skipped) failed: %v", err)
	}

	// Third attempt at T+2h — should run (due)
	t2 := t0.Add(2 * time.Hour)
	if err := handler.handle(context.Background(), t2); err != nil {
		t.Fatalf("third backup failed: %v", err)
	}

	// Verify only two backup files exist (first and third)
	dbName := "source-source.db"
	timestamp0 := t0.UTC().Format(timestampFormat)
	timestamp2 := t2.UTC().Format(timestampFormat)

	backup1 := filepath.Join(backupDir, fmt.Sprintf(backupFmt, dbName, timestamp0))
	backup2 := filepath.Join(backupDir, fmt.Sprintf(backupFmt, dbName, timestamp2))

	if _, err := os.Stat(backup1); os.IsNotExist(err) {
		t.Fatalf("Expected backup 1 not found at %s", backup1)
	}
	if _, err := os.Stat(backup2); os.IsNotExist(err) {
		t.Fatalf("Expected backup 2 not found at %s", backup2)
	}

	// Count total backup files (should be exactly 2)
	timestamp1 := t1.UTC().Format(timestampFormat)
	skippedPath := filepath.Join(backupDir, fmt.Sprintf(backupFmt, dbName, timestamp1))
	if _, err := os.Stat(skippedPath); !os.IsNotExist(err) {
		t.Fatalf("Unexpected backup file for skipped attempt at %s", skippedPath)
	}

	// Latest link should be a hardlink to the third backup (same inode)
	latestPath := filepath.Join(backupDir, fmt.Sprintf(backup.LatestFmt, dbName))
	fiBackup, err := os.Stat(backup2)
	if err != nil {
		t.Fatalf("Failed to stat backup2: %v", err)
	}
	fiLink, err := os.Stat(latestPath)
	if err != nil {
		t.Fatalf("Expected latest link not found at %s: %v", latestPath, err)
	}
	if !os.SameFile(fiBackup, fiLink) {
		t.Fatal("Latest link does not share the same inode as the most recent backup")
	}
}

func TestBackupHandler_Handle_ErrorCases(t *testing.T) {
	mockTime := time.Date(2025, 8, 1, 10, 30, 0, 0, time.UTC)

	t.Run("SourceNotFound", func(t *testing.T) {
		cfg, _, backupDir := setupTest(t, true)
		addDatabase(cfg, "source", "/path/to/nonexistent/source.db", false, config.BackupStrategyOnline, "24h")
		cfg.BackupLocal.BackupDir = backupDir
		provider := config.NewProvider(cfg)
		logger := slog.New(slog.NewTextHandler(io.Discard, nil))
		handler := NewHandler(provider, logger)

		err := handler.handle(context.Background(), mockTime)
		if err == nil {
			t.Fatal("handle() expected an error, but got nil")
		}
	})

	t.Run("BackupDirNotWritable", func(t *testing.T) {
		cfg, sourcePath, backupDir := setupTest(t, true)
		addDatabase(cfg, "source", sourcePath, false, config.BackupStrategyOnline, "24h")
		// Make the backup directory read-only
		if err := os.Chmod(backupDir, 0400); err != nil {
			t.Fatalf("Failed to make backup dir read-only: %v", err)
		}

		provider := config.NewProvider(cfg)
		logger := slog.New(slog.NewTextHandler(io.Discard, nil))
		handler := NewHandler(provider, logger)

		err := handler.handle(context.Background(), mockTime)
		if err == nil {
			t.Fatal("handle() expected an error for non-writable dir, but got nil")
		}
	})
}

func TestBackupHandler_Handle_EmptyDatabase(t *testing.T) {
	cfg, sourcePath, backupDir := setupTest(t, false) // false -> don't add data
	addDatabase(cfg, "source", sourcePath, false, config.BackupStrategyOnline, "24h")
	provider := config.NewProvider(cfg)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := NewHandler(provider, logger)

	mockTime := time.Date(2025, 8, 1, 10, 30, 0, 0, time.UTC)

	err := handler.handle(context.Background(), mockTime)
	if err != nil {
		t.Fatalf("handle() with empty db error = %v, want nil", err)
	}

	timestamp := mockTime.UTC().Format(timestampFormat)
	expectedPath := filepath.Join(backupDir, fmt.Sprintf(backupFmt, "source-source.db", timestamp))

	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Fatalf("Expected backup file not found at %s", expectedPath)
	}

	verifyBackup(t, expectedPath, false, false)
}

func TestBackupHandler_Handle_EmptySource(t *testing.T) {
	mockTime := time.Date(2025, 8, 1, 10, 30, 0, 0, time.UTC)
	tempDir := t.TempDir()
	sourcePath := filepath.Join(tempDir, "empty.db")
	if err := os.WriteFile(sourcePath, nil, 0644); err != nil {
		t.Fatalf("failed to create empty source file: %v", err)
	}
	backupDir := filepath.Join(tempDir, "backups")
	if err := os.Mkdir(backupDir, 0755); err != nil {
		t.Fatalf("failed to create backup dir: %v", err)
	}

	cfg := config.NewDefaultConfig()
	cfg.BackupLocal.BackupDir = backupDir
	cfg.BackupLocal.Files = map[string]config.BackupLocalDbFile{
		"source": {
			SourcePath: sourcePath,
			Frequency:  config.Duration{Duration: 24 * time.Hour},
		},
	}

	provider := config.NewProvider(cfg)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := NewHandler(provider, logger)

	err := handler.handle(context.Background(), mockTime)
	if err != nil {
		t.Fatalf("handle() with empty source db error = %v, want nil", err)
	}

	timestamp := mockTime.UTC().Format(timestampFormat)
	expectedPath := filepath.Join(backupDir, fmt.Sprintf(backupFmt, "source-empty.db", timestamp))
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Fatalf("Expected backup file not found at %s", expectedPath)
	}
}

func TestBackupHandler_Handle_NotADatabaseFile(t *testing.T) {
	mockTime := time.Date(2025, 8, 1, 10, 30, 0, 0, time.UTC)
	tempDir := t.TempDir()
	sourcePath := filepath.Join(tempDir, "garbage.db")
	if err := os.WriteFile(sourcePath, []byte("this is not a sqlite database file"), 0644); err != nil {
		t.Fatalf("failed to create non-database source file: %v", err)
	}
	backupDir := filepath.Join(tempDir, "backups")
	if err := os.Mkdir(backupDir, 0755); err != nil {
		t.Fatalf("failed to create backup dir: %v", err)
	}

	cfg := config.NewDefaultConfig()
	cfg.BackupLocal.BackupDir = backupDir
	cfg.BackupLocal.Files = map[string]config.BackupLocalDbFile{
		"source": {
			SourcePath: sourcePath,
			Frequency:  config.Duration{Duration: 24 * time.Hour},
		},
	}

	provider := config.NewProvider(cfg)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := NewHandler(provider, logger)

	err := handler.handle(context.Background(), mockTime)
	if err == nil {
		t.Fatal("handle() expected an error for a non-database source file, got nil")
	}

	entries, readErr := os.ReadDir(backupDir)
	if readErr != nil {
		t.Fatalf("failed to read backup dir: %v", readErr)
	}
	if len(entries) != 0 {
		t.Fatalf("expected no backup files for a non-database source, found %d", len(entries))
	}
}

func TestBackupHandler_Handle_MissingSourceFile(t *testing.T) {
	mockTime := time.Date(2025, 8, 1, 10, 30, 0, 0, time.UTC)
	tempDir := t.TempDir()
	backupDir := filepath.Join(tempDir, "backups")
	if err := os.Mkdir(backupDir, 0755); err != nil {
		t.Fatalf("failed to create backup dir: %v", err)
	}

	cfg := config.NewDefaultConfig()
	cfg.BackupLocal.BackupDir = backupDir
	cfg.BackupLocal.Files = map[string]config.BackupLocalDbFile{
		"source": {
			SourcePath: filepath.Join(tempDir, "missing.db"),
			Frequency:  config.Duration{Duration: 24 * time.Hour},
		},
	}

	provider := config.NewProvider(cfg)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := NewHandler(provider, logger)

	err := handler.handle(context.Background(), mockTime)
	if err == nil {
		t.Fatal("handle() expected an error for a missing source file, got nil")
	}

	entries, readErr := os.ReadDir(backupDir)
	if readErr != nil {
		t.Fatalf("failed to read backup dir: %v", readErr)
	}
	if len(entries) != 0 {
		t.Fatalf("expected no backup files for missing source, found %d", len(entries))
	}
}

func TestBuildCompressedPath(t *testing.T) {
	mockTime := time.Date(2025, 8, 1, 10, 30, 0, 0, time.UTC)
	backupDir := "/tmp/backups"
	dbName := "app.db"

	handler := &Handler{}
	path := handler.buildCompressedPath(dbName, mockTime, backupDir)

	expected := filepath.Join(backupDir, "app.db-20250801T103000Z.bck.gz")
	if path != expected {
		t.Fatalf("buildCompressedPath() = %q, want %q", path, expected)
	}
}

func TestBuildUncompressedPath(t *testing.T) {
	mockTime := time.Date(2025, 8, 1, 10, 30, 0, 0, time.UTC)
	backupDir := "/tmp/backups"
	dbName := "app.db"

	handler := &Handler{}
	path := handler.buildUncompressedPath(dbName, mockTime, backupDir)

	expected := filepath.Join(backupDir, "app.db-20250801T103000Z.db")
	if path != expected {
		t.Fatalf("buildUncompressedPath() = %q, want %q", path, expected)
	}
}

func TestBuildTempPath(t *testing.T) {
	mockTime := time.Date(2025, 8, 1, 10, 30, 0, 0, time.UTC)
	dbName := "app.db"

	handler := &Handler{}
	path := handler.buildTempPath(dbName, mockTime)

	prefix := filepath.Join(os.TempDir(), "backup-app.db-")
	if !strings.HasPrefix(path, prefix) {
		t.Fatalf("buildTempPath() = %q, want prefix %q", path, prefix)
	}
	if !strings.HasSuffix(path, ".db") {
		t.Fatalf("buildTempPath() = %q, want suffix '.db'", path)
	}
}

func TestParseBackupTimestamp(t *testing.T) {
	handler := &Handler{}

	tests := []struct {
		name     string
		filename string
		dbName   string
		wantOK   bool
		wantTS   string // empty if !wantOK
	}{
		{
			name:     "valid compressed",
			filename: "app.db-20250801T103000Z.bck.gz",
			dbName:   "app.db",
			wantOK:   true,
			wantTS:   "2025-08-01T10:30:00Z",
		},
		{
			name:     "valid uncompressed",
			filename: "app.db-20250801T103000Z.db",
			dbName:   "app.db",
			wantOK:   true,
			wantTS:   "2025-08-01T10:30:00Z",
		},
		{
			name:     "wrong prefix",
			filename: "other.db-20250801T103000Z.bck.gz",
			dbName:   "app.db",
			wantOK:   false,
		},
		{
			name:     "stale tmp file",
			filename: "app.db-20250801T103000Z.bck.gz.tmp",
			dbName:   "app.db",
			wantOK:   false,
		},
		{
			name:     "too short timestamp",
			filename: "app.db-20250801.bck.gz",
			dbName:   "app.db",
			wantOK:   false,
		},
		{
			name:     "invalid date characters",
			filename: "app.db-20ABCD01T103000Z.bck.gz",
			dbName:   "app.db",
			wantOK:   false,
		},
		{
			name:     "latest link ignored",
			filename: "latest-app.db",
			dbName:   "app.db",
			wantOK:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts, ok := handler.parseBackupTimestamp(tt.filename, tt.dbName)
			if ok != tt.wantOK {
				t.Fatalf("parseBackupTimestamp() ok = %v, want %v", ok, tt.wantOK)
			}
			if tt.wantOK {
				expectedTS, _ := time.Parse(time.RFC3339, tt.wantTS)
				if !ts.Equal(expectedTS) {
					t.Fatalf("parseBackupTimestamp() ts = %v, want %v", ts, expectedTS)
				}
			}
		})
	}
}

// TestNewHandler_Panic verifies that NewHandler panics when given a nil provider or logger.
func TestNewHandler_Panic(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	provider := config.NewProvider(config.NewDefaultConfig())

	t.Run("nil provider", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Fatal("expected panic for nil provider, got none")
			}
		}()
		NewHandler(nil, logger)
	})

	t.Run("nil logger", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Fatal("expected panic for nil logger, got none")
			}
		}()
		NewHandler(provider, nil)
	})
}

// TestBackupHandler_Handle_Wrapper verifies that the public Handle method
// works without error (uses time.Now() internally).
func TestBackupHandler_Handle_Wrapper(t *testing.T) {
	cfg, sourcePath, backupDir := setupTest(t, true)
	addDatabase(cfg, "source", sourcePath, false, config.BackupStrategyOnline, "24h")

	provider := config.NewProvider(cfg)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := NewHandler(provider, logger)

	// Use the public Handle method instead of handle
	job := db.Job{}
	err := handler.Handle(context.Background(), job)
	if err != nil {
		t.Fatalf("Handle() error = %v, want nil", err)
	}

	// Verify some backup file was created (Handle uses time.Now(), so exact name unknown)
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		t.Fatalf("Failed to read backup dir: %v", err)
	}
	found := false
	for _, e := range entries {
		if !e.IsDir() && strings.HasPrefix(e.Name(), "source-source.db-") {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("Handle() did not create a backup file in backup dir")
	}
}

// TestCompressFile_SourceNotFound verifies compressFile returns an error
// when the source file does not exist.
func TestCompressFile_SourceNotFound(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := &Handler{logger: logger}

	err := handler.compressFile("/nonexistent/source.db", filepath.Join(t.TempDir(), "out.bck.gz"))
	if err == nil {
		t.Fatal("compressFile() expected error for missing source, got nil")
	}
}

// TestLinkLatest_SourceNotFound verifies linkLatest returns an error
// when the backup file does not exist.
func TestLinkLatest_SourceNotFound(t *testing.T) {
	handler := &Handler{}
	backupDir := t.TempDir()

	err := handler.linkLatest("/nonexistent/backup.db", filepath.Join(backupDir, "latest-link"))
	if err == nil {
		t.Fatal("linkLatest() expected error for missing source, got nil")
	}
}

// TestBackupHandler_Handle_CompressedError verifies error handling for
// compressed backup with non-existent source and read-only backup dir.
func TestBackupHandler_Handle_CompressedError(t *testing.T) {
	mockTime := time.Date(2025, 8, 1, 10, 30, 0, 0, time.UTC)

	t.Run("SourceMissing", func(t *testing.T) {
		cfg, _, backupDir := setupTest(t, true)
		addDatabase(cfg, "source", "/nonexistent/source.db", true, config.BackupStrategyOnline, "24h")
		cfg.BackupLocal.BackupDir = backupDir

		provider := config.NewProvider(cfg)
		logger := slog.New(slog.NewTextHandler(io.Discard, nil))
		handler := NewHandler(provider, logger)

		err := handler.handle(context.Background(), mockTime)
		if err == nil {
			t.Fatal("handle() expected error for missing source, got nil")
		}
	})

	t.Run("BackupDirNotWritable", func(t *testing.T) {
		cfg, sourcePath, backupDir := setupTest(t, true)
		addDatabase(cfg, "source", sourcePath, true, config.BackupStrategyOnline, "24h")
		if err := os.Chmod(backupDir, 0400); err != nil {
			t.Fatalf("Failed to make backup dir read-only: %v", err)
		}

		provider := config.NewProvider(cfg)
		logger := slog.New(slog.NewTextHandler(io.Discard, nil))
		handler := NewHandler(provider, logger)

		err := handler.handle(context.Background(), mockTime)
		if err == nil {
			t.Fatal("handle() expected error for read-only backup dir, got nil")
		}
	})
}

// TestModuloLogger_Log verifies the progress logger's log method is exercised
// by creating a database large enough for multi-step online backup.
func TestModuloLogger_Log(t *testing.T) {
	cfg, sourcePath, backupDir := setupTest(t, false) // empty DB schema first
	// Add enough data to fill multiple database pages
	conn, err := zombiezen.NewConn(sourcePath)
	if err != nil {
		t.Fatalf("Failed to open source db: %v", err)
	}
	// Insert 500 rows to create many pages (each page is 4096 bytes)
	for i := range 500 {
		name := fmt.Sprintf("user-%d", i)
		email := fmt.Sprintf("user%d@example.com", i)
		if sqlErr := sqlitex.Execute(conn, "INSERT INTO users (name, email) VALUES (?, ?);", &sqlitex.ExecOptions{
			Args: []any{name, email},
		}); sqlErr != nil {
			_ = conn.Close()
			t.Fatalf("Failed to insert test data row %d: %v", i, sqlErr)
		}
	}
	if err := conn.Close(); err != nil {
		t.Logf("Failed to close db connection: %v", err)
	}

	addDatabase(cfg, "source", sourcePath, false, config.BackupStrategyOnline, "24h")
	cfg.BackupLocal.OnlinePagesPerStep = 1 // force many small steps

	provider := config.NewProvider(cfg)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := NewHandler(provider, logger)

	mockTime := time.Date(2025, 8, 1, 10, 30, 0, 0, time.UTC)
	err = handler.handle(context.Background(), mockTime)
	if err != nil {
		t.Fatalf("handle() with large db error = %v, want nil", err)
	}

	// Verify backup exists
	timestamp := mockTime.UTC().Format(timestampFormat)
	expectedPath := filepath.Join(backupDir, fmt.Sprintf(backupFmt, "source-source.db", timestamp))
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Fatalf("Expected backup file not found at %s", expectedPath)
	}
}
