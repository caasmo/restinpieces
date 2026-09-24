package config

// Backup holds the backup configuration. Each backup strategy has its own
// TOML table, and the table you scaffold into decides which daemon makes
// the backup. Every entry is keyed by a label you choose (for example
// "app1"). The daemons that make the backups live in restinpieces-backup;
// the framework only defines the configuration and validates it, and runs
// no backup itself.
type Backup struct {
	OnlineAPI   BackupOnlineAPI   `toml:"online"`
	Vacuum      BackupVacuum      `toml:"vacuum"`
	SqliteRsync BackupSqliteRsync `toml:"sqlite-rsync"`
	S3Upload    BackupS3Upload    `toml:"s3_upload"`
}

// BackupOnlineAPI holds per-database configuration for the Online Backup API
// strategy. The map key is a user-chosen label; the value holds the
// database's backup settings.
type BackupOnlineAPI map[string]BackupOnlineAPIEntry

// BackupOnlineAPIEntry is one Online Backup API entry.
//
// Empty source_path or dest_path deactivates the entry. Frequency is
// parsed via time.ParseDuration (e.g. "24h"). Compressed snapshots use
// ".bck.gz" and plain ones ".db". PagesPerStep 0 means "use the 100-page
// default" (Step(0) would copy nothing and never finish). SleepInterval 0
// means no throttling.
type BackupOnlineAPIEntry struct {
	// SourcePath is the filesystem path to the SQLite database to back up.
	// Supports absolute and relative paths. Relative paths resolve against
	// the application's current working directory (CWD).
	// Empty string deactivates this entry.
	SourcePath string `toml:"source_path" comment:"Path to the source database file. Supports absolute and relative paths (relative to the application CWD)."`

	// DestPath is the directory where the backup files are written.
	// Supports absolute and relative paths. Relative paths resolve against
	// the application's current working directory (CWD).
	// Empty string deactivates this entry.
	DestPath string `toml:"dest_path" comment:"Directory where backup files will be stored. Supports absolute and relative paths (relative to the application CWD)."`

	// Frequency defines how often this database should be backed up.
	// The daemon skips a database if its latest backup is newer than
	// this duration. Parsed via time.ParseDuration (e.g. "24h", "6h").
	Frequency Duration `toml:"frequency" comment:"Minimum interval between backups (e.g. '24h')."`

	// Compression enables gzip compression of the backup file.
	// When true, backup files use the ".bck.gz" extension.
	// When false, backup files use the ".db" extension (plain SQLite copy).
	Compression bool `toml:"compression" comment:"Enable gzip compression of the backup."`

	// PagesPerStep controls the number of pages copied in each step.
	// Must be >= 1: Step(0) would copy nothing and never finish.
	PagesPerStep int `toml:"pages_per_step" comment:"Pages to copy in each step (must be >= 1)."`

	// SleepInterval is the duration to sleep between online backup steps.
	// 0 means no throttling.
	SleepInterval Duration `toml:"sleep_interval" comment:"Duration to sleep between steps (0 = no throttling)."`
}

// BackupVacuum holds per-database configuration for the VACUUM INTO strategy.
type BackupVacuum map[string]BackupVacuumEntry

