# Backups

The framework has no backup code, to keep dependencies minimal. It only provides the [`[backup]` config shape](../config/backup.go) that the [restinpieces-backup](https://github.com/caasmo/restinpieces-backup) implementations use. Because the shape is framework-owned, you configure backups with `ripc` in the main app configuration file.

## Content

- [Enabling Backups](#enabling-backups)
- [Deactivating Backups](#deactivating-backups)
- [Configuration](#configuration)
  - [`backup.online.<label>` — Online Backup API](#backuponline-label--online-backup-api)
  - [`backup.vacuum.<label>` — VACUUM INTO](#backupvacuum-label--vacuum-into)
  - [`backup.sqlite-rsync` — sqlite-rsync origin](#backupsqlite-rsync--sqlite-rsync-origin)
- [Stable Hardlink (`latest-`)](#stable-hardlink-latest-)

## Enabling Backups

Each database gets one entry. Scaffold it with defaults, then set its paths:

```bash
ripc scaffold backup-online app-online
ripc set backup.online.app-online.source_path /data/app.db
ripc set backup.online.app-online.dest_path /data/backups
ripc set backup.online.app-online.frequency 24h
```

Scaffold creates the entry with defaults and empty `source_path`/`dest_path` that you must set. Each entry has its own label (e.g. `app-online`) and schedule. `dest_path` is the directory for that database's backups and `latest-` links.

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

Configuration lives under `[backup]` in [config/backup.go](../config/backup.go). Each strategy has its own table. The table you scaffold into selects the engine.

| Strategy | Description |
|---|---|
| `online` | Online Backup API entries. |
| `vacuum` | VACUUM INTO entries. |
| `sqlite-rsync` | sqlite-rsync origin. |

### `backup.online.<label>` — Online Backup API

Each `online` entry is the sync configuration of one database. Fields:

| Field | Type | Default | Description |
|---|---|---|---|
| `source_path` | string | `""` (deactivated) | SQLite file to back up. Empty deactivates. When set, must be an existing file. Supports absolute and relative paths (relative to CWD). |
| `dest_path` | string | `""` (deactivated) | Directory for backups and `latest-` links. Empty deactivates. When set, must be an existing directory. |
| `frequency` | duration | — (required) | Minimum interval between backups (e.g. `24h`). Skips if latest backup is newer. |
| `compression` | bool | `false` | Gzip the snapshot (`.bck.gz` vs `.db`). |
| `pages_per_step` | int | `100` | Pages copied per step. Must be ≥1. |
| `sleep_interval` | duration | `10ms` | Pause between steps. 0 means no throttling. Must be ≥0. |

### `backup.vacuum.<label>` — VACUUM INTO

Each `vacuum` entry is the sync configuration of one database. Fields:

| Field | Type | Default | Description |
|---|---|---|---|
| `source_path` | string | `""` (deactivated) | SQLite file to back up. Same rules as `online`. |
| `dest_path` | string | `""` (deactivated) | Directory for backups. Same rules as `online`. |
| `frequency` | duration | — (required) | Minimum interval between backups. |
| `compression` | bool | `false` | Gzip the snapshot. |

### `backup.sqlite-rsync` — sqlite-rsync origin

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

## Stable Hardlink (`latest-`)

Uncompressed local backups — those made by the onlineapi and vacuum engines in [restinpieces-backup](https://github.com/caasmo/restinpieces-backup) — get a stable hardlink named `latest-{dbName}` pointing to the most recent snapshot.

The naming contract is defined by the [backup package](../backup/backup.go):

```go
const LatestFmt = "latest-%s"   // package backup
const LatestGlob = "latest-*.db"
```


