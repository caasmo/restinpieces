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
)

func TestVerifyPasswordResetOtpHandler_Validation(t *testing.T) {
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
			requestBody: `{"otp":"123456","verification_token":"token"}`,
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
			requestBody: `{"otp":"123456",`,
			wantError:   errorInvalidRequest,
			setupValidator: func(m *MockValidator) {
				m.ContentTypeFunc = func(r *http.Request, allowedType string) (jsonResponse, error) {
					return jsonResponse{}, nil
				}
			},
		},
		{
			name:        "missing otp",
			contentType: "application/json",
			requestBody: `{"verification_token":"token"}`,
			wantError:   errorInvalidRequest,
			setupValidator: func(m *MockValidator) {
				m.ContentTypeFunc = func(r *http.Request, allowedType string) (jsonResponse, error) {
					return jsonResponse{}, nil
				}
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/verify-password-reset-otp", strings.NewReader(tc.requestBody))
			req.Header.Set("Content-Type", tc.contentType)
			rr := httptest.NewRecorder()

			mockValidator := &MockValidator{}
			tc.setupValidator(mockValidator)

			app := &App{
				validator: mockValidator,
				dbAuth:    &mock.Db{},
			}

			app.VerifyPasswordResetOtpHandler(rr, req)

			if rr.Code != tc.wantError.status {
				t.Errorf("expected status %d, got %d", tc.wantError.status, rr.Code)
			}
		})
	}
}

func TestVerifyPasswordResetOtpHandler_Logic(t *testing.T) {
	secret := "test_secret_32_bytes_long_xxxxxx"
	email := "test@example.com"
	otp, token, _ := crypto.NewJwtEmailOtpToken(email, secret, time.Minute)

	testConfig := &config.Config{
		Jwt: config.Jwt{
			PasswordResetSecret:        secret,
			PasswordResetTokenDuration: config.Duration{Duration: 15 * time.Minute},
		},
	}

	testCases := []struct {
		name       string
		otp        string
		token      string
		dbSetup    func(*mock.Db)
		wantStatus int
		wantCode   string
	}{
		{
			name:  "success",
			otp:   otp,
			token: token,
			dbSetup: func(m *mock.Db) {
				m.GetUserByEmailFunc = func(email string) (*db.User, error) {
					return &db.User{ID: "user123", Email: email, Verified: true, Password: "hashed-password"}, nil
				}
			},
			wantStatus: http.StatusAccepted,
			wantCode:   CodeOkPasswordResetOtpVerified,
		},
		{
			name:  "invalid otp",
			otp:   "wrong",
			token: token,
			dbSetup: func(m *mock.Db) {
				// Crypto fails first
			},
			wantStatus: http.StatusUnauthorized,
			wantCode:   CodeErrorInvalidOtp,
		},
		{
			name:  "user not found",
			otp:   otp,
			token: token,
			dbSetup: func(m *mock.Db) {
				m.GetUserByEmailFunc = func(email string) (*db.User, error) {
					return nil, nil
				}
			},
			wantStatus: http.StatusUnauthorized,
			wantCode:   CodeErrorInvalidOtp,
		},
		{
			name:  "user unverified",
			otp:   otp,
			token: token,
			dbSetup: func(m *mock.Db) {
				m.GetUserByEmailFunc = func(email string) (*db.User, error) {
					return &db.User{Email: email, Verified: false}, nil
				}
			},
			wantStatus: http.StatusUnauthorized,
			wantCode:   CodeErrorInvalidOtp,
		},
		{
			name:  "user no password",
			otp:   otp,
			token: token,
			dbSetup: func(m *mock.Db) {
				m.GetUserByEmailFunc = func(email string) (*db.User, error) {
					return &db.User{Email: email, Verified: true, Password: ""}, nil
				}
			},
			wantStatus: http.StatusUnauthorized,
			wantCode:   CodeErrorInvalidOtp,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			reqBody := `{"otp":"` + tc.otp + `", "verification_token":"` + tc.token + `"}`
			req := httptest.NewRequest("POST", "/verify-password-reset-otp", strings.NewReader(reqBody))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()

			mockDb := &mock.Db{}
			tc.dbSetup(mockDb)

			app := &App{
				validator:      &DefaultValidator{},
				dbAuth:         mockDb,
				configProvider: config.NewProvider(testConfig),
			}

			app.VerifyPasswordResetOtpHandler(rr, req)

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
