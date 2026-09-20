# Application Layout Best Practices

This guide describes the recommended project structure and conventions for applications built on top of **restinpieces**. It keeps application logic consolidated while making the routing table explicit and easy to find.

---

## Directory Structure

```
myapp/
├── cmd/
│   └── myapp/
│       └── main.go       # entry point: flags, wiring, srv.Run()
├── routes.go             # explicit route registration
├── app/                  # all application logic and state
│   ├── app.go            # App struct and constructor
│   ├── handler_users.go  # handlers as methods on *App
│   └── middleware_auth.go # middleware as methods on *App
└── web/
    ├── embed.go          # go:embed dist/
    ├── internal/         # HTML pages, hidden from the catch-all file server
    ├── src/              # frontend source (React, Vue, etc.)
    └── dist/             # built assets, bundled by Vite/Webpack
```

---

## The App Package

The application logic lives in the `app/` package. This avoids cluttering the root and allows handlers to access private state safely.

### The App Wrapper (`app/app.go`)

Your application defines its own `App` struct that holds `*core.App` plus whatever state your app needs. Keep the extra fields private and pass them through the constructor.

```go
// app/app.go
package app

import (
    "io/fs"
    "net/http"
    "github.com/caasmo/restinpieces/core"
)

type App struct {
    *core.App
    fs fs.FS
    // your own state here, kept private
}

func NewApp(coreApp *core.App, fsys fs.FS) *App {
    return &App{
        App: coreApp,
        fs:  fsys,
    }
}

func (a *App) FS() fs.FS {
    return a.fs
}

func (a *App) Err(code int) http.HandlerFunc {
    // return your pre-loaded error page for code
    return http.NotFound
}
```

### Handlers (`app/handler_users.go`)

Handlers are methods on `*App`. This gives them direct access to all application dependencies without using global variables or complex interfaces.

```go
// app/handler_users.go
package app

import "net/http"

func (a *App) GetUserHandler(w http.ResponseWriter, r *http.Request) {
    // a.Logger().Info("getting user")
}
```

### Middleware (`app/middleware_auth.go`)

Middleware that needs application state are also methods on `*App`. Name each file after what it does (`middleware_ssr.go`, `middleware_assets.go`), there is no single `middleware.go`.

```go
// app/middleware_auth.go
package app

import "net/http"

func (a *App) TenantMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // use a.App dependencies
        next.ServeHTTP(w, r)
    })
}
```

---

## Explicit Routes

The `routes.go` file lives in the project root. It defines the application's routing map. This makes the "shape" of your API immediately visible at the top level of the project.

```go
// routes.go
package myapp

import (
    "net/http"
    "github.com/caasmo/restinpieces/core"
    r "github.com/caasmo/restinpieces/router"
    "github.com/yourname/myapp/app"
)

func Routes(a *app.App) {
    a.Router().Register(r.Chains{
        "GET /api/users": r.NewChain(http.HandlerFunc(a.GetUserHandler)).
                        WithMiddleware(a.TenantMiddleware),
        "/api/": r.NewChain(http.HandlerFunc(core.NotFoundJSONHandler)),
        "/": r.NewChain(
            core.FSHandler(a.FS(), core.CompressExtGzip, a.Err(404)),
        ).WithMiddleware(a.TenantMiddleware),
    })
}
```

Route keys always start with the method (`GET /api/users`, `POST /api/users/add`). HTML pages live in `web/internal/` so the catch-all `/` file server can only serve static assets and returns 404 for anything else. `a.FS()` and `a.Err()` are your own helpers on `App` that expose the embedded `dist/` filesystem and the pre-loaded error pages. `Routes` only wires ready dependencies, it does not build them.

---

## Frontend (`web/embed.go`)

The frontend is a Go package, not a separate service. `web/embed.go` compiles `dist/` into the binary, and `main.go` opens it once with `fs.Sub`.

```go
// web/embed.go
package web

import "embed"

//go:embed dist
var Assets embed.FS
```

---

## Entry Point (`cmd/myapp/main.go`)

The entry point wires the `app` state to the `Routes` and starts the server. The registration is explicit, so you can see exactly how the application is composed.

```go
// cmd/myapp/main.go
package main

import (
    "io/fs"
    "github.com/caasmo/restinpieces"
    "github.com/yourname/myapp"
    "github.com/yourname/myapp/app"
    "github.com/yourname/myapp/web"
)

func main() {
    // ... setup your own state ...

    coreApp, srv, err := restinpieces.New(...)

    // 1. Build the embedded filesystem once, fail fast if dist/ is missing
    subFS, err := fs.Sub(web.Assets, "dist")
    if err != nil {
        panic(err)
    }

    // 2. Initialize application state
    a := app.NewApp(coreApp, subFS)

    // 3. Explicitly wire routes
    myapp.Routes(a)

    // 4. Register background work here, there are no jobs/ or daemons/ folders
    // srv.AddDaemon(...)
    // srv.AddJobHandler(...)

    // 5. Run
    srv.Run()
}
```

---

## Summary

| Concern     | Location               | Pattern                                          |
|-------------|------------------------|--------------------------------------------------|
| Entry point | `cmd/myapp/main.go`    | Builds `dist/` FS, wires `app` to `Routes`, runs `srv` |
| App State   | `app/app.go`           | `App` struct wrapping `*core.App` plus your own state |
| Handlers    | `app/handler_*.go`     | Methods on `*app.App`                            |
| Middleware  | `app/middleware_*.go`  | Methods on `*app.App` or plain functions         |
| Routes Map  | `routes.go` (root)     | Function `Routes(a *app.App)` with method-prefixed keys |
| Frontend    | `web/`                 | `embed.go` embeds `dist/`, HTML lives in `internal/` |
