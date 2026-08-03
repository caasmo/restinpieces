# Local Backups

The built-in backup system follows a two-step push-pull design:

1. **Push (server-side)**: A recurrent background job creates SQLite snapshots locally on the server — either gzip-compressed (`.bck.gz`) or plain SQLite copies (`.db`).
2. **Pull (client-side)**: Clients retrieve those snapshots from the server via SFTP or rsync, using stable `latest-` hardlinks as sync targets when applicable.

This decouples backup creation from retrieval — backups are produced on the server by the [background job handler](../queue/handlers/backup_local.go) and pulled to any number of client machines.

The filename and hardlink naming conventions are defined in the shared [backup package](../backup/backup.go) — both the server handler and pull clients use the same `backup.LatestFmt` constant (`latest-%s`).

## Job Activation

Run this once to activate the backup job:

```bash
ripc job add backup
```

The job's recurrence interval defaults to `1m`. The actual backup schedule is per-database, controlled by each file's `frequency` setting (see below): a file is backed up only once its frequency has elapsed since its last backup.

## Enabling Backups

After activating the job, configure what to back up. For each database, scaffold a per-file entry with sensible defaults, then set its path and frequency:

```bash
ripc config scaffold backuplocal app_db
ripc config set backup_local.files.app_db.source_path /data/app.db
ripc config set backup_local.files.app_db.frequency 24h
```

The scaffold creates `backup_local.files.app_db` with defaults — `strategy` = `"online"`, `compression` = `false`, `frequency` = `15m` — and an empty `source_path` that you **must** set. Give each database its own key (label, e.g. `app_db`) and its own schedule.

Also set the shared backup directory once:

```bash
ripc config set backup_local.backup_dir /data/backups
```

All backups and `latest-` links are written there. With an empty `backup_dir` the backup feature stays deactivated.

## Deactivating Local Backups

To deactivate the entire backup feature, set `backup_dir` back to the empty string:

```bash
ripc config set backup_local.backup_dir ""
```

Config changes are picked up on `SIGHUP` reload (no restart needed). When deployed via the canonical systemd service ([restinpieces.service](../restinpieces.service)), reload the unit:

```bash
systemctl reload restinpieces
```

Once reloaded, the backup job detects the empty `backup_dir` and skips all work — no backup files and no `latest-` hardlinks are created or updated, and the job logs `backup_dir is empty; backup deactivated.`. This is not an error; the job simply does nothing.

Deactivating `backup_dir`:

- **does not delete** existing backups — files already in `backup_dir` are left untouched.
- **does not require** removing the `backup` job — leaving it scheduled is harmless since it becomes a no-op. You can re-activate backups later by setting `backup_dir` to a directory again and running `systemctl reload restinpieces`; the job picks it up on the next tick.

## Job Configuration

Configuration lives under the `[backup_local]` TOML section, defined in [config/config.go](../config/config.go):

| Field | Type | Default | Description |
|---|---|---|---|
| `backup_dir` | string | `""` | Single directory for all backup files, compressed archives, and latest hardlinks. Empty string **deactivates** the entire backup feature. When non-empty, must be an existing directory. Supports absolute and relative paths. Relative paths resolve against the application's current working directory (CWD). When deployed via the canonical systemd service ([restinpieces.service](../restinpieces.service)), the CWD is `/home/<app>` so a relative path like `data/backups` resolves to `/home/<app>/data/backups`. |
| `online_pages_per_step` | int | `100` | Pages copied per step when using an `"online"` strategy (global across all files). |
| `online_sleep_interval` | duration | `"10ms"` | Pause between steps when using an `"online"` strategy (global). |
| `files` | array of tables | `[]` | List of database files to back up (see below). |

### Per-File Configuration (`files[]`)

Each entry in the `files` array is a TOML table with these fields:

| Field | Type | Default | Description |
|---|---|---|---|
| `source_path` | string | `""` (deactivated) | Filesystem path to the SQLite database to back up. Empty string deactivates the entry. When non-empty, must be an existing file. Supports absolute and relative paths. Relative paths resolve against the application's current working directory (CWD). When deployed via the canonical systemd service ([restinpieces.service](../restinpieces.service)), the CWD is `/home/<app>` and databases typically live under `data/`, so a relative `source_path` should start with `data/` (e.g. `data/app.db`). |
| `compression` | bool | `false` | Enable gzip compression (`.bck.gz`). When false, produces a plain SQLite copy (`.db`). |
| `strategy` | string | `"online"` | Backup strategy: `"online"` or `"vacuum"`. Empty string defaults to `"online"`. |
| `frequency` | duration | — (required) | Minimum interval between backups (e.g. `"24h"`, `"6h"`). The handler skips a file if its latest backup is newer than this duration. |

Set individual fields via `ripc`:

```bash
ripc config set backup_local.backup_dir /data/backups
ripc config set backup_local.online_pages_per_step 50
```

TOML examples:

