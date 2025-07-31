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

func TestPasswordResetHandler_Handle(t *testing.T) {
	cfg := config.NewDefaultConfig()
	provider := config.NewProvider(cfg)

	t.Run("success", func(t *testing.T) {
		var mailerCalled bool
		var capturedURL string

		mockDb := &mock.Db{
			GetUserByIdFunc: func(id string) (*db.User, error) {
				return &db.User{ID: id, Email: "test@example.com", Password: "hashed-pw"}, nil
			},
		}

		mockMailer := &mailerMock{
			SendPasswordResetEmailFunc: func(ctx context.Context, email, callbackURL string) error {
				mailerCalled = true
				capturedURL = callbackURL
				return nil
			},
		}

		handler := NewPasswordResetHandler(mockDb, provider, mockMailer)

		payload := PayloadPasswordReset{UserID: "user-123"}
		payloadBytes, _ := json.Marshal(payload)
		payloadExtra := PayloadPasswordResetExtra{Email: "test@example.com"}
		payloadExtraBytes, _ := json.Marshal(payloadExtra)
		job := db.Job{Payload: payloadBytes, PayloadExtra: payloadExtraBytes}

		err := handler.Handle(context.Background(), job)

		if err != nil {
			t.Fatalf("Handle() error = %v, want nil", err)
		}

		if !mailerCalled {
			t.Fatal("SendPasswordResetEmail should have been called, but it was not")
		}

		// Check that the callback URL contains a valid JWT
		tokenStr := strings.Split(capturedURL, "?token=")[1]
		key, err := crypto.NewJwtSigningKeyWithCredentials("test@example.com", "hashed-pw", cfg.Jwt.PasswordResetSecret)
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

	t.Run("user not found", func(t *testing.T) {
		var mailerCalled bool
		mockDb := &mock.Db{
			GetUserByIdFunc: func(id string) (*db.User, error) {
				return nil, nil // Return nil user, nil error to simulate user not found gracefully
			},
		}

		mockMailer := &mailerMock{
			SendPasswordResetEmailFunc: func(ctx context.Context, email, callbackURL string) error {
				mailerCalled = true
				return nil
			},
		}

		handler := NewPasswordResetHandler(mockDb, provider, mockMailer)

		payload := PayloadPasswordReset{UserID: "not-found-user"}
		payloadBytes, _ := json.Marshal(payload)
		payloadExtra := PayloadPasswordResetExtra{Email: "test@example.com"}
		payloadExtraBytes, _ := json.Marshal(payloadExtra)
		job := db.Job{Payload: payloadBytes, PayloadExtra: payloadExtraBytes}

		err := handler.Handle(context.Background(), job)

		if err != nil {
			t.Fatalf("Handle() error = %v, want nil for non-existent user", err)
		}

		if mailerCalled {
			t.Error("SendPasswordResetEmail should not be called when user is not found")
		}
	})
}
