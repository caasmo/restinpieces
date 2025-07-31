package handlers

import (
	"context"

	"github.com/caasmo/restinpieces/mail"
)

// mailerMock is a mock implementation of the mail.Mailer for testing purposes.
type mailerMock struct {
	SendEmailChangeNotificationFunc func(ctx context.Context, oldEmail, newEmail string, hasOauth2Login bool, callbackURL string) error
	SendPasswordResetEmailFunc      func(ctx context.Context, email, callbackURL string) error
	SendVerificationEmailFunc       func(ctx context.Context, email, callbackURL string) error
}

func (m *mailerMock) SendEmailChangeNotification(ctx context.Context, oldEmail, newEmail string, hasOauth2Login bool, callbackURL string) error {
	if m.SendEmailChangeNotificationFunc != nil {
		return m.SendEmailChangeNotificationFunc(ctx, oldEmail, newEmail, hasOauth2Login, callbackURL)
	}
	return nil
}

func (m *mailerMock) SendPasswordResetEmail(ctx context.Context, email, callbackURL string) error {
	if m.SendPasswordResetEmailFunc != nil {
		return m.SendPasswordResetEmailFunc(ctx, email, callbackURL)
	}
	return nil
}

func (m *mailerMock) SendVerificationEmail(ctx context.Context, email, callbackURL string) error {
	if m.SendVerificationEmailFunc != nil {
		return m.SendVerificationEmailFunc(ctx, email, callbackURL)
	}
	return nil
}

// mailerMock must implement the mail.MailerInterface
var _ mail.MailerInterface = (*mailerMock)(nil)
