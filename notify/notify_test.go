package notify

import (
	"context"
	"errors"
	"testing"
)

func TestNotificationTypeString(t *testing.T) {
	testCases := []struct {
		name             string
		notificationType Type
		expected         string
	}{
		{
			name:             "Alarm type",
			notificationType: Alarm,
			expected:         "Alarm",
		},
		{
			name:             "Metric type",
			notificationType: Metric,
			expected:         "Metric",
		},
		{
			name:             "Unknown type",
			notificationType: Type(99),
			expected:         "Unknown",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if str := tc.notificationType.String(); str != tc.expected {
				t.Errorf("Expected type string %q, got %q", tc.expected, str)
			}
		})
	}
}

func TestNilNotifier(t *testing.T) {
	t.Run("NewNilNotifier", func(t *testing.T) {
		if notifier := NewNilNotifier(); notifier == nil {
			t.Error("NewNilNotifier() returned nil")
		}
	})

	t.Run("Send", func(t *testing.T) {
		notifier := NewNilNotifier()
		err := notifier.Send(context.Background(), Notification{})
		if err != nil {
			t.Errorf("NilNotifier.Send() error = %v, want nil", err)
		}
	})
}

type mockNotifier struct {
	sendCalled bool
	sendError  error
}

func (m *mockNotifier) Send(ctx context.Context, n Notification) error {
	m.sendCalled = true
	return m.sendError
}

func TestMultiNotifier(t *testing.T) {
	t.Run("NewMultiNotifier", func(t *testing.T) {
		notifier1 := NewNilNotifier()
		notifier2 := NewNilNotifier()
		multiNotifier := NewMultiNotifier(notifier1, notifier2)

		if multiNotifier == nil {
			t.Fatal("NewMultiNotifier() returned nil")
		}
		if len(multiNotifier.notifiers) != 2 {
			t.Errorf("Expected 2 notifiers, got %d", len(multiNotifier.notifiers))
		}
	})

	t.Run("SendWithNoNotifiers", func(t *testing.T) {
		multiNotifier := NewMultiNotifier()
		err := multiNotifier.Send(context.Background(), Notification{})
		if err != nil {
			t.Errorf("Send() with no notifiers error = %v, want nil", err)
		}
	})

	t.Run("SendWithMultipleNotifiers", func(t *testing.T) {
		mock1 := &mockNotifier{}
		mock2 := &mockNotifier{}
		multiNotifier := NewMultiNotifier(mock1, mock2)

		err := multiNotifier.Send(context.Background(), Notification{})
		if err != nil {
			t.Errorf("Send() error = %v, want nil", err)
		}

		if !mock1.sendCalled {
			t.Error("Expected first notifier's Send to be called")
		}
		if !mock2.sendCalled {
			t.Error("Expected second notifier's Send to be called")
		}
	})

	t.Run("SendWithNotifierError", func(t *testing.T) {
		mock1 := &mockNotifier{}
		mock2 := &mockNotifier{sendError: errors.New("send error")}
		mock3 := &mockNotifier{}
		multiNotifier := NewMultiNotifier(mock1, mock2, mock3)

		err := multiNotifier.Send(context.Background(), Notification{})
		if err == nil {
			t.Fatal("Expected an error, but got nil")
		}
		if err.Error() != "send error" {
			t.Errorf("Expected error message 'send error', got %q", err.Error())
		}

		if !mock1.sendCalled {
			t.Error("Expected first notifier's Send to be called")
		}
		if !mock2.sendCalled {
			t.Error("Expected second notifier's Send to be called")
		}
		if mock3.sendCalled {
			t.Error("Expected third notifier's Send not to be called after error")
		}
	})
}
