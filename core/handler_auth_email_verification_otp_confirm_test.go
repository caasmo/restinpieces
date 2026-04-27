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

// TestConfirmEmailOtpVerificationHandler_Validation tests input validation for
// the confirm-email-otp-verification handler. It covers content type errors,
// malformed JSON, and missing fields.
func TestConfirmEmailOtpVerificationHandler_Validation(t *testing.T) {
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
			name:        "missing otp field",
			contentType: "application/json",
			requestBody: `{"verification_token":"token"}`,
			wantError:   errorInvalidRequest,
			setupValidator: func(m *MockValidator) {
				m.ContentTypeFunc = func(r *http.Request, allowedType string) (jsonResponse, error) {
					return jsonResponse{}, nil
				}
			},
		},
		{
			name:        "missing verification_token field",
			contentType: "application/json",
			requestBody: `{"otp":"123456"}`,
			wantError:   errorInvalidRequest,
			setupValidator: func(m *MockValidator) {
				m.ContentTypeFunc = func(r *http.Request, allowedType string) (jsonResponse, error) {
					return jsonResponse{}, nil
				}
			},
		},
		{
			name:        "empty otp and verification_token",
			contentType: "application/json",
			requestBody: `{"otp":"","verification_token":""}`,
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
			req := httptest.NewRequest("POST", "/confirm-email-otp-verification", strings.NewReader(tc.requestBody))
			req.Header.Set("Content-Type", tc.contentType)
			rr := httptest.NewRecorder()

			mockValidator := &MockValidator{}
			tc.setupValidator(mockValidator)

			app := &App{
				validator: mockValidator,
				dbAuth:    &mockDbApp{},
			}

			app.ConfirmEmailOtpVerificationHandler(rr, req)

			if rr.Code != tc.wantError.status {
				t.Errorf("expected status %d, got %d", tc.wantError.status, rr.Code)
			}

			var gotBody, wantBody map[string]interface{}
			if err := json.NewDecoder(rr.Body).Decode(&gotBody); err != nil {
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

// TestConfirmEmailOtpVerificationHandler_ConfirmationLogic tests the core
// confirmation logic of the confirm-email-otp-verification handler. It covers
// successful verification, invalid OTP, invalid token, user not found, and
// already verified scenarios. All failure cases uniformly return
// errorInvalidOtp to prevent email enumeration.
func TestConfirmEmailOtpVerificationHandler_ConfirmationLogic(t *testing.T) {
	secret := "test_secret_32_bytes_long_xxxxxx"
	hashedPassword, _ := crypto.GenerateHash("password123")

	// Generate a valid OTP + verification token pair for testing.
	otp, validToken, err := crypto.NewJwtEmailOtpToken("test@example.com", secret, 15*time.Minute)
	if err != nil {
		t.Fatalf("failed to generate test OTP token: %v", err)
	}

	unverifiedUser := &db.User{
		ID:       "user123",
		Email:    "test@example.com",
		Password: string(hashedPassword),
		Verified: false,
	}

	testConfig := &config.Config{
		Jwt: config.Jwt{
			VerificationEmailOtpSecret:       secret,
			VerificationEmailOtpTokenDuration: config.Duration{Duration: 15 * time.Minute},
			AuthSecret:                       "test_secret_32_bytes_long_xxxxxx",
			AuthTokenDuration:                config.Duration{Duration: 15 * time.Minute},
		},
	}

	testCases := []struct {
		name        string
		requestBody string
		dbSetup     func(*mock.Db)
		wantStatus  int
		wantCode    string
	}{
		{
			name:        "successful OTP confirmation",
			requestBody: `{"otp":"` + otp + `","verification_token":"` + validToken + `"}`,
			dbSetup: func(m *mock.Db) {
				m.UpdateVerifiedFunc = func(email string) (*db.User, error) {
					u := *unverifiedUser
					u.Verified = true
					return &u, nil
				}
			},
			wantStatus: http.StatusOK,
			wantCode:   CodeOkAuthentication,
		},
		{
			name:        "invalid OTP with valid token",
			requestBody: `{"otp":"000000","verification_token":"` + validToken + `"}`,
			dbSetup: func(m *mock.Db) {
				// Crypto verification fails first; no DB calls expected.
			},
			wantStatus: http.StatusUnauthorized,
			wantCode:   CodeErrorInvalidOtp,
		},
		{
			name:        "invalid verification token",
			requestBody: `{"otp":"123456","verification_token":"bogus-token"}`,
			dbSetup: func(m *mock.Db) {
				// Crypto verification fails first; no DB calls expected.
			},
			wantStatus: http.StatusUnauthorized,
			wantCode:   CodeErrorInvalidOtp,
		},
		{
			name:        "user not found after valid token",
			requestBody: `{"otp":"` + otp + `","verification_token":"` + validToken + `"}`,
			dbSetup: func(m *mock.Db) {
				m.UpdateVerifiedFunc = func(email string) (*db.User, error) {
					return nil, db.ErrUserNotFound
				}
			},
			wantStatus: http.StatusUnauthorized,
			wantCode:   CodeErrorInvalidOtp,
		},
		{
			name:        "already verified user",
			requestBody: `{"otp":"` + otp + `","verification_token":"` + validToken + `"}`,
			dbSetup: func(m *mock.Db) {
				m.UpdateVerifiedFunc = func(email string) (*db.User, error) {
					return &db.User{
						ID:       "user456",
						Email:    "test@example.com",
						Password: string(hashedPassword),
						Verified: true,
					}, nil
				}
			},
			wantStatus: http.StatusOK,
			wantCode:   CodeOkAuthentication,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/confirm-email-otp-verification", strings.NewReader(tc.requestBody))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()

			mockDb := &mock.Db{}
			tc.dbSetup(mockDb)

			app := &App{
				validator:      &DefaultValidator{},
				dbAuth:         mockDb,
				configProvider: config.NewProvider(testConfig),
			}

			app.ConfirmEmailOtpVerificationHandler(rr, req)

			if rr.Code != tc.wantStatus {
				t.Errorf("expected status %d, got %d", tc.wantStatus, rr.Code)
			}

			var body map[string]interface{}
			if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
				t.Fatalf("failed to decode response body: %v", err)
			}

			if code, _ := body["code"].(string); code != tc.wantCode {
				t.Errorf("expected code %q, got %q", tc.wantCode, code)
			}

			if tc.wantStatus == http.StatusOK && tc.wantCode == CodeOkAuthentication {
				data, ok := body["data"].(map[string]interface{})
				if !ok {
					t.Fatal("expected 'data' field in successful auth response")
				}
				if _, ok := data["access_token"]; !ok {
					t.Error("successful response missing 'access_token'")
				}
			}
		})
	}
}

