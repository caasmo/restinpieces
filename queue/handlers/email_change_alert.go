package handlers

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/caasmo/restinpieces/db"
	"github.com/caasmo/restinpieces/mail"
)

const JobTypeEmailChangeAlert = "job_type_email_change_alert"

type PayloadEmailChangeAlert struct {
	OldEmail string `json:"old_email"`
	NewEmail string `json:"new_email"`
}

type EmailChangeAlertHandler struct {
	mailer mail.MailerInterface
}

func NewEmailChangeAlertHandler(mailer mail.MailerInterface) *EmailChangeAlertHandler {
	return &EmailChangeAlertHandler{
		mailer: mailer,
	}
}

func (h *EmailChangeAlertHandler) Handle(ctx context.Context, job db.Job) error {
	var payload PayloadEmailChangeAlert
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return fmt.Errorf("failed to parse email change alert payload: %w", err)
	}

	if err := h.mailer.SendEmailChangeAlert(ctx, payload.OldEmail, payload.NewEmail); err != nil {
		return fmt.Errorf("failed to send email change alert: %w", err)
	}

	return nil
}
