# `ripc` - Command-Line Interface for RestInPieces

`ripc` is a CLI tool for managing RestInPieces application instances. It handles the creation of new application databases and provides tools for managing the secure configuration store, authentication settings, and background jobs.

## Installation

```bash
go install github.com/caasmo/restinpieces/cmd/ripc
```

## Global Options

All commands require the following global flags:

-   `-age-key`: Path to the `age` identity file (private key).
-   `-dbpath`: Path to the SQLite database file.

## Usage

```
ripc [global options] <command> <subcommand> [options]
```

## Commands

`ripc` uses a command and subcommand structure. For example, to list configuration versions, you would use `ripc config list`.

### `app`

Manages the application lifecycle.

-   **`create`**: Creates a new application instance, including the database file and a default, encrypted configuration. The database file must not already exist.
    ```bash
    ripc -age-key age.key -dbpath app.db app create
    ```

### `config`

Manages the secure configuration store.

-   **`dump`**: Outputs the latest configuration in plaintext.
    -   `ripc -age-key age.key -dbpath app.db config dump -scope myapp`
-   **`get [filter]`**: Retrieves configuration values by path, optionally filtered.
    -   `ripc -age-key age.key -dbpath app.db config get "server.http_port"`
-   **`init`**: Creates a new configuration with default values.
    -   `ripc -age-key age.key -dbpath app.db config init -scope myapp`
-   **`list [scope]`**: Lists configuration versions, optionally filtered by scope.
    -   `ripc -age-key age.key -dbpath app.db config list`
    -   `ripc -age-key age.key -dbpath app.db config list myapp`
-   **`paths [filter]`**: Lists all available TOML paths in the configuration, optionally filtered.
    -   `ripc -age-key age.key -dbpath app.db config paths`
    -   `ripc -age-key age.key -dbpath app.db config paths "server.*"`
-   **`rollback <generation>`**: Rolls back to a previous configuration version by its generation number (from `config list`).
    -   `ripc -age-key age.key -dbpath app.db config rollback -scope myapp 3`
-   **`save <file>`**: Saves the contents of a file to the configuration store.
    -   `ripc -age-key age.key -dbpath app.db config save -scope myapp config.toml`
-   **`scopes`**: Lists all unique configuration scopes.
    -   `ripc -age-key age.key -dbpath app.db config scopes`
-   **`set <path> <value>`**: Sets a configuration value at a given path.
    -   `ripc -age-key age.key -dbpath app.db config set -desc "Update port" server.http_port 8080`
-   **`diff <generation>`**: Shows differences between the latest configuration and a previous version.
    -   `ripc -age-key age.key -dbpath app.db config diff -scope myapp 1`

### `auth`

Manages authentication settings. These commands operate on the `application` scope.

-   **`add-oauth2 <provider>`**: Adds a new, empty OAuth2 provider configuration.
    -   `ripc -age-key age.key -dbpath app.db auth add-oauth2 github`
-   **`rm-oauth2 <provider>`**: Removes an OAuth2 provider configuration.
    -   `ripc -age-key age.key -dbpath app.db auth rm-oauth2 github`
-   **`rotate-jwt-secrets`**: Generates new random secrets for all JWTs.
    -   `ripc -age-key age.key -dbpath app.db auth rotate-jwt-secrets`

### `job`

Manages background jobs in the queue.

-   **`add-backup`**: Adds a new recurrent database backup job.
    -   `ripc -age-key age.key -dbpath app.db job add-backup --interval 24h`
-   **`list [limit]`**: Lists jobs in the queue, optionally limiting the number of results.
    -   `ripc -age-key age.key -dbpath app.db job list 10`
-   **`rm <job_id>`**: Removes a job from the queue by its ID.
    -   `ripc -age-key age.key -dbpath app.db job rm 123`
-   **`add`**: (Advanced) Adds a generic job to the queue with specified parameters.
    -   `ripc -age-key age.key -dbpath app.db job add --type my_job --payload '''{"key":"value"}'''`

### `help`

Shows usage information for a specific command.

```bash
ripc help config
ripc help auth
```