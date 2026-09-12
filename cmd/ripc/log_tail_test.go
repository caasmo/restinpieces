package main

import (
	"bytes"
	"database/sql"
	"errors"
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
		{Level: 0, Message: "first", JsonData: "{}", Created: rdb.TimeFormat(time.Now())},
		{Level: 8, Message: "second", JsonData: "{}", Created: rdb.TimeFormat(time.Now())},
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

	records, err := ldb.tail(0)
	if err != nil {
		t.Fatalf("tail failed: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}
	if records[0].Message != "first" || records[1].Message != "second" {
		t.Errorf("unexpected record order: %+v", records)
	}

	empty, err := ldb.tail(maxID)
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

		err := logTail(ui, mockStore)
		if !errors.Is(err, ErrSecureStoreGet) {
			t.Fatalf("expected error to wrap ErrSecureStoreGet, got %v", err)
		}
	})

	t.Run("MissingPath", func(t *testing.T) {
		mockStore := &MockLogInitSecureStore{data: []byte("")}
		var stdout, stderr bytes.Buffer
		ui := UI{Out: &stdout, Err: &stderr}

		err := logTail(ui, mockStore)
		if err == nil {
			t.Fatal("expected error when log.batch.db_path is empty")
		}
	})

	t.Run("BadDbPath", func(t *testing.T) {
		mockStore := &MockLogInitSecureStore{data: []byte("[log.batch]\ndb_path = \"/dev/null/logs.db\"\n")}
		var stdout, stderr bytes.Buffer
		ui := UI{Out: &stdout, Err: &stderr}

		err := logTail(ui, mockStore)
		if !errors.Is(err, ErrDbConnection) {
			t.Fatalf("expected error to wrap ErrDbConnection, got %v", err)
		}
	})

	t.Run("HandleWrapperGetError", func(t *testing.T) {
		mockStore := &MockLogInitSecureStore{forceGetError: true}
		var stdout, stderr bytes.Buffer
		ui := UI{Out: &stdout, Err: &stderr}

		err := handleLogTailCommand(mockStore, ui)
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

	_, err = ldb.tail(0)
	if !errors.Is(err, ErrQueryPrepare) {
		t.Fatalf("expected tail error to wrap ErrQueryPrepare, got %v", err)
	}
}
