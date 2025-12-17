# Bootstrapping a RestInPieces Application

This document guides you through the initial setup and bootstrapping of a new web application using the RestInPieces framework. The process is designed to be straightforward, establishing a secure and well-configured foundation for your project.

The core philosophy is a clean separation between one-time setup and the application's runtime lifecycle. We use the `ripc` command-line tool to manage the application's persistent state, including its encrypted configuration and database schema. This approach ensures that your application code remains focused on business logic, while the framework handles the underlying infrastructure.

---

## Developer Workflow

### 1. Prerequisites

Before you begin, ensure you have the following tools installed:

*   **Go:** The Go programming language environment.
*   **age:** A simple, modern, and secure file encryption tool.

The first and most critical step is to generate a master encryption key. **All framework configuration—including secrets like JWT keys, SMTP passwords, and TLS certificates—is encrypted at rest within the main application's SQLite database file.** This `age` key is the root of trust used to secure that data. You will need the private key to start the server and to manage its configuration via the `ripc` tool. Guard it carefully.

```bash
age-keygen -o age.key
```
This will create `age.key` containing your private key and print the corresponding public key to the console.

### 2. Create the Application Instance

Next, use the `ripc` tool to create the core application database. This command initializes the database file, applies the necessary schema (for users, jobs, etc.), and saves a default configuration, which is encrypted at rest using your `age` key.

If `age.key` is in the current directory, you can simply run:
```bash
ripc -dbpath ./myapp.db app create
```
The `-dbpath` is specified here to create `myapp.db` instead of the default `app.db`.

*   **Result:** A `myapp.db` file is created. This single file contains your application's core tables and its first encrypted configuration entry. The database must not exist before running this command. After creation, you can rename `myapp.db` to `app.db` to avoid using the `-dbpath` flag in subsequent commands.

### 3. Customize the Configuration

The default configuration provides a sensible starting point, but you will need to customize it for your environment. While you can dump the entire configuration to a file for bulk editing, it is often safer and more precise to manage individual settings directly from the command line using `ripc config`.

This approach is especially recommended for complex, multiline values like TLS certificates or private keys, where editing them in a TOML file can easily introduce formatting errors.

#### Direct Configuration with `ripc`

The `ripc config` command provides `get`, `set`, and `paths` subcommands to interact with specific configuration values.

To discover what settings are available, you can use the `paths` subcommand. It displays a flat list of all configurable fields, making it easy to find the exact path you need to modify.

```bash
# List all available configuration paths
ripc config paths

# You can also filter the list
ripc config paths server
```

Once you know the path, you can retrieve its current value with `get` or modify it with `set`.

```bash
# Get the current server port
ripc config get server.http_port

# Change the server port
ripc config set server.http_port 8081
```

#### Handling Complex Values

For multiline values, such as a TLS certificate, you can instruct `ripc` to read the value directly from a file by prefixing the file path with `@`. This avoids any copy-paste or formatting issues.

For example, to set the TLS certificate and key:
```bash
# First, ensure your certificate and key files are ready
# For example: localhost.pem and localhost-key.pem

# Set the certificate
ripc config set server.tls_cert @/path/to/localhost.pem

# Set the private key
ripc config set server.tls_key @/path/to/localhost-key.pem
```
This method is robust and script-friendly, making it ideal for automated deployments and managing sensitive information.

#### Bulk Editing (If Necessary)

If you still prefer to edit the entire configuration at once, you can dump it to a file:
```bash
ripc config dump > config.toml
```
After editing `config.toml`, you will save it back using the `config save` command as described in the next step.

### 4. Save the Custom Configuration

After editing `config.toml`, save it back into the secure store. This creates a new, versioned entry in the configuration table, which will now be considered the "latest".

```bash
ripc config save config.toml
```

You can view the history of configuration changes at any time:
```bash
ripc config list
```

Should you need to revert to a previous configuration, the `rollback` command allows you to restore any historical version by its generation number (obtained from `config list`). This provides a safety net for configuration changes.

