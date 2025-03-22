package mail

import (
	"context"
	"fmt"
	"log/slog"
	"net/smtp"

	"github.com/domodwyer/mailyak/v3"
)

// Mailer handles sending emails
type Mailer struct {
	server   string
	port     int
	username string
	password string
	from     string
}

// New creates a new Mailer instance
func New(server string, port int, username, password, from string) *Mailer {
	return &Mailer{
		server:   server,
		port:     port,
		username: username,
		password: password,
		from:     from,
	}
}

// SendVerificationEmail sends an email verification message
func (m *Mailer) SendVerificationEmail(ctx context.Context, email, token string) error {
	// Create mail client
	mail := mailyak.New(fmt.Sprintf("%s:%d", m.server, m.port), 
		smtp.PlainAuth("", m.username, m.password, m.server))

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
