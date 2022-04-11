package main

import (
    "github.com/caasmo/restinpieces/db"
)

// App is the application wide context.
// db connections and the like goes here
//
// Handlers that require the context should implement the receive pattern
type App struct {
    dbase *db.Db
}


