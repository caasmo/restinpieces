package log

import (
	"strings"
	"testing"
)

// TestMessageFormatter verifies that the component name and message are correctly
// included in the output of all formatter methods. This test is intentionally
// kept simple and robust, avoiding checks for decorative elements like emojis,
// which could make the test brittle.
func TestMessageFormatter(t *testing.T) {
	testCases := []struct {
		name          string
		componentName string
		message       string
		// method allows us to test each formatting function (e.g., Fail, Ok)
		// within the same table-driven test structure.
		method func(*MessageFormatter, string) string
	}{
		{
			name:          "Fail method includes component and message",
			componentName: "API",
			message:       "request failed",
			method:        (*MessageFormatter).Fail,
		},
		{
			name:          "Ok method includes component and message",
			componentName: "DB",
			message:       "query successful",
			method:        (*MessageFormatter).Ok,
		},
		{
			name:          "Warn method includes component and message",
			componentName: "Cache",
			message:       "entry expired",
			method:        (*MessageFormatter).Warn,
		},
		{
			name:          "Start method includes component and message",
			componentName: "Job",
			message:       "processing job #123",
			method:        (*MessageFormatter).Start,
		},
		{
			name:          "Complete method includes component and message",
			componentName: "Backup",
			message:       "backup finished",
			method:        (*MessageFormatter).Complete,
		},
		{
			name:          "Component method includes component and message",
			componentName: "Auth",
			message:       "user authenticated",
			method:        (*MessageFormatter).Component,
		},
		{
			name:          "Active method includes component and message",
			componentName: "Server",
			message:       "listening on port 8080",
			method:        (*MessageFormatter).Active,
		},
		{
			name:          "Inactive method includes component and message",
			componentName: "Worker",
			message:       "no jobs available",
			method:        (*MessageFormatter).Inactive,
		},
		{
			name:          "Seed method includes component and message",
			componentName: "DB",
			message:       "seeding initial data",
			method:        (*MessageFormatter).Seed,
		},
		{
			name:          "Disabled method includes component and message",
			componentName: "Feature",
			message:       "feature flag is off",
			method:        (*MessageFormatter).Disabled,
		},
		{
			name:          "Handles empty component name",
			componentName: "",
			message:       "a message without a component",
			method:        (*MessageFormatter).Ok,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// The emoji is irrelevant to this test, so we pass an empty string.
			formatter := NewMessageFormatter().WithComponent(tc.componentName, "")

			got := tc.method(formatter, tc.message)

			// 1. Verify the component name is present (if not empty).
			if tc.componentName != "" && !strings.Contains(got, tc.componentName) {
				t.Errorf("expected output to contain component name %q, but it did not.\n     got: %q", tc.componentName, got)
			}

			// 2. Verify the message is present.
			if !strings.Contains(got, tc.message) {
				t.Errorf("expected output to contain message %q, but it did not.\n     got: %q", tc.message, got)
			}
		})
	}
}
