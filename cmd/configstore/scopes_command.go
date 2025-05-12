package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"zombiezen.com/go/sqlite/sqlitex"
)

func handleScopesCommand(logger *slog.Logger, pool *sqlitex.Pool) {
	logger.Info("retrieving unique scopes from app_config table")

	conn, err := pool.Take(context.Background())
	if err != nil {
		logger.Error("failed to get db connection for scopes command", "error", err)
		os.Exit(1)
	}
	defer pool.Put(conn)

	stmt, err := conn.Prepare("SELECT DISTINCT scope FROM app_config ORDER BY scope;")
	if err != nil {
		logger.Error("failed to prepare statement for scopes command", "error", err)
		os.Exit(1)
	}
	defer stmt.Finalize()

	fmt.Println("Unique scopes found in app_config:")
	var count int
	for {
		hasRow, err := stmt.Step()
		if err != nil {
			logger.Error("failed to step through scopes results", "error", err)
			os.Exit(1)
		}
		if !hasRow {
			break
		}
		scope := stmt.GetText("scope")
		fmt.Println(scope)
		count++
	}

	if count == 0 {
		logger.Info("no scopes found in app_config table")
	} else {
		logger.Info("successfully retrieved scopes", "count", count)
	}
}
