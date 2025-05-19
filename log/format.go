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

// Fail formats a failure message
func (f *MessageFormatter) Fail(msg string) string {
	return fmt.Sprintf("%s  %s: ❌  %s", f.componentEmoji, f.component, msg)
}

// Ok formats a success/ok message
func (f *MessageFormatter) Ok(msg string) string {
	return fmt.Sprintf("%s  %s: 👍  %s", f.componentEmoji, f.component, msg)
}

// Warn formats a warning message
func (f *MessageFormatter) Warn(msg string) string {
	return fmt.Sprintf("%s  %s: ⚠️  %s", f.componentEmoji, f.component, msg)
}

// Start formats a startup/begin message with rocket emoji
func (f *MessageFormatter) Start(msg string) string {
	return fmt.Sprintf("%s  %s: 🚀  %s", f.componentEmoji, f.component, msg)
}

// Complete formats a completion message with party popper emoji
func (f *MessageFormatter) Complete(msg string) string {
	return fmt.Sprintf("%s  %s: 🎉  %s", f.componentEmoji, f.component, msg)
}
