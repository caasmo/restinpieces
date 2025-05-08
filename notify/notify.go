package notify 

import (
	"context"
	"log/slog"
	"time"
)

type Type int

const (
	AlarmNotification Type = iota
	MetricNotification
)

func (nt Type) String() string {
	switch nt {
	case AlarmNotification:
		return "Alarm"
	case MetricNotification:
		return "Metric"
	default:
		return "Unknown"
	}
}

type Notification struct {
	Timestamp time.Time
	Type      Type
	Level     slog.Level
	Source    string
	Message   string
	Name      string
	Value     float64
	Unit      string
	Tags      map[string]string
}

// Notifier defines the contract for sending alarms and metrics.
// Implementations of this interface are responsible for formatting and dispatching
// notifications to their respective backends.
// Implementations MUST be safe for concurrent use by multiple goroutines.
type Notifier interface {
	Send(ctx context.Context, n Notification) error
}
