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

// PasswordResetHandler handles password reset requests
type PasswordResetHandler struct {
	db     db.Db
	config *config.Config
	mailer *mail.Mailer
}

// NewPasswordResetHandler creates a new PasswordResetHandler
func NewPasswordResetHandler(db db.Db, cfg *config.Config, mailer *mail.Mailer) *PasswordResetHandler {
	return &PasswordResetHandler{
		db:     db,
		config: cfg,
		mailer: mailer,
	}
}

// Handle implements the JobHandler interface for password reset requests
func (h *PasswordResetHandler) Handle(ctx context.Context, job queue.Job) error {
	var payload queue.PayloadPasswordReset
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return fmt.Errorf("failed to parse password reset payload: %w", err)
	}

	// Get user by email
	user, err := h.db.GetUserByEmail(payload.Email)
	if err != nil {
		return fmt.Errorf("failed to get user by email: %w", err)
	}

	if user == nil {
		slog.Info("User not found for password reset", "email", payload.Email)
		return nil // Not an error since we don't want to reveal if email exists
	}

	// Create password reset token with user ID
	token, err := crypto.NewJwtPasswordResetToken(
		user.ID,
		user.Email,
		user.Password,
		h.config.Jwt.PasswordResetSecret,
		h.config.Jwt.PasswordResetTokenDuration,
	)
	if err != nil {
		return fmt.Errorf("failed to create password reset token: %w", err)
	}

	// Construct callback URL to HTML page that will handle the password reset
	callbackURL := fmt.Sprintf("%s%s?token=%s", 
		h.config.Server.BaseURL(), 
		h.config.Endpoints.ConfirmHtml(h.config.Endpoints.ConfirmPasswordReset),
		token)

	// Send password reset email
	if err := h.mailer.SendPasswordResetEmail(ctx, user.Email, callbackURL); err != nil {
		return fmt.Errorf("failed to send password reset email: %w", err)
	}

	slog.Info("Successfully sent password reset email", "email", user.Email)
	return nil
}
