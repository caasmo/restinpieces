package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	//"log/slog"

	"github.com/caasmo/restinpieces/config"
	"github.com/caasmo/restinpieces/crypto"
	"github.com/caasmo/restinpieces/db"
	"github.com/caasmo/restinpieces/mail"
	"github.com/caasmo/restinpieces/queue"
)

// EmailVerificationHandler handles email verification jobs
type EmailVerificationHandler struct {
	db             db.Db
	configProvider *config.Provider
	mailer         *mail.Mailer
}

// NewEmailVerificationHandler creates a new EmailVerificationHandler
func NewEmailVerificationHandler(db db.Db, provider *config.Provider, mailer *mail.Mailer) *EmailVerificationHandler {
	return &EmailVerificationHandler{
		db:             db,
		configProvider: provider,
		mailer:         mailer,
	}
}

// Handle implements the JobHandler interface for email verification
func (h *EmailVerificationHandler) Handle(ctx context.Context, job queue.Job) error {
	cfg := h.configProvider.Get()

	var payload queue.PayloadEmailVerification
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return fmt.Errorf("failed to parse email verification payload: %w", err)
	}

	// Get user by email
	user, err := h.db.GetUserByEmail(payload.Email)
	if err != nil {
		return fmt.Errorf("failed to get user by email: %w", err)
	}

	if user == nil {
		return fmt.Errorf("user not found for email: %s", payload.Email)
	}

	// Check if user is already verified
	if user.Verified {
		//app.Logger.Info("User already verified, skipping email", "email", user.Email)
		return nil
	}

	// Create verification token with user ID
	token, err := crypto.NewJwtEmailVerificationToken(
		user.ID,
		user.Email,
		user.Password,
		cfg.Jwt.VerificationEmailSecret,
		cfg.Jwt.VerificationEmailTokenDuration,
	)
	if err != nil {
		return fmt.Errorf("failed to create verification token: %w", err)
	}

	// Construct callback URL to HTML page that will handle the verification
	callbackURL := fmt.Sprintf("%s%s?token=%s",
		cfg.Server.BaseURL(),
		cfg.Endpoints.ConfirmHtml(cfg.Endpoints.ConfirmEmailVerification),
		token)

	// Send verification email
	if err := h.mailer.SendVerificationEmail(ctx, user.Email, callbackURL); err != nil {
		return fmt.Errorf("failed to send verification email: %w", err)
	}

	//app.Logger.Info("Successfully sent verification email", "email", user.Email)
	return nil
}
