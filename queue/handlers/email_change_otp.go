package handlers

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/caasmo/restinpieces/db"
	"github.com/caasmo/restinpieces/mail"
)

const JobTypeEmailChangeOtp = "job_type_email_change_otp"

type PayloadEmailChangeOtp struct {
	NewEmail       string `json:"new_email"`
	CooldownBucket int    `json:"cooldown_bucket"`
}

type PayloadEmailChangeOtpExtra struct {
	Otp string `json:"otp"`
}

type EmailChangeOtpHandler struct {
	mailer mail.MailerInterface
}

func NewEmailChangeOtpHandler(mailer mail.MailerInterface) *EmailChangeOtpHandler {
	return &EmailChangeOtpHandler{
		mailer: mailer,
	}
}

func (h *EmailChangeOtpHandler) Handle(ctx context.Context, job db.Job) error {
	var payload PayloadEmailChangeOtp
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return fmt.Errorf("failed to parse email change otp payload: %w", err)
	}

	var payloadExtra PayloadEmailChangeOtpExtra
	if err := json.Unmarshal(job.PayloadExtra, &payloadExtra); err != nil {
		return fmt.Errorf("failed to parse email change otp payload extra: %w", err)
	}

	if err := h.mailer.SendEmailChangeOtpEmail(ctx, payload.NewEmail, payloadExtra.Otp); err != nil {
		return fmt.Errorf("failed to send email change OTP email: %w", err)
	}

	return nil
}
