package main

import (
	"bytes"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/caasmo/restinpieces/db"
	"github.com/caasmo/restinpieces/queue/handlers"
)

// MockJobAddBackupDB is a test-only implementation of the db.DbQueue interface.
type MockJobAddBackupDB struct {
	insertedJob      db.Job
	insertCalled     bool
	forceInsertError bool
}

// InsertJob is the mock implementation for inserting jobs.
func (m *MockJobAddBackupDB) InsertJob(job db.Job) error {
	m.insertCalled = true
	m.insertedJob = job
	if m.forceInsertError {
		return errors.New("forced database error")
	}
	return nil
}

// Claim is a mock implementation for claiming jobs.
func (m *MockJobAddBackupDB) Claim(limit int) ([]*db.Job, error) {
	return nil, nil // Not used in these tests
}

// MarkCompleted is a mock implementation for marking jobs as completed.
func (m *MockJobAddBackupDB) MarkCompleted(jobID int64) error {
	return nil // Not used in these tests
}

// MarkFailed is a mock implementation for marking jobs as failed.
func (m *MockJobAddBackupDB) MarkFailed(jobID int64, err string) error {
	return nil // Not used in these tests
}

// Delete is a mock implementation for deleting jobs.
func (m *MockJobAddBackupDB) Delete(jobID int64) error {
	return nil // Not used in these tests
}

// MarkRecurrentCompleted is a mock implementation for marking recurrent jobs as completed.
func (m *MockJobAddBackupDB) MarkRecurrentCompleted(jobID int64, nextJob db.Job) error {
	return nil // Not used in these tests
}

func TestAddBackupJob_Success(t *testing.T) {
	// --- Setup ---
	mockDB := &MockJobAddBackupDB{}
	var stdout bytes.Buffer
	interval := 24 * time.Hour
	scheduledFor := time.Date(2025, 10, 21, 10, 0, 0, 0, time.UTC)
	maxAttempts := 5

	// --- Execute ---
	err := addBackupJob(&stdout, mockDB, interval, scheduledFor, maxAttempts)

	// --- Assert ---
	if err != nil {
		t.Fatalf("addBackupJob() returned an unexpected error: %v", err)
	}

	if !mockDB.insertCalled {
		t.Fatal("expected InsertJob to be called, but it wasn't")
	}

	// Assert the job passed to the mock
	job := mockDB.insertedJob
	if job.JobType != handlers.JobTypeBackupLocal {
		t.Errorf("expected JobType %q, got %q", handlers.JobTypeBackupLocal, job.JobType)
	}
	if job.Interval != interval {
		t.Errorf("expected Interval %v, got %v", interval, job.Interval)
	}
	if !job.ScheduledFor.Equal(scheduledFor) {
		t.Errorf("expected ScheduledFor %v, got %v", scheduledFor, job.ScheduledFor)
	}
	if job.MaxAttempts != maxAttempts {
		t.Errorf("expected MaxAttempts %d, got %d", maxAttempts, job.MaxAttempts)
	}
	if !job.Recurrent {
		t.Error("expected Recurrent to be true")
	}

	// Assert the output
	output := stdout.String()
	expectedOutput := fmt.Sprintf("Successfully inserted recurrent backup job of type '%s'.\n", handlers.JobTypeBackupLocal)
	expectedOutput += fmt.Sprintf("  - Interval: %s\n", interval)
	expectedOutput += fmt.Sprintf("  - First run scheduled for: %s\n", scheduledFor.Format(time.RFC3339))

	if output != expectedOutput {
		t.Errorf("expected output:\n%q\ngot:\n%q", expectedOutput, output)
	}
}

func TestAddBackupJob_FailureDBError(t *testing.T) {
	// --- Setup ---
	mockDB := &MockJobAddBackupDB{forceInsertError: true}
	var stdout bytes.Buffer

	// --- Execute ---
	err := addBackupJob(&stdout, mockDB, time.Hour, time.Now(), 3)

	// --- Assert ---
	if err == nil {
		t.Fatal("addBackupJob() was expected to return an error, but did not")
	}
	if !errors.Is(err, ErrInsertJobFailed) {
		t.Errorf("expected error to wrap ErrInsertJobFailed, got %v", err)
	}
	if !mockDB.insertCalled {
		t.Error("expected InsertJob to be called even on failure")
	}
}

func TestAddBackupJob_FailureWriteError(t *testing.T) {
	// --- Setup ---
	mockDB := &MockJobAddBackupDB{}
	var failingStdout failingWriter

	// --- Execute ---
	err := addBackupJob(&failingStdout, mockDB, time.Hour, time.Now(), 3)

	// --- Assert ---
	if err == nil {
		t.Fatal("addBackupJob() was expected to return an error, but did not")
	}
	if !errors.Is(err, ErrWriteOutput) {
		t.Errorf("expected error to wrap ErrWriteOutput, got %v", err)
	}
}
