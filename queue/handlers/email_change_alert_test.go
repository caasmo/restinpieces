package handlers

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/caasmo/restinpieces/db"
)

func TestEmailChangeAlertHandler_Handle(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		var mailerCalled bool
		var capturedOldEmail string
		var capturedNewEmail string

		mockMailer := &mailerMock{
			SendEmailChangeAlertFunc: func(ctx context.Context, oldEmail, newEmail string) error {
				mailerCalled = true
				capturedOldEmail = oldEmail
				capturedNewEmail = newEmail
				return nil
			},
		}

		handler := NewEmailChangeAlertHandler(mockMailer)

		payload := PayloadEmailChangeAlert{
			OldEmail: "old@example.com",
			NewEmail: "new@example.com",
		}
		payloadBytes, _ := json.Marshal(payload)
		job := db.Job{
			Payload: payloadBytes,
		}

		err := handler.Handle(context.Background(), job)

		if err != nil {
			t.Fatalf("Handle() error = %v, want nil", err)
		}

		if !mailerCalled {
			t.Fatal("SendEmailChangeAlert should have been called")
		}

		if capturedOldEmail != "old@example.com" {
			t.Errorf("expected old email old@example.com, got %s", capturedOldEmail)
		}

		if capturedNewEmail != "new@example.com" {
			t.Errorf("expected new email new@example.com, got %s", capturedNewEmail)
		}
	})

	t.Run("invalid payload", func(t *testing.T) {
		mockMailer := &mailerMock{}
		handler := NewEmailChangeAlertHandler(mockMailer)

		job := db.Job{Payload: []byte("invalid json")}

		err := handler.Handle(context.Background(), job)
		if err == nil {
			t.Error("Handle() expected error for invalid payload, got nil")
		}
	})
}
