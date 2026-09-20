# Bootstrapping a RestInPieces Application

This guide gets you running on your own machine for local development. It uses `ripc` directly on local files. For production deployment use [`ripdep`](ripdep.md), which builds the binaries and runs `ripc` over SSH on the server.

---

## Local Development Workflow

### 0. Start From the Layout

Create your project files first, following [Application Layout Best Practices](layout-best-practices.md): `routes.go` in the root, application code in `app/`, frontend in `web/` with `embed.go`, entry point in `cmd/myapp/main.go`. The steps below only create the local data those files need to run.

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

Next, use the `ripc` tool to create the core application database. This command creates a SQLite file, applies the necessary schema (for users, jobs, etc.), and saves a default configuration, which is encrypted at rest using your `age` key.

Set the two paths once with env vars, then leave the flags out. All `ripc` commands below reuse them.
```bash
export RIPC_DB=./app.db
export RIPC_AGE_KEY_PATH=./age.key
ripc app create
```
This creates a `./app.db` SQLite file with your application's core tables and its first encrypted configuration entry. The file must not exist before you run this command.

### 3. Customize the Configuration

Adjust the default configuration for local development. See [ripc.md](ripc.md) for the full command reference.

```bash
ripc paths server
ripc get server.addr
ripc set server.addr :8081
```

### 4. Initialize the Logger Database

The framework accepts any logger that complies with the standard `log/slog` interface. If no custom logger is provided, it defaults to a high-performance batch logger that writes application events to a separate SQLite database. Before the main application can start, this database must be created and its schema must be applied.

Use the `ripc log init` command to perform this setup. It requires an explicit path:

```bash
ripc log init ./logs.db
```

Keep it separate from `app.db` when you can. Logs write a lot and are temporary. Putting them in the same file makes `app.db` grow and backups bigger, and you can't clean logs without touching app data. A separate file keeps things clean. Only share the file (`ripc log init ./app.db`) if you consciously want one file to manage.

### 5. Write the Application Code

Write your code as described in [Application Layout Best Practices](layout-best-practices.md).

### 6. Run the Application Locally

Finally, compile and run your server. The env vars from step 2 are reused, no flags needed.

```bash
go run ./cmd/myapp/main.go
```

On startup, `restinpieces.New()` will:
*   Decrypt your configuration using the provided `age.key`.
*   Connect to the main application database (`app.db`).
*   **Verify the logger database is initialized.** It will check for the existence of the log database file and its schema. If this check fails, the application will exit with an error, instructing you to run `ripc log init`.
*   Set up the job scheduler, cache, and all other core services.

---

## Production Deployment

Deploy to production with [`ripdep`](ripdep.md), which builds the release, copies the binaries, and runs the remote setup. After a deploy, review the post-deploy settings in [post-deploy-config.md](post-deploy-config.md).

---

## Ongoing Management

The `ripc` tool is also used for managing the application after it has been bootstrapped. Some common operations include:

*   **Updating a single config value:**
    ```bash
    ripc set server.addr :8081
    ```
*   **Rotating JWT secrets for security:**
    ```bash
    ripc update jwt
    ```
*   **Listing background jobs:**
    ```bash
    ripc job list
    ```

For a complete list of commands and in-depth documentation, refer to the **[`ripc` README](cmd/ripc/README.md)** or use the `ripc help` command.