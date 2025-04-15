[![Go Reference](https://pkg.go.dev/badge/github.com/caasmo/restinpieces)](https://pkg.go.dev/github.com/caasmo/restinpieces)
[![Go Report Card](https://goreportcard.com/badge/github.com/caasmo/restinpieces)](https://goreportcard.com/report/github.com/caasmo/restinpieces)
![sloc](https://sloc.xyz/github/caasmo/restinpieces)

# REST in pieces

A one-file golang server using sqlite, with focus on simplicity, performance and avoiding 3-party packages as much as possible.

# Content

- [Key Features](#key-features)
  - [Authentication](#authentication)
  - [Security](#security)
  - [Core Infrastructure](#core-infrastructure)
  - [Frontend Integration](#frontend-integration)
  - [Background Processing](#background-processing)
  - [Performance](#performance)
- [Building the Project](#building-the-project)
  - [Asset Generation](#asset-generation)
  - [Production Build](#production-build)
  - [Development Build](#development-build)
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
- IP blocking based on rate limiting

### Core Infrastructure
- Embedded file server with gzip compression
- Discoverable API endpoints (/api/refresh-auth, /api/auth-with-oauth2, etc.)
- SQLite database interface supporting multiple drivers ([Zombiezen](https://github.com/zombiezen/go-sqlite), [Crawshaw](https://github.com/caasmo/restinpieces-sqlite-crawshaw))
- Cache interface with [Ristretto](https://github.com/dgraph-io/ristretto) implementation
- Router abstraction supporting standard Mux and [httprouter](https://github.com/julienschmidt/httprouter)
- Middleware-compatible handlers

### Frontend Integration
- JavaScript SDK for seamless frontend-backend interaction
- Example frontend pages demonstrating all core functionality
- Built-in asset pipeline (minification + gzip bundling for HTML/CSS/JS)
- Example usage of the SDK and authentication endpoints available at [restinpieces-js-sdk](https://github.com/caasmo/restinpieces-js-sdk)

### Background Processing  
- Job queue system for async tasks (email sending, etc.)
- Worker implementation for processing background jobs

### Performance
- Optimized for high throughput (thousands of requests/second)
- Minimal external dependencies
- Production-ready builds with size optimization

### Backups
- Built-in Litestream integration for continuous SQLite backups
- Supports incremental backups with minimal overhead
- See [restinpieces-litestream](https://github.com/caasmo/restinpieces-litestream) for implementation details


## Building the Project


### Build

    // default pure go zombiezen
    go build -ldflags="-s -w" -trimpath ./cmd/restinpieces/...

    // crawshaw CGO
    go build -tags sqlite_crawshaw -ldflags="-s -w" -trimpath ./cmd/restinpieces/...


## TODO

[Todos](doc/TODO.md).

