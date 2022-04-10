package main

import (
    "database/sql"
)

// App is the application wide context.
// db connections and the like goes here
//
// Handlers that require the context should implement the receive pattern
type App struct {
    db *sql.DB
}


