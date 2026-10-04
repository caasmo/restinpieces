package config

// Backup holds the backup configuration. Each backup strategy has its own
// TOML table, and the table you scaffold into decides which daemon makes
// the backup. Every entry is keyed by a label you choose (for example
// "app1"). The code that makes the backups lives in restinpieces-backup;
// the framework only defines the configuration and validates it, and runs
// no backup itself.
type Backup struct {
	OnlineAPI   BackupOnlineAPI   `toml:"online"`
	Vacuum      BackupVacuum      `toml:"vacuum"`
	SqliteRsync BackupSqliteRsync `toml:"sqlite-rsync"`
	S3Upload    BackupS3Upload    `toml:"s3-upload"`
	S3Download  BackupS3Download  `toml:"s3-download"`
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
	// The job skips a database if its latest backup is newer than
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
	// The job skips a database if its latest backup is newer than
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

// s3UploadSelectorLatest is the only supported PathPrefixSelector value:
// the entry uploads the file with the newest modification time among the
// prefix matches.
const s3UploadSelectorLatest = "latest"

// BackupS3Upload holds the S3 upload entries. Each entry is keyed by a
// label you choose (for example "app-s3"). An entry uploads one file to its
// bucket, under the object key backup/<label>/<pad>/<filename>.
type BackupS3Upload map[string]BackupS3UploadEntry

// BackupS3UploadEntry is one S3 upload entry.
//
// Path and PathPrefix are mutually exclusive. A Path entry uploads that
// fixed file. A PathPrefix entry uploads the newest file whose name
// starts with the prefix; PathPrefixSelector names how that match is
// chosen and only "latest" is supported. An entry with both paths empty
// is deactivated. An empty AgeRecipient uploads the file
// unchanged.
type BackupS3UploadEntry struct {
	// Bucket is the bucket the file is uploaded to. Required when Path or
	// PathPrefix is set.
	Bucket string `toml:"bucket" comment:"Bucket the file is uploaded to"`

	// Path is the fixed file to upload. Empty uses PathPrefix.
	Path string `toml:"path" comment:"File to upload"`

	// PathPrefix selects a file by name prefix; the newest match is
	// uploaded when PathPrefixSelector is "latest". Empty uses Path.
	PathPrefix string `toml:"path_prefix" comment:"Path prefix; with path_prefix_selector 'latest' the newest match is uploaded"`

	// PathPrefixSelector names how the match under PathPrefix is
	// chosen. Only "latest" is supported.
	PathPrefixSelector string `toml:"path_prefix_selector" comment:"How the match is chosen (only 'latest' is supported)"`

	// AgeRecipient is the age public key the file is encrypted to
	// before upload. Empty string uploads the file unchanged.
	AgeRecipient string `toml:"age_recipient" comment:"age public key the file is encrypted to (e.g. 'age1...'). Empty uploads without encryption."`
}

// BackupS3Download holds the S3 download entries. Each entry is keyed by a
// label you choose and downloads one object from its bucket into DestDir,
// under the name s3download-<label>-<pad>-<name>.
type BackupS3Download map[string]BackupS3DownloadEntry

// BackupS3DownloadEntry is one S3 download entry.
//
// ObjectKeyPrefix is the start of the keys to look at. The job downloads
// the newest backup for the label, or just one object when an exact key
// is set. An empty prefix deactivates the entry; an active entry needs a
// bucket. The job skips the entry until MinInterval has passed since its
// last download, without calling S3. The uploader names keys in time
// order, so the bucket returns the newest backup for the label. The local
// file keeps the object's pad and name, so the same object is never
// downloaded twice.
type BackupS3DownloadEntry struct {
	// Bucket is the bucket the object is downloaded from. Required when
	// ObjectKeyPrefix is set.
	Bucket string `toml:"bucket" comment:"Bucket the object is downloaded from"`

	// ObjectKeyPrefix is the start of the keys to look at; the newest
	// backup for the label is downloaded. An exact object key selects
	// just that object. Empty deactivates the entry.
	ObjectKeyPrefix string `toml:"object_key_prefix" comment:"Key prefix; the newest backup for the label is downloaded"`

	// DestDir is the local directory the downloaded file is written to.
	DestDir string `toml:"dest_dir" comment:"Directory the downloaded file is written to"`

	// MinInterval is the minimum interval between downloads. The job
	// skips the entry without calling S3 until this much time has passed
	// since the last download.
	MinInterval Duration `toml:"min_interval" comment:"Minimum interval between downloads (e.g. '5m')"`
}

func (c Config) BackupSqliteRsync() BackupSqliteRsync {
	return c.Backup.SqliteRsync
}
