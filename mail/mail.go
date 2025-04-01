package mail

import (
	"context"
	"crypto/tls"
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
	// host is the SMTP server hostname or IP address
	// Example: "smtp.example.com" or "192.168.1.100"
	host string

	// port is the SMTP server port number
	// Common ports: 25 (unencrypted), 465 (SSL/TLS), 587 (STARTTLS)
	port int

	// username is the authentication username for the SMTP server
	// Typically an email address or system username
	username string

	// password is the authentication password for the SMTP server
	// Should be stored securely and not logged
	password string

	// fromName is the sender name that will appear in the "From" header
	// Example: "My App"
	fromName string

	// fromAddress is the email address that will appear in the "From" header
	// Example: "noreply@example.com"
	fromAddress string

	// localName is the HELO/EHLO domain to use in SMTP communication
	// If empty, defaults to "localhost"
	localName string

	// authMethod specifies the SMTP authentication mechanism
	// Supported values: "plain", "cram-md5", "none"
	// - "plain": Standard SMTP AUTH PLAIN (RFC 4616) - Recommended for most use cases
	// - "cram-md5": CRAM-MD5 challenge-response (RFC 2195) - Less secure than PLAIN
	// - "none": No authentication - Only for testing with local SMTP servers
	// Note: The LOGIN authentication method has been removed as it is considered deprecated
	// and insecure. Modern SMTP servers should use PLAIN authentication over TLS.
	authMethod string

	// useTLS enables explicit TLS encryption for the SMTP connection
	// Used with port 465 (SMTPS)
	// If true, establishes TLS connection immediately
	useTLS bool

	// useStartTLS enables STARTTLS encryption for the SMTP connection
	// Used with port 587
	// If true, upgrades plain connection to TLS after initial handshake
	useStartTLS bool
}

// Handle implements JobHandler for email verification jobs
func (m *Mailer) Handle(ctx context.Context, job queue.Job) error {
	var payload queue.PayloadEmailVerification
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return fmt.Errorf("failed to parse email verification payload: %w", err)
	}

	// Generate a random callback URL for testing
	callbackURL := "http://localhost:8080/verify-email" // TODO: Make this configurable
	callbackURL = fmt.Sprintf("%s?token=%d", callbackURL, job.ID)
	return m.SendVerificationEmail(ctx, payload.Email, callbackURL)
}

// New creates a new Mailer instance from config
func New(cfg config.Smtp) (*Mailer, error) {
	return &Mailer{
		host:        cfg.Host,
		port:        cfg.Port,
		username:    cfg.Username,
		password:    cfg.Password,
		fromName:    cfg.FromName,
		fromAddress: cfg.FromAddress,
		localName:   cfg.LocalName,
		authMethod:  cfg.AuthMethod,
		useTLS:      cfg.UseTLS,
		useStartTLS: cfg.UseStartTLS,
	}, nil
}

// createMailClient creates a new mailyak instance
func (m *Mailer) createMailClient() (*mailyak.MailYak, error) {
	var auth smtp.Auth
	switch m.authMethod {
	case "cram-md5":
		auth = smtp.CRAMMD5Auth(m.username, m.password)
	case "none":
		auth = nil
	default: // "plain" or empty
		auth = smtp.PlainAuth("", m.username, m.password, m.host)
	}

	if m.useTLS {
		// Use explicit TLS (SMTPS)
		mail, err := mailyak.NewWithTLS(fmt.Sprintf("%s:%d", m.host, m.port), auth, &tls.Config{
			ServerName:         m.host,
			InsecureSkipVerify: false, // Always verify certs in production
		})
		if err != nil {
			return nil, err
		}
		if m.localName != "" {
			mail.LocalName(m.localName)
		}
		return mail, nil
	}

	// Use plain connection (will automatically upgrade to STARTTLS if server supports it)
	mail := mailyak.New(fmt.Sprintf("%s:%d", m.host, m.port), auth)
	if m.localName != "" {
		mail.LocalName(m.localName)
	}
	return mail, nil
}

