package handlers

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/caasmo/restinpieces/db"
	"github.com/caasmo/restinpieces/mail"
)

const JobTypePasswordResetOtp = "job_type_password_reset_otp"

type PayloadPasswordResetOtp struct {
	Email          string `json:"email"`
	CooldownBucket int    `json:"cooldown_bucket"`
}

type PayloadPasswordResetOtpExtra struct {
	Otp string `json:"otp"`
}

type PasswordResetOtpHandler struct {
	mailer mail.MailerInterface
}

func NewPasswordResetOtpHandler(mailer mail.MailerInterface) *PasswordResetOtpHandler {
	return &PasswordResetOtpHandler{
		mailer: mailer,
	}
}

func (h *PasswordResetOtpHandler) Handle(ctx context.Context, job db.Job) error {
	var payload PayloadPasswordResetOtp
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return fmt.Errorf("failed to parse password reset otp payload: %w", err)
	}

	var payloadExtra PayloadPasswordResetOtpExtra
	if err := json.Unmarshal(job.PayloadExtra, &payloadExtra); err != nil {
		return fmt.Errorf("failed to parse password reset otp payload extra: %w", err)
	}

	if err := h.mailer.SendPasswordResetOtpEmail(ctx, payload.Email, payloadExtra.Otp); err != nil {
		return fmt.Errorf("failed to send password reset OTP email: %w", err)
	}

	return nil
}
