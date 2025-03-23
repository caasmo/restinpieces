package mail

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/smtp"

	"github.com/caasmo/restinpieces/config"
	"github.com/caasmo/restinpieces/queue"
	"github.com/domodwyer/mailyak/v3"
)

// Mailer handles sending emails and implements queue.JobHandler
type Mailer struct {
	host     string
	port     int
	username string
	password string
	from     string
}

// Handle implements JobHandler for email verification jobs
func (m *Mailer) Handle(ctx context.Context, job queue.Job) error {
	var payload queue.PayloadEmailVerification
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return fmt.Errorf("failed to parse email verification payload: %w", err)
	}

	return m.SendVerificationEmail(ctx, payload.Email, fmt.Sprintf("%d", job.ID))
}

// New creates a new Mailer instance from config
func New(cfg config.Smtp) *Mailer {
	return &Mailer{
		host:     cfg.Server,
		port:     cfg.Port,
		username: cfg.Username,
		password: cfg.Password,
		from:     cfg.From,
	}
}

// SendVerificationEmail sends an email verification message
func (m *Mailer) SendVerificationEmail(ctx context.Context, email, token string) error {
	// Create mail client
	mail := mailyak.New(fmt.Sprintf("%s:%d", m.host, m.port), 
		smtp.PlainAuth("", m.username, m.password, m.host))

	// Build email
	mail.To(email)
	mail.From(m.from)
	mail.Subject("Email Verification")
	mail.HTML().Set(fmt.Sprintf(`
		<h1>Email Verification</h1>
		<p>Please click the link below to verify your email address:</p>
		<p><a href="http://example.com/verify-email?token=%s">Verify Email</a></p>
	`, token))

	// Send email with context timeout
	done := make(chan error, 1)
	go func() {
		done <- mail.Send()
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-done:
		if err != nil {
			return fmt.Errorf("failed to send verification email: %w", err)
		}
	}

	slog.Info("Successfully sent verification email", "email", email)
	return nil
}
