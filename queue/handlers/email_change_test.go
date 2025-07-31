package handlers

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/caasmo/restinpieces/config"
	"github.com/caasmo/restinpieces/crypto"
	"github.com/caasmo/restinpieces/db"
	"github.com/caasmo/restinpieces/db/mock"
	"github.com/golang-jwt/jwt/v5"
)

func TestEmailChangeHandler_Handle(t *testing.T) {
	cfg := config.NewDefaultConfig()
	provider := config.NewProvider(cfg)

	t.Run("success with oauth2 login", func(t *testing.T) {
		var mailerCalled bool
		var capturedURL string
		var capturedHasOauth2Login bool

		mockDb := &mock.Db{
			GetUserByIdFunc: func(id string) (*db.User, error) {
				return &db.User{ID: id, Email: "old@example.com", Password: "hashed-pw", Oauth2: true}, nil
			},
		}

		mockMailer := &mailerMock{
			SendEmailChangeNotificationFunc: func(ctx context.Context, oldEmail, newEmail string, hasOauth2Login bool, callbackURL string) error {
				mailerCalled = true
				capturedURL = callbackURL
				capturedHasOauth2Login = hasOauth2Login
				return nil
			},
		}

		handler := NewEmailChangeHandler(mockDb, provider, mockMailer)

		payload := PayloadEmailChange{UserID: "user-123"}
		payloadBytes, _ := json.Marshal(payload)
		payloadExtra := PayloadEmailChangeExtra{NewEmail: "new@example.com"}
		payloadExtraBytes, _ := json.Marshal(payloadExtra)
		job := db.Job{Payload: payloadBytes, PayloadExtra: payloadExtraBytes}

		err := handler.Handle(context.Background(), job)

		if err != nil {
			t.Fatalf("Handle() error = %v, want nil", err)
		}

		if !mailerCalled {
			t.Fatal("SendEmailChangeNotification should have been called, but it was not")
		}

		if !capturedHasOauth2Login {
			t.Error("captured has_oauth2_login should be true, but it was false")
		}

		// Check that the callback URL contains a valid JWT
		tokenStr := strings.Split(capturedURL, "?token=")[1]
		key, err := crypto.NewJwtSigningKeyWithCredentials("old@example.com", "hashed-pw", cfg.Jwt.EmailChangeSecret)
		if err != nil {
			t.Fatalf("Failed to create signing key: %v", err)
		}
		token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
			return key, nil
		})

		if err != nil {
			t.Fatalf("Failed to parse JWT: %v", err)
		}

		if !token.Valid {
			t.Error("JWT token is not valid")
		}
	})

	t.Run("success without oauth2 login", func(t *testing.T) {
		var mailerCalled bool
		var capturedHasOauth2Login bool

		mockDb := &mock.Db{
			GetUserByIdFunc: func(id string) (*db.User, error) {
				return &db.User{ID: id, Email: "old@example.com", Password: "hashed-pw", Oauth2: false}, nil
			},
		}

		mockMailer := &mailerMock{
			SendEmailChangeNotificationFunc: func(ctx context.Context, oldEmail, newEmail string, hasOauth2Login bool, callbackURL string) error {
				mailerCalled = true
				capturedHasOauth2Login = hasOauth2Login
				return nil
			},
		}

		handler := NewEmailChangeHandler(mockDb, provider, mockMailer)

		payload := PayloadEmailChange{UserID: "user-456"}
		payloadBytes, _ := json.Marshal(payload)
		payloadExtra := PayloadEmailChangeExtra{NewEmail: "new@example.com"}
		payloadExtraBytes, _ := json.Marshal(payloadExtra)
		job := db.Job{Payload: payloadBytes, PayloadExtra: payloadExtraBytes}

		err := handler.Handle(context.Background(), job)

		if err != nil {
			t.Fatalf("Handle() error = %v, want nil", err)
		}

		if !mailerCalled {
			t.Fatal("SendEmailChangeNotification should have been called, but it was not")
		}

		if capturedHasOauth2Login {
			t.Error("captured has_oauth2_login should be false, but it was true")
		}
	})
}
