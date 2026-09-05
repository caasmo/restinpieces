<p align="center"><img alt="restinpieces" src="doc/logo.png"/></p>

[![Go Reference](https://pkg.go.dev/badge/github.com/caasmo/restinpieces)](https://pkg.go.dev/github.com/caasmo/restinpieces)
[![Test](https://github.com/caasmo/restinpieces/actions/workflows/test.yml/badge.svg)](https://github.com/caasmo/restinpieces/actions/workflows/test.yml) 
[![golangci-lint](https://github.com/caasmo/restinpieces/actions/workflows/golangci-lint.yml/badge.svg)](https://github.com/caasmo/restinpieces/actions/workflows/golangci-lint.yml)
[![Coverage](https://img.shields.io/endpoint?url=https://raw.githubusercontent.com/caasmo/restinpieces/master/.github/badges/coverage.json)](https://github.com/caasmo/restinpieces/actions/workflows/test.yml)
[![sloc](https://img.shields.io/endpoint?url=https://raw.githubusercontent.com/caasmo/restinpieces/master/.github/badges/sloc.json)](https://github.com/caasmo/restinpieces/actions/workflows/sloc.yml)
[![deps](https://img.shields.io/endpoint?url=https://raw.githubusercontent.com/caasmo/restinpieces/master/.github/badges/deps.json)](https://github.com/caasmo/restinpieces/actions/workflows/dependencies.yml)
[![GitHub Release](https://img.shields.io/github/v/release/caasmo/restinpieces?style=flat)]() 
[![Built Go](https://img.shields.io/badge/built_with-Go-00ADD8.svg?style=flat)]()

# REST in pieces

RestInPieces is a Go framework for building servers backed by embedded SQLite. It is extensible via handlers, middleware, jobs and daemons, and [keeps third-party dependencies minimal](https://github.com/caasmo/restinpieces/actions/workflows/dependencies.yml).

To get started, follow the **[Bootstrapping Guide](doc/bootstrap.md)**, which walks you through the initial setup of a new application.

## Core Philosophy: One Process Application

The framework follows the One Process Application approach: the application and its dependencies ship as a single binary.

Database, cache, and job queue run inside the application binary. Instead of operating separate services (e.g., a database server, a Redis instance, a reverse proxy), you deploy a single binary.

One Go binary with embedded SQLite runs on one VM, with no separate services to operate. A single server handles growth until traffic requires sharding or a different architecture.

This approach follows [One Process Programming Notes](https://crawshaw.io/blog/one-process-programming-notes).

# Content

### Framework Key Features
- [Data Durability](#data-durability)
- [Database Drivers](#database-drivers)
- [Router](#router)
- [Cache](#cache)
- [Authentication](#authentication)
- [Security](#security)
- [Core Infrastructure](#core-infrastructure)
- [Configuration Management](#configuration-management)
- [Deployment & Operations](#deployment--operations)
- [Frontend Integration](#frontend-integration)
- [Job Framework](#job-framework)
- [Performance](#performance)
- [Metrics](#metrics)
- [Logger](#logger)
- [Notifications](#notifications)
- [Mailer](#mailer)
- [Middleware](#middleware)
- [Extensibility](#extensibility)

### Building on the Framework
- [Layout Best Practices](#layout-best-practices)
- [Examples](#examples)
- [Building the Project](#building-the-project)
- [TODO](#todo)

## Key Features

### Data Durability
Single process per VM means no separate database server. The framework stores data in embedded SQLite, so the single database file must survive crashes and restarts.

The framework keeps copies of that file with pure-Go tools in the [restinpieces-backup](https://github.com/caasmo/restinpieces-backup) repository, and continuous replication with point-in-time recovery via [restinpieces-litestream](https://github.com/caasmo/restinpieces-litestream):

| Method | Use | Implementation |
| --- | --- | --- |
| [Online Backup API](https://www.sqlite.org/backup.html) | local backup | [`cmd/onlineapi`](https://github.com/caasmo/restinpieces-backup/tree/master/cmd/onlineapi) |
| [`VACUUM INTO`](https://www.sqlite.org/lang_vacuum.html) | local backup | [`cmd/vacuum`](https://github.com/caasmo/restinpieces-backup/tree/master/cmd/vacuum) |
| [sqlite3_rsync](https://github.com/caasmo/go-sqlite-rsync) | remote backup, delta-based | [`cmd/sqlite-rsync`](https://github.com/caasmo/restinpieces-backup/tree/master/cmd/sqlite-rsync) |
| rsync pull client | remote backup, incremental | [`cmd/rsync`](https://github.com/caasmo/restinpieces-backup/tree/master/cmd/rsync) |
| sftp pull client | remote backup | [`cmd/sftp`](https://github.com/caasmo/restinpieces-backup/tree/master/cmd/sftp) |
| [litestream](https://github.com/benbjohnson/litestream) | continuous replication, point-in-time | [restinpieces-litestream](https://github.com/caasmo/restinpieces-litestream) |

For the framework-side `backup` configuration, see [doc/backup.md](doc/backup.md).

### Database Drivers
The framework uses pure-Go [modernc.org/sqlite](https://modernc.org/sqlite); [zombiezen.com/go/sqlite](https://github.com/caasmo/restinpieces-sqlite-zombiezen) as the alternative.

### Router
The framework uses Go's standard `http.ServeMux` as the default router. Since Go 1.22 it supports path parameters. The router is swappable; an alternative based on [julienschmidt/httprouter](https://github.com/julienschmidt/httprouter) is at [restinpieces-httprouter](https://github.com/caasmo/restinpieces-httprouter).

### Cache
For in-memory caching, the framework includes a preallocated LRU cache ([`package cache`](https://pkg.go.dev/github.com/caasmo/restinpieces/cache)) using only the Go standard library. The `cache.Cache` interface lets you swap in your own implementation via `WithCache`; a [ristretto](https://github.com/dgraph-io/ristretto)-based implementation is at [restinpieces-cache](https://github.com/caasmo/restinpieces-cache).

### Authentication
The framework authenticates with JSON Web Tokens (JWT) sent as bearer tokens in the `Authorization` header. JWT signing keys derive from user credentials (email and password hash) plus a server secret, so changing the password invalidates existing tokens.

Authentication and account management use these API endpoints:

- **Password-based**: User registration (`/register-with-password`), login (`/auth-with-password`), and token refresh (`/auth-refresh`).
- **OAuth2**: (`/auth-with-oauth2`) exchanges the provider token, fetches user info, and creates or links the local user account. (`/list-oauth2-providers`) lists configured providers.
- **Account Management**: Email verification, password reset, and email change run as multi-step flows. Each flow sends a unique, short-lived JWT to the user's email via the job queue; the user submits it back to a confirmation endpoint.

### Security
No reverse proxy sits in front of the binary, so the application is directly exposed to the internet. Built-in middleware covers common threats: dynamic IP blocking (`BlockIp`), hostname whitelist (`BlockHost`), request body size limit (`BlockRequestBody`), `User-Agent` filtering (`BlockUaList`), and `Strict-Transport-Security` headers.

No CORS support is provided as it contradicts the One Process philosophy. If you need cross-origin requests, you'll need to implement CORS middleware yourself.

### Core Infrastructure
The framework is built on standard Go patterns, utilizing middleware and handlers to provide a familiar and robust development experience. It features a set of discoverable API endpoints for essential services, such as token refreshing (`/api/refresh-auth`) and OAuth2 authentication (`/api/auth-with-oauth2`), facilitating easy integration and exploration.

### Configuration Management
Configuration lives in the SQLite database as encrypted TOML in the `app_config` table (schema: `migrations/schema/app/app_config.sql`). `ripc` manages it — a server-side tool that reads and writes the local database and age key files on the production machine. It supports versioning, diffing, and rollbacks. `ripc` also handles custom configuration scopes for your modules. See the [`ripc` documentation](doc/ripc.md). 

`ripc` runs on the server and edits local state; [`ripdep`](doc/ripdep.md) ([source](scripts/ripdep)) runs on your machine and calls `ripc` over SSH.

Configuration reloads on `SIGHUP` without restart. Most settings apply on reload; TLS certificates require a full restart.

### Deployment & Operations

**`ripdep`** manages your application from your local machine over SSH by calling `ripc`. `ripc` runs on the production machine against the local filesystem.
-   **Remote DevOps**: Run `ripc` commands remotely for configuration, maintenance modes, and log monitoring.
-   **Disaster Recovery**: Bootstrap new servers and recover from backups (including Litestream) with `build-bootstrap` and `build-recovery`.

Edit configuration locally, then apply it to the remote host. See the **[Deployment Guide](doc/ripdep.md)**.

### Frontend Integration

The framework includes a JavaScript SDK for frontend-backend interaction. The SDK covers password-based and OAuth2 flows, plus error handling, local storage, and request helpers. See example usage at [restinpieces-js-sdk](https://github.com/caasmo/restinpieces-js-sdk).

### Job Framework
The framework includes a job queue for asynchronous tasks, one-time and recurrent. It moves work such as sending emails off the request-response cycle.

A scheduler claims jobs from the `job_queue` table and an executor runs the handler. Built-in handlers send password reset emails, email verifications, and local database backups.

To add your own tasks, in two steps:
1.  **Write a Job Handler**: Implement the `JobHandler` interface with the task logic.
2.  **Insert a Job**: Add a record to the `job_queue` table. The scheduler picks it up and runs it with your handler.

Handlers stay separate from scheduling, so background work lives outside request handlers.

### Performance
Component benchmarks cover cache, database, auth, and prerouter. Run them with `go test -bench=. -run=^$ -benchmem ./...`; releases compare results with benchstat (`.github/workflows/benchmark.yml`).

### Metrics
The framework provides built-in metrics collection using the `prometheus/client_golang` library. It includes a middleware that tracks the total number of HTTP requests (`http_server_requests_total`), a counter labeled by HTTP status code, allowing for detailed monitoring of server responses. Metrics collection can be toggled on or off via configuration without a server restart and is exposed on a configurable endpoint (e.g., `/metrics`) for a Prometheus server to scrape.

### Logger
The framework's logging is built upon the standard `slog` library for structured logging. It includes a high-performance batching handler that writes logs to the SQLite database, with configurable flush intervals and log levels. For incoming requests, a dedicated middleware logs request details but truncates overly long URI, User-Agent, Referer, and IP values to maintain clean logs. The entire logging implementation can be replaced with a user-defined logger to accommodate custom requirements.

### Notifications
The framework's notification system is designed around a `Notifier` interface, which standardizes how notifications are sent. The primary data structure, `Notification`, carries a `Type` (e.g., `Alarm`, `Metric`), `Source`, `Message`, and a map of `Fields` for additional structured data.

An official implementation for Discord is included, which sends formatted messages to a configured webhook URL. This notifier operates asynchronously, using goroutines for non-blocking `Send` calls. It incorporates a rate limiter to prevent API abuse and automatically truncates messages that exceed Discord's 2000-character limit. Developers can create custom notifiers for other services (like Slack or email) by providing their own implementation of the `Notifier` interface.

### Mailer
The framework includes a `Mailer` component for sending transactional emails over SMTP. It is designed to be flexible and resilient, handling common account management workflows.

-   **Configuration**: The mailer is configured through the application's central configuration provider, allowing for dynamic updates to SMTP settings (host, port, credentials, TLS) without a server restart.
-   **Protocol Support**: It supports standard SMTP authentication methods (`PLAIN`, `CRAM-MD5`) and connection security (explicit `TLS` and `STARTTLS`).
-   **Transactional Emails**: Pre-built methods are included for common user actions:
    -   Email address verification
    -   Password reset requests
    -   Email change notifications
-   **Asynchronous Sending**: Emails are sent in a non-blocking manner using goroutines, with context-based timeouts to prevent long-running operations from impacting application performance.
-   **Templating**: It uses simple, embedded HTML templates for emails, which can be easily customized.

### Middleware
The framework provides a collection of built-in middleware to handle common cross-cutting concerns like security, logging, and metrics.

-   **ResponseRecorder**: A utility middleware that wraps the standard `http.ResponseWriter` to capture the status code, response size, and timing information. This is used internally by other middleware like `Metrics` and `RequestLog` and should typically be the first middleware in the chain.
-   **RequestLog**: Provides structured logging for every incoming HTTP request. It captures details like method, URI, status, duration, remote IP, and user agent, with configurable length limits to keep logs concise.
-   **Metrics**: Collects Prometheus-compatible metrics for HTTP requests, labeled by status code. When activated, metrics are exposed on a configurable endpoint (e.g., `/metrics`) for scraping.
-   **BlockIp**: Acts as a dynamic IP blocking mechanism to protect the server from traffic spikes and potential denial-of-service attacks. It uses a Top-K sketch algorithm to identify and temporarily block IP addresses that are responsible for a disproportionate amount of traffic, a circuit breaker under heavy load.
-   **BlockHost**: Enforces security by validating the `Host` header of incoming requests against a configurable whitelist of allowed hostnames. It supports exact matches and wildcard subdomains (e.g., `*.example.com`).
-   **BlockRequestBody**: Limits the size of incoming request bodies to a configurable maximum. This helps prevent resource exhaustion from excessively large payloads and can be configured to exclude specific URL paths.
-   **BlockUaList**: Filters requests by matching the `User-Agent` string against a configurable regular expression. This can be used to block scrapers, bots, or other unwanted clients.
-   **TLSHeaderSTS**: Sets the `Strict-Transport-Security` (HSTS) header for all responses served over a TLS connection, instructing browsers to communicate with the server only over HTTPS.
-   **Maintenance**: When activated via configuration, this middleware puts the server into maintenance mode. It responds to all requests with a `503 Service Unavailable` status code, allowing for system updates without shutting down the server.
-   **Gzip**: Serves pre-compressed static assets (`.gz` files) from a given file system (`fs.FS`) to clients that support gzip encoding. This reduces bandwidth and improves load times. If a compressed file is not found, it seamlessly falls back to the next handler.

## Examples

Detailed examples and integration guides are available to help you build with the framework. You can explore a complete **JavaScript SDK Integration** at [restinpieces-js-sdk](https://github.com/caasmo/restinpieces-js-sdk) to see how to connect your frontend, or review implementations of **Custom Routers and DB Drivers** at [restinpieces-non-default](https://github.com/caasmo/restinpieces-non-default) for advanced customization scenarios.

## Extensibility

Beyond its core features, the framework is designed to be easily extended to meet diverse application needs. It includes a built-in file server with gzip compression for efficient delivery of static assets and a dedicated asset pipeline for minification and bundling of HTML, CSS, and JavaScript, leveraging scripts available at [restinpieces-js-sdk/gen](https://github.com/caasmo/restinpieces-js-sdk/tree/master/gen).

## Layout Best Practices

Applications built on restinpieces follow this structure:

```
myapp/
├── cmd/myapp/main.go    # entry point: flags, wiring, daemons, jobs, srv.Run()
├── app.go               # your App wrapper — embeds *core.App, adds your state
├── handlers/            # HTTP handlers as methods on *App
├── middleware/          # custom middleware (App-aware via closure, or plain funcs)
├── routes.go            # register framework + application routes
├── jobs/                # job handler implementations
├── daemons/             # daemon constructors
└── web/src/, web/dist/  # frontend assets, embedded via go:embed
```

**App wrapper.** Define your own `*App` struct that embeds `*core.App` and adds project-specific state (extra DB pools, third-party clients). Handlers are methods on `*App` — no global variables, trivial to test.

**Handlers and middleware.** Handlers use standard `http.HandlerFunc` signatures. Middleware closes over `*App` when it needs framework services; stateless middleware stays a plain `func(http.Handler) http.Handler`.

**Routes.** All route registration lives in `routes.go`. Call `app.Router().Register()` with your chains — the framework's built-in routes are registered internally by `restinpieces.New()`, yours go on top.

**Jobs and daemons.** Job handlers implement the framework's handler interface; daemons are long-running background processes. Both are constructed with the state they need and registered in `main.go` via `srv.AddJobHandler` / `srv.AddDaemon`.

**Configuration.** Store secrets encrypted via [age](https://age-encryption.org/) in the shared SQLite database. Use your own config scope (never `"application"`, which the framework reserves). Generations are immutable — save always creates a new record, giving you a full audit trail.

For full detail and code examples, see **[Layout Best Practices](doc/layout-best-practices.md)**.

## Building the Project

### Build Server

Builds the example server application.

    go build -ldflags="-s -w" -trimpath -o restinpieces_server ./cmd/example/

### Build CLI

Builds the `ripc` command-line tool.

    go build -ldflags="-s -w" -trimpath -o ripc ./cmd/ripc/

## TODO

[Todos](doc/TODO.md).