// TestConfirmEmailOtpVerificationHandler_DependencyFailures tests how the
// confirm-email-otp-verification handler responds to failures in its
// dependencies, such as the database and token generation.
func TestConfirmEmailOtpVerificationHandler_DependencyFailures(t *testing.T) {
	secret := "test_secret_32_bytes_long_xxxxxx"
	hashedPassword, _ := crypto.GenerateHash("password123")

	otp, validToken, err := crypto.NewJwtEmailOtpToken("test@example.com", secret, 15*time.Minute)
	if err != nil {
		t.Fatalf("failed to generate test OTP token: %v", err)
	}

	baseConfig := &config.Config{
		Jwt: config.Jwt{
			VerificationEmailOtpSecret:       secret,
			VerificationEmailOtpTokenDuration: config.Duration{Duration: 15 * time.Minute},
			AuthSecret:                       "test_secret_32_bytes_long_xxxxxx",
			AuthTokenDuration:                config.Duration{Duration: 15 * time.Minute},
		},
	}

	testCases := []struct {
		name      string
		config    *config.Config
		dbSetup   func(*mock.Db)
		wantError jsonResponse
	}{
		{
			name:   "database failure on UpdateVerified",
			config: baseConfig,
			dbSetup: func(m *mock.Db) {
				m.UpdateVerifiedFunc = func(email string) (*db.User, error) {
					return nil, errors.New("db connection failed")
				}
			},
			wantError: errorInvalidOtp,
		},
		{
			name: "JWT session token generation failure (short secret)",
			config: &config.Config{
				Jwt: config.Jwt{
					VerificationEmailOtpSecret:       secret,
					VerificationEmailOtpTokenDuration: config.Duration{Duration: 15 * time.Minute},
					AuthSecret:                       "short",
				},
			},
			dbSetup: func(m *mock.Db) {
				m.UpdateVerifiedFunc = func(email string) (*db.User, error) {
					return &db.User{
						ID:       "user123",
						Email:    "test@example.com",
						Password: string(hashedPassword),
						Verified: true,
					}, nil
				}
			},
			wantError: errorTokenGeneration,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			reqBody := `{"otp":"` + otp + `","verification_token":"` + validToken + `"}`
			req := httptest.NewRequest("POST", "/confirm-email-otp-verification", strings.NewReader(reqBody))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()

			mockDb := &mock.Db{}
			tc.dbSetup(mockDb)

			app := &App{
				validator:      &DefaultValidator{},
				dbAuth:         mockDb,
				configProvider: config.NewProvider(tc.config),
			}

			app.ConfirmEmailOtpVerificationHandler(rr, req)

			if rr.Code != tc.wantError.status {
				t.Errorf("expected status %d, got %d", tc.wantError.status, rr.Code)
			}

			var gotBody, wantBody map[string]interface{}
			if err := json.NewDecoder(rr.Body).Decode(&gotBody); err != nil {
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

