package main

import (
	"errors"
	"io"
	"testing"
)

// TestRunHelpTopic_Success verifies that valid topics dispatch to the correct function.
func TestRunHelpTopic_Success(t *testing.T) {
	var calledTopic string

	// Replace the real print functions with test fakes.
	originalPrintApp := printAppUsageFunc
	originalPrintGet := printGetUsageFunc
	originalPrintPaths := printPathsUsageFunc
	originalPrintDump := printDumpUsageFunc
	originalPrintScopes := printScopesUsageFunc
	originalPrintSet := printSetUsageFunc
	originalPrintSave := printSaveUsageFunc
	originalPrintScaffold := printScaffoldUsageFunc
	originalPrintMigrate := printMigrateUsageFunc
	originalPrintList := printListUsageFunc
	originalPrintDiff := printDiffUsageFunc
	originalPrintRollback := printRollbackUsageFunc
	originalPrintJob := printJobUsageFunc
	originalPrintLog := printLogUsageFunc
	defer func() {
		printAppUsageFunc = originalPrintApp
		printGetUsageFunc = originalPrintGet
		printPathsUsageFunc = originalPrintPaths
		printDumpUsageFunc = originalPrintDump
		printScopesUsageFunc = originalPrintScopes
		printSetUsageFunc = originalPrintSet
		printSaveUsageFunc = originalPrintSave
		printScaffoldUsageFunc = originalPrintScaffold
		printMigrateUsageFunc = originalPrintMigrate
		printListUsageFunc = originalPrintList
		printDiffUsageFunc = originalPrintDiff
		printRollbackUsageFunc = originalPrintRollback
		printJobUsageFunc = originalPrintJob
		printLogUsageFunc = originalPrintLog
	}()

	printAppUsageFunc = func(w io.Writer) { calledTopic = "app" }
	printGetUsageFunc = func(w io.Writer) { calledTopic = "get" }
	printPathsUsageFunc = func(w io.Writer) { calledTopic = "paths" }
	printDumpUsageFunc = func(w io.Writer) { calledTopic = "dump" }
	printScopesUsageFunc = func(w io.Writer) { calledTopic = "scopes" }
	printSetUsageFunc = func(w io.Writer) { calledTopic = "set" }
	printSaveUsageFunc = func(w io.Writer) { calledTopic = "save" }
	printScaffoldUsageFunc = func(w io.Writer) { calledTopic = "scaffold" }
	printMigrateUsageFunc = func(w io.Writer) { calledTopic = "migrate" }
	printListUsageFunc = func(w io.Writer) { calledTopic = "list" }
	printDiffUsageFunc = func(w io.Writer) { calledTopic = "diff" }
	printRollbackUsageFunc = func(w io.Writer) { calledTopic = "rollback" }
	printJobUsageFunc = func(w io.Writer) { calledTopic = "job" }
	printLogUsageFunc = func(w io.Writer) { calledTopic = "log" }

	testCases := []struct {
		topic       string
		expectTopic string
	}{
		{topic: "app", expectTopic: "app"},
		{topic: "get", expectTopic: "get"},
		{topic: "paths", expectTopic: "paths"},
		{topic: "dump", expectTopic: "dump"},
		{topic: "scopes", expectTopic: "scopes"},
		{topic: "set", expectTopic: "set"},
		{topic: "save", expectTopic: "save"},
		{topic: "scaffold", expectTopic: "scaffold"},
		{topic: "migrate", expectTopic: "migrate"},
		{topic: "list", expectTopic: "list"},
		{topic: "diff", expectTopic: "diff"},
		{topic: "rollback", expectTopic: "rollback"},
		{topic: "job", expectTopic: "job"},
		{topic: "log", expectTopic: "log"},
	}

	for _, tc := range testCases {
		t.Run(tc.topic, func(t *testing.T) {
			calledTopic = "" // Reset before each run.

			err := runHelpTopic(tc.topic, UI{Out: io.Discard, Err: io.Discard})

			if err != nil {
				t.Errorf("runHelpTopic(%q) returned unexpected error: %v", tc.topic, err)
			}
			if calledTopic != tc.expectTopic {
				t.Errorf("runHelpTopic(%q) called %q, want %q", tc.topic, calledTopic, tc.expectTopic)
			}
		})
	}
}

// TestRunHelpTopic_Failure_UnknownTopic tests that an invalid topic returns the correct error.
func TestRunHelpTopic_Failure_UnknownTopic(t *testing.T) {
	topic := "nonexistent"
	err := runHelpTopic(topic, UI{Out: io.Discard, Err: io.Discard})

	if !errors.Is(err, ErrUnknownHelpTopic) {
		t.Errorf("runHelpTopic() error = %v, want error wrapping %v", err, ErrUnknownHelpTopic)
	}
}
