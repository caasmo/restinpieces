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
  - [walk](#walk-filter)
  - [add](#add-path-value)
  - [rm](#rm-path-value)
  - [update](#update-label)
  - [save](#save-file)
  - [scaffold](#scaffold-type-label)
  - [migrate](#migrate)
  - [list](#list-scope)
  - [diff](#diff-generation)
  - [rollback](#rollback-generation)
  - [job](#job)
  - [log](#log)
  - [version](#version)
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

Commands that work on configuration take a `--scope` option to choose which document they read or write. When it is not given, the command uses `application`, the scope the server reads at startup. Scopes sit side by side in the same database, and a value stored in one is not visible in another. `ripc scopes` lists the scopes that have stored configuration.

### `app`

Manages the application lifecycle.

-   **`create`**: Creates a new application instance, including the database file and a default, encrypted configuration. The database file must not already exist.
    ```bash
    ripc app create
    ```

### `get [filter]`

Retrieves stored configuration values by path.

    ripc get "server.addr"

With no filter, every stored value is printed. With a filter, only values whose path contains the filter string are shown; for example, `ripc get backup` shows every value whose path contains `backup`.

    ripc get
    ripc get backup

A single match prints the value itself followed by one newline, so `ripc get <path> > file` writes a normal text file that `ripc set <path> @file` reads back unchanged. Two or more matches print one `path = value` line per match, sorted by path.

Only values stored in the scope are shown; keys the framework fills in with defaults are not. Use `ripc dump --runtime` for the full configuration the server uses.

### `paths [filter]`

Lists the path of every stored configuration value, one per line.

    ripc paths

If a string is given, only paths containing that string are shown. For example, `ripc paths backup` shows all paths containing `backup`.

    ripc paths backup

Like `get`, it only sees values stored in the scope, not defaults filled in by the framework.

### `dump`

Decrypts and outputs the latest configuration stored in the database for the given scope. Three modes are available:

**Default mode (no flags)**:
Writes the decrypted TOML configuration exactly as it was saved to the database, with no transformation. If `ripc save myconfig.toml` stored `[server]\naddr = ":9090"\n`, that is exactly what you get. This is the canonical way to see what was saved.

    ripc dump
    ripc dump --scope myapp

**`--zero`**:
Prints every configuration key: stored keys keep their value, and keys not present in the stored TOML are filled in with zero values (`0`, `""`, `false`). Useful for seeing which fields are explicitly configured.

    ripc dump --zero
    ripc dump --zero --scope myapp

**`--runtime`**:
Merges the stored TOML configuration with the framework's built-in defaults.  Every key in the output has a value — either the value from storage or the framework default. This mirrors the full configuration the server would use at startup.

    ripc dump --runtime
    ripc dump --runtime --scope myapp

`--zero` and `--runtime` are mutually exclusive. Using neither flag produces the default raw output.

### `scopes`

Lists the scopes that have stored configuration, one per line.

    ripc scopes

### `set <path> <value>`

Sets an existing configuration value.

    ripc set server.addr ":9090"

The path must already exist; `set` refuses unknown paths. Use [`paths`](#paths-filter) to find one and [`scaffold`](#scaffold-type-label) to create a new section. `--desc` records a description for the saved version, shown by [`list`](#list-scope).

The value is parsed as a TOML value, falling back to a plain string when it is not valid TOML. A value that starts with `@` is read from the named file instead.

To set several values in a loop, see [`walk`](#walk-filter).

### `walk <filter>`

Sets matching configuration values in a loop, one at a time. It is the bulk form of `set`.

    ripc walk server
    ripc walk --scope myapp backup

`filter` is required; only paths containing the filter string are walked, in sorted order. Each step shows the path, the comment above it, and the stored value:

```
server.addr  (Address the server listens on)
New value [:8080]:
```

An empty line keeps the stored value. Any other line replaces it, parsed the same way as `set`: a leading `@` reads the value from a file, otherwise the line is parsed as a TOML value with a quoted string fallback. End of input stops the walk early. With no edits, the command prints `No changes.` and saves nothing. With at least one edit, the result is validated and saved with the description `Walk updates`.

### `add <path> <value>`

Appends a value to a list key.

    ripc add block_user_agent.agents SemrushBot
    ripc add block_host.allowed_hosts example.com

Only `block_user_agent.agents` and `block_host.allowed_hosts` can be added to. Other list keys, such as `acme.domains`, are changed with [`set`](#set-path-value).

`add` can be run again safely: adding a value that is already present leaves the configuration unchanged.

### `rm <path> [value]`

Removes one item from a configuration collection.

Tables are named by their full path:

    ripc rm backup.vacuum.logs-vacuum

Array items are named by value:

    ripc rm block_user_agent.agents SemrushBot

Removable tables are `backup.online.<label>`, `backup.vacuum.<label>`, `backup.sqlite-rsync.entries.<label>`, `backup.s3-upload.<label>`, `backup.s3-download.<label>`, `scheduler.jobs.<label>`, `oauth2.<label>`, `acme.dns-01.<label>`, `limiter.token_bucket.<label>` and `limiter.fixed_window.<label>`. Removable array items are `block_user_agent.agents`, `block_host.allowed_hosts`, `block_oversized_request.excluded_paths` and `acme.domains`. Scalar keys like `server.addr` are refused.

Framework-owned entries such as `limiter.token_bucket.password_hashing` are refused as well.

Removing a stored item brings back the framework default when one exists. For example, `block_oversized_request.excluded_paths` returns to `/api/upload` and `/api/import`. To keep an array empty instead, set it to an empty array with `ripc set block_oversized_request.excluded_paths "[]"`.

### `update <label>`

`update` writes values you should not write by hand. Give it a label to update all its configuration paths at once, or one configuration path to update a single value. The following labels are supported:

| Label | What it does |
|-------|----------------|
| `jwt` | Generates 43-character random secrets that sign auth tokens, password reset links, email change codes, verification mail, and OAuth2 state. |
| `tls` | Moves the staged `acme.certificate` and `acme.private_key` into the live `server.tls.certificate` and `server.tls.private_key`. |
| `block_user_agent` | Downloads the upstream bot list into `block_user_agent.agents`. |

    ripc update jwt
    ripc update jwt.auth_secret
    ripc update tls

### `save <file>`

Replaces the stored configuration for a scope with the contents of a file.

    ripc save config.toml
    ripc save --scope myapp config.toml

The format is taken from the file extension (`.toml` for `config.toml`) unless `--format` is given. Unlike the other writing commands, `save` neither merges nor validates: the file becomes the stored document as-is, so it must be a complete configuration.

### `scaffold <type> <label>`

Creates a complete configuration section with defaults. `set` changes a single field; `scaffold` creates an entire section at once. `<label>` is the label you choose for the new section; it must not already exist and must not contain dots or spaces. `scaffold` always writes to the `application` scope. If the parent section is missing, run [`migrate`](#migrate) first.

After scaffolding, `ripc walk <path>` shows each field of the new section one at a time — path, the comment above it and its current value — and lets you fill them in a single pass.

| Type | Path | Description |
|------|------|-------------|
| `backup-online` | `backup.online.<label>` | Online Backup API ([restinpieces-backup](https://github.com/caasmo/restinpieces-backup)) |
| `backup-vacuum` | `backup.vacuum.<label>` | VACUUM INTO ([restinpieces-backup](https://github.com/caasmo/restinpieces-backup)) |
| `backup-sqlite-rsync` | `backup.sqlite-rsync.entries.<label>` | sqlite-rsync ([restinpieces-backup](https://github.com/caasmo/restinpieces-backup)) |
| `backup-s3-upload` | `backup.s3-upload.<label>` | Uploads one file to S3 ([restinpieces-backup](https://github.com/caasmo/restinpieces-backup)) |
| `backup-s3-download` | `backup.s3-download.<label>` | Downloads the newest backup for a label from S3 ([restinpieces-backup](https://github.com/caasmo/restinpieces-backup)) |
| `oauth2` | `oauth2.<label>` | OAuth2 provider |
| `acme-dns-01` | `acme.dns-01.<label>` | DNS-01 challenge (Cloudflare, [restinpieces-acme](https://github.com/caasmo/restinpieces-acme)) |
| `job` | `scheduler.jobs.<label>` | Job that runs on a schedule |
| `limiter-token-bucket` | `limiter.token_bucket.<label>` | Token-bucket rate limiter (`limit`, `burst`) |
| `limiter-fixed-window` | `limiter.fixed_window.<label>` | Fixed-window rate limiter per identity (`prefix`, `window`, `max`) |

**Example: configure backup of type `sqlite-rsync` for application SQLite file `/tmp/app.db`**

Scaffold the configuration with defaults by providing a label — a string to identify the backup:

```
ripc scaffold backup-sqlite-rsync myapp
```

```
Successfully scaffolded 'myapp' in scope 'application'

myapp:
  source_path = ""
  sync_timeout = "15m"

Set each value in turn:
	ripc walk backup.sqlite-rsync.entries.myapp

Reload the app:
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

**Example: declare a job that runs on a schedule**

Use the certificate renewal job from [restinpieces-acme](https://github.com/caasmo/restinpieces-acme). Your app registers its handler, then you declare when it runs:

```
ripc scaffold job acme_cert
```

```
Successfully scaffolded 'acme_cert' in scope 'application'

acme_cert:
  job_type = ""
  interval = "1h0m0s"
  activated = false

Set each value in turn:
	ripc walk scheduler.jobs.acme_cert

Reload the app:
	systemctl reload myapp
Deactivate: ripc set scheduler.jobs.acme_cert.activated false
```

Point the entry at the handler from `restinpieces-acme`, then turn it on:

```
ripc set scheduler.jobs.acme_cert.job_type acme_cert
ripc set scheduler.jobs.acme_cert.activated true
systemctl reload myapp
```

Verify:

```
ripc get scheduler.jobs.acme_cert
ripc job list
```

The scheduler adds one pending run. When it completes, the next run is added automatically. See the handler and renewal details in [restinpieces-acme](https://github.com/caasmo/restinpieces-acme).

### `migrate`

Migrates the stored configuration to the current framework version:

- removes configuration keys that no longer exist in the framework
- adds newly introduced configuration keys with their default values
- preserves every configured value


```
ripc migrate
```

The command is safe to run at any time — it never overwrites existing values with defaults unless the field was newly added to the framework. It only touches the `application` scope.

### `list [scope]`

Lists configuration versions, newest first.

    ripc list
    ripc list myapp

The scope is a positional argument here, not an option. The output looks like this:

```
Gen  Scope        Created At             Format  Description
---  ------------ ---------------------  ------  -----------
  0  application  2026-06-01T12:00:00Z   toml    Updated field 'server.addr'
  1  application  2026-06-01T11:59:00Z   toml    Initial default configuration
```

`Gen` numbers the versions from newest (`0`) to oldest. [`diff`](#diff-generation) and [`rollback`](#rollback-generation) take these numbers; rollback requires `1` or higher, because `0` is already the latest. `Description` is the `--desc` value given when the version was saved, or a default chosen by the command that saved it; `Format` is the format it was saved in.

### `diff <generation>`

Shows a unified diff between a previous version and the latest configuration: `-` lines are the previous version, `+` lines are the latest.

    ripc diff --scope myapp 1

### `rollback <generation>`

Restores a previous version as the new latest. The generation number comes from [`list`](#list-scope) and must be `1` or higher; generation `0` is already the latest.

    ripc rollback --scope myapp 3

### `job`

Manages background jobs in the queue.

-   **`list [limit]`**: Lists jobs in the queue, optionally limiting the number of results.
    -   `ripc job list 10`
-   **`rm <job_id>`**: Removes a job from the queue by its ID.
    -   `ripc job rm 123`

### `log`

Reads the log database, printing one line per record, oldest first: creation time, level, message, and the stored JSON data. The optional filter matches the data column with SQL LIKE, so `%` and `_` work as wildcards, and `log.batch.db_path` in the configuration selects the database file.

-   `ripc log` prints the last 20 records.
-   `ripc log 404` prints the last 20 records whose data contains `404`.
-   `ripc log -n 50` prints the last 50 records.
-   `ripc log -f` prints the last 20 records, then keeps printing new records until interrupted.

Write the flags before the filter, for example `ripc log -n 50 404`: the flag parser stops at the first value that is not a flag.

### `version`

Prints the `ripc` build tag and commit. It runs without a database or age key.

    ripc version

### `help`

Shows usage information for a specific command.

```bash
ripc help get
```
