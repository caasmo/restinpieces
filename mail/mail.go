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
	SendPasswordResetEmail(ctx context.Context, email, callbackURL string) error
	SendOtpEmail(ctx context.Context, email, otp string) error
	SendPasswordResetOtpEmail(ctx context.Context, email, otp string) error
	SendEmailChangeOtpEmail(ctx context.Context, newEmail, otp string) error
	SendEmailChangeAlert(ctx context.Context, oldEmail, newEmail string) error
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

func (m *Mailer) SendOtpEmail(ctx context.Context, email, otp string) error {
	mail, err := m.createMailClient()
	if err != nil {
		return fmt.Errorf("failed to create mail client: %w", err)
	}

	cfg := m.configProvider.Get()
	smtpCfg := cfg.Smtp
	expirationMinutes := int(cfg.Jwt.VerificationEmailOtpTokenDuration.Minutes())

	mail.To(email)
	mail.FromName(smtpCfg.FromName)
	mail.From(smtpCfg.FromAddress)
	mail.Subject(fmt.Sprintf("Your %s verification code", smtpCfg.FromName))
	mail.HTML().Set(fmt.Sprintf(`
		<p>Hello,</p>
		<p>Your verification code is:</p>
		<p style="font-size: 32px; font-weight: bold; letter-spacing: 8px; margin: 20px 0; color: #007bff;">%s</p>
		<p>This code expires in %d minutes.</p>
		<p>If you didn't request this code, you can safely ignore this email.</p>
		<p>Thanks,<br>%s team</p>
	`, otp, expirationMinutes, smtpCfg.FromName))

	done := make(chan error, 1)
	go func() {
		done <- mail.Send()
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-done:
		if err != nil {
			return fmt.Errorf("failed to send OTP email: %w", err)
		}
	}

	return nil
}

func (m *Mailer) SendPasswordResetOtpEmail(ctx context.Context, email, otp string) error {
	mail, err := m.createMailClient()
	if err != nil {
		return fmt.Errorf("failed to create mail client: %w", err)
	}

	cfg := m.configProvider.Get()
	smtpCfg := cfg.Smtp
	expirationMinutes := int(cfg.Jwt.PasswordResetTokenDuration.Minutes())

	mail.To(email)
	mail.FromName(smtpCfg.FromName)
	mail.From(smtpCfg.FromAddress)
	mail.Subject(fmt.Sprintf("Your %s password reset code", smtpCfg.FromName))
	mail.HTML().Set(fmt.Sprintf(`
		<p>Hello,</p>
		<p>We received a request to reset your password. Your password reset code is:</p>
		<p style="font-size: 32px; font-weight: bold; letter-spacing: 8px; margin: 20px 0; color: #007bff;">%s</p>
		<p>This code expires in %d minutes.</p>
		<p>If you didn't request this code, you can safely ignore this email.</p>
		<p>Thanks,<br>%s team</p>
	`, otp, expirationMinutes, smtpCfg.FromName))

	done := make(chan error, 1)
	go func() {
		done <- mail.Send()
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-done:
		if err != nil {
			return fmt.Errorf("failed to send password reset OTP email: %w", err)
		}
	}

	return nil
}

func (m *Mailer) SendEmailChangeOtpEmail(ctx context.Context, newEmail, otp string) error {
	mail, err := m.createMailClient()
	if err != nil {
		return fmt.Errorf("failed to create mail client: %w", err)
	}

	cfg := m.configProvider.Get()
	smtpCfg := cfg.Smtp
	expirationMinutes := int(cfg.Jwt.EmailChangeOtpTokenDuration.Minutes())

	mail.To(newEmail)
	mail.FromName(smtpCfg.FromName)
	mail.From(smtpCfg.FromAddress)
	mail.Subject(fmt.Sprintf("Your %s email change code", smtpCfg.FromName))
	mail.HTML().Set(fmt.Sprintf(`
		<p>Hello,</p>
		<p>We received a request to change your email address. Your verification code is:</p>
		<p style="font-size: 32px; font-weight: bold; letter-spacing: 8px; margin: 20px 0; color: #007bff;">%s</p>
		<p>This code expires in %d minutes.</p>
		<p>If you didn't request this change, you can safely ignore this email.</p>
		<p>Thanks,<br>%s team</p>
	`, otp, expirationMinutes, smtpCfg.FromName))

	done := make(chan error, 1)
	go func() {
		done <- mail.Send()
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-done:
		if err != nil {
			return fmt.Errorf("failed to send email change OTP email: %w", err)
		}
	}

	return nil
}

func (m *Mailer) SendEmailChangeAlert(ctx context.Context, oldEmail, newEmail string) error {
	mail, err := m.createMailClient()
	if err != nil {
		return fmt.Errorf("failed to create mail client: %w", err)
	}

	smtpCfg := m.configProvider.Get().Smtp

	mail.To(oldEmail)
	mail.FromName(smtpCfg.FromName)
	mail.From(smtpCfg.FromAddress)
	mail.Subject(fmt.Sprintf("%s security alert: email address changed", smtpCfg.FromName))
	mail.HTML().Set(fmt.Sprintf(`
		<p>Hello,</p>
		<p>This is a security notification to let you know that the email address
		associated with your account has been changed to <strong>%s</strong>.</p>
		<p>This email address (<strong>%s</strong>) is no longer used for authentication.</p>
		<p>If you did not make this change, please contact support immediately.</p>
		<p>Thanks,<br>%s team</p>
	`, newEmail, oldEmail, smtpCfg.FromName))

	done := make(chan error, 1)
	go func() {
		done <- mail.Send()
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-done:
		if err != nil {
			return fmt.Errorf("failed to send email change alert: %w", err)
		}
	}

	return nil
}
