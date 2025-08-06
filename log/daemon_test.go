package log

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"math"
	"sync"
	"testing"
	"time"

	"github.com/caasmo/restinpieces/config"
	"github.com/caasmo/restinpieces/db"
)

// newTestLogger creates a silent logger for tests to avoid noisy output.
func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}



// mockDbLog is a mock implementation of the db.DbLog interface for testing the Daemon.
// It provides mechanisms to inspect calls, simulate errors, and synchronize tests.
type mockDbLog struct {
	mu              sync.Mutex
	insertedBatches [][]db.Log
	insertErr       error
	batchReceived   chan int // Signals the number of records in a received batch
	closeCalled     bool
}

// newMockDbLog creates a new mock database for testing.
func newMockDbLog() *mockDbLog {
	return &mockDbLog{
		// Use a buffered channel to prevent the daemon from blocking if the test isn't ready
		batchReceived: make(chan int, 10),
	}
}

// InsertBatch simulates writing a batch of logs. It records the batch for inspection,
// returns a pre-configured error, and signals that a batch was received.
func (m *mockDbLog) InsertBatch(batch []db.Log) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.insertErr != nil {
		// Signal receipt even on error to unblock tests
		m.batchReceived <- len(batch)
		return m.insertErr
	}

	// Create a copy to avoid data races if the original slice is modified
	batchCopy := make([]db.Log, len(batch))
	copy(batchCopy, batch)
	m.insertedBatches = append(m.insertedBatches, batchCopy)

	// Signal that a batch was received and how large it was
	m.batchReceived <- len(batch)
	return nil
}

// Close marks the mock as closed.
func (m *mockDbLog) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closeCalled = true
	return nil
}

// Ping is a no-op for the mock.
func (m *mockDbLog) Ping(tableName string) error {
	return nil
}

// --- Test Helper Methods ---

func (m *mockDbLog) getInsertedBatches() [][]db.Log {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.insertedBatches
}

func (m *mockDbLog) setInsertError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.insertErr = err
}

func (m *mockDbLog) wasCloseCalled() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.closeCalled
}

// waitForBatch is a crucial helper to synchronize tests with the daemon's goroutine.
// It waits for the mock to signal a batch was received, with a timeout.
func (m *mockDbLog) waitForBatch(t *testing.T, timeout time.Duration) int {
	t.Helper()
	select {
	case batchSize := <-m.batchReceived:
		return batchSize
	case <-time.After(timeout):
		t.Fatal("timed out waiting for log batch to be processed")
		return 0
	}
}



// TestDaemon_FlushOnBatchSize verifies that the daemon writes to the DB when the batch size is reached.
func TestDaemon_FlushOnBatchSize(t *testing.T) {
	// 1. Setup
	mockDB := newMockDbLog()
	cfg := config.NewDefaultConfig()
	cfg.Log.Batch.FlushSize = 3
	cfg.Log.Batch.FlushInterval.Duration = 1 * time.Minute // Long interval to prevent interference
	provider := config.NewProvider(cfg)

	daemon, err := New(provider, newTestLogger(), mockDB)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	// 2. Start daemon and ensure it's stopped
	if err := daemon.Start(); err != nil {
		t.Fatalf("daemon.Start() failed: %v", err)
	}
	defer func() {
		if err := daemon.Stop(context.Background()); err != nil {
			t.Logf("daemon.Stop() failed during cleanup: %v", err)
		}
	}()

	// 3. Action
	recordChan, _ := daemon.Chan()
	record := slog.NewRecord(time.Now(), slog.LevelInfo, "test", 0)

	recordChan <- record // Record 1
	recordChan <- record // Record 2

	// Assert that no batch has been written yet
	if len(mockDB.getInsertedBatches()) != 0 {
		t.Fatal("daemon flushed batch before reaching flush size")
	}

	recordChan <- record // Record 3 - This should trigger the flush

	// 4. Verify
	batchSize := mockDB.waitForBatch(t, 1*time.Second)
	if batchSize != 3 {
		t.Errorf("expected batch size 3, got %d", batchSize)
	}

	batches := mockDB.getInsertedBatches()
	if len(batches) != 1 {
		t.Fatalf("expected 1 batch to be written, got %d", len(batches))
	}
			if len(batches[0]) != 3 {
		t.Errorf("expected the batch to contain 3 records, got %d", len(batches[0]))
	}
}