```bash
ripc config rollback 3
```

### 5. Initialize the Logger Database

The framework accepts any logger that complies with the standard `log/slog` interface. If no custom logger is provided, it defaults to a high-performance batch logger that writes application events to a separate SQLite database. Before the main application can start, this database must be created and its schema must be applied.

Use the `ripc log init` command to perform this one-time setup. The command is idempotent, meaning you can run it multiple times without causing issues.

```bash
ripc log init
```

This command reads the `log.batch.db_path` from your configuration, creates the database file (e.g., `logs.db`) if it doesn't exist, and applies the necessary table schema. If you have not configured a path, it will default to creating a `logs.db` file in the same directory as your main application database.

### 6. Write the Application Code

With the configuration in place, you can write your application's entry point. This is typically a `main.go` file. The framework is initialized by calling `restinpieces.New()`, which handles loading the configuration, setting up the database connections, and preparing all core components.

Here is a minimal `main.go` example:

```go
package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/caasmo/restinpieces"
	"github.com/caasmo/restinpieces/db/zombiezen"
)

func main() {
	// Define and parse command-line flags for required paths
	dbPath := flag.String("dbpath", "", "Path to the application's SQLite database file.")
	ageKeyPath := flag.String("agekey", "", "Path to the age identity file (private key).")
	flag.Parse()

	if *dbPath == "" || *ageKeyPath == "" {
		fmt.Fprintln(os.Stderr, "Error: both -dbpath and -agekey flags are required.")
		flag.Usage()
		os.Exit(1)
	}

	// Create a database pool
	pool, err := restinpieces.NewZombiezenPool(*dbPath)
	if err != nil {
		slog.Error("Failed to create database pool", "error", err, "path", *dbPath)
		os.Exit(1)
	}
	defer pool.Close()

	// Create a new application instance with required options
	app, srv, err := restinpieces.New(
		restinpieces.WithDbApp(zombiezen.New(pool)),
		restinpieces.WithAgeKeyPath(*ageKeyPath),
	)
	if err != nil {
		slog.Error("Failed to initialize restinpieces app", "error", err)
		os.Exit(1)
	}

	// The app object is now available to register your custom routes
	// app.Router().Post("/my/endpoint", myHandler)

	// Start the server. This is a blocking call.
	if err := srv.Run(); err != nil {
		app.Logger().Error("Server shut down with error", "error", err)
		os.Exit(1)
	}
}
```

### 7. Run the Application

Finally, compile and run your server.

```bash
go run ./cmd/myapp/main.go -dbpath ./myapp.db -agekey ./age.key
```

On startup, `restinpieces.New()` will:
*   Decrypt your configuration using the provided `age.key`.
*   Connect to the main application database (`myapp.db`).
*   **Verify the logger database is initialized.** It will check for the existence of the log database file and its schema. If this check fails, the application will exit with an error, instructing you to run `ripc log init`.
*   Set up the job scheduler, cache, and all other core services.

---

## Ongoing Management

The `ripc` tool is also used for managing the application after it has been bootstrapped. Some common operations include:

*   **Updating a single config value:**
    ```bash
    ripc config set server.http_port 8081
    ```
*   **Rotating JWT secrets for security:**
    ```bash
    ripc auth rotate-jwt-secrets
    ```
*   **Adding a recurring database backup job:**
    ```bash
    ripc job add-backup --interval 24h
    ```

The `ripc` tool is also used for managing the application after it has been bootstrapped. Some common operations include:

*   **Updating a single config value:**
    ```bash
    ripc config set server.http_port 8081
    ```
*   **Rotating JWT secrets for security:**
    ```bash
    ripc auth rotate-jwt-secrets
    ```
*   **Adding a recurring database backup job:**
    ```bash
    ripc job add-backup --interval 24h
    ```

For a complete list of commands and in-depth documentation, refer to the **[`ripc` README](cmd/ripc/README.md)** or use the `ripc help` command.