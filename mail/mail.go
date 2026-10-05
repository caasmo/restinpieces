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

// createMailClient creates a new mailyak instance for the given SMTP configuration.
func createMailClient(smtpCfg config.Smtp) (*mailyak.MailYak, error) {
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

// sendEmail delivers one HTML email over SMTP with smtpCfg.
//
// The SMTP send cannot be canceled: when ctx ends, sendEmail returns but the
// send keeps running in the background until the connection times out.
func sendEmail(ctx context.Context, smtpCfg config.Smtp, to, subject, htmlBody string) error {
	client, err := createMailClient(smtpCfg)
	if err != nil {
		return err
	}

	client.To(to)
	client.FromName(smtpCfg.FromName)
	client.From(smtpCfg.FromAddress)
	client.Subject(subject)
	client.HTML().Set(htmlBody)

	done := make(chan error, 1)
	go func() {
		done <- client.Send()
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case sendErr := <-done:
		return sendErr
	}
}

// SendPasswordResetEmail sends a password reset message to the specified email address
// with the password reset callback URL that includes the token
func (m *Mailer) SendPasswordResetEmail(ctx context.Context, email, callbackURL string) error {
	smtpCfg := m.configProvider.Get().Smtp

	subject := fmt.Sprintf("Reset your %s password", smtpCfg.FromName)
	htmlBody := fmt.Sprintf(`
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
	`, smtpCfg.FromName, callbackURL, smtpCfg.FromName)

	err := sendEmail(ctx, smtpCfg, email, subject, htmlBody)
	if err != nil {
		return fmt.Errorf("failed to send password reset email: %w", err)
	}

	//app.Logger.Info("Successfully sent password reset email", "email", email)
	return nil
}

func (m *Mailer) SendOtpEmail(ctx context.Context, email, otp string) error {
	cfg := m.configProvider.Get()
	expirationMinutes := int(cfg.Jwt.VerificationEmailOtpTokenDuration.Minutes())

	subject := fmt.Sprintf("Your %s verification code", cfg.Smtp.FromName)
	htmlBody := fmt.Sprintf(`
		<p>Hello,</p>
		<p>Your verification code is:</p>
		<p style="font-size: 32px; font-weight: bold; letter-spacing: 8px; margin: 20px 0; color: #007bff;">%s</p>
		<p>This code expires in %d minutes.</p>
		<p>If you didn't request this code, you can safely ignore this email.</p>
		<p>Thanks,<br>%s team</p>
	`, otp, expirationMinutes, cfg.Smtp.FromName)

	err := sendEmail(ctx, cfg.Smtp, email, subject, htmlBody)
	if err != nil {
		return fmt.Errorf("failed to send OTP email: %w", err)
	}
	return nil
}

func (m *Mailer) SendPasswordResetOtpEmail(ctx context.Context, email, otp string) error {
	cfg := m.configProvider.Get()
	expirationMinutes := int(cfg.Jwt.PasswordResetTokenDuration.Minutes())

	subject := fmt.Sprintf("Your %s password reset code", cfg.Smtp.FromName)
	htmlBody := fmt.Sprintf(`
		<p>Hello,</p>
		<p>We received a request to reset your password. Your password reset code is:</p>
		<p style="font-size: 32px; font-weight: bold; letter-spacing: 8px; margin: 20px 0; color: #007bff;">%s</p>
		<p>This code expires in %d minutes.</p>
		<p>If you didn't request this code, you can safely ignore this email.</p>
		<p>Thanks,<br>%s team</p>
	`, otp, expirationMinutes, cfg.Smtp.FromName)

	err := sendEmail(ctx, cfg.Smtp, email, subject, htmlBody)
	if err != nil {
		return fmt.Errorf("failed to send password reset OTP email: %w", err)
	}
	return nil
}

func (m *Mailer) SendEmailChangeOtpEmail(ctx context.Context, newEmail, otp string) error {
	cfg := m.configProvider.Get()
	expirationMinutes := int(cfg.Jwt.EmailChangeOtpTokenDuration.Minutes())

	subject := fmt.Sprintf("Your %s email change code", cfg.Smtp.FromName)
	htmlBody := fmt.Sprintf(`
		<p>Hello,</p>
		<p>We received a request to change your email address. Your verification code is:</p>
		<p style="font-size: 32px; font-weight: bold; letter-spacing: 8px; margin: 20px 0; color: #007bff;">%s</p>
		<p>This code expires in %d minutes.</p>
		<p>If you didn't request this change, you can safely ignore this email.</p>
		<p>Thanks,<br>%s team</p>
	`, otp, expirationMinutes, cfg.Smtp.FromName)

	err := sendEmail(ctx, cfg.Smtp, newEmail, subject, htmlBody)
	if err != nil {
		return fmt.Errorf("failed to send email change OTP email: %w", err)
	}
	return nil
}

func (m *Mailer) SendEmailChangeAlert(ctx context.Context, oldEmail, newEmail string) error {
	smtpCfg := m.configProvider.Get().Smtp

	subject := fmt.Sprintf("%s security alert: email address changed", smtpCfg.FromName)
	htmlBody := fmt.Sprintf(`
		<p>Hello,</p>
		<p>This is a security notification to let you know that the email address
		associated with your account has been changed to <strong>%s</strong>.</p>
		<p>This email address (<strong>%s</strong>) is no longer used for authentication.</p>
		<p>If you did not make this change, please contact support immediately.</p>
		<p>Thanks,<br>%s team</p>
	`, newEmail, oldEmail, smtpCfg.FromName)

	err := sendEmail(ctx, smtpCfg, oldEmail, subject, htmlBody)
	if err != nil {
		return fmt.Errorf("failed to send email change alert: %w", err)
	}
	return nil
}
