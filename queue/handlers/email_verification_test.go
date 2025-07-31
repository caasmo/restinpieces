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

func TestEmailVerificationHandler_Handle(t *testing.T) {
	cfg := config.NewDefaultConfig()
	provider := config.NewProvider(cfg)

	t.Run("success", func(t *testing.T) {
		var mailerCalled bool
		var capturedURL string

		mockDb := &mock.Db{
			GetUserByEmailFunc: func(email string) (*db.User, error) {
				return &db.User{ID: "user-123", Email: email, Password: "hashed-pw", Verified: false}, nil
			},
		}

		mockMailer := &mailerMock{
			SendVerificationEmailFunc: func(ctx context.Context, email, callbackURL string) error {
				mailerCalled = true
				capturedURL = callbackURL
				return nil
			},
		}

		handler := NewEmailVerificationHandler(mockDb, provider, mockMailer)

		payload := PayloadEmailVerification{Email: "test@example.com"}
		payloadBytes, _ := json.Marshal(payload)
		job := db.Job{Payload: payloadBytes}

		err := handler.Handle(context.Background(), job)

		if err != nil {
			t.Fatalf("Handle() error = %v, want nil", err)
		}

		if !mailerCalled {
			t.Fatal("SendVerificationEmail should have been called, but it was not")
		}

		// Check that the callback URL contains a valid JWT
		tokenStr := strings.Split(capturedURL, "?token=")[1]
		key, err := crypto.NewJwtSigningKeyWithCredentials("test@example.com", "hashed-pw", cfg.Jwt.VerificationEmailSecret)
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
		mockDb := &mock.Db{
			GetUserByEmailFunc: func(email string) (*db.User, error) {
				return nil, db.ErrUserNotFound
			},
		}

		mockMailer := &mailerMock{}
		handler := NewEmailVerificationHandler(mockDb, provider, mockMailer)

		payload := PayloadEmailVerification{Email: "not-found@example.com"}
		payloadBytes, _ := json.Marshal(payload)
		job := db.Job{Payload: payloadBytes}

		err := handler.Handle(context.Background(), job)

		if err == nil {
			t.Fatal("Handle() should have returned an error, but it did not")
		}
		if !strings.Contains(err.Error(), db.ErrUserNotFound.Error()) {
			t.Errorf("error message = %s, want it to contain '%s'", err.Error(), db.ErrUserNotFound.Error())
		}
	})

	t.Run("user already verified", func(t *testing.T) {
		var mailerCalled bool
		mockDb := &mock.Db{
			GetUserByEmailFunc: func(email string) (*db.User, error) {
				return &db.User{ID: "user-456", Email: email, Verified: true}, nil
			},
		}

		mockMailer := &mailerMock{
			SendVerificationEmailFunc: func(ctx context.Context, email, callbackURL string) error {
				mailerCalled = true
				return nil
			},
		}

		handler := NewEmailVerificationHandler(mockDb, provider, mockMailer)

		payload := PayloadEmailVerification{Email: "verified@example.com"}
		payloadBytes, _ := json.Marshal(payload)
		job := db.Job{Payload: payloadBytes}

		err := handler.Handle(context.Background(), job)

		if err != nil {
			t.Fatalf("Handle() error = %v, want nil", err)
		}

		if mailerCalled {
			t.Error("SendVerificationEmail should not have been called for an already verified user")
		}
	})
}
