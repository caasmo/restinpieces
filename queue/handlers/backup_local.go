package handlers

import (
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/caasmo/restinpieces/backup"
	"github.com/caasmo/restinpieces/config"
	"github.com/caasmo/restinpieces/db"
	"github.com/caasmo/restinpieces/db/zombiezen"
	"zombiezen.com/go/sqlite"
)

const (
	JobTypeBackupLocal = "job_type_backup_local"
	ScopeDbBackup      = "sqlite_backup"

	// timestampFormat is the UTC timestamp layout used in backup filenames.
	// It is lexicographically sortable (chronological order equals string order).
	// Produces e.g. "20250801T103000Z".
	timestampFormat = "20060102T150405Z"

	// timestampLen is the fixed character length of timestampFormat.
	timestampLen = len("20060102T150405Z") // 16

	// backupCompressedFmt is the filename template for compressed (gzip) backups.
	// Example output: app.db-20250801T103000Z.bck.gz
	backupCompressedFmt = "%s-%s.bck.gz"

	// backupFmt is the filename template for uncompressed backups.
	// Example output: app.db-20250801T103000Z.db
	backupFmt = "%s-%s.db"
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

// Handle implements the JobHandler interface for database backups.
// It's a wrapper around the testable handle method.
func (h *Handler) Handle(ctx context.Context, _ db.Job) error {
	return h.handle(ctx, time.Now())
}

// handle contains the actual backup logic and is testable.
func (h *Handler) handle(ctx context.Context, now time.Time) error {
	cfg := h.configProvider.Get().BackupLocal

	// --- early exits ---
	if cfg.BackupDir == "" {
		h.logger.Info("No backup directory configured; backup deactivated.")
		return nil
	}
	if len(cfg.Files) == 0 {
		h.logger.Info("No backup files configured; nothing to do.")
		return nil
	}

	var errs []error
	times := h.latestBackupTimes(cfg.BackupDir, cfg.Files)

	for key, fileCfg := range cfg.Files {
		backupName := h.buildBackupName(key, fileCfg.SourcePath)

		// --- step 1: skip if not yet due ---
		if !h.isBackupDue(times[backupName], fileCfg.Frequency.Duration, now) {
			h.logger.Info("Skipping backup; not yet due", "db", backupName)
			continue
		}

		// --- step 2: run backup ---
		backupFn := h.onlineBackup // default
		if fileCfg.Strategy == config.BackupStrategyVacuum {
			backupFn = h.vacuumInto
		}

		var err error
		var finalPath string
		if fileCfg.Compression {
			finalPath = h.buildCompressedPath(backupName, now, cfg.BackupDir)
			tempPath := h.buildTempPath(backupName, now)
			tempFinalPath := finalPath + ".tmp" // same directory, os.Rename is atomic

			// --- 2a: dump to temp ---
			err = backupFn(fileCfg.SourcePath, tempPath)
			if err != nil {
				removeErr := os.Remove(tempPath)
				if removeErr != nil {
					h.logger.Error("Failed to remove temp file after failed backup", "path", tempPath, "error", removeErr)
				}
				errs = append(errs, fmt.Errorf("%q: %w", backupName, err))
				continue
			}

			// --- 2b: compress temp to .tmp in backupDir ---
			err = h.compressFile(tempPath, tempFinalPath)
			removeErr := os.Remove(tempPath)
			if removeErr != nil {
				h.logger.Error("Failed to remove temp file", "path", tempPath, "error", removeErr)
			}
			if err != nil {
				removeErr = os.Remove(tempFinalPath)
				if removeErr != nil {
					h.logger.Error("Failed to remove partial .tmp file", "path", tempFinalPath, "error", removeErr)
				}
				errs = append(errs, fmt.Errorf("%q: %w", backupName, err))
				continue
			}

			// --- 2c: atomic promote ---
			err = os.Rename(tempFinalPath, finalPath)
			if err != nil {
				removeErr = os.Remove(tempFinalPath)
				if removeErr != nil {
					h.logger.Error("Failed to remove .tmp file after failed rename", "path", tempFinalPath, "error", removeErr)
				}
				errs = append(errs, fmt.Errorf("%q: %w", backupName, err))
				continue
			}
		} else {
			finalPath = h.buildUncompressedPath(backupName, now, cfg.BackupDir)
			tempFinalPath := finalPath + ".tmp" // same directory, os.Rename is atomic

			// --- 2a: dump to .tmp in backupDir ---
			err = backupFn(fileCfg.SourcePath, tempFinalPath)
			if err != nil {
				removeErr := os.Remove(tempFinalPath)
				if removeErr != nil {
					h.logger.Error("Failed to remove .tmp file after failed backup", "path", tempFinalPath, "error", removeErr)
				}
				errs = append(errs, fmt.Errorf("%q: %w", backupName, err))
				continue
			}

			// --- 2b: atomic promote ---
			err = os.Rename(tempFinalPath, finalPath)
			if err != nil {
				removeErr := os.Remove(tempFinalPath)
				if removeErr != nil {
					h.logger.Error("Failed to remove .tmp file after failed rename", "path", tempFinalPath, "error", removeErr)
				}
				errs = append(errs, fmt.Errorf("%q: %w", backupName, err))
				continue
			}

			// --- 2c: update latest link ---
			// Uncompressed only — the rsync pull client needs a stable
			// filename to sync. Compressed .bck.gz is not consumable as-is,
			// so no link is created for it.
			latestPath := h.buildLatestPath(cfg.BackupDir, backupName)
			err = h.linkLatest(finalPath, latestPath)
			if err != nil {
				errs = append(errs, fmt.Errorf("%q: %w", backupName, err))
			}
		}
	}
	return errors.Join(errs...)
}

// buildBackupName returns the prefix used in backup filenames and hardlinks.
// Produces <key>-<basename> so same-basename source paths do not collide
// (AGENTS.md: map keys are labels, not identifiers).
//
// Example: buildBackupName("app_db", "data/app.db") → "app_db-app.db"
func (h *Handler) buildBackupName(key, sourcePath string) string {
	return key + "-" + filepath.Base(sourcePath)
}

// buildCompressedPath returns the final destination path for a compressed
// backup inside backupDir. Produces e.g. "backupDir/app.db-20250801T103000Z.bck.gz".
func (h *Handler) buildCompressedPath(dbName string, now time.Time, backupDir string) string {
	timestamp := now.UTC().Format(timestampFormat)
	filename := fmt.Sprintf(backupCompressedFmt, dbName, timestamp)
	return filepath.Join(backupDir, filename)
}

// buildUncompressedPath returns the final destination path for an uncompressed
// backup inside backupDir. Produces e.g. "backupDir/app.db-20250801T103000Z.db".
func (h *Handler) buildUncompressedPath(dbName string, now time.Time, backupDir string) string {
	timestamp := now.UTC().Format(timestampFormat)
	filename := fmt.Sprintf(backupFmt, dbName, timestamp)
	return filepath.Join(backupDir, filename)
}

// buildTempPath returns a unique staging path in os.TempDir for the
// database dump before compression. Produces e.g. "/tmp/backup-app.db-1234567890.db".
func (h *Handler) buildTempPath(dbName string, now time.Time) string {
	return filepath.Join(os.TempDir(), fmt.Sprintf("backup-%s-%d.db", dbName, now.UnixNano()))
}

// buildLatestPath constructs the stable hardlink path for a dbName.
// Uses the shared backup.LatestFmt convention so clients can discover it.
func (h *Handler) buildLatestPath(backupDir, dbName string) string {
	return filepath.Join(backupDir, fmt.Sprintf(backup.LatestFmt, dbName))
}

// latestBackupTimes scans backupDir once and returns the most recent
// timestamp for each backupName derived from the configured source files.
// Errors are logged internally; an empty map is returned when the directory
// is absent or unreadable (all backups will be treated as due).
func (h *Handler) latestBackupTimes(backupDir string, files map[string]config.BackupLocalDbFile) map[string]time.Time {
	backupNames := make([]string, 0, len(files))
	for key, f := range files {
		backupNames = append(backupNames, h.buildBackupName(key, f.SourcePath))
	}
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		if !os.IsNotExist(err) {
			h.logger.Warn("Failed to scan backup directory", "error", err)
		}
		return map[string]time.Time{}
	}
	times := make(map[string]time.Time, len(backupNames))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		for _, bn := range backupNames {
			ts, ok := h.parseBackupTimestamp(name, bn)
			if ok && ts.After(times[bn]) {
				times[bn] = ts
				break
			}
		}
	}
	return times
}