// TestDaemon_FlushOnInterval verifies that a partial batch is written when the timer fires.
func TestDaemon_FlushOnInterval(t *testing.T) {
	// 1. Setup
	mockDB := newMockDbLog()
	cfg := config.NewDefaultConfig()
	cfg.Log.Batch.FlushSize = 10 // Large size to ensure it doesn't trigger the flush
	cfg.Log.Batch.FlushInterval.Duration = 20 * time.Millisecond // Short interval
	provider := config.NewProvider(cfg)

	daemon, err := New(provider, newTestLogger(), mockDB)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	// 2. Start & Defer Stop
	if err := daemon.Start(); err != nil {
		t.Fatalf("daemon.Start() failed: %v", err)
	}
	defer func() {
		if err := daemon.Stop(context.Background()); err != nil {
			t.Logf("daemon.Stop() failed during cleanup: %v", err)
		}
	}()

	// 3. Action
	recordChan, _ := daemon.Chan()
	record := slog.NewRecord(time.Now(), slog.LevelInfo, "test", 0)
	recordChan <- record // Record 1
	recordChan <- record // Record 2

	// Assert that no batch has been written immediately
	if len(mockDB.getInsertedBatches()) != 0 {
		t.Fatal("daemon flushed batch immediately without waiting for interval")
	}

	// 4. Verify
	batchSize := mockDB.waitForBatch(t, 100*time.Millisecond)
	if batchSize != 2 {
		t.Errorf("expected batch size 2, got %d", batchSize)
	}

	batches := mockDB.getInsertedBatches()
	if len(batches) != 1 {
		t.Fatalf("expected 1 batch to be written, got %d", len(batches))
	}
	if len(batches[0]) != 2 {
		t.Errorf("expected the batch to contain 2 records, got %d", len(batches[0]))
	}
}

// TestDaemon_ShutdownDrainsLogs ensures all pending logs are flushed on graceful shutdown.
func TestDaemon_ShutdownDrainsLogs(t *testing.T) {
	// 1. Setup
	mockDB := newMockDbLog()
	cfg := config.NewDefaultConfig()
	cfg.Log.Batch.FlushSize = 10 // High flush size to prevent premature flush
	provider := config.NewProvider(cfg)

	daemon, err := New(provider, newTestLogger(), mockDB)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	// 2. Start
	if err := daemon.Start(); err != nil {
		t.Fatalf("daemon.Start() failed: %v", err)
	}

	// 3. Action
	recordChan, _ := daemon.Chan()
	record := slog.NewRecord(time.Now(), slog.LevelInfo, "test", 0)
	for i := 0; i < 5; i++ {
		recordChan <- record
	}

	// Stop the daemon, which should trigger the final flush
	if err := daemon.Stop(context.Background()); err != nil {
		t.Fatalf("daemon.Stop() returned an error: %v", err)
	}

	// 4. Assert
	batches := mockDB.getInsertedBatches()
	if len(batches) != 1 {
		t.Fatalf("expected 1 batch to be written on shutdown, got %d", len(batches))
	}
	if len(batches[0]) != 5 {
		t.Errorf("expected batch to contain 5 records, got %d", len(batches[0]))
	}
	if !mockDB.wasCloseCalled() {
		t.Error("expected daemon to call Close() on the database connection")
	}
}

