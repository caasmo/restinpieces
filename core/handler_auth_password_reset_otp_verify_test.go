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

func TestVerifyPasswordResetOtpHandler(t *testing.T) {
	cfg := config.NewDefaultConfig()
	cfg.Jwt.PasswordResetSecret = "a_very_long_and_secure_secret_for_testing_purposes"
	provider := config.NewProvider(cfg)

	t.Run("success", func(t *testing.T) {
		email := "test@example.com"
		otp, token, _ := crypto.NewJwtEmailOtpVerificationToken(email, cfg.Jwt.PasswordResetSecret, time.Minute)

		mockDbAuth := &mock.Db{
			GetUserByEmailFunc: func(e string) (*db.User, error) {
				return &db.User{
					ID:       "user123",
					Email:    email,
					Verified: true,
					Password: "hashed-password",
				}, nil
			},
		}

		app := &App{
			configProvider: provider,
			validator:      &DefaultValidator{},
			dbAuth:         mockDbAuth,
		}

		reqBody := `{"otp":"` + otp + `", "verification_token":"` + token + `"}`
		req := httptest.NewRequest("POST", "/verify-password-reset-otp", strings.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		app.VerifyPasswordResetOtpHandler(rr, req)

		if rr.Code != http.StatusAccepted {
			t.Errorf("expected status 202, got %d", rr.Code)
		}

		var resp map[string]interface{}
		if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}
		if resp["code"] != CodeOkPasswordResetOtpVerified {
			t.Errorf("expected code %s, got %v", CodeOkPasswordResetOtpVerified, resp["code"])
		}

		data := resp["data"].(map[string]interface{})
		if data["token"] == "" {
			t.Error("expected grant token in response data")
		}
	})

	t.Run("invalid otp", func(t *testing.T) {
		email := "test@example.com"
		_, token, _ := crypto.NewJwtEmailOtpVerificationToken(email, cfg.Jwt.PasswordResetSecret, time.Minute)

		app := &App{
			configProvider: provider,
			validator:      &DefaultValidator{},
		}

		reqBody := `{"otp":"wrong", "verification_token":"` + token + `"}`
		req := httptest.NewRequest("POST", "/verify-password-reset-otp", strings.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		app.VerifyPasswordResetOtpHandler(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d", rr.Code)
		}
	})
}
