package main

import (
	"context"
	"fmt"
	"os"

	"zombiezen.com/go/sqlite/sqlitex"
)

func handleListCommand(pool *sqlitex.Pool, scopeFilter string) {
	conn, err := pool.Take(context.Background())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to get db connection for list command: %v\n", err)
		os.Exit(1)
	}
	defer pool.Put(conn)

	var query string
	if scopeFilter != "" {
		query = "SELECT id, scope, created_at, format, description FROM app_config WHERE scope = ? ORDER BY created_at DESC;"
	} else {
		query = "SELECT id, scope, created_at, format, description FROM app_config ORDER BY created_at DESC;"
	}

	stmt, err := conn.Prepare(query)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to prepare statement for list command: %v\n", err)
		os.Exit(1)
	}
	defer stmt.Finalize()

	if scopeFilter != "" {
		stmt.BindText(1, scopeFilter)
	}

	fmt.Println("Gen  Scope        Created At             Fmt  Description")
	fmt.Println("---  ------------ ---------------------  ---  -----------")

	var count int
	for {
		hasRow, err := stmt.Step()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to step through list results: %v\n", err)
			os.Exit(1)
		}
		if !hasRow {
			break
		}
		scope := stmt.GetText("scope")
		createdAt := stmt.GetText("created_at")
		format := stmt.GetText("format")
		description := stmt.GetText("description")

		// Truncate format to 3 chars if needed
		if len(format) > 3 {
			format = format[:3]
		}
		fmt.Printf("%-3d  %-12s  %-21s  %-3s  %s\n", count, scope, createdAt, format, description)
		count++
	}

	if count == 0 {
		if scopeFilter != "" {
			fmt.Printf("No configurations found for scope: %s\n", scopeFilter)
		} else {
			fmt.Println("No configurations found.")
		}
	}
}
