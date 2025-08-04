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

// Helper to generate email change tokens for testing.
func generateEmailChangeToken(t *testing.T, userID, currentEmail, newEmail, passwordHash, secret, purpose string, expiresIn time.Duration) string {
	t.Helper()

	signingKey, err := crypto.NewJwtSigningKeyWithCredentials(currentEmail, passwordHash, secret)
	if err != nil {
		t.Fatalf("failed to generate signing key: %v", err)
	}

	claims := map[string]any{
		crypto.ClaimUserID:   userID,
		crypto.ClaimEmail:    currentEmail,
		crypto.ClaimNewEmail: newEmail,
		crypto.ClaimType:     purpose, // Should be "email_change"
	}

	token, err := crypto.NewJwt(claims, signingKey, expiresIn)
	if err != nil {
		t.Fatalf("failed to generate test token: %v", err)
	}
	return token
}

// TestConfirmEmailChangeHandler_Validation tests basic request format checks.
func TestConfirmEmailChangeHandler_Validation(t *testing.T) {
	testCases := []struct {
		name        string
		contentType string
		requestBody string
		wantError   jsonResponse
	}{
		{
			name:        "invalid content type",
			contentType: "text/plain",
			requestBody: `{"token":"t", "password":"p"}`,
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
			requestBody: `{"password":"p"}`,
			wantError:   errorMissingFields,
		},
		{
			name:        "missing password field",
			contentType: "application/json",
			requestBody: `{"token":"t"}`,
			wantError:   errorMissingFields,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/confirm-email-change", strings.NewReader(tc.requestBody))
			req.Header.Set("Content-Type", tc.contentType)
			rr := httptest.NewRecorder()

			app := &App{validator: &DefaultValidator{}}

			app.ConfirmEmailChangeHandler(rr, req)

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

// TestConfirmEmailChangeHandler_UnverifiedToken tests the initial, unverified parsing of the JWT.
func TestConfirmEmailChangeHandler_UnverifiedToken(t *testing.T) {
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
			token: generateEmailChangeToken(
				t, "user123", "current@example.com", "new@e.com",
				"hashed_password", "a_valid_secret_that_is_long_enough", "wrong_purpose", time.Minute,
			),
			wantError: errorJwtInvalidVerificationToken,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			reqBody := `{"token":"` + tc.token + `", "password":"any_password"}`
			req := httptest.NewRequest("POST", "/confirm-email-change", strings.NewReader(reqBody))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()

			// No DB mock needed, as this should fail before the DB is called.
			app := &App{validator: &DefaultValidator{}}

			app.ConfirmEmailChangeHandler(rr, req)

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

// TestConfirmEmailChangeHandler_VerifiedToken tests the verified parsing of the JWT.
func TestConfirmEmailChangeHandler_VerifiedToken(t *testing.T) {
	testCases := []struct {
		name      string
		token     func(t *testing.T, cfg *config.Config, mockUser *db.User) string
		config    *config.Config
		password  string
		wantError jsonResponse
	}{
		{
			name: "expired token",
			token: func(t *testing.T, cfg *config.Config, mockUser *db.User) string {
				return generateEmailChangeToken(t, mockUser.ID, mockUser.Email, "new@e.com", mockUser.Password, cfg.Jwt.EmailChangeSecret, "email_change", -time.Minute)
			},
			config:    &config.Config{Jwt: config.Jwt{EmailChangeSecret: "a_valid_secret_that_is_long_enough"}},
			password:  "correct_password",
			wantError: errorJwtInvalidVerificationToken,
		},
		{
			name: "handler fails on signing key generation",
			token: func(t *testing.T, cfg *config.Config, mockUser *db.User) string {
				// This token is signed with a different secret than the one the handler will use for verification,
				// which will cause the final `ParseJwt` to fail. However, the critical part of this test
				// is to ensure that the handler's own key generation fails first.
				return generateEmailChangeToken(t, mockUser.ID, mockUser.Email, "new@e.com", mockUser.Password, "a_valid_secret_that_is_long_enough", "email_change", time.Minute)
			},
			config:    &config.Config{Jwt: config.Jwt{EmailChangeSecret: "short"}},
			password:  "correct_password",
			wantError: errorTokenGeneration,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			hashedPassword, err := crypto.GenerateHash("correct_password")
			if err != nil {
				t.Fatalf("failed to hash password: %v", err)
			}
			mockUser := &db.User{ID: "user123", Email: "current@example.com", Password: string(hashedPassword)}

			tokenString := tc.token(t, tc.config, mockUser)

			reqBody := `{"token":"` + tokenString + `", "password":"` + tc.password + `"}`
			req := httptest.NewRequest("POST", "/confirm-email-change", strings.NewReader(reqBody))
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

			app.ConfirmEmailChangeHandler(rr, req)

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

// TestConfirmEmailChangeHandler_UserLookup tests the user lookup logic.
func TestConfirmEmailChangeHandler_UserLookup(t *testing.T) {
	testCases := []struct {
		name      string
		dbSetup   func(m *mock.Db, u *db.User)
		wantError jsonResponse
	}{
		{
			name: "user not found",
			dbSetup: func(m *mock.Db, u *db.User) {
				m.GetUserByIdFunc = func(id string) (*db.User, error) { return nil, db.ErrUserNotFound }
			},
			wantError: errorNotFound,
		},
		{
			name: "generic db error on user lookup",
			dbSetup: func(m *mock.Db, u *db.User) {
				m.GetUserByIdFunc = func(id string) (*db.User, error) { return nil, errors.New("db down") }
			},
			wantError: errorNotFound,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &config.Config{Jwt: config.Jwt{EmailChangeSecret: "a_valid_secret_that_is_long_enough"}}
			hashedPassword, _ := crypto.GenerateHash("correct_password")
			mockUser := &db.User{ID: "user123", Email: "current@example.com", Password: string(hashedPassword)}

			token := generateEmailChangeToken(t, mockUser.ID, mockUser.Email, "new@e.com", mockUser.Password, cfg.Jwt.EmailChangeSecret, "email_change", time.Minute)
			reqBody := `{"token":"` + token + `", "password":"correct_password"}`
			req := httptest.NewRequest("POST", "/confirm-email-change", strings.NewReader(reqBody))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()

			mockDb := &mock.Db{}
			tc.dbSetup(mockDb, mockUser)

			app := &App{
				validator:      &DefaultValidator{},
				dbAuth:         mockDb,
				configProvider: config.NewProvider(cfg),
			}

			app.ConfirmEmailChangeHandler(rr, req)

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

// TestConfirmEmailChangeHandler_PasswordVerification tests the password verification logic.
func TestConfirmEmailChangeHandler_PasswordVerification(t *testing.T) {
	t.Run("incorrect password", func(t *testing.T) {
		cfg := &config.Config{Jwt: config.Jwt{EmailChangeSecret: "a_valid_secret_that_is_long_enough"}}
		hashedPassword, _ := crypto.GenerateHash("correct_password")
		mockUser := &db.User{ID: "user123", Email: "current@example.com", Password: string(hashedPassword)}

		token := generateEmailChangeToken(t, mockUser.ID, mockUser.Email, "new@e.com", mockUser.Password, cfg.Jwt.EmailChangeSecret, "email_change", time.Minute)
		reqBody := `{"token":"` + token + `", "password":"wrong_password"}`
		req := httptest.NewRequest("POST", "/confirm-email-change", strings.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		mockDb := &mock.Db{
			GetUserByIdFunc: func(id string) (*db.User, error) { return mockUser, nil },
		}

		app := &App{
			validator:      &DefaultValidator{},
			dbAuth:         mockDb,
			configProvider: config.NewProvider(cfg),
		}

		app.ConfirmEmailChangeHandler(rr, req)

		wantError := errorInvalidCredentials
		if rr.Code != wantError.status {
			t.Errorf("expected status %d, got %d", wantError.status, rr.Code)
		}

		var gotBody, wantBody map[string]interface{}
		if err := json.Unmarshal(rr.Body.Bytes(), &gotBody); err != nil {
			t.Fatalf("failed to decode response body: %v", err)
		}
		if err := json.Unmarshal(wantError.body, &wantBody); err != nil {
			t.Fatalf("failed to decode wantError body: %v", err)
		}

		if gotBody["code"] != wantBody["code"] {
			t.Errorf("expected error code %q, got %q", wantBody["code"], gotBody["code"])
		}
	})
}

// TestConfirmEmailChangeHandler_DB tests the final database update step.
func TestConfirmEmailChangeHandler_DB(t *testing.T) {
	testCases := []struct {
		name      string
		newEmail  string
		dbSetup   func(*mock.Db)
		wantError jsonResponse
	}{
		{
			name:     "successful email update",
			newEmail: "new.valid@example.com",
			dbSetup: func(m *mock.Db) {
				m.UpdateEmailFunc = func(id, email string) error { return nil }
			},
			wantError: okEmailChange,
		},
		{
			name:     "db error on update",
			newEmail: "new.valid@example.com",
			dbSetup: func(m *mock.Db) {
				m.UpdateEmailFunc = func(id, email string) error { return errors.New("update failed") }
			},
			wantError: errorServiceUnavailable,
		},
		{
			name:      "invalid new_email format in claims",
			newEmail:  "invalid-email-in-token",
			dbSetup:   func(m *mock.Db) {},
			wantError: errorInvalidRequest,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &config.Config{Jwt: config.Jwt{EmailChangeSecret: "a_valid_secret_that_is_long_enough"}}
			hashedPassword, _ := crypto.GenerateHash("correct_password")
			mockUser := &db.User{ID: "user123", Email: "current@example.com", Password: string(hashedPassword)}

			token := generateEmailChangeToken(t, mockUser.ID, mockUser.Email, tc.newEmail, mockUser.Password, cfg.Jwt.EmailChangeSecret, "email_change", time.Minute)
			reqBody := `{"token":"` + token + `", "password":"correct_password"}`
			req := httptest.NewRequest("POST", "/confirm-email-change", strings.NewReader(reqBody))
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

			app.ConfirmEmailChangeHandler(rr, req)

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
