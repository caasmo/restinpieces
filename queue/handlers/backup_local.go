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
	"regexp"
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

	// timestampFormat is the UTC timestamp layout used in backup filenames.
	// It is lexicographically sortable (chronological order equals string order).
	// Produces e.g. "20250801T103000Z".
	timestampFormat = "20060102T150405Z"

	// baseFmt is the shared filename template: backupID and UTC timestamp
	// joined with a dash. The extension is appended separately
	// (compressedExt / uncompressedExt).
	baseFmt         = "%s-%s"
	compressedExt   = ".bck.gz"
	uncompressedExt = ".db"

	// compressedFmt is the filename template for gzip-compressed backups,
	// e.g. "app.db-20250801T103000Z.bck.gz".
	compressedFmt = baseFmt + compressedExt

	// uncompressedFmt is the filename template for plain SQLite copies,
	// e.g. "app.db-20250801T103000Z.db".
	uncompressedFmt = baseFmt + uncompressedExt
)

// The regexes are the parse encoding of the same grammar String() renders,
// with the extension written out escaped. TestBackupFileRoundTrip links the
// two encodings and catches drift between them (e.g. a timestampFormat change
// without a regex update).
var (
	compressedRe   = regexp.MustCompile(`^(.+)-(\d{8}T\d{6}Z)\.bck\.gz$`)
	uncompressedRe = regexp.MustCompile(`^(.+)-(\d{8}T\d{6}Z)\.db$`)
)

// backupFile is one backup file: everything needed to render or parse its
// filename. The backup directory is not part of the struct.
type backupFile struct {
	// backupID identifies the configured source file this backup belongs to:
	// the config label (backup_local.files.<key>) joined with the source
	// file's basename, e.g. label "app" + source_path "data/app.db"
	// → "app-app.db".
	backupID   string
	time       time.Time // UTC timestamp
	compressed bool      // ".bck.gz" vs ".db"
}

// String renders the filename, e.g. "app_db-app.db-20250801T103000Z.bck.gz".
func (f backupFile) String() string {
	format := uncompressedFmt
	if f.compressed {
		format = compressedFmt
	}
	return fmt.Sprintf(format, f.backupID, f.time.UTC().Format(timestampFormat))
}

// errInvalidBackupFile is the sentinel for parseBackupFile: the filename is
// not a valid backup. Always returned wrapped via %w so callers can
// errors.Is(err, errInvalidBackupFile).
var errInvalidBackupFile = errors.New("invalid backup filename")

// parseBackupFile parses a backup filename. The caller has already
// established the name is a backup file (extension gate in latestBackupFiles);
// a name that fails the grammar or has a malformed timestamp returns an error
// wrapping errInvalidBackupFile.
func parseBackupFile(filename string) (backupFile, error) {
	re := uncompressedRe
	compressed := strings.HasSuffix(filename, compressedExt)
	if compressed {
		re = compressedRe
	}
	m := re.FindStringSubmatch(filename)
	if m == nil {
		return backupFile{}, fmt.Errorf("%w: %q", errInvalidBackupFile, filename)
	}
	ts, err := time.Parse(timestampFormat, m[2])
	if err != nil {
		return backupFile{}, fmt.Errorf("%w: %q: invalid timestamp: %v", errInvalidBackupFile, filename, err)
	}
	return backupFile{
		backupID:   m[1],
		time:       ts,
		compressed: compressed,
	}, nil
}

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
		logger:         logger.With("job_handler", JobTypeBackupLocal),
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

	// --- early exits: backup deactivated ---
	if len(cfg.Files) == 0 {
		h.logger.Info("No backup files configured; backup deactivated.")
		return nil
	}
	if cfg.BackupDir == "" {
		h.logger.Info("backup_dir is empty; backup deactivated.")
		return nil
	}

	// --- step 1: backup_dir must be an existing directory ---
	dirInfo, statErr := os.Stat(cfg.BackupDir)
	if statErr != nil {
		return fmt.Errorf("backup_dir: %w", statErr)
	}
	if !dirInfo.IsDir() {
		return fmt.Errorf("backup_dir is not a directory: %s", cfg.BackupDir)
	}

	var errs []error
	latest := h.latestBackupFiles(cfg.BackupDir, cfg.Files)

	for key, fileCfg := range cfg.Files {
		// --- step 2: skip entries with empty source_path (deactivated) ---
		if fileCfg.SourcePath == "" {
			h.logger.Info("Skipping backup; source_path is empty (entry deactivated)", "db", key)
			continue
		}

		backupID := h.buildBackupID(key, fileCfg.SourcePath)

		// --- step 3: skip if not yet due ---
		if !h.isBackupDue(latest[backupID].time, fileCfg.Frequency.Duration, now) {
			h.logger.Info("Skipping backup; not yet due",
				"db", backupID,
				"due_at", latest[backupID].time.Add(fileCfg.Frequency.Duration).Format(timestampFormat),
			)
			continue
		}

		// --- step 4: source file must exist and be a file ---
		srcInfo, srcErr := os.Stat(fileCfg.SourcePath)
		if srcErr != nil {
			errs = append(errs, fmt.Errorf("%q: source database file not found: %s: %w", backupID, fileCfg.SourcePath, srcErr))
			continue
		}
		if srcInfo.IsDir() {
			errs = append(errs, fmt.Errorf("%q: source path is a directory, not a database file: %s", backupID, fileCfg.SourcePath))
			continue
		}

		// --- step 5: backup copy ---
		err := h.handleDbFile(fileCfg, backupID, now, cfg.BackupDir)
		if err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// handleDbFile: backup copy for one configured file.
// Opens its own source conn; the defer closes it on every return path.
func (h *Handler) handleDbFile(fileCfg config.BackupLocalDbFile, backupID string, now time.Time, backupDir string) error {
	srcConn, openErr := zombiezen.NewConn(fileCfg.SourcePath)
	if openErr != nil {
		return fmt.Errorf("%q: open source db: %w", backupID, openErr)
	}
	defer func() {
		if closeErr := srcConn.Close(); closeErr != nil {
			h.logger.Error("Error closing source database connection", "error", closeErr)
		}
	}()

	backupFn := h.onlineBackup // default
	if fileCfg.Strategy == config.BackupStrategyVacuum {
		backupFn = h.vacuumInto
	}

	f := backupFile{
		backupID:   backupID,
		time:       now,
		compressed: fileCfg.Compression,
	}
	finalPath := filepath.Join(backupDir, f.String())

	var err error
	if fileCfg.Compression {
		tempPath := h.buildTempPath(backupID, now)
		tempFinalPath := finalPath + ".tmp" // same directory, os.Rename is atomic

		// --- 5a: dump to temp ---
		err = backupFn(srcConn, tempPath)
		if err != nil {
			removeErr := os.Remove(tempPath)
			if removeErr != nil {
				h.logger.Error("Failed to remove temp file after failed backup", "path", tempPath, "error", removeErr)
			}
			return fmt.Errorf("%q: %w", backupID, err)
		}

		// --- 5b: compress temp to .tmp in backupDir ---
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
			return fmt.Errorf("%q: %w", backupID, err)
		}

		// --- 5c: atomic promote ---
		err = os.Rename(tempFinalPath, finalPath)
		if err != nil {
			removeErr = os.Remove(tempFinalPath)
			if removeErr != nil {
				h.logger.Error("Failed to remove .tmp file after failed rename", "path", tempFinalPath, "error", removeErr)
			}
			return fmt.Errorf("%q: %w", backupID, err)
		}
	} else {
		tempFinalPath := finalPath + ".tmp" // same directory, os.Rename is atomic

		// --- 5a: dump to .tmp in backupDir ---
		err = backupFn(srcConn, tempFinalPath)
		if err != nil {
			removeErr := os.Remove(tempFinalPath)
			if removeErr != nil {
				h.logger.Error("Failed to remove .tmp file after failed backup", "path", tempFinalPath, "error", removeErr)
			}
			return fmt.Errorf("%q: %w", backupID, err)
		}

		// --- 5b: atomic promote ---
		err = os.Rename(tempFinalPath, finalPath)
		if err != nil {
			removeErr := os.Remove(tempFinalPath)
			if removeErr != nil {
				h.logger.Error("Failed to remove .tmp file after failed rename", "path", tempFinalPath, "error", removeErr)
			}
			return fmt.Errorf("%q: %w", backupID, err)
		}

		// --- 5c: update latest link ---
		// Uncompressed only — the rsync pull client needs a stable
		// filename to sync. Compressed .bck.gz is not consumable as-is,
		// so no link is created for it.
		latestPath := h.buildLatestPath(backupDir, backupID)
		err = h.linkLatest(finalPath, latestPath)
		if err != nil {
			return fmt.Errorf("%q: %w", backupID, err)
		}
	}
	return nil
}

