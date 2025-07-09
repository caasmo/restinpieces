package main

import (
	"context"
	"fmt"
	"os"

	"zombiezen.com/go/sqlite/sqlitex"
)

func handleScopesCommand(pool *sqlitex.Pool) {
	// No logging for retrieving scopes

	conn, err := pool.Take(context.Background())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to get db connection for scopes command: %v\n", err)
		os.Exit(1)
	}
	defer pool.Put(conn)

	stmt, err := conn.Prepare("SELECT DISTINCT scope FROM app_config ORDER BY scope;")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to prepare statement for scopes command: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		if err := stmt.Finalize(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to finalize statement: %v\n", err)
		}
	}()

	// fmt.Println("Unique scopes found in app_config:") // Optional: keep or remove this header
	var count int
	for {
		hasRow, err := stmt.Step()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to step through scopes results: %v\n", err)
			os.Exit(1)
		}
		if !hasRow {
			break
		}
		scope := stmt.GetText("scope")
		fmt.Println(scope)
		count++
	}

	// No summary logging for count
}
