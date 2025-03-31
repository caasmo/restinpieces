# REST in pieces

A one-file golang server using sqlite, with focus on simplicity, performance and avoiding 3-party packages as much as possible. 

## Key Features

### Authentication
- Complete authentication workflows:
  - Password-based registration/login
  - OAuth2 integration for social login
  - Email verification with confirmation flow
- JWT-based session management
- Token refresh flow
- User management interfaces

### Security
- Built-in security headers (CSP, CORS, etc.)
- IP blocking and rate limiting

### Core Infrastructure
- Embedded file server with gzip compression
- Discoverable API endpoints (/api/refresh-auth, /api/auth-with-oauth2, etc.)
- SQLite database interface supporting multiple drivers ([Zombiezen](https://github.com/zombiezen/go-sqlite), [Crawshaw](https://github.com/crawshaw/sqlite))
- Cache interface with [Ristretto](https://github.com/dgraph-io/ristretto) implementation
- Router abstraction supporting standard Mux and [httprouter](https://github.com/julienschmidt/httprouter)
- Middleware-compatible handlers

### Frontend Integration
- JavaScript SDK for seamless frontend-backend interaction
- Example frontend pages demonstrating all core functionality
- Built-in asset pipeline (minification + gzip bundling for HTML/CSS/JS)

### Background Processing  
- Job queue system for async tasks (email sending, etc.)
- Worker implementation for processing background jobs

### Performance
- Optimized for high throughput (thousands of requests/second)
- Minimal external dependencies
- Production-ready builds with size optimization


## Building the Project

### Asset Generation
To bundle and optimize frontend assets (HTML, CSS, JavaScript) with minification and gzip compression:

    go generate

This creates production-ready assets in `public/dist/` with both compressed (.gz) and uncompressed versions.

### Production Build
For a production build with optimized static assets, security headers, and proper caching:

    go build -ldflags="-s -w" -trimpath ./cmd/restinpieces/...

### Development Build
For development with relaxed security headers and debugging support:

    go build -ldflags="-s -w" -trimpath -tags dev ./cmd/restinpieces/...

The `dev` tag disables strict security headers for easier local development.


## TODO

[Todos](doc/TODO.md).

