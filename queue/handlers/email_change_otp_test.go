package handlers

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/caasmo/restinpieces/db"
)

func TestEmailChangeOtpHandler_Handle(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		var mailerCalled bool
		var capturedNewEmail string
		var capturedOtp string

		mockMailer := &mailerMock{
			SendEmailChangeOtpEmailFunc: func(ctx context.Context, newEmail, otp string) error {
				mailerCalled = true
				capturedNewEmail = newEmail
				capturedOtp = otp
				return nil
			},
		}

		handler := NewEmailChangeOtpHandler(mockMailer)

		payload := PayloadEmailChangeOtp{
			NewEmail:       "new@example.com",
			CooldownBucket: 123456,
		}
		payloadBytes, _ := json.Marshal(payload)
		payloadExtra := PayloadEmailChangeOtpExtra{
			Otp: "654321",
		}
		payloadExtraBytes, _ := json.Marshal(payloadExtra)
		job := db.Job{
			Payload:      payloadBytes,
			PayloadExtra: payloadExtraBytes,
		}

		err := handler.Handle(context.Background(), job)

		if err != nil {
			t.Fatalf("Handle() error = %v, want nil", err)
		}

		if !mailerCalled {
			t.Fatal("SendEmailChangeOtpEmail should have been called")
		}

		if capturedNewEmail != "new@example.com" {
			t.Errorf("expected email new@example.com, got %s", capturedNewEmail)
		}

		if capturedOtp != "654321" {
			t.Errorf("expected OTP 654321, got %s", capturedOtp)
		}
	})

	t.Run("invalid payload", func(t *testing.T) {
		mockMailer := &mailerMock{}
		handler := NewEmailChangeOtpHandler(mockMailer)

		job := db.Job{Payload: []byte("invalid json")}

		err := handler.Handle(context.Background(), job)
		if err == nil {
			t.Error("Handle() expected error for invalid payload, got nil")
		}
	})

	t.Run("invalid payload extra", func(t *testing.T) {
		mockMailer := &mailerMock{}
		handler := NewEmailChangeOtpHandler(mockMailer)

		payload := PayloadEmailChangeOtp{NewEmail: "new@example.com"}
		payloadBytes, _ := json.Marshal(payload)
		job := db.Job{
			Payload:      payloadBytes,
			PayloadExtra: []byte("invalid json"),
		}

		err := handler.Handle(context.Background(), job)
		if err == nil {
			t.Error("Handle() expected error for invalid payload extra, got nil")
		}
	})
}
