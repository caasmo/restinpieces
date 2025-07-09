package handlers

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/caasmo/restinpieces/config"
	"github.com/caasmo/restinpieces/db"
	"github.com/caasmo/restinpieces/db/zombiezen"
	"zombiezen.com/go/sqlite"
)

const (
	JobTypeBackupLocal = "job_type_backup_local"
	ScopeDbBackup      = "sqlite_backup"
	StrategyVacuum     = "vacuum"
	StrategyOnline     = "online"
)

// Handler handles database backup jobs
type Handler struct {
	configProvider *config.Provider
	logger         *slog.Logger
}

// NewHandler creates a new Handler
func NewHandler(provider *config.Provider, logger *slog.Logger) *Handler {
	if provider == nil || logger == nil {
		panic("NewHandler: received nil provider or logger")
	}
	return &Handler{
		configProvider: provider,
		logger:         logger.With("job_handler", "sqlite_backup"),
	}
}

// Handle implements the JobHandler interface for database backups
func (h *Handler) Handle(ctx context.Context, job db.Job) error {
	cfg := h.configProvider.Get()
	backupCfg := cfg.BackupLocal

	// --- Define Paths and Filenames ---
	sourceDbPath := backupCfg.SourcePath
	backupDir := backupCfg.BackupDir
	tempBackupPath := filepath.Join(os.TempDir(), fmt.Sprintf("backup-%d.db", time.Now().UnixNano()))

	strategyForFilename := backupCfg.Strategy
	if strategyForFilename == "" {
		strategyForFilename = StrategyOnline
	}

	baseName := filepath.Base(sourceDbPath)
	fileNameOnly := strings.TrimSuffix(baseName, filepath.Ext(baseName))
	timestamp := time.Now().UTC().Format("2006-01-02T15-04-05Z")
	finalBackupName := fmt.Sprintf("%s-%s-%s.bck.gz", fileNameOnly, timestamp, strategyForFilename)

	finalBackupPath := filepath.Join(backupDir, finalBackupName)

	h.logger.Info("Starting database backup process", "source", sourceDbPath, "strategy", backupCfg.Strategy, "destination", finalBackupPath)

	// --- Dispatch to the chosen backup strategy ---
	var backupErr error
	switch backupCfg.Strategy {
	case StrategyVacuum:
		backupErr = h.vacuumInto(sourceDbPath, tempBackupPath)
	case StrategyOnline, "":
		backupErr = h.onlineBackup(sourceDbPath, tempBackupPath)
	default:
		return fmt.Errorf("unknown backup strategy: %q", backupCfg.Strategy)
	}

	if backupErr != nil {
		return fmt.Errorf("backup creation failed: %w", backupErr)
	}
	defer func() {
		if err := os.Remove(tempBackupPath); err != nil {
			h.logger.Error("Error removing temporary backup file", "error", err)
		}
	}()
	h.logger.Info("Successfully created temporary backup database", "path", tempBackupPath)

	// --- Gzip and Finalize ---
	if err := h.compressFile(tempBackupPath, finalBackupPath); err != nil {
		return fmt.Errorf("failed to gzip backup file: %w", err)
	}
	h.logger.Info("Successfully compressed backup", "path", finalBackupPath)

	h.logger.Info("Database backup process completed successfully")
	return nil
}

// validateOnlineConfig checks if the configuration for the online strategy is valid.
func (h *Handler) validateOnlineConfig() error {
	backupCfg := h.configProvider.Get().BackupLocal
	if backupCfg.PagesPerStep <= 0 {
		return fmt.Errorf("invalid configuration for online backup: pages_per_step must be a positive value, but was %d", backupCfg.PagesPerStep)
	}
	if backupCfg.SleepInterval.Duration < 0 {
		return fmt.Errorf("invalid configuration for online backup: sleep_interval cannot be negative, but was %v", backupCfg.SleepInterval)
	}
	return nil
}

// vacuumInto creates a clean, defragmented copy of the database.
func (h *Handler) vacuumInto(sourcePath, destPath string) error {
	sourceConn, err := zombiezen.NewConn(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to open source db for vacuum: %w", err)
	}
	defer func() {
		if err := sourceConn.Close(); err != nil {
			h.logger.Error("Error closing source database connection", "error", err)
		}
	}()

	stmt, err := sourceConn.Prepare(fmt.Sprintf("VACUUM INTO '%s';", destPath))
	if err != nil {
		return fmt.Errorf("failed to prepare vacuum statement: %w", err)
	}
	defer stmt.Finalize()

	if _, err := stmt.Step(); err != nil {
		return fmt.Errorf("failed to execute vacuum statement: %w", err)
	}
	return nil
}

