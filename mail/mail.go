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


// loginAuth implements the LOGIN authentication mechanism
type loginAuth struct {
	username string
	password string
}

func (a *loginAuth) Start(server *smtp.ServerInfo) (string, []byte, error) {
	return "LOGIN", []byte{}, nil
}

func (a *loginAuth) Next(fromServer []byte, more bool) ([]byte, error) {
	if more {
		switch string(fromServer) {
		case "Username:":
			return []byte(a.username), nil
		case "Password:":
			return []byte(a.password), nil
		default:
			return nil, fmt.Errorf("unexpected server challenge: %s", fromServer)
		}
	}
	return nil, nil
}

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

	// from is the email address that will appear in the "From" header
	// Example: "noreply@example.com"
	from string

	// authMethod specifies the SMTP authentication mechanism
	// Supported values: "plain", "login", "cram-md5", "none"
	// - "plain": Standard SMTP AUTH PLAIN (RFC 4616)
	// - "login": Legacy LOGIN mechanism
	// - "cram-md5": CRAM-MD5 challenge-response (RFC 2195)
	// - "none": No authentication
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

	return m.SendVerificationEmail(ctx, payload.Email, fmt.Sprintf("%d", job.ID))
}

// New creates a new Mailer instance from config
func New(cfg config.Smtp) *Mailer {
	return &Mailer{
		host:        cfg.Host,
		port:        cfg.Port,
		username:    cfg.Username,
		password:    cfg.Password,
		from:        cfg.From,
		authMethod:  cfg.AuthMethod,
		useTLS:      cfg.UseTLS,
		useStartTLS: cfg.UseStartTLS,
	}
}

// SendVerificationEmail sends an email verification message
func (m *Mailer) SendVerificationEmail(ctx context.Context, email, token string) error {
	// Create mail client
	var auth smtp.Auth
	switch m.authMethod {
	case "login":
		auth = &loginAuth{username: m.username, password: m.password}
	case "cram-md5":
		auth = smtp.CRAMMD5Auth(m.username, m.password)
	case "none":
		auth = nil
	default: // "plain" or empty
		auth = smtp.PlainAuth("", m.username, m.password, m.host)
	}

	mail, err := mailyak.NewWithTLS(fmt.Sprintf("%s:%d", m.host, m.port), auth, &tls.Config{
		ServerName:         m.host,
		InsecureSkipVerify: !m.useTLS, // Only verify cert if using TLS
	})
	if err != nil {
		return fmt.Errorf("failed to create mail client: %w", err)
	}

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
