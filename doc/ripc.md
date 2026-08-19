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

Creates a group of configuration properties in one command. Unlike `set` which writes a single property, scaffold writes all the properties of an entry at once, populated with sensible defaults. The key is a user-chosen label and must not already exist.

Types:
- `backup` — writes `strategy` (online), `compression` (false), `frequency` (15m), `online_api_pages_per_step` (100), `online_api_sleep_interval` (10ms), and `source_path` (empty) under `backup.files.<key>`.  After scaffolding, you **must** set `source_path` to the path of your database file. Supports absolute and relative paths. Relative paths resolve against the application's current working directory (CWD). When deployed via the canonical systemd service ([restinpieces.service](../restinpieces.service)), the CWD is `/home/<app>` and databases typically live under `data/`, so a relative `source_path` should start with `data/` (e.g. `data/app.db`).
- `oauth2` — writes `pkce` (true), `name`, `client_id`, `client_secret`, and URLs (all empty) under `oauth2_providers.<key>`.


```
ripc scaffold backup app_db
ripc set backup.files.app_db.source_path /var/data/app.db
```

```
ripc scaffold oauth2 my_google
ripc set oauth2_providers.my_google.client_id "..."
```

Use `get` to inspect the entry and `paths` to list its properties.

**Example: add a new backup db file**

Scaffold creates `strategy` (online), `compression` (false), `frequency` (15m), and an empty `source_path` under `backup.files.app_db`:

```
ripc scaffold backup app_db
```

Set the database path and adjust the defaults:

```
ripc set backup.files.app_db.source_path /var/data/app.db
ripc set backup.files.app_db.frequency 6h
ripc set backup.files.app_db.compression true
```

Verify with `get` and `paths`:

```
ripc get backup.files.app_db
ripc paths backup.files.app_db
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