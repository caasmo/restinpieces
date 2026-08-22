# `ripc` - Command-Line Interface for RestInPieces

`ripc` is a CLI tool for managing RestInPieces application instances. It handles the creation of new application databases and provides tools for managing the secure configuration store, authentication settings, and background jobs.

# Content

- [Relationship with `ripdep`](#relationship-with-ripdep)
- [Installation](#installation)
- [Global Options](#global-options)
- [Usage](#usage)
- [Commands](#commands)
  - [app](#app)
  - [get](#get)
  - [paths](#paths)
  - [dump](#dump)
  - [scopes](#scopes)
  - [set](#set)
  - [save](#save)
  - [scaffold](#scaffold)
  - [migrate](#migrate)
  - [list](#list)
  - [diff](#diff)
  - [rollback](#rollback)
  - [job](#job)
  - [log](#log)
  - [help](#help)

## Relationship with `ripdep`

`ripc` is a **low-level primitive** that runs on the server machine, operating directly on the local SQLite database and age key files. [`ripdep`](ripdep.md) ([source](../scripts/ripdep)) is a high-level orchestrator that runs on your local machine (or any machine with SSH access to the server), managing remote operations over SSH.

-   **Server-side:** `ripc` is designed to run on the production server itself — it reads and writes the database file and age key from the local filesystem. It is not a remote-control tool.
-   **Unopinionated:** `ripc` provides direct configuration and state manipulation without enforcing workflows.
-   **Composable:** It is designed with a stable interface meant to be called by other tools, CI/CD, or custom scripts.
-   **Stable Foundation:** `ripc` is versioned conservatively. This allows `ripdep` to iterate on new workflows and user-facing features without modifying the core server-side tool.

## Installation

You can download pre-built binaries from the [GitHub Releases](https://github.com/caasmo/restinpieces/releases) page, or install it using Go:

```bash
go install github.com/caasmo/restinpieces/cmd/ripc
```

## Global Options

`ripc` uses global settings that can be provided via flags or environment variables.

-   `-agekey`: Path to the `age` identity file (private key). Can also be set via the `RIPC_AGE_KEY_PATH` environment variable. One of the two must be provided.
-   `-dbpath`: Path to the SQLite database file. Can also be set via the `RIPC_DB` environment variable. One of the two must be provided.

The resolution order is: **flag → environment variable → error**. A flag, when present, always takes precedence over its corresponding environment variable.  If neither is provided, `ripc` exits with an error.

## Usage

```
ripc [global options] <command> [options]
```

If `RIPC_DB` and `RIPC_AGE_KEY_PATH` are set in your environment, flags can be omitted
entirely:
```
export RIPC_DB=data/app.db
export RIPC_AGE_KEY_PATH=age_key.txt
ripc list
```

## Commands

`ripc` uses flat top-level commands, one per operation. For example, to list configuration versions, you would use `ripc list`.

### `app`

Manages the application lifecycle.

-   **`create`**: Creates a new application instance, including the database file and a default, encrypted configuration. The database file must not already exist.
    ```bash
    ripc app create
    ```

### `get [filter]`

Retrieves configuration values by path, optionally filtered.

    ripc get "server.http_port"

### `paths [filter]`

Lists all available TOML paths in the configuration, optionally filtered.

    ripc paths
    ripc paths "server.*"

### `dump`

Decrypts and outputs the latest configuration stored in the database for the given scope. Three modes are available:

**Default mode (no flags)**:
Writes the decrypted TOML configuration exactly as it was saved to the database, with no transformation. If `ripc save myconfig.toml` stored `[server]\naddr = ":9090"\n`, that is exactly what you get. This is the canonical way to see what was saved.

    ripc dump
    ripc dump --scope myapp

**`--zero`**:
Fills in zero values (`0`, `""`, `false`, `null`) for every configuration key not present in the stored TOML, producing a complete TOML document where only the keys that were explicitly configured carry non-zero values. Useful for seeing which fields are explicitly configured vs left at their zero value.

    ripc dump --zero
    ripc dump --zero --scope myapp

**`--runtime`**:
Merges the stored TOML configuration with the framework's built-in defaults.  Every key in the output has a value — either the value from storage or the framework default. This mirrors the full configuration the server would use at startup.

**Warning**: framework defaults include dynamically generated secrets (JWT signing keys, OTP secrets, etc.). If those fields are not present in the stored TOML, the output shows freshly generated random strings on every invocation — they do not correspond to any secret actually in use. To see what the server actually uses, ensure secrets are part of the stored TOML or use default mode to inspect the stored data directly.

    ripc dump --runtime
    ripc dump --runtime --scope myapp

`--zero` and `--runtime` are mutually exclusive. Using neither flag
produces the default raw output.

### `scopes`

Lists all unique configuration scopes.

    ripc scopes

### `set <path> <value>`

Sets a configuration value at a given path.

    ripc set -desc "Update port" server.http_port 8080

### `save <file>`

Saves the contents of a file to the configuration store.

    ripc save -scope myapp config.toml

### `scaffold <type> <key>`

Scaffold writes a full config entry with defaults. Unlike `set` which writes one value, scaffold writes the whole entry at once. The key is a label you choose. It must not already exist.

Types:
- `backup-online` — writes `backup.online.<key>` for the Online Backup API. Defaults: `frequency` 15m, `pages_per_step` 100, `sleep_interval` 10ms, `compression` false, `source_path` and `dest_path` empty.
- `backup-vacuum` — writes `backup.vacuum.<key>` for VACUUM INTO. Defaults: `frequency` 15m, `compression` false, `source_path` and `dest_path` empty.
- `backup-sqlite-rsync` — writes `backup.sqlite-rsync.entries.<key>` for sqlite-rsync. Defaults: `sync_timeout` 15m, `source_path` empty. Creates `[backup.sqlite-rsync]` with `listen_addr` 127.0.0.1:54321 if missing.
- `oauth2` — writes `oauth2_providers.<key>` with `pkce` true and empty `name`, `client_id`, `client_secret`, and URLs.

After scaffolding you must set `source_path`. It supports absolute and relative paths. Relative paths resolve against the app's current working directory (CWD). With the canonical systemd service ([restinpieces.service](../restinpieces.service)) the CWD is `/home/<app>`, so use `data/` as prefix (e.g. `data/app.db`).


```
ripc scaffold backup-online app-online
ripc scaffold backup-vacuum app-vacuum
ripc scaffold backup-sqlite-rsync app-rsync
ripc set backup.online.app-online.source_path /var/data/app.db
```

```
ripc scaffold oauth2 my_google
ripc set oauth2_providers.my_google.client_id "..."
```

Use `get` to inspect and `paths` to list properties.

**Example: add a new backup**

Scaffold creates `backup.online.app-online` with defaults:

```
ripc scaffold backup-online app-online
```

Set the database path and adjust defaults:

```
ripc set backup.online.app-online.source_path /var/data/app.db
ripc set backup.online.app-online.dest_path /var/backups
ripc set backup.online.app-online.frequency 6h
ripc set backup.online.app-online.compression true
```

Verify:

```
ripc get backup.online.app-online
ripc paths backup.online.app-online
```

For sqlite-rsync:

```
ripc scaffold backup-sqlite-rsync app-rsync
ripc set backup.sqlite-rsync.entries.app-rsync.source_path /var/data/app.db
```

### `migrate`

Migrates the stored configuration to the current framework version. If no
configuration exists for the scope, it creates one with default values.

    ripc migrate

**After upgrading the restinpieces framework**, run `ripc migrate` to:
- Remove stale configuration keys that no longer exist in the framework
- Add new configuration keys with their default values
- Preserve all existing configured values

The command is safe to run at any time — it never overwrites existing values with
defaults unless the field was newly added to the framework.

### `list [scope]`

Lists configuration versions, optionally filtered by scope.

    ripc list
    ripc list myapp

### `diff <generation>`

Shows differences between the latest configuration and a previous version.

    ripc diff -scope myapp 1

### `rollback <generation>`

Rolls back to a previous configuration version by its generation number (from `list`).

    ripc rollback -scope myapp 3

### `job`

Manages background jobs in the queue.

-   **`list [limit]`**: Lists jobs in the queue, optionally limiting the number of results.
    -   `ripc job list 10`
-   **`rm <job_id>`**: Removes a job from the queue by its ID.
    -   `ripc job rm 123`

### `log`

Manages the log database.

-   **`init`**: Initializes the log database and schema.
    -   `ripc log init`

### `help`

Shows usage information for a specific command.

```bash
ripc help get
```