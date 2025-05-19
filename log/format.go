package log

import "fmt"

// MessageFormatter provides consistent log message formatting
type MessageFormatter struct {
	component     string
	componentEmoji string
}

// NewMessageFormatter creates a new formatter instance
func NewMessageFormatter() *MessageFormatter {
	return &MessageFormatter{}
}

// WithComponent sets the component name and emoji
func (f *MessageFormatter) WithComponent(name, emoji string) *MessageFormatter {
	f.component = name
	f.componentEmoji = emoji
	return f
}

// Error formats an error message
func (f *MessageFormatter) Error(msg string) string {
	return fmt.Sprintf("%s  %s: ❌  %s", f.componentEmoji, f.component, msg)
}

// Info formats an info message
func (f *MessageFormatter) Info(msg string) string {
	return fmt.Sprintf("%s  %s: ✅  %s", f.componentEmoji, f.component, msg)
}

// Warn formats a warning message
func (f *MessageFormatter) Warn(msg string) string {
	return fmt.Sprintf("%s  %s: ⚠️  %s", f.componentEmoji, f.component, msg)
}
