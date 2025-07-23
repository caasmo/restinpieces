<p align="center"><img alt="restinpieces" src="doc/logo.png"/></p>

[![Go Reference](https://pkg.go.dev/badge/github.com/caasmo/restinpieces)](https://pkg.go.dev/github.com/caasmo/restinpieces)
[![Test](https://github.com/caasmo/restinpieces/actions/workflows/test.yml/badge.svg)](https://github.com/caasmo/restinpieces/actions/workflows/test.yml) 
[![golangci-lint](https://github.com/caasmo/restinpieces/actions/workflows/golangci-lint.yml/badge.svg)](https://github.com/caasmo/restinpieces/actions/workflows/golangci-lint.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/caasmo/restinpieces)](https://goreportcard.com/report/github.com/caasmo/restinpieces)
[![Coverage](.github/badges/coverage.svg)](https://github.com/caasmo/restinpieces/actions/workflows/test.yml)
[![sloc](.github/badges/sloc.svg)](https://github.com/caasmo/restinpieces/actions/workflows/sloc.yml)
[![GitHub Release](https://img.shields.io/github/v/release/caasmo/restinpieces?style=flat)]() 
[![Built Go](https://img.shields.io/badge/built_with-Go-00ADD8.svg?style=flat)]()

# REST in pieces

RestInPieces is a Go framework for building secure, high-performance API servers. It is designed to be extended and customized, providing a solid foundation for your own applications while remaining lightweight and focused. The framework uses SQLite as its default database and minimizes reliance on third-party packages, emphasizing simplicity and performance.

To get started, follow the **[Bootstrapping Guide](doc/bootstrap.md)**, which walks you through the initial setup of a new application.


# Content

- [Key Features](#key-features)
  - [Authentication](#authentication)
  - [Security](#security)
  - [Core Infrastructure](#core-infrastructure)
  - [Configuration Management](#configuration-management)
  - [Frontend Integration](#frontend-integration)
  - [Background Processing](#background-processing)
  - [Performance](#performance)
  - [Backups](#backups)
  - [Metrics](#metrics)
  - [Logger](#logger)
  - [Notifications](#notifications)
  - [Middleware](#middleware)
- [Examples](#examples)
- [Extensibility](#extensibility)
- [Building the Project](#building-the-project)
- [TODO](#todo)

## Key Features

### Authentication
The framework provides a comprehensive authentication system built around JSON Web Tokens (JWT). Session management is handled via bearer tokens sent in the `Authorization` header. A key security feature is the use of dynamic JWT signing keys, which are derived from a combination of user-specific credentials (email and password hash) and a global server secret. This ensures that a token's signature is invalidated if a user's password changes.

The system supports multiple authentication and account management workflows through a set of API endpoints:

- **Password-based**: Includes endpoints for user registration (`/register-with-password`), login (`/auth-with-password`), and token refresh (`/auth-refresh`).
- **OAuth2**: Provides a generic flow (`/auth-with-oauth2`) to authenticate users via third-party providers. It handles the token exchange, fetches user information, and creates or links the user account in the local database. An endpoint (`/list-oauth2-providers`) is available to discover configured providers.
- **Account Management**: All account management processes, such as email verification, password reset, and email address changes, are handled through secure, multi-step flows. These flows typically involve generating a unique, short-lived JWT that is sent to the user's email via a background job queue, which the user then submits back to a confirmation endpoint.

### Security
- Built-in security headers (CSP, CORS, etc.)
- Dynamic IP blocking based on traffic patterns
- User-Agent blocking based on configuration
- Request body blocking based on configurable size limits
- Hostname validation against a configurable whitelist.

### Core Infrastructure
- Uses middleware and handler standard Go patterns
- Router abstraction supporting standard Mux and [httprouter](https://github.com/julienschmidt/httprouter) (example implementation at [restinpieces-httprouter](https://github.com/caasmo/restinpieces-httprouter))
- Discoverable API endpoints (/api/refresh-auth, /api/auth-with-oauth2, etc.)
- SQLite database interface with pure Go [Zombiezen](https://github.com/zombiezen/go-sqlite) as default driver
  - Alternative drivers available in separate repos (like [Crawshaw](https://github.com/caasmo/restinpieces-sqlite-crawshaw))
- Cache interface with [Ristretto](https://github.com/dgraph-io/ristretto) implementation
- Hot reloading of configuration without server restart

### Configuration Management
- All configuration is stored encrypted in the SQLite database as serialized TOML files. The `ripc` command-line tool is provided to manage this configuration.
- Key features of `ripc` include:
  - Versioned configuration with rollback support
  - JWT secret rotation
  - OAuth2 provider management


### Frontend Integration
- JavaScript SDK for seamless frontend-backend interaction
- Example usage of the SDK and authentication endpoints available at [restinpieces-js-sdk](https://github.com/caasmo/restinpieces-js-sdk)

### Background Processing  
- Job queue system for async tasks (email sending, etc.)
- Worker implementation for processing background jobs

### Performance
- Optimized for high throughput (thousands of requests/second)
- Minimal external dependencies
- Production-ready builds with size optimization

### Backups
- **Built-in Local Backups**: The framework includes a simple, integrated backup solution for SQLite databases, managed as a background job. This can be configured and activated directly in the application's settings. It operates in two modes:
  - **Online Mode**: Performs a live backup using SQLite's Online Backup API. This allows the application to continue its operations with minimal interruption, making it ideal for active databases. The backup process copies the database page by page, with configurable pauses to reduce I/O contention.
  - **Vacuum Mode**: Creates a clean, defragmented, and compact copy of the database using the `VACUUM INTO` command. This method is thorough but requires more significant locking, making it suitable for maintenance windows or less active databases.
- Backups are saved as compressed `.bck.gz` archives in a configurable directory, with filenames containing a timestamp and the strategy used. You can pull those gz files from a client available at [restinpieces-sqlite-backup](https://github.com/caasmo/restinpieces-sqlite-backup/tree/master/cmd/client).
- **Litestream Integration**: For more robust, real-time replication and point-in-time recovery, a Litestream-based integration is available in a separate repository. See [restinpieces-litestream](https://github.com/caasmo/restinpieces-litestream) for implementation details.

### Metrics
The framework provides built-in metrics collection using the `prometheus/client_golang` library. It includes a middleware that tracks the total number of HTTP requests (`http_server_requests_total`), a counter labeled by HTTP status code, allowing for detailed monitoring of server responses. Metrics collection can be toggled on or off via configuration without a server restart and is exposed on a configurable endpoint (e.g., `/metrics`) for a Prometheus server to scrape.

### Logger
The framework's logging is built upon the standard `slog` library for structured logging. It includes a high-performance batching handler that writes logs to the SQLite database, with configurable flush intervals and log levels. For incoming requests, a dedicated middleware logs request details but truncates overly long URI, User-Agent, Referer, and IP values to maintain clean logs. The entire logging implementation can be replaced with a user-defined logger to accommodate custom requirements.

### Notifications

The framework's notification system is designed around a `Notifier` interface, which standardizes how notifications are sent. The primary data structure, `Notification`, carries a `Type` (e.g., `Alarm`, `Metric`), `Source`, `Message`, and a map of `Fields` for additional structured data.

An official implementation for Discord is included, which sends formatted messages to a configured webhook URL. This notifier operates asynchronously, using goroutines for non-blocking `Send` calls. It incorporates a rate limiter to prevent API abuse and automatically truncates messages that exceed Discord's 2000-character limit. Developers can create custom notifiers for other services (like Slack or email) by providing their own implementation of the `Notifier` interface.

### Middleware
The framework provides a collection of built-in middleware to handle common cross-cutting concerns like security, logging, and metrics.

-   **ResponseRecorder**: A utility middleware that wraps the standard `http.ResponseWriter` to capture the status code, response size, and timing information. This is used internally by other middleware like `Metrics` and `RequestLog` and should typically be the first middleware in the chain.
-   **RequestLog**: Provides structured logging for every incoming HTTP request. It captures details like method, URI, status, duration, remote IP, and user agent, with configurable length limits to keep logs concise.
-   **Metrics**: Collects Prometheus-compatible metrics for HTTP requests, labeled by status code. When activated, metrics are exposed on a configurable endpoint (e.g., `/metrics`) for scraping.
-   **BlockIp**: Acts as a dynamic IP blocking mechanism to protect the server from traffic spikes and potential denial-of-service attacks. It uses a Top-K sketch algorithm to identify and temporarily block IP addresses that are responsible for a disproportionate amount of traffic, functioning as a circuit breaker under heavy load.
-   **BlockHost**: Enforces security by validating the `Host` header of incoming requests against a configurable whitelist of allowed hostnames. It supports exact matches and wildcard subdomains (e.g., `*.example.com`).
-   **BlockRequestBody**: Limits the size of incoming request bodies to a configurable maximum. This helps prevent resource exhaustion from excessively large payloads and can be configured to exclude specific URL paths.
-   **BlockUaList**: Filters requests by matching the `User-Agent` string against a configurable regular expression. This can be used to block scrapers, bots, or other unwanted clients.
-   **TLSHeaderSTS**: Sets the `Strict-Transport-Security` (HSTS) header for all responses served over a TLS connection, instructing browsers to communicate with the server only over HTTPS.
-   **Maintenance**: When activated via configuration, this middleware puts the server into maintenance mode. It responds to all requests with a `503 Service Unavailable` status code, allowing for system updates without shutting down the server.

## Examples

- **JavaScript SDK Integration**: See how to integrate with the frontend using the official JavaScript SDK at [restinpieces-js-sdk](https://github.com/caasmo/restinpieces-js-sdk).
- **Custom Routers and DB Drivers**: Explore examples of using non-default routers and database drivers at [restinpieces-non-default](https://github.com/caasmo/restinpieces-non-default).

## Extensibility
- Embedded file server with gzip compression for serving static assets.
- Built-in asset pipeline (minification + gzip bundling for HTML/CSS/JS) including scripts at [restinpieces-js-sdk/gen](https://github.com/caasmo/restinpieces-js-sdk/tree/master/gen).

## Building the Project

### Build Server

Builds the example server application.

    go build -ldflags="-s -w" -trimpath -o restinpieces_server ./cmd/example/

### Build CLI

Builds the `ripc` command-line tool.

    go build -ldflags="-s -w" -trimpath -o ripc ./cmd/ripc/

## TODO

[Todos](doc/TODO.md).
