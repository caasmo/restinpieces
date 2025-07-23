package log

import (
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/caasmo/restinpieces/config"
)

// newTestConfigProvider creates a config.Provider with a specific log level for testing.
func newTestConfigProvider(level slog.Level) *config.Provider {
	return config.NewProvider(&config.Config{
		Log: config.Log{
			Batch: config.BatchLogger{
				Level: config.LogLevel{Level: level},
			},
		},
	})
}

// TestNewBatchHandler tests the constructor for BatchHandler, including its panic conditions.
func TestNewBatchHandler(t *testing.T) {
	provider := newTestConfigProvider(slog.LevelInfo)
	recordChan := make(chan slog.Record, 1)
	ctx := context.Background()

	testCases := []struct {
		name          string
		provider      *config.Provider
		recordChan    chan<- slog.Record
		ctx           context.Context
		shouldPanic   bool
		panicContains string
	}{
		{"Valid arguments", provider, recordChan, ctx, false, ""},
		{"Nil config provider", nil, recordChan, ctx, true, "configProvider cannot be nil"},
		{"Nil record channel", provider, nil, ctx, true, "recordChan cannot be nil"},
		{"Nil daemon context", provider, recordChan, nil, true, "daemonCtx cannot be nil"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				r := recover()
				if tc.shouldPanic {
					if r == nil {
						t.Errorf("expected a panic, but did not get one")
					}
					if msg, ok := r.(string); !ok || !strings.Contains(msg, tc.panicContains) {
						t.Errorf("expected panic message to contain %q, but got %q", tc.panicContains, r)
					}
				} else if r != nil {
					t.Errorf("expected no panic, but got one: %v", r)
				}
			}()
			_ = NewBatchHandler(tc.provider, tc.recordChan, tc.ctx)
		})
	}
}

// TestBatchHandler_Enabled verifies the handler correctly enables/disables logging
// based on the dynamic log level from the config provider.
func TestBatchHandler_Enabled(t *testing.T) {
	provider := newTestConfigProvider(slog.LevelInfo)
	handler := NewBatchHandler(provider, make(chan slog.Record, 1), context.Background())

	testCases := []struct {
		name          string
		levelToCheck  slog.Level
		expectEnabled bool
	}{
		{"Level below threshold", slog.LevelDebug, false},
		{"Level at threshold", slog.LevelInfo, true},
		{"Level above threshold", slog.LevelWarn, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := handler.Enabled(context.Background(), tc.levelToCheck); got != tc.expectEnabled {
				t.Errorf("Enabled() = %v, want %v", got, tc.expectEnabled)
			}
		})
	}
}

// TestBatchHandler_Handle tests the core logic of the Handle method, including
// successful sends, full channel errors, and shutdown behavior.
func TestBatchHandler_Handle(t *testing.T) {
	provider := newTestConfigProvider(slog.LevelInfo)
	record := slog.NewRecord(time.Now(), slog.LevelInfo, "test message", 0)

	t.Run("Successful send", func(t *testing.T) {
		recordChan := make(chan slog.Record, 1)
		handler := NewBatchHandler(provider, recordChan, context.Background())

		if err := handler.Handle(context.Background(), record); err != nil {
			t.Fatalf("Handle() returned an unexpected error: %v", err)
		}

		select {
		case rec := <-recordChan:
			if rec.Message != "test message" {
				t.Errorf("received wrong message: got %q, want %q", rec.Message, "test message")
			}
		default:
			t.Fatal("handler did not send the record to the channel")
		}
	})

	t.Run("Channel full", func(t *testing.T) {
		recordChan := make(chan slog.Record) // Unbuffered channel is always full
		handler := NewBatchHandler(provider, recordChan, context.Background())

		err := handler.Handle(context.Background(), record)
		if err == nil {
			t.Fatal("Handle() did not return an error for a full channel")
		}
		if !strings.Contains(err.Error(), "log channel full") {
			t.Errorf("unexpected error message: got %q", err.Error())
		}
	})

	t.Run("Daemon shutting down", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Simulate shutdown

		// Use a full channel to ensure shutdown check takes priority.
		recordChan := make(chan slog.Record)
		handler := NewBatchHandler(provider, recordChan, ctx)

		err := handler.Handle(context.Background(), record)
		if err == nil {
			t.Fatal("Handle() did not return an error during shutdown")
		}
		if !strings.Contains(err.Error(), "daemon shutting down") {
			t.Errorf("unexpected error message: got %q", err.Error())
		}
	})
}

// TestBatchHandler_WithAttrs verifies that attributes are correctly added to a new handler
// and subsequently included in handled records.
func TestBatchHandler_WithAttrs(t *testing.T) {
	provider := newTestConfigProvider(slog.LevelInfo)
	recordChan := make(chan slog.Record, 1)
	baseHandler := NewBatchHandler(provider, recordChan, context.Background())

	// Create a new handler with attributes
	attrHandler := baseHandler.WithAttrs([]slog.Attr{slog.String("key1", "val1")})
	finalHandler := attrHandler.WithAttrs([]slog.Attr{slog.String("key2", "val2")})

	record := slog.NewRecord(time.Now(), slog.LevelInfo, "message with attrs", 0)
	if err := finalHandler.Handle(context.Background(), record); err != nil {
		t.Fatalf("Handle() returned an unexpected error: %v", err)
	}

	// Verify the record on the channel has the attributes
	rec := <-recordChan
	foundKey1, foundKey2 := false, false
	rec.Attrs(func(a slog.Attr) bool {
		switch a.Key {
		case "key1":
			foundKey1 = a.Value.String() == "val1"
		case "key2":
			foundKey2 = a.Value.String() == "val2"
		}
		return true
	})

	if !foundKey1 || !foundKey2 {
		t.Error("expected record to have attributes for key1 and key2, but it did not")
	}

	// Verify the original handler was not modified
	if len(baseHandler.attrs) != 0 {
		t.Error("WithAttrs modified the original handler's attributes")
	}
}

// TestBatchHandler_WithGroup confirms that WithGroup returns a new handler instance.
func TestBatchHandler_WithGroup(t *testing.T) {
	var baseHandler slog.Handler = NewBatchHandler(newTestConfigProvider(slog.LevelInfo), make(chan slog.Record, 1), context.Background())
	groupHandler := baseHandler.WithGroup("my-group")

	if groupHandler == baseHandler {
		t.Fatal("WithGroup should return a new handler instance, but returned the same one")
	}
	if _, ok := groupHandler.(*BatchHandler); !ok {
		t.Fatalf("WithGroup did not return a *BatchHandler, got %T", groupHandler)
	}
}
