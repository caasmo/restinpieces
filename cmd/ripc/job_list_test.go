package main

import (
	"bytes"
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/caasmo/restinpieces/db"
)

// MockJobListDB is a test-only implementation of the db.DbQueueAdmin interface.
type MockJobListDB struct {
	JobsToReturn []*db.Job
	ForceDBError bool
}

// ListJobs is the mock implementation for listing jobs.
func (m *MockJobListDB) ListJobs(limit int) ([]*db.Job, error) {
	if m.ForceDBError {
		return nil, errors.New("forced database error")
	}
	if limit > 0 && len(m.JobsToReturn) > limit {
		return m.JobsToReturn[:limit], nil
	}
	return m.JobsToReturn, nil
}

// DeleteJob is not used by the list command but is part of the interface.
func (m *MockJobListDB) DeleteJob(id int64) error {
	panic("not implemented")
}

func TestListJobs_SuccessWithJobs(t *testing.T) {
	// --- Setup ---
	mockDB := &MockJobListDB{
		JobsToReturn: []*db.Job{
			{
				ID:           1,
				JobType:      "backup",
				Status:       "pending",
				ScheduledFor: time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC),
				Recurrent:    true,
				Interval:     24 * time.Hour,
				Attempts:     0,
				MaxAttempts:  3,
				Payload:      []byte("short payload"),
				PayloadExtra: []byte("this is a very long payload extra that should definitely be truncated"),
				LastError:    "",
			},
			{
				ID:           2,
				JobType:      "email",
				Status:       "failed",
				ScheduledFor: time.Date(2023, 1, 2, 12, 0, 0, 0, time.UTC),
				Recurrent:    false,
				Attempts:     3,
				MaxAttempts:  3,
				Payload:      []byte("{}"),
				PayloadExtra: []byte("{}"),
				LastError:    "SMTP connection failed: timeout after 30s, this is a very long error message that will be truncated for sure",
			},
		},
	}
	var stdout bytes.Buffer

	// --- Execute ---
	err := listJobs(&stdout, mockDB, 0)

	// --- Assert ---
	if err != nil {
		t.Fatalf("listJobs() returned an unexpected error: %v", err)
	}

	output := stdout.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")

	if len(lines) < 4 {
		t.Fatalf("expected at least 4 lines of output (header, separator, 2 jobs), but got %d", len(lines))
	}

	// Helper to check if a line contains all required substrings, ignoring spacing.
	assertLineContains := func(t *testing.T, line string, expected ...string) {
		t.Helper()
		// Normalize whitespace to make the check robust against alignment changes.
		normalizedLine := regexp.MustCompile(`\s+`).ReplaceAllString(line, " ")
		for _, exp := range expected {
			if !strings.Contains(normalizedLine, exp) {
				t.Errorf("line\n'%s'\ndoes not contain expected substring\n'%s'", normalizedLine, exp)
			}
		}
	}

	// Check header
	assertLineContains(t, lines[0], "ID", "TYPE", "STATUS", "SCHEDULED FOR")

	// Check job 1 data and truncation
	assertLineContains(t, lines[2], "1", "backup", "pending", "this is a very lo...")

	// Check job 2 data and truncation
	assertLineContains(t, lines[3], "2", "email", "failed", "N/A", "SMTP connection failed: timeout after 30s, this...")
}

func TestListJobs_SuccessNoJobs(t *testing.T) {
	// --- Setup ---
	mockDB := &MockJobListDB{
		JobsToReturn: []*db.Job{}, // No jobs
	}
	var stdout bytes.Buffer

	// --- Execute ---
	err := listJobs(&stdout, mockDB, 0)

	// --- Assert ---
	if err != nil {
		t.Fatalf("listJobs() returned an unexpected error: %v", err)
	}

	expectedOutput := "No jobs found in the queue.\n"
	if stdout.String() != expectedOutput {
		t.Errorf("expected output %q, got %q", expectedOutput, stdout.String())
	}
}

func TestListJobs_FailureDBError(t *testing.T) {
	// --- Setup ---
	mockDB := &MockJobListDB{
		ForceDBError: true,
	}
	var stdout bytes.Buffer

	// --- Execute ---
	err := listJobs(&stdout, mockDB, 0)

	// --- Assert ---
	if err == nil {
		t.Fatal("listJobs() was expected to return an error, but did not")
	}
	if !errors.Is(err, ErrListJobsFailed) {
		t.Errorf("expected error to wrap ErrListJobsFailed, got %v", err)
	}
}

func TestListJobs_FailureWriteError(t *testing.T) {
	// --- Setup ---
	mockDB := &MockJobListDB{
		JobsToReturn: []*db.Job{{ID: 1}}, // Need at least one job to trigger a write
	}
	var failingStdout failingWriter

	// --- Execute ---
	err := listJobs(&failingStdout, mockDB, 0)

	// --- Assert ---
	if err == nil {
		t.Fatal("listJobs() was expected to return an error, but did not")
	}
	if !errors.Is(err, ErrWriteOutput) {
		t.Errorf("expected error to wrap ErrWriteOutput, got %v", err)
	}
}