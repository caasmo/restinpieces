<p align="center"><img alt="restinpieces" src="doc/logo.png"/></p>

[![Test](https://github.com/caasmo/restinpieces/actions/workflows/test.yml/badge.svg)](https://github.com/caasmo/restinpieces/actions/workflows/test.yml) [![golangci-lint](https://github.com/caasmo/restinpieces/actions/workflows/golangci-lint.yml/badge.svg)](https://github.com/caasmo/restinpieces/actions/workflows/golangci-lint.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/caasmo/restinpieces)](https://pkg.go.dev/github.com/caasmo/restinpieces)
[![Go Report Card](https://goreportcard.com/badge/github.com/caasmo/restinpieces)](https://goreportcard.com/report/github.com/caasmo/restinpieces)
![sloc](https://sloc.xyz/github/caasmo/restinpieces)
[![GitHub Release](https://img.shields.io/badge/built_with-Go-00ADD8.svg?style=flat)]()
![Coverage](.github/badges/coverage.svg)


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
- [Examples](#examples)
- [Extensibility](#extensibility)
- [Building the Project](#building-the-project)
- [TODO](#todo)

## Key Features

### Authentication
- Complete authentication workflows:
  - Password-based registration/login
  - OAuth2 integration for social login
  - Email verification with confirmation flow
  - Password reset via email
  - Email address change with confirmation
- JWT-based session management

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
- Integrated Prometheus client for collecting application metrics.
- Configurable endpoint for exposing metrics (e.g., `/metrics`).
- Toggle metrics collection on/off via configuration without requiring a server restart.

### Logger
- Default structured logger based on `slog`.
- High-performance batch logging to SQLite database.
- Configurable log levels and flush intervals.
- Request logging with configurable limits for URI, User-Agent, Referer, and Remote IP lengths.
- Supports overriding the default logger with a custom user-defined logger.

### Notifications
- Flexible notification system for various events.
- Default implementation for sending notifications to Discord webhooks.
- Extensible to support other notification channels.

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
