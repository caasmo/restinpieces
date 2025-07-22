package executor

import (
	"context"
	"errors"
	"testing"

	"github.com/caasmo/restinpieces/db"
)

// mockJobHandler is a mock implementation of the JobHandler interface for testing.
// It allows us to control the outcome of the Handle method and track its calls.
type mockJobHandler struct {
	handleFunc func(ctx context.Context, job db.Job) error
}

// Handle executes the mock's handleFunc.
func (m *mockJobHandler) Handle(ctx context.Context, job db.Job) error {
	if m.handleFunc != nil {
		return m.handleFunc(ctx, job)
	}
	return nil
}

func TestNewExecutor(t *testing.T) {
	t.Run("with initial handlers", func(t *testing.T) {
		handlers := map[string]JobHandler{
			"test_job": &mockJobHandler{},
		}
		executor := NewExecutor(handlers)
		if executor == nil {
			t.Fatal("NewExecutor returned nil")
		}
		if len(executor.registry) != 1 {
			t.Errorf("expected 1 handler to be registered, got %d", len(executor.registry))
		}
	})

	t.Run("with nil handlers", func(t *testing.T) {
		executor := NewExecutor(nil)
		if executor == nil {
			t.Fatal("NewExecutor returned nil")
		}
		if len(executor.registry) != 0 {
			t.Errorf("expected 0 handlers for nil input, got %d", len(executor.registry))
		}
	})
}

func TestDefaultExecutor_Register(t *testing.T) {
	executor := NewExecutor(nil)
	handler1 := &mockJobHandler{}
	handler2 := &mockJobHandler{}

	// Register a new handler
	executor.Register("job1", handler1)
	if executor.registry["job1"] != handler1 {
		t.Error("handler1 was not registered correctly")
	}

	// Overwrite an existing handler
	executor.Register("job1", handler2)
	if executor.registry["job1"] != handler2 {
		t.Error("handler1 was not overwritten by handler2")
	}
}

func TestDefaultExecutor_Execute(t *testing.T) {
	ctx := context.Background()
	failErr := errors.New("handler failed")

	var handledJob db.Job
	successHandler := &mockJobHandler{
		handleFunc: func(ctx context.Context, job db.Job) error {
			handledJob = job
			return nil
		},
	}

	failHandler := &mockJobHandler{
		handleFunc: func(ctx context.Context, job db.Job) error {
			return failErr
		},
	}

	executor := NewExecutor(map[string]JobHandler{
		"success_job": successHandler,
		"fail_job":    failHandler,
	})

	testCases := []struct {
		name      string
		job       db.Job
		wantErr   error
		checkFunc func(t *testing.T)
	}{
		{
			name:    "successful execution",
			job:     db.Job{ID: 1, JobType: "success_job"},
			wantErr: nil,
			checkFunc: func(t *testing.T) {
				if handledJob.ID != 1 {
					t.Errorf("handler did not receive the correct job, got ID %d, want 1", handledJob.ID)
				}
			},
		},
		{
			name:    "handler not found",
			job:     db.Job{JobType: "unknown_job"},
			wantErr: errors.New("no handler registered for job type: unknown_job"),
		},
		{
			name:    "handler returns error",
			job:     db.Job{JobType: "fail_job"},
			wantErr: failErr,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := executor.Execute(ctx, tc.job)

			if tc.wantErr != nil {
				if err == nil {
					t.Fatalf("Execute() did not return an error, want %v", tc.wantErr)
				}
				if err.Error() != tc.wantErr.Error() {
					t.Errorf("Execute() error = %v, want %v", err, tc.wantErr)
				}
			} else if err != nil {
				t.Errorf("Execute() returned unexpected error: %v", err)
			}

			if tc.checkFunc != nil {
				tc.checkFunc(t)
			}
		})
	}
}
