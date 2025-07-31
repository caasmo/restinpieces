package handlers

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/caasmo/restinpieces/config"
	"github.com/caasmo/restinpieces/crypto"
	"github.com/caasmo/restinpieces/db"
	"github.com/caasmo/restinpieces/mail"
)

const JobTypeEmailChange = "job_type_email_change"

type PayloadEmailChange struct {
	UserID         string `json:"user_id"`
	CooldownBucket int    `json:"cooldown_bucket"`
}

type PayloadEmailChangeExtra struct {
	NewEmail string `json:"new_email"`
}

// EmailChangeHandler handles email change requests
type EmailChangeHandler struct {
	dbAuth         db.DbAuth
	configProvider *config.Provider
	mailer         mail.MailerInterface
}

// NewEmailChangeHandler creates a new EmailChangeHandler
func NewEmailChangeHandler(dbAuth db.DbAuth, provider *config.Provider, mailer mail.MailerInterface) *EmailChangeHandler {
	return &EmailChangeHandler{
		dbAuth:         dbAuth,
		configProvider: provider,
		mailer:         mailer,
	}
}

// Handle implements the JobHandler interface for email change requests
func (h *EmailChangeHandler) Handle(ctx context.Context, job db.Job) error {
	// Get current config snapshot
	cfg := h.configProvider.Get()

	var payload PayloadEmailChange
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return fmt.Errorf("failed to parse email change payload: %w", err)
	}

	var payloadExtra PayloadEmailChangeExtra
	if err := json.Unmarshal(job.PayloadExtra, &payloadExtra); err != nil {
		return fmt.Errorf("failed to parse email change extra payload: %w", err)
	}

	// Get user by ID
	user, err := h.dbAuth.GetUserById(payload.UserID)
	if err != nil {
		return fmt.Errorf("failed to get user by ID: %w", err)
	}

	if user == nil {
		//app.Logger.Info("User not found for email change", "user_id", payload.UserID)
		return fmt.Errorf("user not found")
	}

	// Create email change token with user ID
	token, err := crypto.NewJwtEmailChangeToken(
		user.ID,
		user.Email,
		payloadExtra.NewEmail,
		user.Password,
		cfg.Jwt.EmailChangeSecret,
		cfg.Jwt.EmailChangeTokenDuration.Duration,
	)
	if err != nil {
		return fmt.Errorf("failed to create email change token: %w", err)
	}

	// Construct callback URL to HTML page that will handle the email change
	callbackURL := fmt.Sprintf("%s%s?token=%s",
		cfg.Server.BaseURL(),
		cfg.Endpoints.ConfirmHtml(cfg.Endpoints.ConfirmEmailChange),
		token)

	// Send email change notification including OAuth2 warning if needed
	if err := h.mailer.SendEmailChangeNotification(ctx, user.Email, payloadExtra.NewEmail, user.Oauth2, callbackURL); err != nil {
		return fmt.Errorf("failed to send email change notification: %w", err)
	}

	//app.Logger.Info("Successfully sent email change notification", "old_email", user.Email, "new_email", payloadExtra.NewEmail)
	return nil
}
