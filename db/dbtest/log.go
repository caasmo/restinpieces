package dbtest

import (
	"testing"
	"time"

	"github.com/caasmo/restinpieces/db"
)

// LogSuite provides the interface-conforming tests for the DbLog interface.
// The DbLog interface can only write logs; it cannot read them back, so the
// tests assert the contract's error behaviour (nil vs non-nil) rather than
// verifying the stored rows.
type LogSuite struct {
	// New creates a fresh, schema-initialized log database for every test.
	// A factory is used instead of a single Db field because Close is
	// destructive: sharing one connection across the Close tests would leave
	// later subtests operating on an already-closed connection.
	New func(t *testing.T) db.DbLog
}

func (s LogSuite) TestLog_InsertBatch(t *testing.T) {
	logDB := s.New(t)

	t.Run("Successful Batch Insert", func(t *testing.T) {
		batch := []db.Log{
			{Level: 1, Message: "message 1", JsonData: `{"key":"value1"}`, Created: db.TimeFormat(time.Now())},
			{Level: 2, Message: "message 2", JsonData: `{"key":"value2"}`, Created: db.TimeFormat(time.Now())},
		}
		if err := logDB.InsertBatch(batch); err != nil {
			t.Fatalf("InsertBatch() failed: %v", err)
		}
	})

	t.Run("Empty Batch", func(t *testing.T) {
		if err := logDB.InsertBatch([]db.Log{}); err != nil {
			t.Fatalf("InsertBatch() with empty slice failed: %v", err)
		}
	})

	t.Run("Single Entry Batch", func(t *testing.T) {
		batch := []db.Log{
			{Level: 1, Message: "single message", JsonData: "{}", Created: db.TimeFormat(time.Now())},
		}
		if err := logDB.InsertBatch(batch); err != nil {
			t.Fatalf("InsertBatch() with single entry failed: %v", err)
		}
	})
}

func (s LogSuite) TestLog_Ping(t *testing.T) {
	logDB := s.New(t)

	t.Run("ValidTable", func(t *testing.T) {
		if err := logDB.Ping("logs"); err != nil {
			t.Fatalf("Ping() on an existing table returned an error: %v", err)
		}
	})

	t.Run("NonExistentTable", func(t *testing.T) {
		if err := logDB.Ping("non_existent_table"); err == nil {
			t.Error("Ping() on a non-existent table should have returned an error")
		}
	})
}

func (s LogSuite) TestLog_Close(t *testing.T) {
	t.Run("Successful Close", func(t *testing.T) {
		logDB := s.New(t)
		if err := logDB.Close(); err != nil {
			t.Errorf("Close() returned an unexpected error: %v", err)
		}
	})

	t.Run("Double Close", func(t *testing.T) {
		logDB := s.New(t)
		if err := logDB.Close(); err != nil {
			t.Fatalf("first Close() failed unexpectedly: %v", err)
		}
		if err := logDB.Close(); err == nil {
			t.Error("second Close() should have returned an error")
		}
	})

	t.Run("Operations After Close", func(t *testing.T) {
		logDB := s.New(t)
		if err := logDB.Close(); err != nil {
			t.Fatalf("failed to close log db for test setup: %v", err)
		}
		if err := logDB.Ping("logs"); err == nil {
			t.Error("Ping() after Close() should have returned an error")
		}
		if err := logDB.InsertBatch([]db.Log{{}}); err == nil {
			t.Error("InsertBatch() after Close() should have returned an error")
		}
	})
}

func (s LogSuite) RunAll(t *testing.T) {
	s.TestLog_InsertBatch(t)
	s.TestLog_Ping(t)
	s.TestLog_Close(t)
}
