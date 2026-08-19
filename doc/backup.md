# Backups

The backup system follows a two-phase push-pull design:

1. **Push (server-side)**: A snapshot daemon creates SQLite snapshots locally on the server — either gzip-compressed (`.bck.gz`) or plain SQLite copies (`.db`).
2. **Pull (client-side)**: Clients retrieve those snapshots from the server via SFTP or rsync, using stable `latest-` hardlinks as sync targets when applicable.

The framework hosts the backup **configuration shape** (the `[backup]` section in [config/config.go](../config/config.go)), its validation, the `ripc scaffold backup` command, and the shared [naming contract](../backup/backup.go) (`backup.LatestFmt`, `backup.LatestGlob`). The engines that consume this shape live in the separate [restinpieces-backup](https://github.com/caasmo/restinpieces-backup) repository: `cmd/local-copy` produces the snapshots on the database machine, and `cmd/rsync`, `cmd/rsync-daemon`, and `cmd/sftp` pull them to a backup machine and verify every received database with `PRAGMA integrity_check`.

## Enabling Backups

For each database, scaffold a per-file entry with sensible defaults, then set its paths and frequency:

```bash
ripc scaffold backup app_db
ripc set backup.files.app_db.source_path /data/app.db
ripc set backup.files.app_db.dest_path /data/backups
ripc set backup.files.app_db.frequency 24h
```

The scaffold creates `backup.files.app_db` with defaults — `strategy` = `"online"`, `compression` = `false`, `frequency` = `15m`, `online_api_pages_per_step` = `100`, `online_api_sleep_interval` = `10ms` — and empty `source_path` and `dest_path` that you **must** set. Give each database its own key (label, e.g. `app_db`) and its own schedule. The `dest_path` is the directory where that database's backups and `latest-` links are written.

## Deactivating Backups

To deactivate an individual entry, set its `source_path` (or `dest_path`) back to the empty string:

```bash
ripc set backup.files.app_db.source_path ""
```

To deactivate the entire backup feature, empty the `files` map:

```bash
ripc set backup.files ""
```

An empty `files` map is valid and makes the backup feature a no-op. Deactivating:

- **does not delete** existing backups — files already in `dest_path` are left untouched.
- **does not require** uninstalling the snapshot daemon — with no configured files it has nothing to do, and you can re-activate later by setting the fields again.

Config changes are picked up on `SIGHUP` reload (no restart needed). When deployed via the canonical systemd service ([restinpieces.service](../restinpieces.service)), reload the unit:

```bash
systemctl reload restinpieces
```

## Configuration

Configuration lives under the `[backup]` TOML section, defined in [config/config.go](../config/config.go):

| Field | Type | Default | Description |
|---|---|---|---|
| `files` | map of tables | `{}` | Per-database backup entries, keyed by an arbitrary user-chosen label (e.g. `app_db`). An empty map deactivates the backup feature. |

### Per-File Configuration (`files.<label>`)

Each entry in the `files` map is a TOML table with these fields:

| Field | Type | Default | Description |
|---|---|---|---|
| `source_path` | string | `""` (deactivated) | Filesystem path to the SQLite database to back up. Empty string deactivates the entry. When non-empty, must be an existing file. Supports absolute and relative paths. Relative paths resolve against the application's current working directory (CWD). When deployed via the canonical systemd service ([restinpieces.service](../restinpieces.service)), the CWD is `/home/<app>` and databases typically live under `data/`, so a relative `source_path` should start with `data/` (e.g. `data/app.db`). |
| `dest_path` | string | `""` (deactivated) | Directory where the backup files and `latest-` links are written. Empty string deactivates the entry. When non-empty, must be an existing directory. Supports absolute and relative paths. Relative paths resolve against the application CWD. |
| `frequency` | duration | — (required) | Minimum interval between backups (e.g. `"24h"`, `"6h"`). The daemon skips a file if its latest backup is newer than this duration. |
| `compression` | bool | `false` | Enable gzip compression (`.bck.gz`). When false, produces a plain SQLite copy (`.db`). |
| `strategy` | string | `"online"` | Backup strategy: `"online"` or `"vacuum"`. Empty string defaults to `"online"`. |
| `online_api_pages_per_step` | int | `100` | For the `"online"` strategy, pages copied per step (must be ≥ 1). |
| `online_api_sleep_interval` | duration | `"10ms"` | For the `"online"` strategy, pause between steps (`"0s"` = no throttling). |

Set individual fields via `ripc`:

```bash
ripc set backup.files.app_db.source_path /data/app.db
ripc set backup.files.app_db.frequency 24h
```

TOML examples:

```toml
[backup.files.app_db]
source_path = "/data/app.db"
dest_path = "/data/backups"
frequency = "24h"
compression = true
strategy = "online"

[backup.files.analytics_db]
source_path = "/data/analytics.db"
dest_path = "/data/backups"
frequency = "6h"
compression = false
strategy = "vacuum"
```

### Validation

Configuration validation (in [config/config_validate.go](../config/config_validate.go)) catches the following at startup and on `SIGHUP` reload:

- **Empty `files` map**: Backup feature deactivated (no error, all fields ignored).
- **Empty map key**: A `files` entry with an empty label is rejected.
- **Empty `source_path` / `dest_path`**: Entry deactivated (valid).
- **Non-empty `source_path`**: Must be an existing file, resolved against the application CWD.
- **Non-empty `dest_path`**: Must be an existing directory, resolved against the application CWD.
- **Invalid strategy**: Must be `"online"`, `"vacuum"`, or empty (defaults to online).
- **Non-positive `frequency`**: Frequency must be a positive duration.
- **Non-positive `online_api_pages_per_step`**: Must be positive (checked for every entry, whatever the strategy).
- **Negative `online_api_sleep_interval`**: Cannot be negative (checked for every entry, whatever the strategy).

Configuration is hot-reloadable via `SIGHUP`.

## Stable Hardlink (`latest-`) for Rsync

For uncompressed backups, the snapshot daemon creates a stable hardlink named `latest-{dbName}` pointing to the most recent backup file. This gives rsync a stable file path to sync against without needing to scan filenames. Compressed backups (`.bck.gz`) produce no latest link, as they are not directly consumable.

The hardlink naming convention is defined by the shared constant from the [backup package](../backup/backup.go):

```go
const LatestFmt = "latest-%s"  // package backup
```

The same package defines `LatestGlob = "latest-*.db"`, the shell glob matching every latest hardlink in a backup directory. Rsync clients pass it as the remote file argument to sync all configured databases in a single run; the remote login shell expands it (a glob with zero matches expands to the literal pattern, so clients must treat "no files transferred" as a failure). The trailing `.db` deliberately excludes transient `.tmp` hardlinks and timestamped snapshots, which never match because they start with the backup ID, not `latest-`.

## Pull Clients

Two approaches are available for pulling backups:

### SFTP

The [restinpieces-backup](https://github.com/caasmo/restinpieces-backup) `cmd/sftp` client scans the backup directory by filename pattern and downloads the most recent files:

```bash
go run github.com/caasmo/restinpieces-backup/cmd/sftp@latest \
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

To sync every configured database in one run, pass the `latest-*.db` glob as the remote argument — the remote login shell expands it. The `latest-` hardlink is atomically updated by the daemon, so rsync never observes a missing target.
