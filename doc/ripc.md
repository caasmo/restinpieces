# `ripc` - Command-Line Interface for RestInPieces

`ripc` is a CLI tool for managing RestInPieces application instances. It handles the creation of new application databases and provides tools for managing the secure configuration store, authentication settings, and background jobs.

## Relationship with `ripdep`

`ripc` is a **low-level primitive**, whereas [`ripdep`](ripdep.md) ([source](../scripts/ripdep)) is a high-level orchestrator.

-   **Unopinionated:** `ripc` provides direct configuration and state manipulation without enforcing workflows.
-   **Composable:** It is designed with a stable interface meant to be called by other tools, CI/CD, or custom scripts.
-   **Stable Foundation:** `ripc` is versioned conservatively. This allows `ripdep` to iterate on new workflows and user-facing features without modifying the core server-side tool.

## Installation

```bash
go install github.com/caasmo/restinpieces/cmd/ripc
```

## Global Options

`ripc` uses global settings that can be provided via flags or discovered automatically.

-   `-agekey`: Path to the `age` identity file (private key). If not provided, `ripc` will look for `age_key.txt` or `age.key` in the current directory.
-   `-dbpath`: Path to the SQLite database file. If not provided, `ripc` will look for `app.db` in the current directory.

Flags always take precedence over discovered files.

## Usage

```
ripc [global options] <command> <subcommand> [options]
```

If `age.key` and `app.db` are in the current directory, you can run commands without global options:
```
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

-   **`dump`**: Outputs the latest configuration in plaintext.
    -   `ripc  config dump -scope myapp`
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
