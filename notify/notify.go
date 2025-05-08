package notify 

import (
	"context"
	// "log/slog" // No longer needed here
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