// isBackupDue returns true if frequency has elapsed since latestTime.
// A zero latestTime means no previous backup exists (always due).
func (h *Handler) isBackupDue(latestTime time.Time, frequency time.Duration, now time.Time) bool {
	if latestTime.IsZero() {
		return true
	}
	return now.Sub(latestTime) >= frequency
}

// parseBackupTimestamp extracts the UTC timestamp from a backup filename.
// Filenames follow the pattern {dbName}-{YYYYMMDDTHHMMSSZ}{ext}.
// Filenames ending in .tmp are rejected to prevent stale artifacts from
// being mistaken for valid backups.
func (h *Handler) parseBackupTimestamp(filename, dbName string) (time.Time, bool) {
	if strings.HasSuffix(filename, ".tmp") {
		return time.Time{}, false
	}
	prefix := dbName + "-"
	rest, ok := strings.CutPrefix(filename, prefix)
	if !ok || len(rest) < timestampLen {
		return time.Time{}, false
	}
	ts, err := time.Parse(timestampFormat, rest[:timestampLen])
	return ts, err == nil
}

// linkLatest atomically replaces latestPath to point at the backup file.
//
// Uses link(2) + rename(2) per POSIX.1-2024: os.Link fails when the
// target already exists, so the new hardlink is created under a temp
// name first. os.Rename then atomically replaces latestPath — clients
// always observe a valid link, never ENOENT.
//
// The os.Remove handles stale .tmp left by a prior crash between
// link and rename.
func (h *Handler) linkLatest(backupPath, latestPath string) error {
	tmp := latestPath + ".tmp"
	_ = os.Remove(tmp) // crash recovery, ignore "not found"
	if err := os.Link(backupPath, tmp); err != nil {
		return fmt.Errorf("linkLatest: link: %w", err)
	}
	return os.Rename(tmp, latestPath)
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
	defer func() {
		if err := stmt.Finalize(); err != nil {
			h.logger.Error("Error finalizing vacuum statement", "error", err)
		}
	}()

	if _, err := stmt.Step(); err != nil {
		return fmt.Errorf("failed to execute vacuum statement: %w", err)
	}
	return nil
}

// onlineBackup performs a live backup using the SQLite Online Backup API.
func (h *Handler) onlineBackup(sourcePath, destPath string) error {
	backupCfg := h.configProvider.Get().BackupLocal
	pagesPerStep := backupCfg.OnlinePagesPerStep
	sleepInterval := backupCfg.OnlineSleepInterval.Duration

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
			logger.logFinal(backup)
			h.logger.Info("Online backup copy completed successfully.")
			return nil
		}

		logger.log(backup)

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

// log checks if the backup has progressed enough to warrant a log message.
func (m *moduloLogger) log(backup *sqlite.Backup) {
	copiedPages := m.totalPages - backup.Remaining()
	if copiedPages >= m.nextLogTarget {
		m.logProgress(backup)
		m.nextLogTarget += m.logPageInterval
	}
}

// logFinal logs the final progress message.
func (m *moduloLogger) logFinal(backup *sqlite.Backup) {
	m.logProgress(backup)
}

// logProgress is a private helper to format and write the progress log message.
func (m *moduloLogger) logProgress(backup *sqlite.Backup) {
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