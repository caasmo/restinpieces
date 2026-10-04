# Backups

The framework has no backup code, to keep dependencies minimal. It only provides the [`[backup]` config shape](../config/backup.go) that the [restinpieces-backup](https://github.com/caasmo/restinpieces-backup) implementations use. Because the shape is framework-owned, you configure backups with `ripc` in the main app configuration file.

## Content

- [Enabling Backups](#enabling-backups)
- [Deactivating Backups](#deactivating-backups)
- [Configuration](#configuration)
  - [`backup.online.<label>` — Online Backup API](#backuponline-label--online-backup-api)
  - [`backup.vacuum.<label>` — VACUUM INTO](#backupvacuum-label--vacuum-into)
  - [`backup.sqlite-rsync` — sqlite-rsync origin](#backupsqlite-rsync--sqlite-rsync-origin)
  - [`backup.s3-upload.<label>` — S3 upload](#backups3-uploadlabel--s3-upload)
  - [`backup.s3-download.<label>` — S3 download](#backups3-downloadlabel--s3-download)
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
ripc set backup.s3-upload.app-s3.path ""
ripc set backup.s3-download.app-dl.object_key_prefix ""
```

To deactivate all backups, remove every entry. Empty maps are valid and make backups a no-op. Deactivating does not delete files on disk and does not require removing the job. You can reactivate by setting the paths again.

Config changes apply on `SIGHUP` reload (no restart). With the canonical systemd service ([systemd.service](../systemd.service)):

```bash
systemctl reload restinpieces
```

## Configuration

Configuration lives under `[backup]` in [config/backup.go](../config/backup.go). Each strategy has its own TOML table. The TOML table you scaffold into selects the engine.

A label is unique across all tables. Validation rejects the same label in two tables.

| Strategy | Description |
|---|---|
| `online` | Online Backup API entries. |
| `vacuum` | VACUUM INTO entries. |
| `sqlite-rsync` | sqlite-rsync origin. |
| `s3-upload` | Uploads one file to an S3-compatible bucket: a fixed path or the newest match under a path prefix. |
| `s3-download` | Pulls the newest backup for a label from an S3-compatible bucket; an exact object key pulls just that object. |

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

The app runs each entry as a job of type `online`; the interval of that `scheduler.jobs` entry is the check tick, and an entry is only backed up when its `frequency` has elapsed.

### `backup.vacuum.<label>` — VACUUM INTO

Each `vacuum` entry is the sync configuration of one database. Fields:

| Field | Type | Default | Description |
|---|---|---|---|
| `source_path` | string | `""` (deactivated) | SQLite file to back up. Same rules as `online`. |
| `dest_path` | string | `""` (deactivated) | Directory for backups. Same rules as `online`. |
| `frequency` | duration | — (required) | Minimum interval between backups. |
| `compression` | bool | `false` | Gzip the snapshot. |

The app runs each entry as a job of type `vacuum`; the interval of that `scheduler.jobs` entry is the check tick, and an entry is only backed up when its `frequency` has elapsed.

### `backup.sqlite-rsync` — sqlite-rsync origin

| Field | Type | Default | Description |
|---|---|---|---|
| `listen_addr` | string | `127.0.0.1:54321` | TCP address the origin listens on. Empty uses default. |

Each `backup.sqlite-rsync.entries.<label>` entry:

| Field | Type | Default | Description |
|---|---|---|---|
| `source_path` | string | `""` (deactivated) | SQLite file to serve. Empty deactivates. |
| `sync_timeout` | duration | `15m` | Longest one sync may run. 0 uses default 15m. |

### `backup.s3-upload.<label>` — S3 upload

Each `s3-upload` entry uploads one file to the bucket the entry sets. The object key is `backup/<label>/<pad>/<filename>`, where `<pad>` is the file's modification time counted down from year 9999 and zero-padded, so a bucket listing shows the newest object first. A file whose object already exists is never uploaded twice. The app runs the entry as a job of type `s3_upload`; the interval of that `scheduler.jobs` entry sets how often the bucket is checked.

| Field | Type | Default | Description |
|---|---|---|---|
| `bucket` | string | `""` | Bucket the file is uploaded to. Required when `path` or `path_prefix` is set. |
| `path` | string | `""` | Fixed file to upload. Empty uses `path_prefix`. |
| `path_prefix` | string | `""` | Path prefix; the newest matching file is uploaded. Requires `path_prefix_selector`. Empty uses `path`. |
| `path_prefix_selector` | string | `"latest"` | How the match under `path_prefix` is chosen. Only `latest` is supported. |
| `age_recipient` | string | `""` | age public key the file is encrypted to before upload. Empty uploads without encryption. |

`path` and `path_prefix` are mutually exclusive; an entry with both empty is deactivated.

1. Choose age recipient — most probably the one from the master key is the best tradeoff: it protects the backups if S3 is breached, and if the server is breached your live database is already compromised.

```bash
age-keygen -y age.key | tr -d '\n' > s3-backup-recipient.txt
ripc set backup.s3-upload.app-s3.age_recipient @s3-backup-recipient.txt
```

2. Scaffold the entry:

```bash
ripc scaffold backup-s3-upload app-s3
```

3. Point it at the file, either fixed or by prefix:

```bash
ripc set backup.s3-upload.app-s3.bucket my-backups
ripc set backup.s3-upload.app-s3.path_prefix /data/backups/app-online-app.db-
ripc set backup.s3-upload.app-s3.path_prefix_selector latest
```

### `backup.s3-download.<label>` — S3 download

Each `s3-download` entry lists its bucket under `object_key_prefix` and downloads the newest backup for the label. The uploader puts the inverted time in the key, so the bucket returns the newest backup for the label; the prefix `backup/app-s3/` gets the newest backup for `app-s3`. An exact object key gets just that object. The file is written into `dest_dir` as `s3download-<label>-<pad>-<name>`, and the download is skipped when that file already exists, so the same object is never downloaded twice. The file is never decrypted.

| Field | Type | Default | Description |
|---|---|---|---|
| `object_key_prefix` | string | `""` (deactivated) | Start of the keys to look at; the newest backup for the label is downloaded. An exact object key gets just that object. |
| `bucket` | string | `""` | Bucket the object is downloaded from. Required when `object_key_prefix` is set. |
| `dest_dir` | string | `""` | Directory the downloaded file is written to. Must be an existing directory when the entry is active. |
| `min_interval` | duration | `5m` | The entry is skipped without calling S3 until this much time has passed since the last download. |

An entry with an empty `object_key_prefix` is deactivated. An active entry needs a bucket.

Scaffold the entry and point it at the label:

```bash
ripc scaffold backup-s3-download app-dl
ripc set backup.s3-download.app-dl.bucket my-backups
ripc set backup.s3-download.app-dl.object_key_prefix backup/app-s3/
ripc set backup.s3-download.app-dl.dest_dir /data/downloads
```

Set an exact object key instead to get just that object:

```bash
ripc set backup.s3-download.app-dl.object_key_prefix backup/app-s3/251611468335/app.db
```

## Stable Hardlink (`latest-`)

Uncompressed local backups — those made by the onlineapi and vacuum engines in [restinpieces-backup](https://github.com/caasmo/restinpieces-backup) — get a stable hardlink named `latest-{dbName}` pointing to the most recent snapshot.

The naming contract is defined by the [backup package](../backup/backup.go):

```go
const LatestFmt = "latest-%s"   // package backup
const LatestGlob = "latest-*.db"
```


