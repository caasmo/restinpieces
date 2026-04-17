package handlers

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/caasmo/restinpieces/db"
	"github.com/caasmo/restinpieces/mail"
)

const JobTypeEmailVerificationOtp = "job_type_email_verification_otp"

// PayloadEmailVerificationOtp contains the details for deduplicating queued email tasks
type PayloadEmailVerificationOtp struct {
	Email string `json:"email"`
	// CooldownBucket prevents rapid duplicate requests
	CooldownBucket int `json:"cooldown_bucket"`
}

// PayloadEmailVerificationOtpExtra contains the unique OTP code data to act upon
type PayloadEmailVerificationOtpExtra struct {
	Otp string `json:"otp"`
}

// EmailVerificationOtpHandler handles sending OTP emails
type EmailVerificationOtpHandler struct {
	mailer mail.MailerInterface
}

// NewEmailVerificationOtpHandler creates a new handler
func NewEmailVerificationOtpHandler(mailer mail.MailerInterface) *EmailVerificationOtpHandler {
	return &EmailVerificationOtpHandler{
		mailer: mailer,
	}
}

// Handle implements the JobHandler interface
func (h *EmailVerificationOtpHandler) Handle(ctx context.Context, job db.Job) error {
	var payload PayloadEmailVerificationOtp
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return fmt.Errorf("failed to parse email verification otp payload: %w", err)
	}

	var payloadExtra PayloadEmailVerificationOtpExtra
	if err := json.Unmarshal(job.PayloadExtra, &payloadExtra); err != nil {
		return fmt.Errorf("failed to parse email verification otp payload extra: %w", err)
	}

	if err := h.mailer.SendOtpEmail(ctx, payload.Email, payloadExtra.Otp); err != nil {
		return fmt.Errorf("failed to send out OTP email: %w", err)
	}

	return nil
}
