package log

import "fmt"

// MessageFormatter provides consistent log message formatting
type MessageFormatter struct {
	component      string
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

func (f *MessageFormatter) Fail(msg string) string {
	return fmt.Sprintf("%s %s: ❌%s", f.componentEmoji, f.component, msg)
}

func (f *MessageFormatter) Ok(msg string) string {
	return fmt.Sprintf("%s %s: 👍 %s", f.componentEmoji, f.component, msg)
}

func (f *MessageFormatter) Warn(msg string) string {
	return fmt.Sprintf("%s %s: ⚠️  %s", f.componentEmoji, f.component, msg)
}

func (f *MessageFormatter) Start(msg string) string {
	return fmt.Sprintf("%s %s: 🚀 %s", f.componentEmoji, f.component, msg)
}

func (f *MessageFormatter) Complete(msg string) string {
	return fmt.Sprintf("%s %s: 🎉 %s", f.componentEmoji, f.component, msg)
}

func (f *MessageFormatter) Component(msg string) string {
	return fmt.Sprintf("%s %s: 📦 %s", f.componentEmoji, f.component, msg)
}

func (f *MessageFormatter) Active(msg string) string {
	return fmt.Sprintf("%s %s: ⚡ %s", f.componentEmoji, f.component, msg)
}

func (f *MessageFormatter) Inactive(msg string) string {
	return fmt.Sprintf("%s %s: 😴 %s", f.componentEmoji, f.component, msg)
}
