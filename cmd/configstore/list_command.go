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
	var args []interface{}

	if scopeFilter != "" {
		query = "SELECT id, scope, created_at, format, description FROM app_config WHERE scope = ? ORDER BY created_at DESC;"
		args = append(args, scopeFilter)
	} else {
		query = "SELECT id, scope, created_at, format, description FROM app_config ORDER BY created_at DESC;"
	}

	stmt, err := conn.Prepare(query)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to prepare statement for list command: %v\n", err)
		os.Exit(1)
	}
	defer stmt.Finalize()

	// Bind arguments if any
	for i, arg := range args {
		// Assuming all args are text for simplicity with scope filter
		if err := stmt.BindText(i+1, arg.(string)); err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to bind argument for list command: %v\n", err)
			os.Exit(1)
		}
	}

	fmt.Println("ID | Scope | Created At | Format | Description")
	fmt.Println("---|-------|--------------|--------|-------------")

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
		id := stmt.GetInt64("id")
		scope := stmt.GetText("scope")
		createdAt := stmt.GetText("created_at")
		format := stmt.GetText("format")
		description := stmt.GetText("description")

		fmt.Printf("%d | %s | %s | %s | %s\n", id, scope, createdAt, format, description)
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
