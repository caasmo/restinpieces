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

	// Create verification token
	token, err := h.createVerificationToken(user.Email, user.Password)
	if err != nil {
		return fmt.Errorf("failed to create verification token: %w", err)
	}
	
	// Construct callback URL using server's base URL
	callbackURL := fmt.Sprintf("%s/verify-email?token=%s", h.config.Server.BaseURL(), token)

	// Send verification email
	if err := h.mailer.SendVerificationEmail(ctx, user.Email, callbackURL); err != nil {
		return fmt.Errorf("failed to send verification email: %w", err)
	}

	slog.Info("Successfully sent verification email", "email", user.Email)
	return nil
}

// createVerificationToken generates a JWT verification token using the email, password hash and verification secret
func (h *EmailVerificationHandler) createVerificationToken(email, passwordHash string) (string, error) {
	// Create signing key from credentials and verification secret
	signingKey, err := crypto.NewJwtSigningKeyWithCredentials(
		email,
		passwordHash,
		h.config.Jwt.VerificationEmailSecret,
	)
	if err != nil {
		return "", fmt.Errorf("failed to create signing key: %w", err)
	}

	// Create JWT claims
	// TODO to jwt to not export 3 party
	claims := jwt.MapClaims{
		crypto.ClaimEmail: email,
		crypto.ClaimType:  crypto.ClaimVerificationValue,
	}

	// Generate JWT token with verification duration
	token, _, err := crypto.NewJwt(
		claims,
		signingKey,
		h.config.Jwt.VerificationEmailTokenDuration,
	)
	if err != nil {
		return "", fmt.Errorf("failed to create JWT: %w", err)
	}

	return token, nil
}
