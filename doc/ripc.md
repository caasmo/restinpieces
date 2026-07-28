# `ripc` - Command-Line Interface for RestInPieces

`ripc` is a CLI tool for managing RestInPieces application instances. It handles the creation of new application databases and provides tools for managing the secure configuration store, authentication settings, and background jobs.

# Content

- [Relationship with ripdep](#relationship-with-ripdep)
- [Installation](#installation)
- [Global Options](#global-options)
- [Usage](#usage)
- [Commands](#commands)
  - [app](#app)
  - [config](#config)
  - [job](#job)
  - [help](#help)

## Relationship with `ripdep`

`ripc` is a **low-level primitive**, whereas [`ripdep`](ripdep.md) ([source](../scripts/ripdep)) is a high-level orchestrator.

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

-   `-agekey`: Path to the `age` identity file (private key). Can also be set via the
    `RIPC_AGE_KEY_PATH` environment variable. One of the two must be provided.
-   `-dbpath`: Path to the SQLite database file. Can also be set via the
    `RIPC_DB` environment variable. One of the two must be provided.

The resolution order is: **flag → environment variable → error**. A flag, when
present, always takes precedence over its corresponding environment variable.
If neither is provided, `ripc` exits with an error.

## Usage

```
ripc [global options] <command> <subcommand> [options]
```

If `RIPC_DB` and `RIPC_AGE_KEY_PATH` are set in your environment, flags can be omitted
entirely:
```
export RIPC_DB=data/app.db
export RIPC_AGE_KEY_PATH=age_key.txt
ripc config list
```

## Commands

`ripc` uses a command and subcommand structure. For example, to list configuration versions, you would use `ripc config list`.

### `app`

Manages the application lifecycle.

-   **`create`**: Creates a new application instance, including the database file and a default, encrypted configuration. The database file must not already exist.
    ```bash
    ripc  app create
    ```

### `config`

Manages the secure configuration store.

-   **`dump`**: Decrypts and outputs the latest configuration stored in the database
    for the given scope. Three modes are available:

    **Default mode (no flags)**:
    Writes the decrypted TOML configuration exactly as it was saved to the
    database, with no transformation. If `ripc config save myconfig.toml` stored
    `[server]\naddr = ":9090"\n`, that is exactly what you get. This is the
    canonical way to see what was saved.

        ripc config dump
        ripc config dump --scope myapp

    **`--zero`**:
    Fills in zero values (`0`, `""`, `false`, `null`) for every configuration
    key not present in the stored TOML, producing a complete TOML document where
    only the keys that were explicitly configured carry non-zero values. Useful
    for seeing which fields are explicitly configured vs left at their zero value.

        ripc config dump --zero
        ripc config dump --zero --scope myapp

    **`--runtime`**:
    Merges the stored TOML configuration with the framework's built-in defaults.
    Every key in the output has a value — either the value from storage or the
    framework default. This mirrors the full configuration the server would use
    at startup.

    **Warning**: framework defaults include dynamically generated secrets
    (JWT signing keys, OTP secrets, etc.). If those fields are not present in
    the stored TOML, the output shows freshly generated random strings on every
    invocation — they do not correspond to any secret actually in use. To see
    what the server actually uses, ensure secrets are part of the stored TOML
    or use default mode to inspect the stored data directly.

        ripc config dump --runtime
        ripc config dump --runtime --scope myapp

    `--zero` and `--runtime` are mutually exclusive. Using neither flag
    produces the default raw output.
-   **`get [filter]`**: Retrieves configuration values by path, optionally filtered.
    -   `ripc  config get "server.http_port"`
-   **`init`**: Creates a new configuration with default values.
    -   `ripc  config init -scope myapp`
-   **`list [scope]`**: Lists configuration versions, optionally filtered by scope.
    -   `ripc  config list`
    -   `ripc  config list myapp`
-   **`paths [filter]`**: Lists all available TOML paths in the configuration, optionally filtered.
    -   `ripc  config paths`
    -   `ripc  config paths "server.*"`
-   **`rollback <generation>`**: Rolls back to a previous configuration version by its generation number (from `config list`).
    -   `ripc  config rollback -scope myapp 3`
-   **`save <file>`**: Saves the contents of a file to the configuration store.
    -   `ripc  config save -scope myapp config.toml`
-   **`scopes`**: Lists all unique configuration scopes.
    -   `ripc  config scopes`
-   **`set <path> <value>`**: Sets a configuration value at a given path.
    -   `ripc  config set -desc "Update port" server.http_port 8080`
-   **`diff <generation>`**: Shows differences between the latest configuration and a previous version.
    -   `ripc  config diff -scope myapp 1`

### `job`

Manages background jobs in the queue.

-   **`add-backup`**: Adds a new recurrent database backup job.
    -   `ripc  job add-backup --interval 24h`
-   **`list [limit]`**: Lists jobs in the queue, optionally limiting the number of results.
    -   `ripc  job list 10`
-   **`rm <job_id>`**: Removes a job from the queue by its ID.
    -   `ripc  job rm 123`
-   **`add`**: (Advanced) Adds a generic job to the queue with specified parameters.
    -   `ripc  job add --type my_job --payload '''{"key":"value"}'''`

### `help`

Shows usage information for a specific command.

```bash
ripc help config
```
