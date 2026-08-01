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
	originalPrintJob := printJobUsageFunc
	originalPrintConfig := printConfigUsageFunc
	originalPrintLog := printLogUsageFunc
	defer func() {
		printAppUsageFunc = originalPrintApp
		printJobUsageFunc = originalPrintJob
		printConfigUsageFunc = originalPrintConfig
		printLogUsageFunc = originalPrintLog
	}()

	printAppUsageFunc = func(w io.Writer) { calledTopic = "app" }
	printJobUsageFunc = func(w io.Writer) { calledTopic = "job" }
	printConfigUsageFunc = func(w io.Writer) { calledTopic = "config" }
	printLogUsageFunc = func(w io.Writer) { calledTopic = "log" }

	testCases := []struct {
		topic       string
		expectTopic string
	}{
		{topic: "app", expectTopic: "app"},
		{topic: "job", expectTopic: "job"},
		{topic: "config", expectTopic: "config"},
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
