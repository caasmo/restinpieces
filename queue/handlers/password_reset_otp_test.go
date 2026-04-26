package handlers

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/caasmo/restinpieces/db"
)

func TestPasswordResetOtpHandler_Handle(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		var mailerCalled bool
		var capturedEmail string
		var capturedOtp string

		mockMailer := &mailerMock{
			SendPasswordResetOtpEmailFunc: func(ctx context.Context, email, otp string) error {
				mailerCalled = true
				capturedEmail = email
				capturedOtp = otp
				return nil
			},
		}

		handler := NewPasswordResetOtpHandler(mockMailer)

		payload := PayloadPasswordResetOtp{Email: "test@example.com"}
		payloadBytes, _ := json.Marshal(payload)
		payloadExtra := PayloadPasswordResetOtpExtra{Otp: "123456"}
		payloadExtraBytes, _ := json.Marshal(payloadExtra)
		job := db.Job{Payload: payloadBytes, PayloadExtra: payloadExtraBytes}

		err := handler.Handle(context.Background(), job)

		if err != nil {
			t.Fatalf("Handle() error = %v, want nil", err)
		}

		if !mailerCalled {
			t.Fatal("SendPasswordResetOtpEmail should have been called, but it was not")
		}

		if capturedEmail != "test@example.com" {
			t.Errorf("expected email test@example.com, got %s", capturedEmail)
		}

		if capturedOtp != "123456" {
			t.Errorf("expected OTP 123456, got %s", capturedOtp)
		}
	})

	t.Run("invalid payload", func(t *testing.T) {
		mockMailer := &mailerMock{}
		handler := NewPasswordResetOtpHandler(mockMailer)

		job := db.Job{Payload: []byte("invalid json")}

		err := handler.Handle(context.Background(), job)
		if err == nil {
			t.Error("Handle() expected error for invalid payload, got nil")
		}
	})

	t.Run("invalid payload extra", func(t *testing.T) {
		mockMailer := &mailerMock{}
		handler := NewPasswordResetOtpHandler(mockMailer)

		payload := PayloadPasswordResetOtp{Email: "test@example.com"}
		payloadBytes, _ := json.Marshal(payload)
		job := db.Job{Payload: payloadBytes, PayloadExtra: []byte("invalid json")}

		err := handler.Handle(context.Background(), job)
		if err == nil {
			t.Error("Handle() expected error for invalid payload extra, got nil")
		}
	})
}
