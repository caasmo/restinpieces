package main

import (
	"errors"
	"testing"
)

// TestRunHelpTopic_Success verifies that valid topics dispatch to the correct function.
func TestRunHelpTopic_Success(t *testing.T) {
	var calledTopic string

	// Replace the real print functions with test fakes.
	originalPrintJob := printJobUsageFunc
	originalPrintConfig := printConfigUsageFunc
	originalPrintAuth := printAuthUsageFunc
	defer func() {
		printJobUsageFunc = originalPrintJob
		printConfigUsageFunc = originalPrintConfig
		printAuthUsageFunc = originalPrintAuth
	}()

	printJobUsageFunc = func() { calledTopic = "job" }
	printConfigUsageFunc = func() { calledTopic = "config" }
	printAuthUsageFunc = func() { calledTopic = "auth" }

	testCases := []struct {
		topic       string
		expectTopic string
	}{
		{topic: "job", expectTopic: "job"},
		{topic: "config", expectTopic: "config"},
		{topic: "auth", expectTopic: "auth"},
	}

	for _, tc := range testCases {
		t.Run(tc.topic, func(t *testing.T) {
			calledTopic = "" // Reset before each run.

			err := runHelpTopic(tc.topic)

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
	err := runHelpTopic(topic)

	if !errors.Is(err, ErrUnknownHelpTopic) {
		t.Errorf("runHelpTopic() error = %v, want error wrapping %v", err, ErrUnknownHelpTopic)
	}
}