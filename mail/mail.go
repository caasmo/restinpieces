package mail

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/smtp"

	"github.com/caasmo/restinpieces/config"
	"github.com/domodwyer/mailyak/v3"
)

// MailerInterface defines the methods for sending emails.
type MailerInterface interface {
	SendVerificationEmail(ctx context.Context, email, callbackURL string) error
	SendEmailChangeNotification(ctx context.Context, oldEmail, newEmail string, hasOauth2Login bool, callbackURL string) error
	SendPasswordResetEmail(ctx context.Context, email, callbackURL string) error
}

// Mailer handles sending emails using configuration from a provider.
type Mailer struct {
	configProvider *config.Provider
}

// New creates a new Mailer instance using a config provider.
func New(provider *config.Provider) (MailerInterface, error) {
	if provider == nil {
		return nil, fmt.Errorf("config provider cannot be nil")
	}
	// Initial check if SMTP config is present? Or defer to send time?
	// Let's defer to send time for now, allows starting without SMTP configured.
	return &Mailer{
		configProvider: provider,
	}, nil
}

var _ MailerInterface = (*Mailer)(nil)


// createMailClient creates a new mailyak instance using current config from provider.
func (m *Mailer) createMailClient() (*mailyak.MailYak, error) {
	// Get the current SMTP configuration
	smtpCfg := m.configProvider.Get().Smtp
	if smtpCfg.Host == "" {
		return nil, fmt.Errorf("SMTP host is not configured")
	}

	var auth smtp.Auth
	switch smtpCfg.AuthMethod {
	case "cram-md5":
		auth = smtp.CRAMMD5Auth(smtpCfg.Username, smtpCfg.Password)
	case "none":
		auth = nil
	default: // "plain" or empty
		auth = smtp.PlainAuth("", smtpCfg.Username, smtpCfg.Password, smtpCfg.Host)
	}

	addr := fmt.Sprintf("%s:%d", smtpCfg.Host, smtpCfg.Port)

	if smtpCfg.UseTLS {
		// Use explicit TLS (SMTPS)
		mail, err := mailyak.NewWithTLS(addr, auth, &tls.Config{
			ServerName:         smtpCfg.Host,
			InsecureSkipVerify: false, // Always verify certs in production
		})
		if err != nil {
			return nil, err
		}
		if smtpCfg.LocalName != "" {
			mail.LocalName(smtpCfg.LocalName)
		}
		return mail, nil
	}

	// Use plain connection (mailyak handles STARTTLS automatically if available)
	mail := mailyak.New(addr, auth)
	if smtpCfg.LocalName != "" {
		mail.LocalName(smtpCfg.LocalName)
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

	// Get current SMTP config for FromName/FromAddress
	smtpCfg := m.configProvider.Get().Smtp

	// Build email
	mail.To(email)
	mail.FromName(smtpCfg.FromName)
	mail.From(smtpCfg.FromAddress)
	mail.Subject(fmt.Sprintf("Verify your %s email", smtpCfg.FromName))
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
	`, smtpCfg.FromName, callbackURL, smtpCfg.FromName))

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

	//app.Logger.Info("Successfully sent verification email", "email", email)
	return nil
}

// SendEmailChangeNotification sends an email change notification to both old and new email addresses
//
// hasOauth2Login determines if we should include a warning about passwordless login being invalidated
func (m *Mailer) SendEmailChangeNotification(ctx context.Context, oldEmail, newEmail string, hasOauth2Login bool, callbackURL string) error {
	// Create new mail client for this email
	mail, err := m.createMailClient()
	if err != nil {
		return fmt.Errorf("failed to create mail client: %w", err)
	}

	// Get current SMTP config for FromName/FromAddress
	smtpCfg := m.configProvider.Get().Smtp

	// Create warning message if user has OAuth2 login
	warning := ""
	if hasOauth2Login {
		warning = `<p style="color: #d32f2f;">
			Please consider that your old email is used for passwordless login (OAuth2). 
			By changing your email you will invalidate that login method.
		</p>`
	}

	// Build email - send to new email address for verification
	mail.To(newEmail)
	mail.FromName(smtpCfg.FromName)
	mail.From(smtpCfg.FromAddress)
	mail.Subject(fmt.Sprintf("Confirm your email change to %s", newEmail))
	mail.HTML().Set(fmt.Sprintf(`
		<p>Hello,</p>
		<p>We received a request to change your email from %s to %s.</p>
		%s
		<p>Click on the button below to confirm this change:</p>
		<p style="margin: 20px 0;">
			<a href="%s"
				style="background-color: #007bff; color: white; padding: 10px 20px; text-decoration: none; border-radius: 5px;">
				Confirm Email Change
			</a>
		</p>
		<p>If you didn't request this change, please contact support immediately.</p>
		<p>Thanks,<br>%s team</p>
	`, oldEmail, newEmail, warning, callbackURL, smtpCfg.FromName))

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

	//app.Logger.Info("Successfully sent email change notification", "old_email", oldEmail, "new_email", newEmail)
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

	// Get current SMTP config for FromName/FromAddress
	smtpCfg := m.configProvider.Get().Smtp

	// Build email
	mail.To(email)
	mail.FromName(smtpCfg.FromName)
	mail.From(smtpCfg.FromAddress)
	mail.Subject(fmt.Sprintf("Reset your %s password", smtpCfg.FromName))
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
	`, smtpCfg.FromName, callbackURL, smtpCfg.FromName))

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

	//app.Logger.Info("Successfully sent password reset email", "email", email)
	return nil
}
