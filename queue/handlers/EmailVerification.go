package handlers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"

	"github.com/caasmo/restinpieces/config"
	"github.com/caasmo/restinpieces/db"
	"github.com/caasmo/restinpieces/mail"
	"github.com/caasmo/restinpieces/queue"
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

	// Create verification token
	token := h.createVerificationToken(user.Email, user.Password)
	
	// Construct callback URL
	callbackURL := fmt.Sprintf("%s/verify-email?token=%s", h.config.Server.Addr, token)

	// Send verification email
	if err := h.mailer.SendVerificationEmail(ctx, user.Email, callbackURL); err != nil {
		return fmt.Errorf("failed to send verification email: %w", err)
	}

	slog.Info("Successfully sent verification email", "email", user.Email)
	return nil
}

// createVerificationToken generates a verification token using the email, password hash and verification secret
func (h *EmailVerificationHandler) createVerificationToken(email, passwordHash string) string {
	// Combine email, password hash and verification secret
	data := fmt.Sprintf("%s:%s:%s", email, passwordHash, h.config.Jwt.VerificationEmailSecret)

	// Hash the combined data
	hash := sha256.Sum256([]byte(data))
	
	// Return hex encoded hash
	return hex.EncodeToString(hash[:])
}
