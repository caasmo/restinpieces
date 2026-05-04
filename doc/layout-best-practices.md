# Application Layout Best Practices

This guide describes the recommended project structure and conventions for applications
built on top of **restinpieces**. It follows a pattern that keeps application logic
consolidated while making the routing table explicit and easy to find.

---

## Directory Structure

```
myapp/
├── cmd/
│   └── myapp/
│       └── main.go       # entry point: flags, wiring, srv.Run()
├── routes.go             # explicit route registration (pure function)
├── app/                  # all application logic and state
│   ├── app.go            # App struct and constructor
│   ├── handler_users.go  # handlers as methods on *App
│   └── middleware.go     # custom middleware methods on *App
├── jobs/                 # job handler implementations
├── daemons/              # daemon constructors and configuration
└── web/
    ├── src/              # frontend source
    └── dist/             # built assets, embedded via go:embed
```

---

## The App Package

The application logic lives in the `app/` package. This avoids cluttering the root
and allows handlers to access private state safely.

### The App Wrapper (`app/app.go`)

Your application defines its own `App` struct that holds `*core.App` plus any additional
heavy state — extra database pools, third-party clients, etc.

```go
// app/app.go
package app

import (
    "github.com/caasmo/restinpieces/core"
    "github.com/caasmo/restinpieces/db"
)

type App struct {
    *core.App
    analyticsDB db.Pool
}

func NewApp(core *core.App, analyticsDB db.Pool) *App {
    return &App{
        App:         core,
        analyticsDB: analyticsDB,
    }
}
```

### Handlers (`app/users.go`)

Handlers are methods on `*App`. This gives them direct access to all application
dependencies without using global variables or complex interfaces.

```go
// app/users.go
package app

import "net/http"

func (a *App) GetUserHandler(w http.ResponseWriter, r *http.Request) {
    // a.Logger().Info("getting user")
    // a.analyticsDB.Query(...)
}
```

### Middleware (`app/middleware.go`)

Middleware that needs application state are also methods on `*App`.

```go
// app/middleware.go
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

The `routes.go` file lives in the project root. It is a pure function that defines
the application's routing map. This makes the "shape" of your API immediately
visible at the top level of the project.

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
        "/api/users": r.NewChain(http.HandlerFunc(a.GetUserHandler)).
                        WithMiddleware(a.TenantMiddleware),
    })
}
```

---

## Entry Point (`cmd/myapp/main.go`)

The entry point wires the `app` state to the `Routes` and starts the server.
The registration is explicit, so you can see exactly how the application is composed.

```go
// cmd/myapp/main.go
package main

import (
    "github.com/caasmo/restinpieces"
    "github.com/yourname/myapp"
    "github.com/yourname/myapp/app"
)

func main() {
    // ... setup dbPool ...

    coreApp, srv, err := restinpieces.New(...)

    // 1. Initialize application state
    a := app.NewApp(coreApp, dbPool)

    // 2. Explicitly wire routes
    myapp.Routes(a)

    // 3. Run
    srv.Run()
}
```

---

## Summary

| Concern     | Location            | Pattern                                   |
|-------------|---------------------|-------------------------------------------|
| Entry point | `cmd/myapp/main.go` | Wires `app` to `Routes` and runs `srv`    |
| App State   | `app/app.go`        | `App` struct wrapping `*core.App`         |
| Handlers    | `app/*.go`          | Methods on `*app.App`                     |
| Middleware  | `app/middleware.go` | Methods on `*app.App` or plain functions  |
| Routes Map  | `routes.go` (root)  | Pure function `Routes(a *app.App)`        |
| Jobs        | `jobs/`             | Structs using `*app.App`                  |
| Daemons     | `daemons/`          | Constructors for background processes     |
| Frontend    | `web/`              | Embedded assets, served via routes        |
