package core

import (
	"encoding/json"
	"errors"
	
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/caasmo/restinpieces/config"
	"github.com/caasmo/restinpieces/crypto"
	"github.com/caasmo/restinpieces/db"
	"github.com/caasmo/restinpieces/db/mock"
)

// Helper to generate password reset tokens for testing.
func generatePasswordResetToken(t *testing.T, userID, email, passwordHash, secret, purpose string, expiresIn time.Duration) string {
	t.Helper()

	signingKey, err := crypto.NewJwtSigningKeyWithCredentials(email, passwordHash, secret)
	if err != nil {
		t.Fatalf("failed to generate signing key: %v", err)
	}

	claims := map[string]any{
		crypto.ClaimUserID: userID,
		crypto.ClaimEmail:  email,
		crypto.ClaimType:   purpose, // Should be "password_reset"
	}

	token, err := crypto.NewJwt(claims, signingKey, expiresIn)
	if err != nil {
		t.Fatalf("failed to generate test token: %v", err)
	}
	return token
}

// TestConfirmPasswordResetHandler_Validation tests basic request format checks.
func TestConfirmPasswordResetHandler_Validation(t *testing.T) {
	testCases := []struct {
		name        string
		contentType string
		requestBody string
		wantError   jsonResponse
	}{
		{
			name:        "invalid content type",
			contentType: "text/plain",
			requestBody: `{"token":"t", "password":"p", "password_confirm":"p"}`,
			wantError:   errorInvalidContentType,
		},
		{
			name:        "malformed json",
			contentType: "application/json",
			requestBody: `{"token":`,
			wantError:   errorInvalidRequest,
		},
		{
			name:        "missing token field",
			contentType: "application/json",
			requestBody: `{"password":"p", "password_confirm":"p"}`,
			wantError:   errorMissingFields,
		},
		{
			name:        "missing password field",
			contentType: "application/json",
			requestBody: `{"token":"t", "password_confirm":"p"}`,
			wantError:   errorMissingFields,
		},
		{
			name:        "missing password_confirm field",
			contentType: "application/json",
			requestBody: `{"token":"t", "password":"p"}`,
			wantError:   errorMissingFields,
		},
		{
			name:        "password mismatch",
			contentType: "application/json",
			requestBody: `{"token":"t", "password":"p1", "password_confirm":"p2"}`,
			wantError:   errorPasswordMismatch,
		},
		{
			name:        "password complexity",
			contentType: "application/json",
			requestBody: `{"token":"t", "password":"short", "password_confirm":"short"}`,
			wantError:   errorPasswordComplexity,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/confirm-password-reset", strings.NewReader(tc.requestBody))
			req.Header.Set("Content-Type", tc.contentType)
			rr := httptest.NewRecorder()

			app := &App{validator: &DefaultValidator{}}

			app.ConfirmPasswordResetHandler(rr, req)

			if rr.Code != tc.wantError.status {
				t.Errorf("expected status %d, got %d", tc.wantError.status, rr.Code)
			}

			var gotBody, wantBody map[string]interface{}
			if err := json.Unmarshal(rr.Body.Bytes(), &gotBody); err != nil {
				t.Fatalf("failed to decode response body: %v", err)
			}
			if err := json.Unmarshal(tc.wantError.body, &wantBody); err != nil {
				t.Fatalf("failed to decode wantError body: %v", err)
			}

			if gotBody["code"] != wantBody["code"] {
				t.Errorf("expected error code %q, got %q", wantBody["code"], gotBody["code"])
			}
		})
	}
}

