# `ripc` - Command-Line Interface for RestInPieces

`ripc` is a CLI tool for managing RestInPieces application configuration. Configuration is stored as an age-encrypted TOML row in the main SQLite database.

# Content

- [Relationship with `ripdep`](#relationship-with-ripdep)
- [Installation](#installation)
- [Global Options](#global-options)
- [Usage](#usage)
- [Commands](#commands)
  - [app](#app)
  - [get](#get-filter)
  - [paths](#paths-filter)
  - [dump](#dump)
  - [scopes](#scopes)
  - [set](#set-path-value)
  - [save](#save-file)
  - [scaffold](#scaffold-type-label)
  - [migrate](#migrate)
  - [list](#list-scope)
  - [diff](#diff-generation)
  - [rollback](#rollback-generation)
  - [job](#job)
  - [log](#log)
  - [help](#help)

## Relationship with `ripdep`

`ripc` runs on the server machine, operating directly on the local SQLite database and age key files. [`ripdep`](ripdep.md) ([source](../scripts/ripdep)) is a high-level orchestrator that runs on your local machine (or any machine with SSH access to the server), managing remote operations over SSH.

-   **Server-side:** `ripc` is designed to run on the production server itself 
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

The resolution order is: flag → environment variable → error. A flag, when present, always takes precedence over its corresponding environment variable.  If neither is provided, `ripc` exits with an error.

## Usage

```
ripc [global options] <command> [options]
```

If `RIPC_DB` and `RIPC_AGE_KEY_PATH` are set in your environment, flags can be omitted entirely:
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

Retrieves configuration values by path.

    ripc get "server.addr"

If a string is given, only values whose path contains that string are shown. For example, `ripc get backup` shows all configuration whose path contains `backup`.

    ripc get backup

### `paths [filter]`

Lists all available TOML paths in the configuration.

    ripc paths

If a string is given, only paths containing that string are shown. For example, `ripc paths backup` shows all paths containing `backup`.

    ripc paths backup

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

    ripc dump --runtime
    ripc dump --runtime --scope myapp

`--zero` and `--runtime` are mutually exclusive. Using neither flag produces the default raw output.

### `scopes`

Lists all unique configuration scopes.

    ripc scopes

### `set <path> <value>`

Sets a configuration value at a given path.

    ripc set -desc "Update port" server.http_port 8080

### `save <file>`

Saves the contents of a file to the configuration store.

    ripc save -scope myapp config.toml

### `scaffold <type> <label>`

Creates a complete configuration section with defaults. `set` changes a single field; `scaffold` creates an entire section at once. `<label>` is the label you choose for the new section and must not already exist.

| Type | Path | Description |
|------|------|-------------|
| `backup-online` | `backup.online.<label>` | Online Backup API |
| `backup-vacuum` | `backup.vacuum.<label>` | VACUUM INTO |
| `backup-sqlite-rsync` | `backup.sqlite-rsync.entries.<label>` | sqlite-rsync |
| `oauth2` | `oauth2_providers.<label>` | OAuth2 provider |

**Example: configure backup of type `sqlite-rsync` for application SQLite file `/tmp/app.db`**

Scaffold the configuration with defaults by providing a label — a string to identify the backup:

```
ripc scaffold backup-sqlite-rsync myapp
```

```
Successfully scaffolded backup 'myapp' in scope 'application'

myapp:
  source_path = ""
  sync_timeout = "15m"

Next steps:
1. Set the origin file to replicate (required):
	ripc set backup.sqlite-rsync.entries.myapp.source_path /path/to/app.db
2. Reload the app:
	systemctl reload myapp
Deactivate: ripc set backup.sqlite-rsync.entries.myapp.source_path ""
```

Set `source_path` using the same label:

```
ripc set backup.sqlite-rsync.entries.myapp.source_path /tmp/app.db
```

Verify:

```
ripc get myapp
ripc paths myapp
```

### `migrate`

Migrates the stored configuration to the current framework version. 

- it removes stale configuration keys that no longer exist in the framework
- adds the new framewrok configuration keys with their default values
- preserves all existing configured values

    ripc migrate

The command is safe to run at any time — it never overwrites existing values with defaults unless the field was newly added to the framework.

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
