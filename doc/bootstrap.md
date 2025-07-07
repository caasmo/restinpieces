# Bootstrapping a RestInPieces Application

This document guides you through the initial setup and bootstrapping of a new web application using the RestInPieces framework. The process is designed to be straightforward, establishing a secure and well-configured foundation for your project.

The core philosophy is a clean separation between one-time setup and the application's runtime lifecycle. We use the `ripc` command-line tool to manage the application's persistent state, including its encrypted configuration and database schema. This approach ensures that your application code remains focused on business logic, while the framework handles the underlying infrastructure.

---

## Developer Workflow

### 1. Prerequisites

Before you begin, ensure you have the following tools installed:

*   **Go:** The Go programming language environment.
*   **age:** A simple, modern, and secure file encryption tool.

You'll start by generating a master encryption key. This key is the root of trust for your application's configuration. Guard it carefully.

```bash
age-keygen -o age.key
```
This will create `age.key` containing your private key and print the corresponding public key.

### 2. Create the Application Instance

Next, use the `ripc` tool to create the core application database. This command initializes the database file, applies the necessary schema (for users, jobs, etc.), and saves a default configuration, which is encrypted at rest using your `age` key.

```bash
ripc -age-key age.key -dbpath ./myapp.db app create
```

*   **Result:** A `myapp.db` file is created. This single file contains your application's core tables and its first encrypted configuration entry. The database must not exist before running this command.

### 3. Customize the Configuration

The default configuration provides a sensible starting point, but you will need to customize it for your environment. While you can dump the entire configuration to a file for bulk editing, it is often safer and more precise to manage individual settings directly from the command line using `ripc config`.

This approach is especially recommended for complex, multiline values like TLS certificates or private keys, where editing them in a TOML file can easily introduce formatting errors.

#### Direct Configuration with `ripc`

The `ripc config` command provides `get` and `set` subcommands to interact with specific configuration values by their path.

First, you can list all available configuration paths:
```bash
ripc -age-key age.key -dbpath ./myapp.db config paths
```

To retrieve the current value of a specific setting:
```bash
ripc -age-key age.key -dbpath ./myapp.db config get server.http_port
```

To change a simple value:
```bash
ripc -age-key age.key -dbpath ./myapp.db config set server.http_port 8081
```

#### Handling Complex Values

For multiline values, such as a TLS certificate, you can instruct `ripc` to read the value directly from a file by prefixing the file path with `@`. This avoids any copy-paste or formatting issues.

For example, to set the TLS certificate and key:
```bash
# First, ensure your certificate and key files are ready
# For example: localhost.pem and localhost-key.pem

# Set the certificate
ripc -age-key age.key -dbpath ./myapp.db config set server.tls_cert @/path/to/localhost.pem

# Set the private key
ripc -age-key age.key -dbpath ./myapp.db config set server.tls_key @/path/to/localhost-key.pem
```
This method is robust and script-friendly, making it ideal for automated deployments and managing sensitive information.

#### Bulk Editing (If Necessary)

If you still prefer to edit the entire configuration at once, you can dump it to a file:
```bash
ripc -age-key age.key -dbpath ./myapp.db config dump > config.toml
```
After editing `config.toml`, you will save it back using the `config save` command as described in the next step.

### 4. Save the Custom Configuration

After editing `config.toml`, save it back into the secure store. This creates a new, versioned entry in the configuration table, which will now be considered the "latest".

```bash
ripc -age-key age.key -dbpath ./myapp.db config save config.toml
```

You can view the history of configuration changes at any time:
```bash
ripc -age-key age.key -dbpath ./myapp.db config list
```

### 5. Write the Application Code

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
	ageKeyPath := flag.String("age-key", "", "Path to the age identity file (private key).")
	flag.Parse()

	if *dbPath == "" || *ageKeyPath == "" {
		fmt.Fprintln(os.Stderr, "Error: both -dbpath and -age-key flags are required.")
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

### 6. Run the Application

Finally, compile and run your server.

```bash
go run ./cmd/myapp/main.go -dbpath ./myapp.db -age-key ./age.key
```

On the first run, `restinpieces.New()` will:
*   Decrypt your configuration using the provided `age.key`.
*   Connect to the main application database (`myapp.db`).
*   **Automatically initialize the logger database.** It reads the `log.batch.db_path` from your config, creates the database file if it doesn't exist, and applies the required schema. There is no need for a separate `log init` command.
*   Set up the job scheduler, cache, and all other core services.

Your application is now running.

---

## Ongoing Management

The `ripc` tool is also used for managing the application after it has been bootstrapped. Some common operations include:

*   **Updating a single config value:**
    ```bash
    ripc -age-key age.key -dbpath ./myapp.db config set server.http_port 8081
    ```
*   **Rotating JWT secrets for security:**
    ```bash
    ripc -age-key age.key -dbpath ./myapp.db auth rotate-jwt-secrets
    ```
*   **Adding a recurring database backup job:**
    ```bash
    ripc -age-key age.key -dbpath ./myapp.db job add-backup --interval 24h
    ```

Refer to the `ripc help` command for a full list of capabilities.