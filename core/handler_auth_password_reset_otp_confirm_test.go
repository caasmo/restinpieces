package core

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/caasmo/restinpieces/config"
	"github.com/caasmo/restinpieces/crypto"
	"github.com/caasmo/restinpieces/db"
	"github.com/caasmo/restinpieces/db/mock"
	"github.com/golang-jwt/jwt/v5"
)

func TestConfirmPasswordResetOtpHandler_Validation(t *testing.T) {
	testCases := []struct {
		name           string
		contentType    string
		requestBody    string
		wantError      jsonResponse
		setupValidator func(*MockValidator)
	}{
		{
			name:        "invalid content type",
			contentType: "text/plain",
			requestBody: `{"token":"t","password":"p","password_confirm":"p"}`,
			wantError:   errorInvalidContentType,
			setupValidator: func(m *MockValidator) {
				m.ContentTypeFunc = func(r *http.Request, allowedType string) (jsonResponse, error) {
					return errorInvalidContentType, errors.New("invalid content type")
				}
			},
		},
		{
			name:        "malformed json",
			contentType: "application/json",
			requestBody: `{"token":"t",`,
			wantError:   errorInvalidRequest,
			setupValidator: func(m *MockValidator) {
				m.ContentTypeFunc = func(r *http.Request, allowedType string) (jsonResponse, error) {
					return jsonResponse{}, nil
				}
			},
		},
		{
			name:        "missing fields",
			contentType: "application/json",
			requestBody: `{"token":"t"}`,
			wantError:   errorMissingFields,
			setupValidator: func(m *MockValidator) {
				m.ContentTypeFunc = func(r *http.Request, allowedType string) (jsonResponse, error) {
					return jsonResponse{}, nil
				}
			},
		},
		{
			name:        "password mismatch",
			contentType: "application/json",
			requestBody: `{"token":"t","password":"p1","password_confirm":"p2"}`,
			wantError:   errorPasswordMismatch,
			setupValidator: func(m *MockValidator) {
				m.ContentTypeFunc = func(r *http.Request, allowedType string) (jsonResponse, error) {
					return jsonResponse{}, nil
				}
			},
		},
		{
			name:        "weak password",
			contentType: "application/json",
			requestBody: `{"token":"t","password":"p","password_confirm":"p"}`,
			wantError:   errorWeakPassword,
			setupValidator: func(m *MockValidator) {
				m.ContentTypeFunc = func(r *http.Request, allowedType string) (jsonResponse, error) {
					return jsonResponse{}, nil
				}
				m.PasswordFunc = func(p string) error {
					return errors.New("weak")
				}
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/confirm-password-reset-otp", strings.NewReader(tc.requestBody))
			req.Header.Set("Content-Type", tc.contentType)
			rr := httptest.NewRecorder()

			mockValidator := &MockValidator{}
			tc.setupValidator(mockValidator)

			app := &App{
				validator: mockValidator,
				dbAuth:    &mock.Db{},
			}

			app.ConfirmPasswordResetOtpHandler(rr, req)

			if rr.Code != tc.wantError.status {
				t.Errorf("expected status %d, got %d", tc.wantError.status, rr.Code)
			}
		})
	}
}