// TestDaemon_SurvivesDbError verifies the daemon continues running after a DB error.
func TestDaemon_SurvivesDbError(t *testing.T) {
	// 1. Setup
	mockDB := newMockDbLog()
	mockDB.setInsertError(errors.New("simulated db error"))

	var logOutput bytes.Buffer
	opLogger := slog.New(slog.NewTextHandler(&logOutput, nil))

	cfg := config.NewDefaultConfig()
	cfg.Log.Batch.FlushSize = 2
	provider := config.NewProvider(cfg)

	daemon, err := New(provider, opLogger, mockDB)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	// 2. Start & Defer Stop
	if err := daemon.Start(); err != nil {
		t.Fatalf("daemon.Start() failed: %v", err)
	}
	defer func() {
		if err := daemon.Stop(context.Background()); err != nil {
			t.Logf("daemon.Stop() failed during cleanup: %v", err)
		}
	}()

	// 3. Action & Verify First Batch (which will fail)
	recordChan, _ := daemon.Chan()
	record := slog.NewRecord(time.Now(), slog.LevelInfo, "test", 0)
	recordChan <- record
	recordChan <- record

	_ = mockDB.waitForBatch(t, 1*time.Second) // Wait for the failed batch

	// Assert that the error was logged by the daemon
	if !bytes.Contains(logOutput.Bytes(), []byte("simulated db error")) {
		t.Fatal("daemon did not log the database error")
	}

	// 4. Action & Verify Second Batch (should succeed)
	mockDB.setInsertError(nil) // Fix the database
	recordChan <- record
	recordChan <- record

	batchSize := mockDB.waitForBatch(t, 1*time.Second)
	if batchSize != 2 {
		t.Errorf("expected batch size 2 for the second batch, got %d", batchSize)
	}

	// Assert that the second batch was inserted successfully
	batches := mockDB.getInsertedBatches()
	if len(batches) != 1 {
		t.Fatalf("expected 1 successful batch, got %d", len(batches))
	}
	if len(batches[0]) != 2 {
		t.Errorf("expected the successful batch to contain 2 records, got %d", len(batches[0]))
	}
}

// TestDaemon_SkipsUnserializableRecord verifies that a record that cannot be marshaled
// is skipped without crashing the daemon.
func TestDaemon_SkipsUnserializableRecord(t *testing.T) {
	// 1. Setup
	mockDB := newMockDbLog()
	var logOutput bytes.Buffer
	opLogger := slog.New(slog.NewTextHandler(&logOutput, nil))

	cfg := config.NewDefaultConfig()
	cfg.Log.Batch.FlushSize = 2 // Flush after two records are processed
	provider := config.NewProvider(cfg)

	daemon, err := New(provider, opLogger, mockDB)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	// 2. Start & Defer Stop
	if err := daemon.Start(); err != nil {
		t.Fatalf("daemon.Start() failed: %v", err)
	}
	defer func() {
		if err := daemon.Stop(context.Background()); err != nil {
			t.Logf("daemon.Stop() failed during cleanup: %v", err)
		}
	}()

	// 3. Action
	recordChan, _ := daemon.Chan()
	// This record is unserializable because json.Marshal cannot handle NaN.
	badRecord := slog.NewRecord(time.Now(), slog.LevelInfo, "bad record", 0)
	badRecord.AddAttrs(slog.Float64("bad_attr", math.NaN()))

	goodRecord := slog.NewRecord(time.Now(), slog.LevelInfo, "good record", 0)

	recordChan <- badRecord
	recordChan <- goodRecord
	recordChan <- goodRecord // Send a second good record to trigger the flush

	// 4. Verify
	// The daemon will skip the bad record and batch the two good ones.
	batchSize := mockDB.waitForBatch(t, 200*time.Millisecond)
	if batchSize != 2 {
		t.Fatalf("expected batch size 2, got %d", batchSize)
	}

	// Check that an error was logged, without being brittle.
	if logOutput.Len() == 0 {
		t.Fatal("daemon did not log the serialization error")
	}

	batches := mockDB.getInsertedBatches()
	if len(batches) != 1 {
		t.Fatalf("expected 1 batch to be written, got %d", len(batches))
	}
	if len(batches[0]) != 2 {
		t.Fatalf("expected batch to contain 2 (the good) records, got %d", len(batches[0]))
	}
	if batches[0][0].Message != "good record" || batches[0][1].Message != "good record" {
		t.Errorf("batch did not contain the correct records, got: %s, %s",
			batches[0][0].Message, batches[0][1].Message)
	}
}
