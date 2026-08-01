package main

import (
	"bytes"
	"errors"
	"fmt"
	"testing"

	"github.com/caasmo/restinpieces/db"
)

// MockJobRmDB is a test-only implementation of the db.DbQueueAdmin interface
// for testing the 'job rm' command.
type MockJobRmDB struct {
	// ListJobs is not used by the rm command but is part of the interface.
	ListJobsFunc func(limit int) ([]*db.Job, error)

	// DeleteJob is the method we are testing against.
	deleteJobCalledWith int64
	forceDeleteError    bool
}

// ListJobs is a mock implementation.
func (m *MockJobRmDB) ListJobs(limit int) ([]*db.Job, error) {
	if m.ListJobsFunc != nil {
		return m.ListJobsFunc(limit)
	}
	panic("ListJobs not implemented for rm test")
}

// DeleteJob is the mock implementation for deleting jobs.
func (m *MockJobRmDB) DeleteJob(id int64) error {
	m.deleteJobCalledWith = id
	if m.forceDeleteError {
		return errors.New("forced database error")
	}
	return nil
}

func TestRemoveJob_Success(t *testing.T) {
	// --- Setup ---
	mockDB := &MockJobRmDB{}
	var stdout bytes.Buffer
	jobID := int64(42)

	// --- Execute ---
	err := removeJob(&stdout, mockDB, jobID)

	// --- Assert ---
	if err != nil {
		t.Fatalf("removeJob() returned an unexpected error: %v", err)
	}

	if mockDB.deleteJobCalledWith != jobID {
		t.Errorf("expected DeleteJob to be called with ID %d, but got %d", jobID, mockDB.deleteJobCalledWith)
	}

	expectedOutput := fmt.Sprintf("Successfully deleted job %d\n", jobID)
	if stdout.String() != expectedOutput {
		t.Errorf("expected output %q, got %q", expectedOutput, stdout.String())
	}
}

func TestRemoveJob_FailureDBError(t *testing.T) {
	// --- Setup ---
	mockDB := &MockJobRmDB{forceDeleteError: true}
	var stdout bytes.Buffer
	jobID := int64(13)

	// --- Execute ---
	err := removeJob(&stdout, mockDB, jobID)

	// --- Assert ---
	if err == nil {
		t.Fatal("removeJob() was expected to return an error, but did not")
	}
	if !errors.Is(err, ErrDeleteJobFailed) {
		t.Errorf("expected error to wrap ErrDeleteJobFailed, got %v", err)
	}
	if mockDB.deleteJobCalledWith != jobID {
		t.Errorf("expected DeleteJob to be called with ID %d even on failure, but got %d", jobID, mockDB.deleteJobCalledWith)
	}
}

func TestRemoveJob_FailureWriteError(t *testing.T) {
	// --- Setup ---
	mockDB := &MockJobRmDB{}
	var failingStdout failingWriter
	jobID := int64(99)

	// --- Execute ---
	err := removeJob(&failingStdout, mockDB, jobID)

	// --- Assert ---
	if err == nil {
		t.Fatal("removeJob() was expected to return an error, but did not")
	}
	if !errors.Is(err, ErrWriteOutput) {
		t.Errorf("expected error to wrap ErrWriteOutput, got %v", err)
	}
}
