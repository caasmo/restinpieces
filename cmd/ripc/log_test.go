package main

import (
	"bytes"
	"database/sql"
	"errors"
	"flag"
	"path/filepath"
	"testing"
	"time"

	rdb "github.com/caasmo/restinpieces/db"
	dbm "github.com/caasmo/restinpieces/db/databasesql"
)

func TestReadLogDbPath(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		mockStore := &MockLogInitSecureStore{data: []byte("[log.batch]\ndb_path = \"data/logs.db\"\n")}

		path, err := readLogDbPath(mockStore)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if path != "data/logs.db" {
			t.Errorf("expected data/logs.db, got %q", path)
		}
	})

	t.Run("MissingPath", func(t *testing.T) {
		mockStore := &MockLogInitSecureStore{data: []byte("")}

		_, err := readLogDbPath(mockStore)
		if err == nil {
			t.Fatal("expected error when log.batch.db_path is empty")
		}
	})

	t.Run("GetError", func(t *testing.T) {
		mockStore := &MockLogInitSecureStore{forceGetError: true}

		_, err := readLogDbPath(mockStore)
		if !errors.Is(err, ErrSecureStoreGet) {
			t.Fatalf("expected error to wrap ErrSecureStoreGet, got %v", err)
		}
	})

	t.Run("InvalidToml", func(t *testing.T) {
		mockStore := &MockLogInitSecureStore{data: []byte("[log.batch\ninvalid")}

		_, err := readLogDbPath(mockStore)
		if !errors.Is(err, ErrConfigUnmarshal) {
			t.Fatalf("expected error to wrap ErrConfigUnmarshal, got %v", err)
		}
	})
}