// SendVerificationEmail sends an email verification message to the specified email address
// with the verification callback URL that includes the token
func (m *Mailer) SendVerificationEmail(ctx context.Context, email, callbackURL string) error {
	// Create new mail client for this email
	mail, err := m.createMailClient()
	if err != nil {
		return fmt.Errorf("failed to create mail client: %w", err)
	}

	// Build email
	mail.To(email)
	mail.FromName(m.fromName)
	mail.From(m.fromAddress)
	mail.Subject(fmt.Sprintf("Verify your %s email", m.fromName))
	mail.HTML().Set(fmt.Sprintf(`
		<p>Hello,</p>
		<p>Thank you for joining us at %s.</p>
		<p>Click on the button below to verify your email address.</p>
		<p style="margin: 20px 0;">
			<a href="%s" 
				style="background-color: #007bff; color: white; padding: 10px 20px; text-decoration: none; border-radius: 5px;">
				Verify
			</a>
		</p>
		<p>Thanks,<br>%s team</p>
	`, m.fromName, callbackURL, m.fromName))

	return fmt.Errorf("IN MAIL DEBUG: %s", callbackURL)
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

// SendEmailChangeNotification sends an email change notification to both old and new email addresses
func (m *Mailer) SendEmailChangeNotification(ctx context.Context, newEmail, oldEmail, callbackURL string) error {
	// Create new mail client for this email
	mail, err := m.createMailClient()
	if err != nil {
		return fmt.Errorf("failed to create mail client: %w", err)
	}

	// Build email - send to new email address for verification
	mail.To(newEmail)
	mail.FromName(m.fromName)
	mail.From(m.fromAddress)
	mail.Subject(fmt.Sprintf("Confirm your email change to %s", newEmail))
	mail.HTML().Set(fmt.Sprintf(`
		<p>Hello,</p>
		<p>We received a request to change your email from %s to %s.</p>
		<p>Click on the button below to confirm this change:</p>
		<p style="margin: 20px 0;">
			<a href="%s"
				style="background-color: #007bff; color: white; padding: 10px 20px; text-decoration: none; border-radius: 5px;">
				Confirm Email Change
			</a>
		</p>
		<p>If you didn't request this change, please contact support immediately.</p>
		<p>Thanks,<br>%s team</p>
	`, oldEmail, newEmail, callbackURL, m.fromName))

	return fmt.Errorf("CHANGE EMAIL SEND DEBUG: %s", callbackURL)
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
			return fmt.Errorf("failed to send email change notification: %w", err)
		}
	}

	slog.Info("Successfully sent email change notification", 
		"old_email", oldEmail, 
		"new_email", newEmail)
	return nil
}

// SendPasswordResetEmail sends a password reset message to the specified email address
// with the password reset callback URL that includes the token
func (m *Mailer) SendPasswordResetEmail(ctx context.Context, email, callbackURL string) error {
	// Create new mail client for this email
	mail, err := m.createMailClient()
	if err != nil {
		return fmt.Errorf("failed to create mail client: %w", err)
	}

	// Build email
	mail.To(email)
	mail.FromName(m.fromName)
	mail.From(m.fromAddress)
	mail.Subject(fmt.Sprintf("Reset your %s password", m.fromName))
	mail.HTML().Set(fmt.Sprintf(`
		<p>Hello,</p>
		<p>We received a request to reset your %s password.</p>
		<p>Click on the button below to reset your password:</p>
		<p style="margin: 20px 0;">
			<a href="%s"
				style="background-color: #007bff; color: white; padding: 10px 20px; text-decoration: none; border-radius: 5px;">
				Reset Password
			</a>
		</p>
		<p>If you didn't request this, you can safely ignore this email.</p>
		<p>Thanks,<br>%s team</p>
	`, m.fromName, callbackURL, m.fromName))

	return fmt.Errorf("IN MAIL DEBUGGGGGGGGGGGGGGGGGGGGGGGGG: %s", callbackURL)

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
			return fmt.Errorf("failed to send password reset email: %w", err)
		}
	}

	slog.Info("Successfully sent password reset email", "email", email)
	return nil
}
