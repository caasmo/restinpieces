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
	"testing"
	"time"

	"github.com/caasmo/restinpieces/config"
	"github.com/caasmo/restinpieces/db"
	"github.com/caasmo/restinpieces/db/zombiezen"
	"github.com/caasmo/restinpieces/migrations"
	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"
)

// setupTest creates a temporary directory, a source database with the full schema
// and some data, and a config provider pointing to the temporary paths.
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
	defer conn.Close()

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
	cfg.BackupLocal.SourcePath = sourceDbPath
	cfg.BackupLocal.BackupDir = backupDir
	cfg.BackupLocal.Strategy = StrategyOnline // Default strategy

	return cfg, sourceDbPath, backupDir
}

// verifyBackup decompresses a gzipped backup file and checks if it's a valid, non-empty SQLite database.
func verifyBackup(t *testing.T, backupPath string, expectData bool) {
	t.Helper()

	// Decompress the backup file
	gzFile, err := os.Open(backupPath)
	if err != nil {
		t.Fatalf("Failed to open gzipped backup file: %v", err)
	}
	defer gzFile.Close()

	gzReader, err := gzip.NewReader(gzFile)
	if err != nil {
		t.Fatalf("Failed to create gzip reader: %v", err)
	}
	defer gzReader.Close()

	decompressedPath := backupPath + ".db"
	destFile, err := os.Create(decompressedPath)
	if err != nil {
		t.Fatalf("Failed to create decompressed destination file: %v", err)
	}
	defer destFile.Close()

	if _, err := io.Copy(destFile, gzReader); err != nil {
		t.Fatalf("Failed to decompress file: %v", err)
	}

	// Verify the contents of the decompressed database
	conn, err := zombiezen.NewConn(decompressedPath)
	if err != nil {
		t.Fatalf("Failed to open decompressed database: %v", err)
	}
	defer conn.Close()

	var count int
	err = sqlitex.Execute(conn, "SELECT count(*) FROM users", &sqlitex.ExecOptions{
		ResultFunc: func(stmt *sqlite.Stmt) error {
			count = stmt.ColumnInt(0)
			return nil
		},
	})
	if err != nil {
		t.Fatalf("Failed to query decompressed database: %v", err)
	}

	if expectData && count == 0 {
		t.Error("Expected data in backup, but users table is empty")
	}
	if !expectData && count > 0 {
		t.Errorf("Expected empty backup, but found %d users", count)
	}
}

func TestBackupHandler_Handle_HappyPaths(t *testing.T) {
	mockTime := time.Date(2025, 8, 1, 10, 30, 0, 0, time.UTC)
	job := db.Job{} // Job payload is not used by this handler

	testCases := []struct {
		name               string
		strategy           string
		expectedFileSuffix string
	}{
		{"OnlineStrategy", StrategyOnline, "-online.bck.gz"},
		{"VacuumStrategy", StrategyVacuum, "-vacuum.bck.gz"},
		{"DefaultStrategy", "", "-online.bck.gz"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg, _, backupDir := setupTest(t, true)
			cfg.BackupLocal.Strategy = tc.strategy

			provider := config.NewProvider(cfg)
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			handler := NewHandler(provider, logger)

			err := handler.handle(context.Background(), job, mockTime)
			if err != nil {
				t.Fatalf("handle() error = %v, want nil", err)
			}

			expectedFilename := "source-20250801T103000Z" + tc.expectedFileSuffix
			expectedPath := filepath.Join(backupDir, expectedFilename)

			if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
				t.Fatalf("Expected backup file not found at %s", expectedPath)
			}

			verifyBackup(t, expectedPath, true)

			tempBackupPath := filepath.Join(os.TempDir(), fmt.Sprintf("backup-%d.db", mockTime.UnixNano()))
			if _, err := os.Stat(tempBackupPath); !os.IsNotExist(err) {
				t.Errorf("Expected temporary backup file to be removed, but it still exists at %s", tempBackupPath)
			}
		})
	}
}

func TestBackupHandler_Handle_ErrorCases(t *testing.T) {
	mockTime := time.Date(2025, 8, 1, 10, 30, 0, 0, time.UTC)
	job := db.Job{}

	t.Run("UnknownStrategy", func(t *testing.T) {
		cfg, _, _ := setupTest(t, true)
		cfg.BackupLocal.Strategy = "invalid-strategy"
		provider := config.NewProvider(cfg)
		logger := slog.New(slog.NewTextHandler(io.Discard, nil))
		handler := NewHandler(provider, logger)

		err := handler.handle(context.Background(), job, mockTime)
		if err == nil {
			t.Fatal("handle() expected an error, but got nil")
		}
		if err.Error() != `unknown backup strategy: "invalid-strategy"` {
			t.Errorf("unexpected error message: got %q", err.Error())
		}
	})

	t.Run("SourceNotFound", func(t *testing.T) {
		cfg, _, _ := setupTest(t, true)
		cfg.BackupLocal.SourcePath = "/path/to/nonexistent/source.db"
		provider := config.NewProvider(cfg)
		logger := slog.New(slog.NewTextHandler(io.Discard, nil))
		handler := NewHandler(provider, logger)

		err := handler.handle(context.Background(), job, mockTime)
		if err == nil {
			t.Fatal("handle() expected an error, but got nil")
		}
	})

	t.Run("BackupDirNotWritable", func(t *testing.T) {
		cfg, _, backupDir := setupTest(t, true)
		// Make the backup directory read-only
		if err := os.Chmod(backupDir, 0400); err != nil {
			t.Fatalf("Failed to make backup dir read-only: %v", err)
		}

		provider := config.NewProvider(cfg)
		logger := slog.New(slog.NewTextHandler(io.Discard, nil))
		handler := NewHandler(provider, logger)

		err := handler.handle(context.Background(), job, mockTime)
		if err == nil {
			t.Fatal("handle() expected an error for non-writable dir, but got nil")
		}
	})
}

func TestBackupHandler_Handle_EmptyDatabase(t *testing.T) {
	cfg, _, backupDir := setupTest(t, false) // false -> don't add data
	provider := config.NewProvider(cfg)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := NewHandler(provider, logger)

	mockTime := time.Date(2025, 8, 1, 10, 30, 0, 0, time.UTC)
	job := db.Job{}

	err := handler.handle(context.Background(), job, mockTime)
	if err != nil {
		t.Fatalf("handle() with empty db error = %v, want nil", err)
	}

	expectedFilename := "source-20250801T103000Z-online.bck.gz"
	expectedPath := filepath.Join(backupDir, expectedFilename)

	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Fatalf("Expected backup file not found at %s", expectedPath)
	}

	verifyBackup(t, expectedPath, false)
}
