# Application Layout Best Practices

This guide describes the recommended project structure and conventions for applications
built on top of **restinpieces**. It follows standard Go project layout with no
unnecessary indirection.

---

## Directory Structure

```
myapp/
├── cmd/
│   └── myapp/
│       └── main.go       # entry point: flags, wiring, daemons, jobs, srv.Run()
├── app.go                # your App wrapper — own state + access to framework App
├── handlers/             # HTTP handlers as methods on *App
├── middleware/           # custom middleware functions
├── routes.go             # full route registration (framework + yours)
├── jobs/                 # job handler implementations
├── daemons/              # daemon constructors and configuration
└── web/
    ├── src/              # frontend source
    └── dist/             # built assets, embedded via go:embed
```

---

## The App Wrapper

The framework provides `*core.App` as its runtime context. Your application defines
its own `App` struct that holds `*core.App` plus any additional heavy state — extra
database pools, third-party clients, compiled templates, and so on.

```go
// app.go
package myapp

import (
    "github.com/caasmo/restinpieces/core"
    "github.com/caasmo/restinpieces/db"
)

// App is your application's runtime context.
// It wraps the framework App and adds project-specific heavy state.
type App struct {
    *core.App

    // Additional pools or clients owned by your application.
    // Use the framework's shared SQLite pool (via core.App) for the main database.
    // Add a second pool only when you need a separate SQLite file.
    analyticsDB db.Pool
}

func NewApp(core *core.App, analyticsDB db.Pool) *App {
    return &App{
        App:         core,
        analyticsDB: analyticsDB,
    }
}
```

**Why your own wrapper and not directly `*core.App`?**

Handlers need access to your state — not just the framework's. Attaching them to your
own `*App` keeps that dependency explicit, avoids global variables, and makes handlers
trivially testable by constructing a minimal `*App` in tests.

---

## Handlers

Handlers are methods on `*App`. They use the standard `http.HandlerFunc` signature
and rely on the framework via `a.App`.

```go
// handlers/users.go  (or admin.go, etc.)
package myapp

import (
    "net/http"
)

func (a *App) GetUserHandler(w http.ResponseWriter, r *http.Request) {
    // a.DbAuth()  — framework database
    // a.Cache()   — framework cache
    // a.Logger()  — framework logger
    // a.analyticsDB — your own state
}

func (a *App) CreateUserHandler(w http.ResponseWriter, r *http.Request) {
    // ...
}
```

---

## Middleware

Custom middleware follows the standard Go signature. It receives `*App` via closure
when it needs application state.

```go
// middleware/tenant.go
package myapp

import "net/http"

// TenantMiddleware resolves the tenant from the request and injects it into context.
func (a *App) TenantMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // use a.DbAuth(), a.Cache(), etc.
        next.ServeHTTP(w, r)
    })
}

// Stateless middleware that needs no App state can be a plain function.
func RequestIDMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // inject request ID
        next.ServeHTTP(w, r)
    })
}
```

Middlewares execute left-to-right as written in `WithMiddleware`. The first argument
is the outermost handler — it runs first.

```go
router.NewChain(http.HandlerFunc(a.GetUserHandler)).
    WithMiddleware(
        RequestIDMiddleware,    // runs first
        a.TenantMiddleware,     // runs second
    )
```

---

## Routes

`routes.go` owns all route registration. The framework registers its own built-in
routes internally during `restinpieces.New()`; your file adds your application routes
on top using the same `router.Chains` map.

```go
// routes.go
package myapp

import (
    "net/http"

    "github.com/caasmo/restinpieces/core"
    r "github.com/caasmo/restinpieces/router"
)

func (a *App) RegisterRoutes() {
    a.Router().Register(r.Chains{
        "/api/users":       r.NewChain(http.HandlerFunc(a.GetUserHandler)).
                                WithMiddleware(a.TenantMiddleware),
        "/api/users/create": r.NewChain(http.HandlerFunc(a.CreateUserHandler)).
                                WithMiddleware(
                                    RequestIDMiddleware,
                                    a.TenantMiddleware,
                                ),
    })
}
```

Call `a.RegisterRoutes()` from `main.go` after `restinpieces.New()` returns.

---

## Jobs

A job handler processes a single job type from the queue. Implement the interface the
framework expects and attach it to `*App` if it needs application state.

```go
// jobs/cert_renewal.go
package myapp

import (
    "context"
    "log/slog"
)

// CertRenewalHandler handles certificate renewal jobs.
type CertRenewalHandler struct {
    app *App
}

func NewCertRenewalHandler(app *App) *CertRenewalHandler {
    return &CertRenewalHandler{app: app}
}

func (h *CertRenewalHandler) Handle(ctx context.Context, payload []byte) error {
    h.app.Logger().Info("renewing certificate")
    // use h.app.ConfigStore() to read/write encrypted config
    return nil
}
```

Register in `main.go`:

```go
srv.AddJobHandler(JobTypeCertRenewal, myapp.NewCertRenewalHandler(app))
```

---