func TestLogDbMaxLogIDAndLogsAfter(t *testing.T) {
	db, err := sql.Open("sqlite", "file::memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory database: %v", err)
	}
	// One connection: an in-memory database lives per connection.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	defer func() {
		if err := db.Close(); err != nil {
			t.Errorf("failed to close database: %v", err)
		}
	}()

	ldb := &logDb{db: db}
	if err := ldb.createSchemas(); err != nil {
		t.Fatalf("createSchemas failed: %v", err)
	}

	logWriter, err := dbm.NewLog(db, 2)
	if err != nil {
		t.Fatalf("failed to create log writer: %v", err)
	}
	batch := []rdb.Log{
		{Level: 0, Message: "first", JsonData: `{"log_label":"llm:request"}`, Created: rdb.TimeFormat(time.Now())},
		{Level: 8, Message: "second", JsonData: `{"log_label":"llj:cleanup"}`, Created: rdb.TimeFormat(time.Now())},
	}
	if err := logWriter.InsertBatch(batch); err != nil {
		t.Fatalf("InsertBatch failed: %v", err)
	}

	maxID, err := ldb.maxID()
	if err != nil {
		t.Fatalf("maxID failed: %v", err)
	}
	if maxID != 2 {
		t.Errorf("expected max id 2, got %d", maxID)
	}

	records, err := ldb.tail(0, "")
	if err != nil {
		t.Fatalf("tail failed: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}
	if records[0].Message != "first" || records[1].Message != "second" {
		t.Errorf("unexpected record order: %+v", records)
	}

	filtered, err := ldb.tail(0, "llj:")
	if err != nil {
		t.Fatalf("tail with filter failed: %v", err)
	}
	if len(filtered) != 1 || filtered[0].Message != "second" {
		t.Errorf("expected only 'second' for filter 'llj:', got %+v", filtered)
	}

	empty, err := ldb.tail(maxID, "")
	if err != nil {
		t.Fatalf("tail(max) failed: %v", err)
	}
	if len(empty) != 0 {
		t.Errorf("expected no records after max id, got %d", len(empty))
	}
}

func TestLogTailEarlyErrors(t *testing.T) {
	t.Run("GetError", func(t *testing.T) {
		mockStore := &MockLogInitSecureStore{forceGetError: true}
		var stdout, stderr bytes.Buffer
		ui := UI{Out: &stdout, Err: &stderr}

		err := logTail(ui, mockStore, LogOptions{})
		if !errors.Is(err, ErrSecureStoreGet) {
			t.Fatalf("expected error to wrap ErrSecureStoreGet, got %v", err)
		}
	})

	t.Run("MissingPath", func(t *testing.T) {
		mockStore := &MockLogInitSecureStore{data: []byte("")}
		var stdout, stderr bytes.Buffer
		ui := UI{Out: &stdout, Err: &stderr}

		err := logTail(ui, mockStore, LogOptions{})
		if err == nil {
			t.Fatal("expected error when log.batch.db_path is empty")
		}
	})

	t.Run("BadDbPath", func(t *testing.T) {
		mockStore := &MockLogInitSecureStore{data: []byte("[log.batch]\ndb_path = \"/dev/null/logs.db\"\n")}
		var stdout, stderr bytes.Buffer
		ui := UI{Out: &stdout, Err: &stderr}

		err := logTail(ui, mockStore, LogOptions{})
		if !errors.Is(err, ErrDbConnection) {
			t.Fatalf("expected error to wrap ErrDbConnection, got %v", err)
		}
	})

	t.Run("HandleWrapperGetError", func(t *testing.T) {
		mockStore := &MockLogInitSecureStore{forceGetError: true}
		var stdout, stderr bytes.Buffer
		ui := UI{Out: &stdout, Err: &stderr}

		err := handleLogCommand(mockStore, nil, ui)
		if !errors.Is(err, ErrSecureStoreGet) {
			t.Fatalf("expected error to wrap ErrSecureStoreGet, got %v", err)
		}
	})
}

func TestLogDbCursorQueryErrors(t *testing.T) {
	db, err := sql.Open("sqlite", "file::memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory database: %v", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	defer func() {
		if err := db.Close(); err != nil {
			t.Errorf("failed to close database: %v", err)
		}
	}()

	ldb := &logDb{db: db}

	_, err = ldb.maxID()
	if !errors.Is(err, ErrQueryPrepare) {
		t.Fatalf("expected maxID error to wrap ErrQueryPrepare, got %v", err)
	}

	_, err = ldb.tail(0, "")
	if !errors.Is(err, ErrQueryPrepare) {
		t.Fatalf("expected tail error to wrap ErrQueryPrepare, got %v", err)
	}

	_, err = ldb.tailBackwards(0, logChunkSize, "")
	if !errors.Is(err, ErrQueryPrepare) {
		t.Fatalf("expected tailBackwards error to wrap ErrQueryPrepare, got %v", err)
	}
}

func TestLogDbTailBackwards(t *testing.T) {
	db, err := sql.Open("sqlite", "file::memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory database: %v", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	defer func() {
		if err := db.Close(); err != nil {
			t.Errorf("failed to close database: %v", err)
		}
	}()

	ldb := &logDb{db: db}
	if err := ldb.createSchemas(); err != nil {
		t.Fatalf("createSchemas failed: %v", err)
	}

	logWriter, err := dbm.NewLog(db, 5)
	if err != nil {
		t.Fatalf("failed to create log writer: %v", err)
	}
	batch := []rdb.Log{
		{Level: 0, Message: "first", JsonData: `{"log_label":"llm:request"}`, Created: rdb.TimeFormat(time.Now())},
		{Level: 0, Message: "second", JsonData: `{"log_label":"llj:cleanup"}`, Created: rdb.TimeFormat(time.Now())},
		{Level: 0, Message: "third", JsonData: `{"log_label":"llm:request"}`, Created: rdb.TimeFormat(time.Now())},
		{Level: 0, Message: "fourth", JsonData: `{"log_label":"llj:cleanup"}`, Created: rdb.TimeFormat(time.Now())},
		{Level: 0, Message: "fifth", JsonData: `{"log_label":"llm:request"}`, Created: rdb.TimeFormat(time.Now())},
	}
	if err := logWriter.InsertBatch(batch); err != nil {
		t.Fatalf("InsertBatch failed: %v", err)
	}

	records, err := ldb.tailBackwards(6, 3, "")
	if err != nil {
		t.Fatalf("tailBackwards failed: %v", err)
	}
	if len(records) != 3 {
		t.Fatalf("expected 3 records, got %d", len(records))
	}
	if records[0].ID != 5 || records[1].ID != 4 || records[2].ID != 3 {
		t.Errorf("expected ids 5, 4, 3, got %d, %d, %d", records[0].ID, records[1].ID, records[2].ID)
	}

	records, err = ldb.tailBackwards(6, 3, "llj:")
	if err != nil {
		t.Fatalf("tailBackwards with filter failed: %v", err)
	}
	if len(records) != 1 || records[0].Message != "fourth" {
		t.Errorf("expected only 'fourth' for filter 'llj:', got %+v", records)
	}

	records, err = ldb.tailBackwards(3, 100, "")
	if err != nil {
		t.Fatalf("tailBackwards below the first record failed: %v", err)
	}
	if len(records) != 2 || records[0].ID != 2 || records[1].ID != 1 {
		t.Errorf("expected ids 2, 1, got %+v", records)
	}
}

func TestParseLogArgs(t *testing.T) {
	testCases := []struct {
		name     string
		args     []string
		expected LogOptions
	}{
		{
			name:     "Defaults",
			args:     nil,
			expected: LogOptions{Number: logDefaultNumber},
		},
		{
			name:     "FlagsBeforeFilter",
			args:     []string{"-f", "-n", "5", "llj:"},
			expected: LogOptions{Follow: true, Number: 5, Filter: "llj:"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			opts, err := parseLogArgs(tc.args)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if opts != tc.expected {
				t.Errorf("expected options %+v, but got %+v", tc.expected, opts)
			}
		})
	}
}

func TestParseLogArgsErrors(t *testing.T) {
	testCases := []struct {
		name        string
		args        []string
		expectedErr error
	}{
		{
			name:        "FlagsAfterFilter",
			args:        []string{"llj:", "-f"},
			expectedErr: ErrTooManyArguments,
		},
		{
			name:        "NegativeNumber",
			args:        []string{"-n=-1"},
			expectedErr: ErrInvalidFlag,
		},
		{
			name:        "InvalidFlag",
			args:        []string{"-x"},
			expectedErr: ErrInvalidFlag,
		},
		{
			name:        "Help",
			args:        []string{"-h"},
			expectedErr: flag.ErrHelp,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseLogArgs(tc.args)
			if !errors.Is(err, tc.expectedErr) {
				t.Fatalf("expected error to wrap %v, but got %v", tc.expectedErr, err)
			}
		})
	}
}

func TestLogTailPrintsOldestFirst(t *testing.T) {
	logDbPath := filepath.Join(t.TempDir(), "logs.db")

	db, err := sql.Open("sqlite", logDbPath)
	if err != nil {
		t.Fatalf("failed to open log database: %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			t.Errorf("failed to close database: %v", err)
		}
	}()

	ldb := &logDb{db: db}
	if err := ldb.createSchemas(); err != nil {
		t.Fatalf("createSchemas failed: %v", err)
	}

	logWriter, err := dbm.NewLog(db, 3)
	if err != nil {
		t.Fatalf("failed to create log writer: %v", err)
	}
	batch := []rdb.Log{
		{Level: 0, Message: "first", JsonData: "{}", Created: "2025-01-01T00:00:01Z"},
		{Level: 0, Message: "second", JsonData: "{}", Created: "2025-01-01T00:00:02Z"},
		{Level: 0, Message: "third", JsonData: "{}", Created: "2025-01-01T00:00:03Z"},
	}
	if err := logWriter.InsertBatch(batch); err != nil {
		t.Fatalf("InsertBatch failed: %v", err)
	}

	mockStore := &MockLogInitSecureStore{data: []byte("[log.batch]\ndb_path = \"" + logDbPath + "\"\n")}
	var stdout, stderr bytes.Buffer
	ui := UI{Out: &stdout, Err: &stderr}

	err = logTail(ui, mockStore, LogOptions{Number: 2})
	if err != nil {
		t.Fatalf("logTail failed: %v", err)
	}

	expected := "2025-01-01T00:00:02Z INFO second {}\n2025-01-01T00:00:03Z INFO third {}\n"
	if stdout.String() != expected {
		t.Errorf("expected output %q, got %q", expected, stdout.String())
	}
}
