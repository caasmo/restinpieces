# Local Backups

The built-in backup system follows a two-step push-pull design:

1. **Push (server-side)**: A recurrent background job creates compressed SQLite snapshots locally on the server.
2. **Pull (client-side)**: A standalone client retrieves those snapshots from the server via SFTP.

This decouples backup creation from retrieval — backups are produced on the server by the [background job handler](../queue/handlers/backup_local.go) and pulled to any number of client machines using the [restinpieces-backup-client](https://github.com/caasmo/restinpieces-backup-client) client.

## Job Activation

Use `ripc` to insert a recurrent backup job into the queue:

```bash
ripc job add-backup --interval 24h
```

This creates a job with type `job_type_backup_local` that the scheduler picks up on its next tick. The job is recurrent — after each successful run it reschedules itself for the next interval.

## Job Configuration

Configuration lives under the `[backup_local]` TOML section, defined in [config/config.go](../config/config.go):

| Field | Type | Default | Description |
|---|---|---|---|
| `source_path` | string | `"restinpieces.db"` | Path to the source database to back up |
| `backup_dir` | string | `"backups"` | Directory where backup files are written |
| `strategy` | string | `"online"` | Backup strategy: `"online"` or `"vacuum"` |
| `pages_per_step` | int | `100` | Pages copied per step (online only) |
| `sleep_interval` | duration | `"10ms"` | Pause between steps (online only) |

Set via `ripc`:

```bash
ripc config set backup_local.source_path /data/myapp.db
ripc config set backup_local.backup_dir /data/backups
```

Configuration is hot-reloadable via `SIGHUP`.

## Backup Strategies

### Online (default)

Uses SQLite's [Online Backup API](https://sqlite.org/backup.html). Copies the database page-by-page, yielding between steps. Does **not** block writers. **Recommended for most production systems.**

Tune `pages_per_step` and `sleep_interval` to balance speed against I/O impact. Smaller values are gentler on concurrent workloads; `"0s"` runs the backup as fast as possible.

### Vacuum

Uses `VACUUM INTO` to create a clean, defragmented copy. Faster than online but places a read lock on the database, **blocking all write operations** for the entire duration. Suitable for low-write databases or scheduled maintenance windows.

## Pull Client

Backups are written to `backup_dir` as compressed archives named `{dbname}-{timestamp}-{strategy}.bck.gz`. To retrieve them, use the SFTP pull client from [restinpieces-backup-client](https://github.com/caasmo/restinpieces-backup-client):

```bash
go run github.com/caasmo/restinpieces-backup-client/cmd/sftp@latest \
  -host myserver.example.com \
  -user backup \
  -remote-dir /data/backups \
  -local-dir ./backups
```

The client finds the latest backup by filename, downloads it, decompresses it, and verifies integrity with `PRAGMA integrity_check`.