// TestConfirmPasswordResetHandler_UnverifiedToken tests the initial, unverified parsing of the JWT.
func TestConfirmPasswordResetHandler_UnverifiedToken(t *testing.T) {
	testCases := []struct {
		name      string
		token     string
		wantError jsonResponse
	}{
		{
			name:      "malformed jwt",
			token:     "invalid.token.string",
			wantError: errorJwtInvalidVerificationToken,
		},
		{
			name: "token with wrong purpose",
			token: generatePasswordResetToken(
				t, "user123", "current@example.com",
				"hashed_password", "a_valid_secret_that_is_long_enough", "wrong_purpose", time.Minute,
			),
			wantError: errorJwtInvalidVerificationToken,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			reqBody := `{"token":"` + tc.token + `", "password":"a_valid_password", "password_confirm":"a_valid_password"}`
			req := httptest.NewRequest("POST", "/confirm-password-reset", strings.NewReader(reqBody))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()

			app := &App{validator: &DefaultValidator{}}

			app.ConfirmPasswordResetHandler(rr, req)

			if rr.Code != tc.wantError.status {
				t.Errorf("expected status %d, got %d", tc.wantError.status, rr.Code)
			}
		})
	}
}

// TestConfirmPasswordResetHandler_UserLookup tests the user lookup logic.
func TestConfirmPasswordResetHandler_UserLookup(t *testing.T) {
	cfg := &config.Config{Jwt: config.Jwt{PasswordResetSecret: "a_valid_secret_that_is_long_enough"}}
	hashedPassword, _ := crypto.GenerateHash("old_password")
	mockUser := &db.User{ID: "user123", Email: "current@example.com", Password: string(hashedPassword)}
	validToken := generatePasswordResetToken(t, mockUser.ID, mockUser.Email, mockUser.Password, cfg.Jwt.PasswordResetSecret, "password_reset", time.Minute)

	testCases := []struct {
		name      string
		dbSetup   func(m *mock.Db)
		wantError jsonResponse
	}{
		{
			name: "user not found",
			dbSetup: func(m *mock.Db) {
				m.GetUserByIdFunc = func(id string) (*db.User, error) { return nil, db.ErrUserNotFound }
			},
			wantError: errorNotFound,
		},
		{
			name: "generic db error on user lookup",
			dbSetup: func(m *mock.Db) {
				m.GetUserByIdFunc = func(id string) (*db.User, error) { return nil, errors.New("db down") }
			},
			wantError: errorNotFound,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			reqBody := `{"token":"` + validToken + `", "password":"new_password_123", "password_confirm":"new_password_123"}`
			req := httptest.NewRequest("POST", "/confirm-password-reset", strings.NewReader(reqBody))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()

			mockDb := &mock.Db{}
			tc.dbSetup(mockDb)

			app := &App{
				validator:      &DefaultValidator{},
				dbAuth:         mockDb,
				configProvider: config.NewProvider(cfg),
			}

			app.ConfirmPasswordResetHandler(rr, req)

			if rr.Code != tc.wantError.status {
				t.Errorf("expected status %d, got %d", tc.wantError.status, rr.Code)
			}
		})
	}
}

// TestConfirmPasswordResetHandler_VerifiedToken tests the verified parsing of the JWT.
func TestConfirmPasswordResetHandler_VerifiedToken(t *testing.T) {
	hashedPassword, _ := crypto.GenerateHash("old_password")
	mockUser := &db.User{ID: "user123", Email: "current@example.com", Password: string(hashedPassword)}

	testCases := []struct {
		name      string
		token     func(t *testing.T, cfg *config.Config) string
		config    *config.Config
		wantError jsonResponse
	}{
		{
			name: "expired token",
			token: func(t *testing.T, cfg *config.Config) string {
				return generatePasswordResetToken(t, mockUser.ID, mockUser.Email, mockUser.Password, cfg.Jwt.PasswordResetSecret, "password_reset", -time.Minute)
			},
			config:    &config.Config{Jwt: config.Jwt{PasswordResetSecret: "a_valid_secret_that_is_long_enough"}},
			wantError: errorJwtInvalidVerificationToken,
		},
		{
			name: "token with wrong signature",
			token: func(t *testing.T, cfg *config.Config) string {
				return generatePasswordResetToken(t, mockUser.ID, mockUser.Email, mockUser.Password, "a_different_but_valid_secret_long_enough", "password_reset", time.Minute)
			},
			config:    &config.Config{Jwt: config.Jwt{PasswordResetSecret: "a_valid_secret_that_is_long_enough"}},
			wantError: errorJwtInvalidVerificationToken,
		},
		{
			name: "handler fails on signing key generation",
			token: func(t *testing.T, cfg *config.Config) string {
				return generatePasswordResetToken(t, mockUser.ID, mockUser.Email, mockUser.Password, "a_valid_secret_that_is_long_enough", "password_reset", time.Minute)
			},
			config:    &config.Config{Jwt: config.Jwt{PasswordResetSecret: "short"}},
			wantError: errorPasswordResetFailed,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tokenString := tc.token(t, tc.config)
			reqBody := `{"token":"` + tokenString + `", "password":"new_password_123", "password_confirm":"new_password_123"}`
			req := httptest.NewRequest("POST", "/confirm-password-reset", strings.NewReader(reqBody))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()

			mockDb := &mock.Db{
				GetUserByIdFunc: func(id string) (*db.User, error) { return mockUser, nil },
			}

			app := &App{
				validator:      &DefaultValidator{},
				dbAuth:         mockDb,
				configProvider: config.NewProvider(tc.config),
			}

			app.ConfirmPasswordResetHandler(rr, req)

			if rr.Code != tc.wantError.status {
				t.Errorf("expected status %d, got %d", tc.wantError.status, rr.Code)
			}
		})
	}
}

