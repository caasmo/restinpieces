# Restinpieces Deployment & Operations Tool (ripdep)

This script, `ripdep`, is a comprehensive tool for building, packaging, and deploying [RestInPieces](https://github.com/caasmo/restinpieces) framework applications.

It is designed to be run from a developer's local machine and acts as a high-level, user-friendly facade for all operational tasks. It orchestrates complex remote operations over SSH by wrapping the lower-level, on-server configuration tool, `ripc`. For most use cases, you should only need to interact with `ripdep`.

## Use Cases

This section provides concrete, step-by-step instructions for common operational scenarios.

### 1. First-Time Application Bootstrap

**Goal:** Deploy a new application to a fresh server. This includes compiling the binary, generating a new database, and installing everything.

**Strategy:** Use the `build-bootstrap` command to create a complete artifact with a fresh database and configuration. Then use `deploy` to ship it.

**Commands:**

```bash
PROJECT_PATH="$PWD"
BUILD_BASE="/tmp"
HOST="user@target-server.com"

# 1. Build a complete, bootstrap artifact from source
./ripdep build-bootstrap "$BUILD_BASE" "$PROJECT_PATH"

# 2. Deploy
./ripdep deploy "$HOST" "${BUILD_BASE}/my-app"
```

#### Low-Level Breakdown (Educational)
The `deploy` command automates the following manual steps:

```bash
PROJECT_PATH="$PWD"
BUILD_BASE="/tmp"
HOST="user@target-server.com"

# 1. Build
./ripdep build-bootstrap "$BUILD_BASE" "$PROJECT_PATH"

# 2. Package the artifact into a tarball
./ripdep pack "${BUILD_BASE}/my-app"
TARBALL_PATH=$(find ~/src/backup/releases/my-app -name "*.tar.gz" -print -quit) 

# 3. Push the tarball to the remote server
./ripdep push "$HOST" "$TARBALL_PATH"

# 4. SSH to the host and run the remote installer
REMOTE_INSTALL_PATH=$(./ripdep get_remote_dir_from_tarball_path "$TARBALL_PATH")/bin/ripdep 
ssh -t "$HOST" "sudo $REMOTE_INSTALL_PATH install"
```

### 2. Update Application Binary Version

**Goal:** Deploy a new version of the application code to an existing server, preserving all existing data (database, keys, etc.).

**Strategy:** Build an artifact containing only the new binary and supporting tools, but no data. The `deploy` (and underlying `install`) command ensures existing data files are not overwritten.

**Commands:**

```bash
PROJECT_PATH="$PWD"
BUILD_BASE="/tmp"
HOST="user@target-server.com"

# 1. Build an update artifact
./ripdep build-release "$BUILD_BASE" "$PROJECT_PATH"

# 2. Deploy
./ripdep deploy "$HOST" "${BUILD_BASE}/my-app"

# 3. Restart application to pick up new binary
ssh -t "$HOST" "sudo systemctl restart my-app"
```

### 3. Restore Application from Backup

**Goal:** Provision a new server (e.g., a new standby replica) using a database from an existing backup.

**Strategy:** Build a recovery artifact using `build-recovery`, then ship it with `deploy`.

**Commands:**

```bash
BUILD_BASE="/tmp"
HOST="user@new-server.com"
RELEASE_PATH="/path/to/previous/release.tar.gz" 
DB_PATH="/path/to/backup/data/app.db"

# 1. Build the recovery artifact
./ripdep build-recovery "$BUILD_BASE" --with-release "$RELEASE_PATH" --with-db "$DB_PATH"

# 2. Deploy
./ripdep deploy "$HOST" "${BUILD_BASE}/my-app"
```

### 4. Sync Database to a Standby Server

**Goal:** Provision or update a standby server with the latest database state restored from backups (e.g., S3 via Litestream).

**Strategy:** Use `build-recovery` to restore the DB (potentially from S3) and package it, then use `deploy` to ship it.

**Commands:**

```bash
BUILD_BASE="/tmp"
HOST="user@standby-server.com"
LITESTREAM_CONFIG="config/my-app/litestream.yml" # Restores from S3

# 1. Build recovery artifact (only DB)
./ripdep build-recovery "$BUILD_BASE" --with-db "$LITESTREAM_CONFIG"

# 2. Deploy
./ripdep deploy "$HOST" "${BUILD_BASE}/my-app"
```

## Commands

### `build-release`
Creates a complete, self-contained build directory from the application's source code. It compiles the Go binary and downloads the `ripc` tool.

**Arguments:**
*   `build-base-dir`: The base directory where the build output will be created (e.g., `/tmp`). The script creates a subdirectory named after your project inside this directory.
*   `project-path`: The path to the project source code to be compiled.

**Example:**
```bash
# Creates a release build in /tmp/my-app
./ripdep build-release /tmp /path/to/my-app
```

### `build-bootstrap`
Similar to `build-release`, but also generates a new encryption key, a fresh database, and initializes service configurations (Litestream and systemd). Use this for the first-ever deployment of an application.

**Arguments:**
*   `build-base-dir`: The base directory where the build output will be created.
*   `project-path`: The path to the project source code to be compiled.

**Example:**
```bash
# Creates a bootstrap build in /tmp/my-app
./ripdep build-bootstrap /tmp /path/to/my-app
```

### `build-recovery`
Creates a build directory from existing backups. This is used for disaster recovery or for provisioning a new server from an existing application's data.

**Arguments & Flags:**
*   `build-base-dir`: The base directory for the build output.
*   `--with-release <path>`: Path to an existing release tarball (`.tar.gz`) to extract tools and configuration from.
*   `--with-db <source>`: Path to a database source, which can be a database file (`.db`), a compressed backup (`.tar.gz`), or a Litestream config (`.yml`).

**Example:**
```bash
# Creates a recovery build in /tmp/my-app-recovery
./ripdep build-recovery /tmp --with-release ./release.tar.gz --with-db ./app.db
```

### `pack`
Packages the specified build directory into a compressed TAR archive (`.tar.gz`), ready for deployment. The resulting artifact is placed in the releases directory (e.g., `~/src/backup/releases/<project_name>/`).

**Arguments & Flags:**
*   `build-dir`: **(Required)** The path to the completed build directory that you want to package.

**Example:**
```bash
# Packages the contents of /tmp/my-app
./ripdep pack /tmp/my-app
```

### `push`
Uploads a release TAR archive to a remote server and extracts it into a temporary, version-stamped directory (e.g., `/tmp/<project_name>/<version>`). This stages the application for the `install` command.

**Arguments & Flags:**
*   `host`: **(Required)** The remote server address (e.g., `user@server.com`).
*   `tarball-path`: **(Required)** The local path to the release archive created by the `pack` command.

**Example:**
```bash
./ripdep push user@server.com ./my-app-v1.0.0.tar.gz
```

### `install` (Remote)
Finalizes the installation on the remote server. This command is intended to be run *on the remote machine* via `ssh` and requires `sudo` privileges. It handles creating the application user, setting up directories, deploying files, and installing the `systemd` unit.

It also establishes a secure file structure with strict permissions:
*   `drwx------ (700)` for the `/home/{app-name}/data` directory.
*   `-rw------- (600)` for the `/home/{app-name}/age.key` and all database files.
*   `-rwx------ (700)` for binaries in the `/home/{app-name}/bin` directory.

**Arguments & Flags:**
*   This command is self-configuring and does not require path arguments, as it determines them from its own location on the remote server.

**Example:**
```bash
# Run this on the remote server after a 'push'
sudo /tmp/my-app/v1.0.0/bin/ripdep install
```

### `deploy`
A high-level orchestrator that automates the `pack`, `push`, and `install` sequence for a pre-existing build directory. This is the simplest way to get a finished build running on a remote server. **It does not create the build itself.**

**Arguments & Flags:**
*   `host`: **(Required)** The remote server address (e.g., `user@server.com`).
*   `build-dir`: **(Required)** The path to the completed build directory to be deployed.

**Example:**
```bash
# Deploys the build located in /tmp/my-app
./ripdep deploy user@server.com /tmp/my-app
```
