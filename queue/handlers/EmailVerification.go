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
)

const JobTypeEmailVerification = "job_type_email_verification"

// PayloadEmailVerification contains the email verification details
type PayloadEmailVerification struct {
	Email string `json:"email"`
	// CooldownBucket is the time bucket number calculated from the current time divided by the cooldown duration.
	// This provides a basic rate limiting mechanism where only one email verification request is allowed per time bucket.
	// The bucket number is calculated as: floor(current Unix time / cooldown duration in seconds)
	CooldownBucket int `json:"cooldown_bucket"`
}

// EmailVerificationHandler handles email verification jobs
type EmailVerificationHandler struct {
	dbAuth         db.DbAuth
	configProvider *config.Provider
	mailer         *mail.Mailer
}

// NewEmailVerificationHandler creates a new EmailVerificationHandler
func NewEmailVerificationHandler(dbAuth db.DbAuth, provider *config.Provider, mailer *mail.Mailer) *EmailVerificationHandler {
	return &EmailVerificationHandler{
		dbAuth:         dbAuth,
		configProvider: provider,
		mailer:         mailer,
	}
}

// Handle implements the JobHandler interface for email verification
func (h *EmailVerificationHandler) Handle(ctx context.Context, job db.Job) error {
	cfg := h.configProvider.Get()

	var payload PayloadEmailVerification
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return fmt.Errorf("failed to parse email verification payload: %w", err)
	}

	// Get user by email
	user, err := h.dbAuth.GetUserByEmail(payload.Email)
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
		cfg.Jwt.VerificationEmailTokenDuration.Duration,
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