// TestConfirmPasswordResetHandler_DB tests the final database update step.
func TestConfirmPasswordResetHandler_DB(t *testing.T) {
	cfg := &config.Config{Jwt: config.Jwt{PasswordResetSecret: "a_valid_secret_that_is_long_enough"}}
	oldHashedPassword, _ := crypto.GenerateHash("old_password")
	mockUser := &db.User{ID: "user123", Email: "current@example.com", Password: string(oldHashedPassword)}
	validToken := generatePasswordResetToken(t, mockUser.ID, mockUser.Email, mockUser.Password, cfg.Jwt.PasswordResetSecret, "password_reset", time.Minute)

	testCases := []struct {
		name        string
		password    string
		dbSetup     func(*mock.Db)
		wantError   jsonResponse
	}{
		{
			name:     "successful password update",
			password: "new_valid_password",
			dbSetup: func(m *mock.Db) {
				m.UpdatePasswordFunc = func(id, hash string) error {
					if id != mockUser.ID {
						t.Errorf("UpdatePassword called with wrong user ID")
					}
					if !crypto.CheckPassword("new_valid_password", hash) {
						t.Errorf("UpdatePassword called with incorrect hash")
					}
					return nil
				}
			},
			wantError: okPasswordReset,
		},
		{
			name:        "new password is same as old",
			password:    "old_password",
			dbSetup:     func(m *mock.Db) {},
			wantError:   okPasswordResetNotNeeded,
		},
		{
			name:     "db error on update",
			password: "new_valid_password",
			dbSetup: func(m *mock.Db) {
				m.UpdatePasswordFunc = func(id, hash string) error { return errors.New("update failed") }
			},
			wantError: errorServiceUnavailable,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			reqBody := `{"token":"` + validToken + `", "password":"` + tc.password + `", "password_confirm":"` + tc.password + `"}`
			req := httptest.NewRequest("POST", "/confirm-password-reset", strings.NewReader(reqBody))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()

			mockDb := &mock.Db{
				GetUserByIdFunc: func(id string) (*db.User, error) { return mockUser, nil },
			}
			tc.dbSetup(mockDb)

			app := &App{
				validator:      &DefaultValidator{},
				dbAuth:         mockDb,
				configProvider: config.NewProvider(cfg),
			}

			app.ConfirmPasswordResetHandler(rr, req)

			if rr.Code != tc.wantError.status {
				t.Errorf("expected status %d, got %d", tc.wantError.status, rr.Code)
			}

			var gotBody, wantBody map[string]interface{}
			if err := json.Unmarshal(rr.Body.Bytes(), &gotBody); err != nil {
				t.Fatalf("failed to decode response body: %v", err)
			}
			if err := json.Unmarshal(tc.wantError.body, &wantBody); err != nil {
				t.Fatalf("failed to decode wantError body: %v", err)
			}

			if gotBody["code"] != wantBody["code"] {
				t.Errorf("expected error code %q, got %q", wantBody["code"], gotBody["code"])
			}
		})
	}
}