// BackupVacuumEntry is one VACUUM INTO entry.
//
// Empty source_path or dest_path deactivates the entry. Frequency is
// parsed via time.ParseDuration (e.g. "24h"). Compressed snapshots use
// ".bck.gz" and plain ones ".db".
type BackupVacuumEntry struct {
	// SourcePath is the filesystem path to the SQLite database to back up.
	// Supports absolute and relative paths. Relative paths resolve against
	// the application's current working directory (CWD).
	// Empty string deactivates this entry.
	SourcePath string `toml:"source_path" comment:"Path to the source database file. Supports absolute and relative paths (relative to the application CWD)."`

	// DestPath is the directory where the backup files are written.
	// Supports absolute and relative paths. Relative paths resolve against
	// the application's current working directory (CWD).
	// Empty string deactivates this entry.
	DestPath string `toml:"dest_path" comment:"Directory where backup files will be stored. Supports absolute and relative paths (relative to the application CWD)."`

	// Frequency defines how often this database should be backed up.
	// The daemon skips a database if its latest backup is newer than
	// this duration. Parsed via time.ParseDuration (e.g. "24h", "6h").
	Frequency Duration `toml:"frequency" comment:"Minimum interval between backups (e.g. '24h')."`

	// Compression enables gzip compression of the backup file.
	// When true, backup files use the ".bck.gz" extension.
	// When false, backup files use the ".db" extension (plain SQLite copy).
	Compression bool `toml:"compression" comment:"Enable gzip compression of the backup."`
}

// BackupSqliteRsync holds the sqlite-rsync configuration. It has a parent
// section because it needs topology (listen_addr) in addition to the
// per-database entries.
type BackupSqliteRsync struct {
	ListenAddr string                            `toml:"listen_addr" comment:"TCP address the origin daemon listens on (e.g. '127.0.0.1:54321')."`
	Entries    map[string]BackupSqliteRsyncEntry `toml:"entries"`
}

// BackupSqliteRsyncEntry is one sqlite-rsync origin entry.
//
// Empty source_path deactivates the entry. SyncTimeout 0 means "use the
// daemon default of 15m".
type BackupSqliteRsyncEntry struct {
	// SourcePath is the filesystem path to the SQLite database to serve.
	// Supports absolute and relative paths. Relative paths resolve against
	// the application's current working directory (CWD).
	// Empty string deactivates this entry.
	SourcePath string `toml:"source_path" comment:"Path to the source database file. Supports absolute and relative paths (relative to the application CWD)."`

	// SyncTimeout is the longest one sync may run. Zero uses the default of 15 minutes.
	SyncTimeout Duration `toml:"sync_timeout" comment:"Longest one sync may run (e.g. '15m'). Zero uses the default of 15 minutes."`
}

// BackupS3Upload holds the S3 upload entries. Each entry is keyed by a
// label you choose (for example "app-s3"). backup_label names the online
// or vacuum backup to upload, and the entry uploads that backup's newest
// backup to the bucket configured in the top-level [s3] section.
type BackupS3Upload map[string]BackupS3UploadEntry

// BackupS3UploadEntry is one S3 upload entry.
//
// Empty backup_label deactivates the entry. Frequency is parsed via
// time.ParseDuration (e.g. "5m"); zero means the 5m default. An empty
// AgeRecipient uploads the backup without encryption.
type BackupS3UploadEntry struct {
	// BackupLabel is the label of the online or vacuum backup to upload,
	// for example "app-online". The label must be unique across all backup
	// tables.
	BackupLabel string `toml:"backup_label" comment:"Label of the backup entry to upload (e.g. 'app-online')"`

	// Frequency defines how often the daemon checks for a new backup to
	// upload. Parsed via time.ParseDuration (e.g. "5m"). Zero uses the
	// default of 5m.
	Frequency Duration `toml:"frequency" comment:"How often to check for a new backup to upload (e.g. '5m'). Zero uses the default of 5m."`

	// AgeRecipient is the age public key the backup is encrypted to
	// before upload. Empty string uploads the backup unchanged.
	AgeRecipient string `toml:"age_recipient" comment:"age public key the backup is encrypted to (e.g. 'age1...'). Empty uploads without encryption."`
}

func (c Config) BackupSqliteRsync() BackupSqliteRsync {
	return c.Backup.SqliteRsync
}

func (c Config) BackupOnlineAPI() BackupOnlineAPI {
	return c.Backup.OnlineAPI
}

func (c Config) BackupVacuum() BackupVacuum {
	return c.Backup.Vacuum
}

func (c Config) BackupS3Upload() BackupS3Upload {
	return c.Backup.S3Upload
}
