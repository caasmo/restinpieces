# `ripdep` - Restinpieces Deployment & Operations Tool

`ripdep` is a CLI tool for building, packaging, and deploying [RestInPieces](https://github.com/caasmo/restinpieces) framework applications, and for running operations on the servers where they are deployed.

## Content

- [Relationship with ripc](#relationship-with-ripc)
- [Installation](#installation)
- [Standard Application Layout](#standard-application-layout)
- [Use Cases](#use-cases)
  - [First-Time Application Bootstrap](#1-first-time-application-bootstrap)
  - [Update Application](#2-update-application)
  - [Restore Application from Backup](#3-restore-application-from-backup)
  - [Deploy the Same Application Under a Different Name](#4-deploy-the-same-application-under-a-different-name)
- [Commands](#commands)
  - [build](#build)
  - [build-binary-release](#build-binary-release)
  - [pack](#pack)
  - [unpack](#unpack)
  - [restore](#restore)
  - [push](#push)
  - [install (Remote)](#install-remote)
  - [deploy](#deploy)
  - [undeploy](#undeploy)
  - [backup](#backup)
  - [cp](#cp)
  - [maintenance](#maintenance)
  - [ripc](#ripc)
  - [shell](#shell)
  - [status](#status)
  - [logs](#logs)
  - [restart](#restart)
  - [reload](#reload)
  - [stop](#stop)
  - [enable](#enable)
- [Debugging on a Remote Server](#debugging-on-a-remote-server)
  - [Check Status and Logs](#1-check-status-and-logs)
  - [Log in and Run Manually](#2-log-in-and-run-manually)
  - [Debug the Systemd Sandbox](#3-debug-the-systemd-sandbox)
- [Systemd Unit Contract](#systemd-unit-contract)
- [Project Configuration Scripts](#project-configuration-scripts)

## Relationship with `ripc`

`ripdep` runs on your local machine. It builds the application binary and [`ripc`](ripc.md) ([source](../cmd/ripc)), copies them to the remote server over SSH, and runs remote operations there. `ripc` is the on-server tool: it reads and writes the local SQLite database and age key files directly.

## Installation

Download the `ripdep` script and make it executable:

```bash
curl -O https://raw.githubusercontent.com/caasmo/restinpieces/refs/heads/master/scripts/ripdep
chmod +x ripdep
```

## Standard Application Layout

On the remote server the application lives in `/home/<app-name>`. The build commands produce artifacts that mirror the `bin/` and `data/` parts of this layout.

```text
/home/<app-name>/
├── age.key
├── bin
│   ├── <app-name> (e.g. restinpieces-litestream)
│   ├── ripc
│   └── ripdep-remote
├── data
│   └── app.db
```

*   **`age.key`**: the encryption key for the application's configuration.
*   **`bin/`**: the application binary, the `ripc` CLI for on-server management, and `ripdep-remote`, the server-side installer.
*   **`data/`**: the SQLite databases, including the log database. Systemd only allows writes here, so keep `log.batch.db_path` under `data/`.

## Use Cases

### 1. First-Time Application Bootstrap

Deploy a new application to a fresh server. `build` compiles the binary; `--with-agekey`, `--with-db-local`, and `--with-systemd-service` add the project's `age.key`, database, and systemd unit. `deploy` packs the build, uploads it, and runs the remote installer, which creates the service user, the `/home/<app-name>` layout, and the systemd unit.

Requirements:

*   The project is a git repository with no uncommitted changes and HEAD exactly on a tag; the tag is the version. `app.db` and `age.key` must be gitignored so the worktree check passes.
*   `age.key` and the database exist in the project directory; the options fail when a path is missing.
*   The target server is fresh (no application user or `/home/<app-name>` yet) and reachable over SSH with `sudo`.

Commands:

```bash
PROJECT_PATH="$PWD"
PROJECT_NAME=$(basename "$PROJECT_PATH")
BUILD_BASE="/tmp"
HOST="user@target-server.com"
VERSION=$(git -C "$PROJECT_PATH" describe --tags --abbrev=0)
BUILD_DIR="${BUILD_BASE}/${PROJECT_NAME}-${VERSION}"

# 1. Build a complete bootstrap artifact locally
./ripdep build "$PROJECT_PATH" "$BUILD_BASE" --with-agekey --with-systemd-service --with-db-local "$PROJECT_PATH/app.db"

# 2. Deploy to remote
./ripdep deploy "$HOST" "$BUILD_DIR"
```

The `deploy` command performs these steps:

```bash
HOST="user@target-server.com"
BUILD_DIR="/tmp/my-app"

# 1. Package the artifact into a tarball
TARBALL_PATH="${BUILD_DIR}.tar.gz"
./ripdep pack "$BUILD_DIR" "$TARBALL_PATH"

# 2. Push the tarball to the remote server
# 'push' stages the build under /tmp/my-app/<version>/ and prints the exact installer command
./ripdep push "$HOST" "$TARBALL_PATH"

# 3. SSH to the host and run the remote installer
ssh -t "$HOST" "sudo /tmp/my-app/v1.0.0/bin/ripdep-remote install"
```

### 2. Update Application

Deploy a new version of the application code to an existing server, preserving all existing data. `build` builds an artifact with an empty `data/`, so the installer has no data files to overwrite and the live database and key stay as they are. Restart the service to run the new binary.

Requirements:

*   The application is already installed: the service user, `/home/<app-name>`, and the systemd unit exist.
*   The project is a git repository with no uncommitted changes and HEAD exactly on a tag; the tag is the version.
*   The target server is reachable over SSH with `sudo`.

Commands:

```bash
PROJECT_PATH="$PWD"
PROJECT_NAME=$(basename "$PROJECT_PATH")
BUILD_BASE="/tmp"
HOST="user@target-server.com"
VERSION=$(git -C "$PROJECT_PATH" describe --tags --abbrev=0)
BUILD_DIR="${BUILD_BASE}/${PROJECT_NAME}-${VERSION}"

# 1. Build an update artifact
./ripdep build "$PROJECT_PATH"

# 2. Deploy
./ripdep deploy "$HOST" "$BUILD_DIR"

# 3. Restart application to pick up new binary
./ripdep restart "$HOST" "$PROJECT_NAME"
```

### 3. Restore Application from Backup

Provision a new server (for example, a standby replica) from an existing backup. Check out the release that was running, build it with the database backup, the age key, and the systemd unit, and `deploy` ships the artifact to a fresh server reachable over SSH with `sudo`.

Requirements:

*   `--with-db-local` points to a database file (`.db`) or a compressed snapshot (`.tar.gz`). A `.db` file is copied to `data/app.db`; a snapshot is extracted into `data/`.
*   `--with-agekey` adds the project's age key, so the restored database can be decrypted.
*   `--with-systemd-service` adds the systemd unit, required on a fresh server.

Commands:

```bash
PROJECT_PATH="/path/to/my-app"
HOST="user@new-server.com"
DB_PATH="/path/to/backup/data/app.db"
VERSION="v1.0.0"

# 1. Build the artifact from the release that was running
git -C "$PROJECT_PATH" checkout "$VERSION"
./ripdep build "$PROJECT_PATH" --with-agekey --with-systemd-service --with-db-local "$DB_PATH"

# 2. Deploy
./ripdep deploy "$HOST" "/tmp/my-app-${VERSION}"
```

### 4. Deploy the Same Application Under a Different Name

To run the same application twice on one server under two names, build through a symlink named after the second app. The deployed name is the last part of the project path, so the symlink name becomes the service name.

The link must sit in the same parent directory as the real project, so run the commands from inside the project directory and let `../` be that parent.

The second service must already exist on the server; a release build only updates it, so create it first with a build that carries the age key, the database, and the systemd unit.

```bash
cd /path/to/my-app
APP_NAME="my-app-2"
PROJECT_PATH="$PWD"

ln -s "$PROJECT_PATH" "../${APP_NAME}"
./ripdep build "../${APP_NAME}"

# The build prints its directory, e.g. /tmp/my-app-2-v1.0.0
./ripdep deploy user@server.com "/tmp/${APP_NAME}-v1.0.0"
./ripdep restart user@server.com "${APP_NAME}"
```

## Commands

### `build`
Builds `<project>-<version>/` from source for a release:

```text
<project>-<version>/
├── bin/
│   ├── <project> # compiled app binary
│   ├── ripc # on-server config tool
│   └── ripdep-remote # remote installer
└── data/ # empty
```

Options add the pieces a first deployment or a recovery needs:

*   `--with-agekey`: copy the project's `age.key` into the build.
*   `--with-systemd-service`: add the project's `systemd.service`, or download the framework unit when the project has none.
*   `--with-db-local <path>`: restore a database into `data/app.db`. A `.db` file is copied; a `.tar.gz` snapshot is extracted into `data/`.

**Arguments:**
*   `project-path`: the project source to compile. It must be a Go project whose worktree is clean and whose HEAD is exactly on the latest tag; the build fails otherwise. The tag is the version. The deployed name is the last part of this path, so building through a symlink deploys under the symlink's name (see [Deploy the Same Application Under a Different Name](#4-deploy-the-same-application-under-a-different-name)).
*   `build-base-dir`: the base directory for the build output (default `/tmp`). The build directory `<project>-<version>` is created inside it.

Cross-compile by setting `GOOS` and `GOARCH`; the host platform is the default.

**Example:**
```bash
# Creates a release build in /tmp/my-app-v1.0.0
./ripdep build /path/to/my-app /tmp

# A complete build for a fresh server
./ripdep build /path/to/my-app --with-agekey --with-systemd-service --with-db-local /path/to/app.db

# Cross-compile for another target
GOOS=linux GOARCH=arm64 ./ripdep build /path/to/my-app
```

### `build-binary-release`
Builds `<project>-<version>/` for a project whose binaries are already built:

```text
<project>-<version>/
├── <home files and systemd units> # copied from the source root
├── bin/
│   ├── <binaries> # copied from the source bin/
│   └── ripdep-remote # remote installer
└── data/ # empty
```

Use this for projects that ship ready-to-run binaries: put them in the source `bin/` directory, put the other files (for example a Prometheus scrape file and the systemd units) in the source root, and tag the repository. The version is the latest git tag; the worktree must be clean and HEAD must be on the tag, exactly like `build`. No database and no `age.key` are required.

**Arguments:**
*   `source-dir`: the project directory holding the binaries and the other files. The deployed name is the last part of this path.
*   `build-base-dir`: the base directory for the build output (default `/tmp`). The build directory `<project>-<version>` is created inside it.

**Example:**
```bash
# Creates a binary release build in /tmp/my-app
./ripdep build-binary-release /path/to/my-app /tmp
```

### `pack`
Packs a build directory into a compressed tarball at the given path. The build directory must contain `bin/` and `data/`.

**Arguments:**
*   `build-dir`: the completed build directory to package.
*   `tarball-path`: where to write the tarball. `deploy` uses `<build-dir>.tar.gz`, next to the build directory.

**Example:**
```bash
# Packages /tmp/my-app-v1.0.0 into /tmp/my-app-v1.0.0.tar.gz
./ripdep pack /tmp/my-app-v1.0.0 /tmp/my-app-v1.0.0.tar.gz
```

### `unpack`
Extracts a release tarball into a directory.

**Arguments:**
*   `tarball`: the release tarball to extract.
*   `dir`: the target directory.

**Example:**
```bash
./ripdep unpack ./my-app-v1.0.0.tar.gz /tmp/my-app
```

### `restore`
Restores a database into `<dir>/data/app.db`. A `.db` source is copied; a `.tar.gz` source is extracted.

**Arguments:**
*   `source`: the database file (`.db`) or compressed snapshot (`.tar.gz`).
*   `dir`: the target directory.

**Example:**
```bash
./ripdep restore ./app.db /tmp/my-app
```

### `push`
Uploads a release tarball to `/tmp/<project>/<version>/` on the server and extracts it there. This stages the release without touching a running installation. Prints the `install` command for the next step.

**Arguments:**
*   `host`: the remote server address (e.g. `user@server.com`).
*   `tarball-path`: the local release archive created by `pack`.

**Example:**
```bash
./ripdep push user@server.com ./my-app-v1.0.0.tar.gz
```

### `install` (Remote)
Runs on the server as root. Creates the service user, the `/home/<app-name>` layout, installs binaries, data files, and the remaining files from the build root, and installs the systemd units. It derives the project name and paths from its own location, so it takes no arguments.

Permissions:

*   `700` for `/home/<app-name>/data`.
*   `600` for every file copied from the build root, including `/home/<app-name>/age.key`, and all database files.
*   `700` for binaries in `/home/<app-name>/bin`.

Every `*.service` file in the build root named `<project>.service` or `<project>-*.service` is installed into `/etc/systemd/system` after `systemd-analyze verify` accepts it; any other `*.service` file is skipped.

The generated `ripdep-remote` also provides `uninstall`, which stops the services, removes the systemd units, and deletes the user and home directory.

**Example:**
```bash
# Run this on the remote server after a 'push'
sudo /tmp/my-app/v1.0.0/bin/ripdep-remote install
```

### `deploy`
Runs `pack`, `push`, and `install` for a pre-built directory, then removes the staging directory on the server. It does not build the artifact itself.

**Arguments:**
*   `host`: the remote server address (e.g. `user@server.com`).
*   `build-dir`: the completed build directory to deploy.
*   `install-options`: passed through to the remote `install` command.

**Example:**
```bash
# Deploys the build located in /tmp/my-app
./ripdep deploy user@server.com /tmp/my-app
```

### `undeploy`
Stops and disables the project's systemd units, removes them, and deletes the service user and home directory. Prompts for confirmation unless `-y` or `--force` is given.

**Arguments:**
*   `host`: the remote server address.
*   `project-name`: the application name.
*   `-y`, `--force`: skip the confirmation prompt.

**Example:**
```bash
./ripdep undeploy user@server.com my-app -y
```

### `backup`
Creates a tarball of `/home/<project-name>` on the server, downloads it to the current directory as `<project-name>-backup-<timestamp>.tar.gz`, and deletes the remote copy.

**Arguments:**
*   `host`: the remote server address.
*   `project-name`: the application name.

**Example:**
```bash
./ripdep backup user@server.com my-app
```

### `cp`
Copies a local file into `/home/<project-name>/` on the server, owned by the service user with `600` permissions.

**Arguments:**
*   `host`: the remote server address.
*   `project-name`: the application name.
*   `local-file`: the local file to copy.

**Example:**
```bash
./ripdep cp user@server.com my-app ./age.key
```

### `maintenance`
Sets `maintenance.activated` with `ripc` and reloads the service.

**Arguments:**
*   `host`: the remote server address.
*   `project-name`: the application name.
*   `state`: `true` or `false`.

**Example:**
```bash
./ripdep maintenance user@server.com my-app true
```

### `ripc`
Runs a `ripc` command on the server as the project user from the application home directory, so relative config paths resolve like the running service.

**Arguments:**
*   `host`: the remote server address.
*   `project-name`: the application name.
*   `ripc-args`: arguments passed to `ripc`.

**Example:**
```bash
./ripdep ripc user@server.com my-app set server.port 8080
```

### `shell`
Opens an interactive shell as the project user. `ripc` is on `PATH` and points at the database and age key, so config commands run directly.

**Arguments:**
*   `host`: the remote server address (e.g. `user@server.com`).
*   `project-name`: the application name.

**Example:**
```bash
./ripdep shell user@server.com my-app
```

### `status`
Runs `systemctl status` for the service.

**Arguments:**
*   `host`: the remote server address.
*   `project-name`: the application name.

**Example:**
```bash
./ripdep status user@server.com my-app
```

### `journal`
Follows the systemd journal, showing the last `lines` entries.

**Arguments:**
*   `host`: the remote server address.
*   `project-name`: the application name.
*   `lines`: number of entries to show before following (default `50`).

**Example:**
```bash
./ripdep journal user@server.com my-app
```

### `restart`
Restarts the service.

**Arguments:**
*   `host`: the remote server address.
*   `project-name`: the application name.

**Example:**
```bash
./ripdep restart user@server.com my-app
```

### `reload`
Reloads the service configuration without restarting the process.

**Arguments:**
*   `host`: the remote server address.
*   `project-name`: the application name.

**Example:**
```bash
./ripdep reload user@server.com my-app
```

### `stop`
Stops every systemd unit the project owns.

**Arguments:**
*   `host`: the remote server address.
*   `project-name`: the application name.

**Example:**
```bash
./ripdep stop user@server.com my-app
```

### `enable`
Enables and starts every systemd unit the project owns, `<project-name>.service` and `<project-name>-*.service`, in one `systemctl enable --now`.

**Arguments:**
*   `host`: the remote server address.
*   `project-name`: the application name.

**Example:**
```bash
./ripdep enable user@server.com my-app
```

## Debugging on a Remote Server

The systemd unit is sandboxed, which can hide the cause of a failure. Work through these steps.

### 1. Check Status and Logs

-   `sudo systemctl status my-app.service` shows the service state and recent log lines. Or use `./ripdep status <host> my-app`.
-   `sudo journalctl -u my-app.service -f` follows the full journal, the primary source of errors. Or use `./ripdep journal <host> my-app`.

### 2. Log in and Run Manually

If the logs are unclear, run the start command as the service user to bypass the systemd sandbox. The `install` command creates the service user with `/bin/bash` for this purpose.

1.  Become the service user: `sudo su - my-app`. This drops you into `/home/my-app`.
2.  Run the `ExecStart` command from the unit file:

```bash
./bin/my-app -dbpath data/app.db -agekey age.key
```

Panics, configuration errors, and file permission errors now print directly to your terminal.

### 3. Debug the Systemd Sandbox

If the application runs manually but fails under `systemctl`, a security directive is blocking it. `ProtectSystem=strict` makes most of the filesystem read-only; only paths in `ReadWritePaths` (such as `/home/my-app/data`) are writable.

1.  Edit the unit: `sudo nano /etc/systemd/system/my-app.service`.
2.  Comment out a block of directives (for example, everything under `=== FILESYSTEM HARDENING ===`).
3.  Reload systemd: `sudo systemctl daemon-reload`.
4.  Restart the service: `sudo systemctl restart my-app.service`.

If the service starts, the cause is in the block you commented out. Re-enable directives one at a time, repeating steps 3 and 4, to find the exact one.

## Systemd Unit Contract

The unit hardcodes the flags and values your app gets: `bin/<app> -dbpath data/app.db -agekey age.key`. Make sure your app uses those flags.

A project may ship more than one systemd unit in the build root, for example `my-app.service` and `my-app-tunnel.service`. Every `*.service` file named `my-app.service` or `my-app-*.service` is installed; any other `*.service` file is skipped.

Commands that act on the service — `status`, `journal`, `stop`, `restart`, `reload`, and `enable` — operate on every unit the project owns, in one systemd transaction.

When a unit reads a value from an environment file, write the reference as `${VAR}`. systemd expands `${VAR}` inside an argument such as `-flag=${VAR}`, but a bare `$VAR` only when it stands as a whole word. A unit written as `-flag=$VAR` is passed the literal text `$VAR` and the service fails to start.

## Project Configuration Scripts

The build commands create the build directory to allow for further configuration before the deploy. A project that needs it may add one or both of the scripts below.

`scripts/configure.sh` configures the build. Run it after the build command and before `deploy`:

```bash
./ripdep build-binary-release /path/to/my-app /tmp
bash scripts/configure.sh /tmp/my-app-v1.0.0
./ripdep deploy user@server.com /tmp/my-app-v1.0.0
```

It writes the settings the operator gives it into the build directory, so the release carries them.

`bin/configure-remote.sh` configures the deployed machine. It sits in `bin/` with the other executables, which the deploy installs at `/home/<app-name>/bin/configure-remote.sh`; run it there as root after the deploy for the settings that belong to the machine rather than to the application, such as firewall rules.
