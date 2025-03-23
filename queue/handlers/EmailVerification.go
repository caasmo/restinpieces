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
	"github.com/golang-jwt/jwt/v5"
)

// EmailVerificationHandler handles email verification jobs
type EmailVerificationHandler struct {
	db     db.Db
	config *config.Config
	mailer *mail.Mailer
}

// NewEmailVerificationHandler creates a new EmailVerificationHandler
func NewEmailVerificationHandler(db db.Db, cfg *config.Config, mailer *mail.Mailer) *EmailVerificationHandler {
	return &EmailVerificationHandler{
		db:     db,
		config: cfg,
		mailer: mailer,
	}
}

// Handle implements the JobHandler interface for email verification
func (h *EmailVerificationHandler) Handle(ctx context.Context, job queue.Job) error {
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
		slog.Info("User already verified, skipping email", "email", user.Email)
		return nil
	}

	// Create verification token with user ID
	token, _, err := crypto.NewJwtEmailVerificationToken(
		user.ID,
		user.Email,
		user.Password,
		h.config.Jwt.VerificationEmailSecret,
		h.config.Jwt.VerificationEmailTokenDuration,
	)
	if err != nil {
		return fmt.Errorf("failed to create verification token: %w", err)
	}
	
	// Construct callback URL using server's base URL and HTML verification page
	callbackURL := fmt.Sprintf("%s/verify-email.html?token=%s", h.config.Server.BaseURL(), token)

	// Send verification email
	if err := h.mailer.SendVerificationEmail(ctx, user.Email, callbackURL); err != nil {
		return fmt.Errorf("failed to send verification email: %w", err)
	}

	slog.Info("Successfully sent verification email", "email", user.Email)
	return nil
}