```toml
[backup_local]
backup_dir = "/data/backups"
online_pages_per_step = 100
online_sleep_interval = "10ms"

[[backup_local.files]]
source_path = "/data/app.db"
compression = true
strategy = "online"
frequency = "24h"

[[backup_local.files]]
source_path = "/data/analytics.db"
compression = false
strategy = "vacuum"
frequency = "6h"
```

### Validation

Configuration validation catches the following at startup and on `SIGHUP` reload:

- **Empty `backup_dir`**: Backup feature deactivated (no error, all fields ignored).
- **Duplicate basenames**: Two files with the same `Base(source_path)` (e.g. `/data/a/app.db` and `/data/b/app.db`) are rejected because they share a backup directory and would overwrite each other's latest links.
- **Empty `source_path`**: Entry deactivated (valid).
- **Non-empty `backup_dir`**: Must be an existing directory, resolved against the application CWD. Checked at startup and on `SIGHUP` reload.
- **Non-empty `source_path`**: Must be an existing file, resolved against the application CWD. Checked at startup and on `SIGHUP` reload.
- **Invalid strategy**: Must be `"online"`, `"vacuum"`, or empty (defaults to online).
- **Non-positive frequency**: Frequency must be a positive duration.
- **Non-positive `online_pages_per_step`**: Must be positive.
- **Negative `online_sleep_interval`**: Cannot be negative.

Configuration is hot-reloadable via `SIGHUP`.

## Failure Behavior

At runtime, the backup handler enforces the same path rules. An entry with an empty `source_path` is skipped as deactivated. A non-empty `source_path` that does not exist or is not a file fails that entry's backup with a clear error — a missing file would previously be auto-created as an empty database and backed up silently. A file that SQLite cannot read as a database (e.g. a text file) also fails the job with SQLite's own error. A genuinely empty database file is backed up as-is. Failed backup jobs are marked failed by the scheduler and retried; no backup file and no `latest-` hardlink are produced for a failed entry.

## Backup Strategies

### Online (default)

Uses SQLite's [Online Backup API](https://sqlite.org/backup.html). Copies the database page-by-page, yielding between steps. Does **not** block writers. **Recommended for most production systems.**

Tune `online_pages_per_step` and `online_sleep_interval` to balance speed against I/O impact. Smaller values are gentler on concurrent workloads; `"0s"` runs the backup as fast as possible.

### Vacuum

Uses `VACUUM INTO` to create a clean, defragmented copy. Faster than online but places a read lock on the database, **blocking all write operations** for the entire duration. Suitable for low-write databases or scheduled maintenance windows.

## Per-File Frequency

Each database has its own `frequency` setting. The handler checks the most recent backup timestamp for each file by scanning `backup_dir` for filenames matching the pattern `{backupID}-{YYYYMMDDTHHMMSSZ}{ext}`. If the elapsed time since the latest backup is less than the configured frequency, that file is skipped. A file with no prior backup is always due.

This allows different schedules for different databases (e.g. critical data every hour, analytics once a day).

## Filename Naming Convention

Backup files are written to `backup_dir` using a lexicographically sortable UTC timestamp:

- **Compressed**: `{backupID}-{YYYYMMDDTHHMMSSZ}.bck.gz`  
  Example: `app.db-20250801T103000Z.bck.gz`
- **Uncompressed**: `{backupID}-{YYYYMMDDTHHMMSSZ}.db`  
  Example: `app.db-20250801T103000Z.db`

The timestamp format `20060102T150405Z` sorts chronologically by string order.

## Stable Hardlink (`latest-`) for Rsync

For uncompressed backups, the handler creates a stable hardlink at `backup_dir/latest-{dbName}` pointing to the most recent backup file. This hardlink is atomically updated using `link(2) + rename(2)` — rsync clients always observe a valid link, never `ENOENT`. This gives rsync a stable file path to sync against without needing to scan filenames.

Compressed backups (`.bck.gz`) produce **no latest link**, as they are not directly consumable.

The hardlink naming convention is defined by the shared constant from the [backup package](../backup/backup.go):

```go
const LatestFmt = "latest-%s"  // package backup
```

Both the server handler and rsync clients use this constant to construct and discover the stable reference path.

## Pull Clients

Two approaches are available for pulling backups:

### SFTP

The [restinpieces-backup-client](https://github.com/caasmo/restinpieces-backup-client) provides an SFTP client that scans the backup directory by filename pattern and downloads the most recent files:

```bash
go run github.com/caasmo/restinpieces-backup-client/cmd/sftp@latest \
  -host myserver.example.com \
  -user backup \
  -remote-dir /data/backups \
  -local-dir ./backups
```

### Rsync

For uncompressed backups (`.db`), rsync can target the stable `latest-{dbName}` hardlink directly, giving a deterministic file path without filename scanning:

```bash
rsync -av backup@myserver:/data/backups/latest-app.db ./backups/
```

The `latest-` hardlink is always atomically updated — rsync never observes a missing target.
