# Backups

The backup system follows a two-phase push-pull design:

1. **Push (server-side)**: A snapshot daemon creates SQLite snapshots locally on the server — either gzip-compressed (`.bck.gz`) or plain SQLite copies (`.db`).
2. **Pull (client-side)**: Clients retrieve those snapshots from the server via SFTP or rsync, using stable `latest-` hardlinks as sync targets when applicable.

The framework hosts the backup shape (`[backup]` in [config/config.go](../config/config.go)), its validation, the `ripc scaffold` commands, and the shared naming contract (`backup.LatestFmt`, `backup.LatestGlob`). Engines live in [restinpieces-backup](https://github.com/caasmo/restinpieces-backup): `cmd/local-copy` creates snapshots, `cmd/rsync`, `cmd/rsync-daemon`, and `cmd/sftp` pull and verify them with `PRAGMA integrity_check`.

## Enabling Backups

Each database gets one entry. Scaffold it with defaults, then set its paths:

```bash
ripc scaffold backup-online app-online
ripc set backup.online.app-online.source_path /data/app.db
ripc set backup.online.app-online.dest_path /data/backups
ripc set backup.online.app-online.frequency 24h
ripc scaffold backup-vacuum app-vacuum
ripc set backup.vacuum.app-vacuum.source_path /data/other.db
ripc set backup.vacuum.app-vacuum.dest_path /data/backups
ripc scaffold backup-sqlite-rsync app-rsync
ripc set backup.sqlite-rsync.entries.app-rsync.source_path /data/app.db
```

Scaffold creates the entry with defaults and empty `source_path`/`dest_path` that you must set. Each entry has its own label (e.g. `app-online`) and schedule. `dest_path` is the directory for that database's backups and `latest-` links. `sqlite-rsync` needs no `dest_path`; it serves the live file over the network.

## Deactivating Backups

To deactivate one entry, empty its `source_path` (or `dest_path` for online/vacuum):

```bash
ripc set backup.online.app-online.source_path ""
ripc set backup.sqlite-rsync.entries.app-rsync.source_path ""
```

To deactivate all backups, remove every entry. Empty maps are valid and make backups a no-op. Deactivating does not delete files on disk and does not require removing the daemon. You can reactivate by setting the paths again.

Config changes apply on `SIGHUP` reload (no restart). With the canonical systemd service ([restinpieces.service](../restinpieces.service)):

```bash
systemctl reload restinpieces
```

## Configuration

Configuration lives under `[backup]` in [config/config.go](../config/config.go). Each strategy has its own table. The table you scaffold into selects the engine.

| Field | Type | Default | Description |
|---|---|---|---|
| `online` | map of tables | `{}` | Online Backup API entries, keyed by label (e.g. `app-online`). |
| `vacuum` | map of tables | `{}` | VACUUM INTO entries, keyed by label (e.g. `app-vacuum`). |
| `sqlite-rsync` | table | — | Origin for sqlite-rsync. Holds `listen_addr` and `entries`. |

### `backup.online.<label>` — Online Backup API

Each `online` entry is one database. Fields:

| Field | Type | Default | Description |
|---|---|---|---|
| `source_path` | string | `""` (deactivated) | SQLite file to back up. Empty deactivates. When set, must be an existing file. Supports absolute and relative paths (relative to CWD). |
| `dest_path` | string | `""` (deactivated) | Directory for backups and `latest-` links. Empty deactivates. When set, must be an existing directory. |
| `frequency` | duration | — (required) | Minimum interval between backups (e.g. `24h`). Skips if latest backup is newer. |
| `compression` | bool | `false` | Gzip the snapshot (`.bck.gz` vs `.db`). |
| `pages_per_step` | int | `100` | Pages copied per step. Must be ≥1. |
| `sleep_interval` | duration | `10ms` | Pause between steps. 0 means no throttling. Must be ≥0. |

### `backup.vacuum.<label>` — VACUUM INTO

Each `vacuum` entry is one database. Fields:

| Field | Type | Default | Description |
|---|---|---|---|
| `source_path` | string | `""` (deactivated) | SQLite file to back up. Same rules as `online`. |
| `dest_path` | string | `""` (deactivated) | Directory for backups. Same rules as `online`. |
| `frequency` | duration | — (required) | Minimum interval between backups. |
| `compression` | bool | `false` | Gzip the snapshot. |

### `backup.sqlite-rsync` — sqlite-rsync origin

One section with topology plus per-database entries.

| Field | Type | Default | Description |
|---|---|---|---|
| `listen_addr` | string | `127.0.0.1:54321` | TCP address the origin listens on. Empty uses default. |

Each `backup.sqlite-rsync.entries.<label>` entry:

| Field | Type | Default | Description |
|---|---|---|---|
| `source_path` | string | `""` (deactivated) | SQLite file to serve. Empty deactivates. |
| `sync_timeout` | duration | `15m` | Longest one sync may run. 0 uses default 15m. |

Set fields via `ripc`:

```bash
ripc set backup.online.app-online.source_path /data/app.db
ripc set backup.online.app-online.frequency 24h
ripc set backup.sqlite-rsync.entries.app-rsync.source_path /data/app.db
```

TOML examples:

```toml
[backup.online.app1]
source_path = "/data/app.db"
dest_path = "/backups"
frequency = "24h"
compression = false

[backup.vacuum.app2]
source_path = "/data/other.db"
dest_path = "/backups"
frequency = "24h"

[backup.sqlite-rsync]
listen_addr = "127.0.0.1:54321"

[backup.sqlite-rsync.entries.app3]
source_path = "/data/app3.db"
sync_timeout = "15m"
```

### Validation

Validation in [config/config_validate.go](../config/config_validate.go) runs at startup and on `SIGHUP` reload:

- **Empty maps**: No `online`/`vacuum`/`sqlite-rsync` entries means backups are deactivated (no error).
- **Invalid label**: Key must not be empty and must not contain whitespace or `.`.
- **Empty `source_path` / `dest_path`**: Deactivates that entry (valid).
- **Non-empty `source_path`**: Must be an existing file (resolved against CWD).
- **Non-empty `dest_path`**: Must be an existing directory (resolved against CWD).
- **`frequency`**: Must be positive (`online`, `vacuum`).
- **`pages_per_step`**: Must be ≥1 (`online`).
- **`sleep_interval`**: Must be ≥0 (`online`).
- **`sync_timeout`**: Must be ≥0 (`sqlite-rsync`).
- **`listen_addr`**: Must be `host:port` when set.

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
