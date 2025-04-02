package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/caasmo/restinpieces/config"
	"github.com/caasmo/restinpieces/crypto"
	"github.com/caasmo/restinpieces/db"
	"github.com/caasmo/restinpieces/mail"
	"github.com/caasmo/restinpieces/queue"
)

// EmailChangeHandler handles email change requests
type EmailChangeHandler struct {
	db     db.Db
	config *config.Config
	mailer *mail.Mailer
}

// NewEmailChangeHandler creates a new EmailChangeHandler
func NewEmailChangeHandler(db db.Db, cfg *config.Config, mailer *mail.Mailer) *EmailChangeHandler {
	return &EmailChangeHandler{
		db:     db,
		config: cfg,
		mailer: mailer,
	}
}

// Handle implements the JobHandler interface for email change requests
func (h *EmailChangeHandler) Handle(ctx context.Context, job queue.Job) error {

	var payload queue.PayloadEmailChange
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return fmt.Errorf("failed to parse email change payload: %w", err)
	}

	var payloadExtra queue.PayloadEmailChangeExtra
	if err := json.Unmarshal(job.PayloadExtra, &payloadExtra); err != nil {
		return fmt.Errorf("failed to parse email change extra payload: %w", err)
	}

	// Get user by ID
	user, err := h.db.GetUserById(payload.UserID)
	if err != nil {
		return fmt.Errorf("failed to get user by ID: %w", err)
	}

	if user == nil {
		slog.Info("User not found for email change", "user_id", payload.UserID)
		return fmt.Errorf("user not found")
	}

	// Create email change token with user ID
	token, err := crypto.NewJwtEmailChangeToken(
		user.ID,
		user.Email,
		payloadExtra.NewEmail,
		user.Password,
		h.config.Jwt.EmailChangeSecret,
		h.config.Jwt.EmailChangeTokenDuration,
	)
	if err != nil {
		return fmt.Errorf("failed to create email change token: %w", err)
	}

	// Construct callback URL to HTML page that will handle the email change
	callbackURL := fmt.Sprintf("%s%s?token=%s",
		h.config.Server.BaseURL(),
		h.config.Endpoints.ConfirmHtml(h.config.Endpoints.ConfirmEmailChange),
		token)

	// Send email change notification including OAuth2 warning if needed
	if err := h.mailer.SendEmailChangeNotification(ctx, user.Email, payloadExtra.NewEmail, user.Oauth2, callbackURL); err != nil {
		return fmt.Errorf("failed to send email change notification: %w", err)
	}

	slog.Info("Successfully sent email change notification",
		"old_email", user.Email,
		"new_email", payloadExtra.NewEmail)
	return nil
}
