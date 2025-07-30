<p align="center"><img alt="restinpieces" src="doc/logo.png"/></p>

[![Go Reference](https://pkg.go.dev/badge/github.com/caasmo/restinpieces)](https://pkg.go.dev/github.com/caasmo/restinpieces)
[![Test](https://github.com/caasmo/restinpieces/actions/workflows/test.yml/badge.svg)](https://github.com/caasmo/restinpieces/actions/workflows/test.yml) 
[![golangci-lint](https://github.com/caasmo/restinpieces/actions/workflows/golangci-lint.yml/badge.svg)](https://github.com/caasmo/restinpieces/actions/workflows/golangci-lint.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/caasmo/restinpieces)](https://goreportcard.com/report/github.com/caasmo/restinpieces)
[![Coverage](https://img.shields.io/endpoint?url=https://raw.githubusercontent.com/caasmo/restinpieces/master/.github/badges/coverage.json)](https://github.com/caasmo/restinpieces/actions/workflows/test.yml)
[![sloc](https://img.shields.io/endpoint?url=https://raw.githubusercontent.com/caasmo/restinpieces/master/.github/badges/sloc.json)](https://github.com/caasmo/restinpieces/actions/workflows/sloc.yml)
[![deps](https://img.shields.io/endpoint?url=https://raw.githubusercontent.com/caasmo/restinpieces/master/.github/badges/deps.json)](https://github.com/caasmo/restinpieces/actions/workflows/dependencies.yml)
[![GitHub Release](https://img.shields.io/github/v/release/caasmo/restinpieces?style=flat)]() 
[![Built Go](https://img.shields.io/badge/built_with-Go-00ADD8.svg?style=flat)]()

# REST in pieces

RestInPieces is a Go framework for building secure, high-performance API servers. It is designed to be extended and customized, providing a solid foundation for your own applications while remaining lightweight and focused. The framework uses SQLite as its default database and minimizes reliance on third-party packages, emphasizing simplicity and performance.

To get started, follow the **[Bootstrapping Guide](doc/bootstrap.md)**, which walks you through the initial setup of a new application.

## Core Philosophy: One Process Paradigm

This framework is built on the principle of "one process programming," a design philosophy that prioritizes simplicity and effectiveness by leveraging a single server instead of a complex distributed system. The core idea is straightforward: **don't use N computers when 1 will do.**

By running a single Go binary with an embedded SQLite database on one VM, this approach dramatically simplifies development, deployment, and maintenance. It provides a robust and high-performance foundation that can serve thousands of concurrent requests and support a growing business for years.

This model embraces the idea that most services never reach the scaling limits of a large, modern server. It allows developers to focus on building features and responding to customer needs rather than managing complex infrastructure. When the time comes to scale beyond a single process, the business will have the resources and clarity to do so effectively.

This approach is heavily inspired by the ideas in [One Process Programming Notes](https://crawshaw.io/blog/one-process-programming-notes).

# Content

- [Key Features](#key-features)
  - [Data Durability](#data-durability)
  - [Database Drivers](#database-drivers)
  - [Router](#router)
  - [Cache](#cache)
  - [Authentication](#authentication)
  - [Security](#security)
  - [Core Infrastructure](#core-infrastructure)
  - [Configuration Management](#configuration-management)
  - [Frontend Integration](#frontend-integration)
  - [Job Framework](#job-framework)
  - [Performance](#performance)
  - [Metrics](#metrics)
  - [Logger](#logger)
  - [Notifications](#notifications)
  - [Mailer](#mailer)
  - [Middleware](#middleware)
- [Examples](#examples)
- [Extensibility](#extensibility)
- [Building the Project](#building-the-project)
- [TODO](#todo)

## Key Features

### Data Durability
The "one process" paradigm intentionally avoids external dependencies like separate database servers, as they would violate the architectural principle of maintaining a single process per virtual machine. Consequently, the framework relies on an embedded SQLite database for data persistence. This design choice places critical importance on the durability of the single database file.

To address this, the framework provides robust mechanisms for data protection and recovery:

- **Local Backups**: The framework includes a simple, integrated backup solution for SQLite databases, managed as a background job. This can be configured and activated directly in the application's settings. It operates in two modes:
  - **Online Mode**: Performs a live backup using SQLite's Online Backup API. This allows the application to continue its operations with minimal interruption, making it ideal for active databases. The backup process copies the database page by page, with configurable pauses to reduce I/O contention.
  - **Vacuum Mode**: Creates a clean, defragmented, and compact copy of the database using the `VACUUM INTO` command. This method is thorough but requires more significant locking, making it suitable for maintenance windows or less active databases.
  
  Backups are saved as compressed `.bck.gz` archives in a configurable directory, with filenames containing a timestamp and the strategy used. You can pull those gz files from a client available at [restinpieces-sqlite-backup](https://github.com/caasmo/restinpieces-sqlite-backup/tree/master/cmd/client).

- **Real-Time Replication**: For more robust, real-time replication and point-in-time recovery, a Litestream-based integration is available in a separate repository. See [restinpieces-litestream](https://github.com/caasmo/restinpieces-litestream) for implementation details. This approach ensures that the state of the SQLite database is continuously synchronized to a remote location, providing a strong guarantee against data loss.

### Database Drivers
The framework defaults to using [zombiezen/go-sqlite](https://github.com/zombiezen/go-sqlite), a pure Go SQLite driver that offers excellent performance without relying on CGo. This simplifies the build process and ensures portability. For users who require an alternative, the framework is designed to be modular, and an implementation using the popular [crawshaw.io/sqlite](https://github.com/caasmo/restinpieces-sqlite-crawshaw) driver is also available.

### Router
The framework uses Go's standard `http.ServeMux` as its default router for simplicity and compatibility with the standard library. As of Go 1.22, the standard mux includes support for path parameters. Recognizing that different applications have different routing needs, the router is implemented as a swappable component. For those seeking maximum performance, an alternative implementation using the highly optimized [julienschmidt/httprouter](https://github.com/julienschmidt/httprouter) is also provided. See [restinpieces-httprouter](https://github.com/caasmo/restinpieces-httprouter) for details.

### Cache
For in-memory caching, the framework uses [Ristretto](https://github.com/dgraph-io/ristretto), a high-performance, concurrent cache. The caching system is designed around a simple interface, allowing developers to easily swap in their own caching implementation if needed.

### Authentication
The framework provides a comprehensive authentication system built around JSON Web Tokens (JWT). Session management is handled via bearer tokens sent in the `Authorization` header. A key security feature is the use of dynamic JWT signing keys, which are derived from a combination of user-specific credentials (email and password hash) and a global server secret. This ensures that a token's signature is invalidated if a user's password changes.

The system supports multiple authentication and account management workflows through a set of API endpoints:

- **Password-based**: Includes endpoints for user registration (`/register-with-password`), login (`/auth-with-password`), and token refresh (`/auth-refresh`).
- **OAuth2**: Provides a generic flow (`/auth-with-oauth2`) to authenticate users via third-party providers. It handles the token exchange, fetches user information, and creates or links the user account in the local database. An endpoint (`/list-oauth2-providers`) is available to discover configured providers.
- **Account Management**: All account management processes, such as email verification, password reset, and email address changes, are handled through secure, multi-step flows. These flows typically involve generating a unique, short-lived JWT that is sent to the user's email via a background job queue, which the user then submits back to a confirmation endpoint.

### Security
The "one process" paradigm simplifies deployment by running a single binary on a single VM, but it also means the application is directly exposed to the internet without a reverse proxy like Nginx acting as a first line of defense. This necessitates a defensive approach to security. The framework addresses this with a suite of built-in middleware designed to protect the server from common threats. These include dynamic IP blocking (`BlockIp`) to mitigate traffic spikes, hostname validation against a whitelist (`BlockHost`), request body size limitation (`BlockRequestBody`), and `User-Agent` filtering (`BlockUaList`). The framework also helps secure client communications by automatically setting security headers like `Strict-Transport-Security`.

### Core Infrastructure
- Uses middleware and handler standard Go patterns
- Discoverable API endpoints (/api/refresh-auth, /api/auth-with-oauth2, etc.)

### Configuration Management
The framework's configuration is securely managed within the SQLite database. The configuration is stored as encrypted, TOML-formatted content in the `app_config` table, the schema for which is detailed in `migrations/schema/app/app_config.sql`. Management is performed using the `ripc` command-line tool, which supports versioning, diffing, and rollbacks. Beyond managing the core application's settings, `ripc` can be extended to handle custom configuration scopes for your own modules. For more details on the tool, see the [`ripc` documentation](doc/ripc.md).

A key feature is support for dynamic updates. The server listens for the `SIGHUP` signal to trigger a hot-reload of the configuration, allowing most settings to be changed in real-time without service interruption. While the majority of parameters can be updated on-the-fly, critical changes like modifications to TLS certificates require a full server reload to be applied.

### Frontend Integration
- JavaScript SDK for seamless frontend-backend interaction
- Example usage of the SDK and authentication endpoints available at [restinpieces-js-sdk](https://github.com/caasmo/restinpieces-js-sdk)

### Job Framework
The framework includes a robust job queue system for handling asynchronous tasks, supporting both one-time and recurrent jobs. This is essential for offloading work from the request-response cycle, such as sending emails, processing data, or performing periodic maintenance.

The system is composed of a scheduler that claims jobs from the `job_queue` table and an executor that runs the corresponding handler. The framework provides built-in handlers for core functionalities like sending password reset emails, email verifications, and performing local database backups.

You can easily extend the system to run your own custom tasks. This involves two main steps:
1.  **Write a Job Handler**: Create a new handler that implements the `JobHandler` interface. This is where you define the logic for your task.
2.  **Insert a Job**: Add a new record to the `job_queue` table in the database. The scheduler will automatically pick it up and execute it using your custom handler.

This design allows for a clean separation of concerns and makes it straightforward to add new background processing capabilities to your application.

### Performance
- Optimized for high throughput (thousands of requests/second)
- Minimal external dependencies
- Production-ready builds with size optimization

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