func TestConfirmPasswordResetOtpHandler_Logic(t *testing.T) {
	secret := "test_secret_32_bytes_long_xxxxxx"
	email := "test@example.com"
	userID := "user123"
	oldPassword := "old-password"
	oldPasswordHash, _ := crypto.GenerateHash(oldPassword)

	token, _ := crypto.NewJwtPasswordResetToken(userID, email, string(oldPasswordHash), secret, time.Minute)

	// Token with missing claims
	t2 := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"email": email})
	tokenNoUserID, _ := t2.SignedString([]byte(secret))

	t3 := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"user_id": userID})
	tokenNoEmail, _ := t3.SignedString([]byte(secret))

	testConfig := &config.Config{
		Jwt: config.Jwt{
			PasswordResetSecret: secret,
		},
	}

	testCases := []struct {
		name       string
		token      string
		password   string
		dbSetup    func(*mock.Db)
		wantStatus int
		wantCode   string
	}{
		{
			name:     "success",
			token:    token,
			password: "NewPassword123!",
			dbSetup: func(m *mock.Db) {
				m.GetUserByIdFunc = func(id string) (*db.User, error) {
					return &db.User{ID: userID, Email: email, Password: string(oldPasswordHash)}, nil
				}
				m.UpdatePasswordFunc = func(id, hash string) error {
					return nil
				}
			},
			wantStatus: http.StatusOK,
			wantCode:   CodeOkPasswordReset,
		},
		{
			name:     "invalid token format",
			token:    "invalid",
			password: "NewPassword123!",
			dbSetup:  func(m *mock.Db) {},
			wantStatus: http.StatusUnauthorized,
			wantCode:   CodeErrorJwtInvalidVerificationToken,
		},
		{
			name:     "token user not found (silent timing equalization)",
			token:    token,
			password: "NewPassword123!",
			dbSetup: func(m *mock.Db) {
				m.GetUserByIdFunc = func(id string) (*db.User, error) {
					return nil, nil
				}
			},
			wantStatus: http.StatusUnauthorized,
			wantCode:   CodeErrorJwtInvalidVerificationToken,
		},
		{
			name:     "invalid token claims (missing user id)",
			token:    tokenNoUserID,
			password: "NewPassword123!",
			dbSetup:  func(m *mock.Db) {},
			wantStatus: http.StatusUnauthorized,
			wantCode:   CodeErrorJwtInvalidVerificationToken,
		},
		{
			name:     "invalid token claims (missing email)",
			token:    tokenNoEmail,
			password: "NewPassword123!",
			dbSetup:  func(m *mock.Db) {},
			wantStatus: http.StatusUnauthorized,
			wantCode:   CodeErrorJwtInvalidVerificationToken,
		},
		{
			name:     "invalid token signature (password changed)",
			token:    token,
			password: "NewPassword123!",
			dbSetup: func(m *mock.Db) {
				m.GetUserByIdFunc = func(id string) (*db.User, error) {
					return &db.User{ID: userID, Email: email, Password: "different-hash"}, nil
				}
			},
			wantStatus: http.StatusUnauthorized,
			wantCode:   CodeErrorJwtInvalidVerificationToken,
		},
		{
			name:     "password reset not needed (same as old)",
			token:    token,
			password: oldPassword,
			dbSetup: func(m *mock.Db) {
				m.GetUserByIdFunc = func(id string) (*db.User, error) {
					return &db.User{ID: userID, Email: email, Password: string(oldPasswordHash)}, nil
				}
			},
			wantStatus: http.StatusOK,
			wantCode:   CodeOkPasswordResetNotNeeded,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			reqBody := `{"token":"` + tc.token + `", "password":"` + tc.password + `", "password_confirm":"` + tc.password + `"}`
			req := httptest.NewRequest("POST", "/confirm-password-reset-otp", strings.NewReader(reqBody))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()

			mockDb := &mock.Db{}
			tc.dbSetup(mockDb)

			app := &App{
				validator:      &DefaultValidator{},
				dbAuth:         mockDb,
				configProvider: config.NewProvider(testConfig),
			}

			app.ConfirmPasswordResetOtpHandler(rr, req)

			if rr.Code != tc.wantStatus {
				t.Errorf("%s: expected status %d, got %d. Body: %s", tc.name, tc.wantStatus, rr.Code, rr.Body.String())
			}

			var body map[string]interface{}
			_ = json.NewDecoder(rr.Body).Decode(&body)
			if code, _ := body["code"].(string); code != tc.wantCode {
				t.Errorf("%s: expected code %q, got %q", tc.name, tc.wantCode, code)
			}
		})
	}
}

func TestConfirmPasswordResetOtpHandler_Failures(t *testing.T) {
	secret := "test_secret_32_bytes_long_xxxxxx"
	email := "test@example.com"
	userID := "user123"
	oldPasswordHash, _ := crypto.GenerateHash("old-password")
	token, _ := crypto.NewJwtPasswordResetToken(userID, email, string(oldPasswordHash), secret, time.Minute)

	testCases := []struct {
		name       string
		dbSetup    func(*mock.Db)
		wantStatus int
		wantCode   string
	}{
		{
			name: "database update failure",
			dbSetup: func(m *mock.Db) {
				m.GetUserByIdFunc = func(id string) (*db.User, error) {
					return &db.User{ID: userID, Email: email, Password: string(oldPasswordHash)}, nil
				}
				m.UpdatePasswordFunc = func(id, hash string) error {
					return errors.New("db error")
				}
			},
			wantStatus: http.StatusServiceUnavailable,
			wantCode:   CodeErrorServiceUnavailable,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			reqBody := `{"token":"` + token + `", "password":"NewPassword123!", "password_confirm":"NewPassword123!"}`
			req := httptest.NewRequest("POST", "/confirm-password-reset-otp", strings.NewReader(reqBody))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()

			mockDb := &mock.Db{}
			tc.dbSetup(mockDb)

			app := &App{
				validator:      &DefaultValidator{},
				dbAuth:         mockDb,
				configProvider: config.NewProvider(&config.Config{Jwt: config.Jwt{PasswordResetSecret: secret}}),
			}

			app.ConfirmPasswordResetOtpHandler(rr, req)

			if rr.Code != tc.wantStatus {
				t.Errorf("%s: expected status %d, got %d", tc.name, tc.wantStatus, rr.Code)
			}
		})
	}
}
