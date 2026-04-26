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
)

func TestConfirmPasswordResetOtpHandler(t *testing.T) {
	cfg := config.NewDefaultConfig()
	cfg.Jwt.PasswordResetSecret = "a_very_long_and_secure_secret_for_testing_purposes"
	cfg.Jwt.AuthSecret = "another_very_long_and_secure_secret_for_testing_purposes"
	provider := config.NewProvider(cfg)

	t.Run("success", func(t *testing.T) {
		email := "test@example.com"
		userID := "user123"
		passwordHash, _ := crypto.GenerateHash("old-password")
		
		token, _ := crypto.NewJwtPasswordResetToken(
			userID,
			email,
			string(passwordHash),
			cfg.Jwt.PasswordResetSecret,
			time.Minute,
		)

		mockDbAuth := &mock.Db{
			GetUserByIdFunc: func(id string) (*db.User, error) {
				return &db.User{
					ID:       userID,
					Email:    email,
					Verified: true,
					Password: string(passwordHash),
				}, nil
			},
			UpdatePasswordFunc: func(id, hash string) error {
				return nil
			},
		}

		app := &App{
			configProvider: provider,
			validator:      &DefaultValidator{},
			dbAuth:         mockDbAuth,
		}

		reqBody := `{"token":"` + token + `", "password":"NewPassword123!", "password_confirm":"NewPassword123!"}`
		req := httptest.NewRequest("POST", "/confirm-password-reset-otp", strings.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		app.ConfirmPasswordResetOtpHandler(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d. Body: %s", rr.Code, rr.Body.String())
		}

		var resp map[string]interface{}
		if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}
		if resp["code"] != CodeOkAuthentication {
			t.Errorf("expected code %s, got %v", CodeOkAuthentication, resp["code"])
		}
	})

	t.Run("password mismatch", func(t *testing.T) {
		app := &App{
			validator: &DefaultValidator{},
		}

		reqBody := `{"token":"some-token", "password":"p1", "password_confirm":"p2"}`
		req := httptest.NewRequest("POST", "/confirm-password-reset-otp", strings.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		app.ConfirmPasswordResetOtpHandler(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", rr.Code)
		}
	})
}