## Daemons

A daemon is a long-running background process managed by the server lifecycle.
Construct it with whatever state it needs, then hand it to `srv.AddDaemon`.

```go
// daemons/litestream.go
package myapp

import (
    "github.com/caasmo/restinpieces/litestream"
)

func NewLitestream(app *App) (*litestream.Litestream, error) {
    return litestream.New(app.App) // passes the framework App
}
```

Register in `main.go`:

```go
ls, err := myapp.NewLitestream(app)
if err != nil {
    slog.Error("failed to init litestream", "error", err)
    os.Exit(1)
}
srv.AddDaemon(ls)
```

---

## Configuration and Secrets

The framework stores configuration as encrypted records in the shared SQLite database
using [age](https://age-encryption.org/) encryption. Each record belongs to a **scope**.
The framework reserves the `"application"` scope for its own config.

**Your application must use its own scope** — one scope per logical subsystem is
recommended.

```go
// Save a secret (e.g. on first setup or config update)
err := app.ConfigStore().Save(
    "payments",                   // your scope — never "application"
    []byte(`{"api_key":"sk_..."}`),
    "json",
    "initial payments config",
)

// Load it at startup or on demand
data, format, err := app.ConfigStore().Get("payments", 0) // 0 = latest generation
```

The framework CLI provides `diff`, `rollback`, and `history` commands that work
across all scopes, including yours. Generations are immutable — saving always creates
a new record, giving you a full audit trail with zero extra code.

---

## Entry Point

`cmd/myapp/main.go` does one thing: wire everything together and start the server.
No business logic, no handler code.

```go
// cmd/myapp/main.go
package main

import (
    "flag"
    "fmt"
    "io/fs"
    "log/slog"
    "net/http"
    "os"

    "github.com/caasmo/restinpieces"
    "github.com/caasmo/restinpieces/core"
    r "github.com/caasmo/restinpieces/router"

    "github.com/yourname/myapp"
    "github.com/yourname/myapp/web"
)

func main() {
    dbPath    := flag.String("dbpath",  "", "Path to the SQLite database file (required)")
    ageKeyPath := flag.String("age-key", "", "Path to the age identity file (required)")
    flag.Usage = func() {
        fmt.Fprintf(os.Stderr, "Usage: %s -dbpath <path> -age-key <path>\n", os.Args[0])
        flag.PrintDefaults()
    }
    flag.Parse()
    if *dbPath == "" || *ageKeyPath == "" {
        flag.Usage()
        os.Exit(1)
    }

    // 1. Database pool
    dbPool, err := restinpieces.NewZombiezenPerformancePool(*dbPath)
    if err != nil {
        slog.Error("failed to create database pool", "error", err)
        os.Exit(1)
    }
    defer func() {
        if err := dbPool.Close(); err != nil {
            slog.Error("error closing database pool", "error", err)
        }
    }()

    // 2. Framework App + server
    coreApp, srv, err := restinpieces.New(
        restinpieces.WithZombiezenPool(dbPool),
        restinpieces.WithAgeKeyPath(*ageKeyPath),
    )
    if err != nil {
        slog.Error("failed to initialize application", "error", err)
        os.Exit(1)
    }

    // 3. Your App wrapper (add your own pools/clients here)
    app := myapp.NewApp(coreApp)

    // 4. Static assets
    subFS, err := fs.Sub(web.Assets, "dist")
    if err != nil {
        slog.Error("failed to create sub filesystem", "error", err)
        os.Exit(1)
    }
    ffs := http.FileServerFS(subFS)
    app.Router().Register(r.Chains{
        "/": r.NewChain(ffs).WithMiddleware(
            core.StaticHeadersMiddleware,
            core.GzipMiddleware(subFS),
        ),
    })

    // 5. Application routes
    app.RegisterRoutes()

    // 6. Daemons
    ls, err := myapp.NewLitestream(app)
    if err != nil {
        slog.Error("failed to init litestream", "error", err)
        os.Exit(1)
    }
    srv.AddDaemon(ls)

    // 7. Job handlers
    srv.AddJobHandler(myapp.JobTypeCertRenewal, myapp.NewCertRenewalHandler(app))

    // 8. Run
    srv.Run()
}
```

---

## Summary

| Concern        | Directory/File      | Receiver / Pattern                        |
|----------------|---------------------|-------------------------------------------|
| Entry point    | `cmd/myapp/main.go` | none — wiring only                        |
| App state      | `app.go`            | `*App` wrapping `*core.App`               |
| Handlers       | `handlers/`         | methods on `*App`                         |
| Middleware     | `middleware/`       | closure over `*App`, or plain func        |
| Routes         | `routes.go`         | method on `*App`, called from main        |
| Jobs           | `jobs/`             | struct with `*App`, implements job iface  |
| Daemons        | `daemons/`          | constructor returning framework type      |
| Secrets/config | anywhere            | `app.ConfigStore()` with your own scope   |
| Frontend       | `web/`              | embedded `fs.FS`, served from `main.go`   |
