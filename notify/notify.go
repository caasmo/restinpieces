package notify

import (
	"context"
	"time"
)

type Type int

const (
	Alarm Type = iota
	Metric
)

func (nt Type) String() string {
	switch nt {
	case Alarm:
		return "Alarm"
	case Metric:
		return "Metric"
	default:
		return "Unknown"
	}
}

type Notification struct {
	Timestamp time.Time
	Type      Type
	Source    string
	Message   string
	Fields    map[string]interface{}
}

// Notifier defines the contract for sending alarms and metrics.
// Implementations of this interface are responsible for formatting and dispatching
// notifications to their respective backends.
// Implementations MUST be safe for concurrent use by multiple goroutines.
type Notifier interface {
	Send(ctx context.Context, n Notification) error
}

type NilNotifier struct{}

func NewNilNotifier() *NilNotifier {
	return &NilNotifier{}
}

func (nn *NilNotifier) Send(ctx context.Context, n Notification) error {
	return nil
}

// MultiNotifier sends notifications to multiple notifiers.
type MultiNotifier struct {
	notifiers []Notifier
}

// NewMultiNotifier creates a new MultiNotifier.
func NewMultiNotifier(notifiers ...Notifier) *MultiNotifier {
	return &MultiNotifier{notifiers: notifiers}
}

// Send sends the notification to all notifiers.
// It stops and returns the error if any of the notifiers fail.
func (mn *MultiNotifier) Send(ctx context.Context, n Notification) error {
	for _, notifier := range mn.notifiers {
		if err := notifier.Send(ctx, n); err != nil {
			return err
		}
	}
	return nil
}
