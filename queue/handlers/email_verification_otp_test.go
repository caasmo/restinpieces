package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/caasmo/restinpieces/db"
)

func TestEmailVerificationOtpHandler_Handle(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		var mailerCalled bool
		var capturedEmail string
		var capturedOtp string

		mockMailer := &mailerMock{
			SendOtpEmailFunc: func(ctx context.Context, email, otp string) error {
				mailerCalled = true
				capturedEmail = email
				capturedOtp = otp
				return nil
			},
		}

		handler := NewEmailVerificationOtpHandler(mockMailer)

		payload := PayloadEmailVerificationOtp{
			Email:          "user@example.com",
			CooldownBucket: 12345,
		}
		payloadBytes, _ := json.Marshal(payload)
		payloadExtra := PayloadEmailVerificationOtpExtra{
			Otp: "123456",
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
			t.Fatal("SendOtpEmail should have been called")
		}

		if capturedEmail != "user@example.com" {
			t.Errorf("expected email user@example.com, got %s", capturedEmail)
		}

		if capturedOtp != "123456" {
			t.Errorf("expected OTP 123456, got %s", capturedOtp)
		}
	})

	t.Run("invalid payload", func(t *testing.T) {
		mockMailer := &mailerMock{}
		handler := NewEmailVerificationOtpHandler(mockMailer)

		job := db.Job{Payload: []byte("invalid json")}

		err := handler.Handle(context.Background(), job)
		if err == nil {
			t.Error("Handle() expected error for invalid payload, got nil")
		}
	})

	t.Run("invalid payload extra", func(t *testing.T) {
		mockMailer := &mailerMock{}
		handler := NewEmailVerificationOtpHandler(mockMailer)

		payload := PayloadEmailVerificationOtp{Email: "user@example.com"}
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

	t.Run("mailer error", func(t *testing.T) {
		mockMailer := &mailerMock{
			SendOtpEmailFunc: func(ctx context.Context, email, otp string) error {
				return errors.New("smtp error")
			},
		}

		handler := NewEmailVerificationOtpHandler(mockMailer)

		payload := PayloadEmailVerificationOtp{Email: "user@example.com"}
		payloadBytes, _ := json.Marshal(payload)
		payloadExtra := PayloadEmailVerificationOtpExtra{Otp: "123456"}
		payloadExtraBytes, _ := json.Marshal(payloadExtra)
		job := db.Job{
			Payload:      payloadBytes,
			PayloadExtra: payloadExtraBytes,
		}

		err := handler.Handle(context.Background(), job)
		if err == nil {
			t.Error("Handle() expected error for mailer failure, got nil")
		}
	})
}