// buildBackupID returns the prefix used in backup filenames and hardlinks.
// Produces <key>-<basename> so same-basename source paths do not collide
// (AGENTS.md: map keys are labels, not identifiers).
//
// Example: buildBackupID("app_db", "data/app.db") → "app_db-app.db"
func (h *Handler) buildBackupID(key, sourcePath string) string {
	return key + "-" + filepath.Base(sourcePath)
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

// latestBackupFiles scans backupDir once and returns the most recent backup
// file for each backupID derived from the configured source files.
// Errors are logged internally; an empty map is returned when the directory
// is absent or unreadable (all backups will be treated as due).
func (h *Handler) latestBackupFiles(backupDir string, files map[string]config.BackupLocalDbFile) map[string]backupFile {
	backupIDs := make([]string, 0, len(files))
	for key, f := range files {
		if f.SourcePath == "" {
			continue // deactivated entry, never backed up
		}
		backupIDs = append(backupIDs, h.buildBackupID(key, f.SourcePath))
	}
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		if !os.IsNotExist(err) {
			h.logger.Warn("Failed to scan backup directory", "error", err)
		}
		return map[string]backupFile{}
	}
	latest := make(map[string]backupFile, len(backupIDs))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		// Extension gate: only backup filenames reach the parser; everything
		// else (stale .tmp, logs, etc.) is not a backup.
		if !strings.HasSuffix(name, uncompressedExt) && !strings.HasSuffix(name, compressedExt) {
			continue
		}
		parsed, err := parseBackupFile(name)
		if err != nil {
			continue // link, pre-feature backup, or junk — never a backup
		}
		for _, id := range backupIDs {
			if parsed.backupID == id && parsed.time.After(latest[id].time) {
				latest[id] = parsed
				break
			}
		}
	}
	return latest
}

// isBackupDue returns true if frequency has elapsed since latestTime.
// A zero latestTime means no previous backup exists (always due).
func (h *Handler) isBackupDue(latestTime time.Time, frequency time.Duration, now time.Time) bool {
	if latestTime.IsZero() {
		return true
	}
	return now.Sub(latestTime) >= frequency
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
func (h *Handler) vacuumInto(srcConn *sqlite.Conn, destPath string) error {
	stmt, err := srcConn.Prepare(fmt.Sprintf("VACUUM INTO '%s';", destPath))
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
func (h *Handler) onlineBackup(srcConn *sqlite.Conn, destPath string) error {
	backupCfg := h.configProvider.Get().BackupLocal
	pagesPerStep := backupCfg.OnlinePagesPerStep
	sleepInterval := backupCfg.OnlineSleepInterval.Duration

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
