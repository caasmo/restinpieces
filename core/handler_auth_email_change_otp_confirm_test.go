package core

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/caasmo/restinpieces/config"
	"github.com/caasmo/restinpieces/crypto"
	"github.com/caasmo/restinpieces/db"
	"github.com/caasmo/restinpieces/db/mock"
	"github.com/caasmo/restinpieces/queue/handlers"
)

func TestConfirmEmailChangeOtpHandler(t *testing.T) {
	cfg := config.NewDefaultConfig()
	cfg.Jwt.EmailChangeOtpSecret = "a_very_long_and_secure_secret_for_testing_purposes"
	provider := config.NewProvider(cfg)

	t.Run("success", func(t *testing.T) {
		newEmail := "new@example.com"
		oldEmail := "old@example.com"
		userID := "user123"

		otp, token, _ := crypto.NewJwtEmailOtpToken(newEmail, cfg.Jwt.EmailChangeOtpSecret, time.Minute)

		mockDbAuth := &mock.Db{
			UpdateEmailFunc: func(id, email string) error {
				if id != userID || email != newEmail {
					t.Errorf("unexpected arguments to UpdateEmail: got id=%s, email=%s", id, email)
				}
				return nil
			},
		}

		var alertJobInserted bool
		mockDbQueue := &mock.Db{
			InsertJobFunc: func(job db.Job) error {
				if job.JobType == handlers.JobTypeEmailChangeAlert {
					alertJobInserted = true
				}
				return nil
			},
		}

		mockAuth := &MockAuth{
			AuthenticateFunc: func(r *http.Request) (*db.User, jsonResponse, error) {
				return &db.User{
					ID:       userID,
					Email:    oldEmail,
					Verified: true,
				}, jsonResponse{}, nil
			},
		}

		app := &App{
			configProvider: provider,
			validator:      &DefaultValidator{},
			dbAuth:         mockDbAuth,
			dbQueue:        mockDbQueue,
			authenticator:  mockAuth,
		}

		reqBody := `{"otp":"` + otp + `", "verification_token":"` + token + `"}`
		req := httptest.NewRequest("POST", "/api/confirm-email-change-otp", strings.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		app.ConfirmEmailChangeOtpHandler(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d. Body: %s", rr.Code, rr.Body.String())
		}

		var resp map[string]interface{}
		if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}
		if resp["code"] != CodeOkEmailChange {
			t.Errorf("expected code %s, got %v", CodeOkEmailChange, resp["code"])
		}

		if !alertJobInserted {
			t.Error("expected email change alert job to be inserted")
		}
	})

	t.Run("invalid otp", func(t *testing.T) {
		newEmail := "new@example.com"
		_, token, _ := crypto.NewJwtEmailOtpToken(newEmail, cfg.Jwt.EmailChangeOtpSecret, time.Minute)

		mockAuth := &MockAuth{
			AuthenticateFunc: func(r *http.Request) (*db.User, jsonResponse, error) {
				return &db.User{ID: "user123", Email: "old@example.com", Verified: true}, jsonResponse{}, nil
			},
		}

		app := &App{
			configProvider: provider,
			validator:      &DefaultValidator{},
			authenticator:  mockAuth,
		}

		reqBody := `{"otp":"wrong", "verification_token":"` + token + `"}`
		req := httptest.NewRequest("POST", "/api/confirm-email-change-otp", strings.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		app.ConfirmEmailChangeOtpHandler(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d", rr.Code)
		}
	})

	t.Run("invalid token", func(t *testing.T) {
		mockAuth := &MockAuth{
			AuthenticateFunc: func(r *http.Request) (*db.User, jsonResponse, error) {
				return &db.User{ID: "user123", Email: "old@example.com", Verified: true}, jsonResponse{}, nil
			},
		}

		app := &App{
			configProvider: provider,
			validator:      &DefaultValidator{},
			authenticator:  mockAuth,
		}

		reqBody := `{"otp":"123456", "verification_token":"invalid-token"}`
		req := httptest.NewRequest("POST", "/api/confirm-email-change-otp", strings.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		app.ConfirmEmailChangeOtpHandler(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d", rr.Code)
		}
	})
}
