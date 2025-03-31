# REST in pieces

A one-file golang server using sqlite, with focus on simplicity, performance and avoiding 3-party packages as much as possible.

## What it provides out of the box

- auth workflows: login, register with password and oauth2, verifying email, confirm email. 
- oauth2 implementation for register and login
- authentication, session management with JWT
- core API discoverable endpoints: /api/refresh-auth, /api/auth-with-oauth2, /api/list-oauth2-providers ....
- File server embedded in the binary with gzip support. 
- in house bundler (minifier, gzip) for css, javascript and html 
- db interface for suport of differentt slite providers. zombiezen and crawshaw implementations
- cache interface for cache providers. Ristretto implementation
- router interface for router providers. standard Mux and httprouter implementations
- standard handlers and middleware support. Just plug in 3 party middleware
- smtp client implementation 
- simple javascript SDK
- internal async worker with queue implementation to process asyn jobs like sending emails
- Security headers
- Ip blocking, easily extensible to other headers
- Performance: thousand of request per second
- Working html pages examples for the CORE API, login, register, session management.


## Building the Project

### Asset Generation
To bundle and optimize frontend assets (HTML, CSS, JavaScript) with minification and gzip compression:

    go generate

This creates production-ready assets in `public/dist/` with both compressed (.gz) and uncompressed versions.

### Production Build
For a production build with optimized static assets, security headers, and proper caching:

    go build -ldflags="-s -w" -trimpath ./cmd/restinpieces/...

Flags explanation:
- `-ldflags="-s -w"` - Strips debug symbols to reduce binary size
- `-trimpath` - Removes filesystem paths from compiled binary for reproducibility

### Development Build
For development with relaxed security headers and debugging support:

    go build -ldflags="-s -w" -trimpath -tags dev ./cmd/restinpieces/...

The `dev` tag disables strict security headers for easier local development.


## TODO

[Todos](doc/TODO.md).