// onlineBackup performs a live backup using the SQLite Online Backup API.
func (h *Handler) onlineBackup(sourcePath, destPath string) error {
	if err := h.validateOnlineConfig(); err != nil {
		return err
	}

	backupCfg := h.configProvider.Get().BackupLocal
	pagesPerStep := backupCfg.PagesPerStep
	sleepInterval := backupCfg.SleepInterval.Duration

	srcConn, err := zombiezen.NewConn(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to open source db for online backup: %w", err)
	}
	defer func() {
		if err := srcConn.Close(); err != nil {
			h.logger.Error("Error closing source database connection", "error", err)
		}
	}()

	destConn, err := zombiezen.NewConn(destPath)
	if err != nil {
		return fmt.Errorf("failed to create destination db for online backup: %w", err)
	}
	defer func() {
		if err := destConn.Close(); err != nil {
			h.logger.Error("Error closing destination database connection", "error", err)
		}
	}()

	backup, err := sqlite.NewBackup(destConn, "main", srcConn, "main")
	if err != nil {
		return fmt.Errorf("failed to initialize backup: %w", err)
	}
	defer func() {
		if err := backup.Close(); err != nil {
			h.logger.Error("error closing backup resource", "error", err)
		}
	}()

	// Initialize the progress logger
	logger, err := newModuloLogger(h.logger, backup)
	if err != nil {
		return err
	}
	if logger == nil { // This happens if the database is empty
		h.logger.Info("Source database is empty. Backup completed immediately.")
		return nil
	}

	h.logger.Info("Starting online backup copy", "pages_per_step", pagesPerStep, "sleep_interval", sleepInterval, "total_pages", logger.totalPages)

	for {
		more, err := backup.Step(pagesPerStep)
		if err != nil {
			return fmt.Errorf("backup step failed: %w", err)
		}

		if !more {
			logger.LogFinal(backup)
			h.logger.Info("Online backup copy completed successfully.")
			return nil
		}

		logger.Log(backup)

		if sleepInterval > 0 {
			time.Sleep(sleepInterval)
		}
	}
}

// --- Modulo Logger ---

// moduloLogger encapsulates the logic for logging backup progress.
type moduloLogger struct {
	logger          *slog.Logger
	totalPages      int
	logPageInterval int
	nextLogTarget   int
}

// newModuloLogger creates and initializes a progress logger.
func newModuloLogger(logger *slog.Logger, backup *sqlite.Backup) (*moduloLogger, error) {
	if _, err := backup.Step(0); err != nil {
		return nil, fmt.Errorf("backup step(0) failed: %w", err)
	}
	totalPages := backup.PageCount()
	if totalPages == 0 {
		return nil, nil
	}

	const numLogPoints = 10
	logPageInterval := totalPages / numLogPoints
	if logPageInterval == 0 {
		logPageInterval = 1
	}

	return &moduloLogger{
		logger:          logger,
		totalPages:      totalPages,
		logPageInterval: logPageInterval,
		nextLogTarget:   logPageInterval,
	}, nil
}

// Log checks if the backup has progressed enough to warrant a log message.
func (m *moduloLogger) Log(backup *sqlite.Backup) {
	copiedPages := m.totalPages - backup.Remaining()
	if copiedPages >= m.nextLogTarget {
		m.log(backup)
		m.nextLogTarget += m.logPageInterval
	}
}

// LogFinal logs the final progress message.
func (m *moduloLogger) LogFinal(backup *sqlite.Backup) {
	m.log(backup)
}

// log is a private helper to format and write the progress log message.
func (m *moduloLogger) log(backup *sqlite.Backup) {
	copiedPages := m.totalPages - backup.Remaining()
	m.logger.Info("Online backup in progress",
		"pages_copied", copiedPages,
		"total_pages", m.totalPages,
	)
}

// --- Other Helpers ---

// compressFile reads a source file, compresses it with gzip, and writes to a destination file.
func (h *Handler) compressFile(sourcePath, destPath string) error {
	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to open source file for compression: %w", err)
	}
	defer func() {
		if err := sourceFile.Close(); err != nil {
			h.logger.Error("Error closing source file", "error", err)
		}
	}()

	destFile, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create destination file for compression: %w", err)
	}
	defer func() {
		if err := destFile.Close(); err != nil {
			h.logger.Error("Error closing destination file", "error", err)
		}
	}()

	gzipWriter := gzip.NewWriter(destFile)
	defer func() {
		if err := gzipWriter.Close(); err != nil {
			h.logger.Error("Error closing gzip writer", "error", err)
		}
	}()

	if _, err := io.Copy(gzipWriter, sourceFile); err != nil {
		return fmt.Errorf("failed to copy and compress data: %w", err)
	}

	return nil
}
