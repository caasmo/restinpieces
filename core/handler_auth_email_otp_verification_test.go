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

// TestRequestEmailOtpVerificationHandler_Validation tests input validation for
// the request-email-otp-verification handler. It covers content type errors,
// malformed JSON, missing fields, invalid email, and weak password scenarios.
func TestRequestEmailOtpVerificationHandler_Validation(t *testing.T) {
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
			requestBody: `{"email":"test@example.com","password":"password123"}`,
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
			requestBody: `{"email":"test@example.com",`,
			wantError:   errorInvalidRequest,
			setupValidator: func(m *MockValidator) {
				m.ContentTypeFunc = func(r *http.Request, allowedType string) (jsonResponse, error) {
					return jsonResponse{}, nil
				}
			},
		},
		{
			name:        "missing email field",
			contentType: "application/json",
			requestBody: `{"password":"password123"}`,
			wantError:   errorInvalidRequest,
			setupValidator: func(m *MockValidator) {
				m.ContentTypeFunc = func(r *http.Request, allowedType string) (jsonResponse, error) {
					return jsonResponse{}, nil
				}
			},
		},
		{
			name:        "missing password field",
			contentType: "application/json",
			requestBody: `{"email":"test@example.com"}`,
			wantError:   errorInvalidRequest,
			setupValidator: func(m *MockValidator) {
				m.ContentTypeFunc = func(r *http.Request, allowedType string) (jsonResponse, error) {
					return jsonResponse{}, nil
				}
			},
		},
		{
			name:        "empty email and password",
			contentType: "application/json",
			requestBody: `{"email":"","password":""}`,
			wantError:   errorInvalidRequest,
			setupValidator: func(m *MockValidator) {
				m.ContentTypeFunc = func(r *http.Request, allowedType string) (jsonResponse, error) {
					return jsonResponse{}, nil
				}
			},
		},
		{
			name:        "invalid email format",
			contentType: "application/json",
			requestBody: `{"email":"not-an-email","password":"password123"}`,
			wantError:   errorInvalidRequest,
			setupValidator: func(m *MockValidator) {
				m.ContentTypeFunc = func(r *http.Request, allowedType string) (jsonResponse, error) {
					return jsonResponse{}, nil
				}
				m.EmailFunc = func(email string) error {
					return errors.New("invalid email format")
				}
			},
		},
		{
			name:        "weak password",
			contentType: "application/json",
			requestBody: `{"email":"test@example.com","password":"short"}`,
			wantError:   errorWeakPassword,
			setupValidator: func(m *MockValidator) {
				m.ContentTypeFunc = func(r *http.Request, allowedType string) (jsonResponse, error) {
					return jsonResponse{}, nil
				}
				m.PasswordFunc = func(password string) error {
					return errors.New("weak password")
				}
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/request-email-otp-verification", strings.NewReader(tc.requestBody))
			req.Header.Set("Content-Type", tc.contentType)
			rr := httptest.NewRecorder()

			mockValidator := &MockValidator{}
			tc.setupValidator(mockValidator)

			app := &App{
				validator: mockValidator,
				dbAuth:    &mockDbApp{},
			}

			app.RequestEmailOtpVerificationHandler(rr, req)

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

// TestRequestEmailOtpVerificationHandler_RequestLogic tests the core business logic
// of the request-email-otp-verification handler. It covers successful OTP
// generation, already verified user, user not found, wrong password, DB
// cooldown constraint, and input normalization scenarios.
func TestRequestEmailOtpVerificationHandler_RequestLogic(t *testing.T) {
	hashedPassword, _ := crypto.GenerateHash("password123")
	testUser := &db.User{
		ID:       "user123",
		Email:    "test@example.com",
		Password: string(hashedPassword),
		Verified: false,
	}
	verifiedUser := &db.User{
		ID:       "user456",
		Email:    "verified@example.com",
		Password: string(hashedPassword),
		Verified: true,
	}

	testConfig := &config.Config{
		Jwt: config.Jwt{
			AuthSecret:                       "test_secret_32_bytes_long_xxxxxx",
			AuthTokenDuration:                config.Duration{Duration: 15 * time.Minute},
			VerificationEmailOtpSecret:       "test_secret_32_bytes_long_xxxxxx",
			VerificationEmailOtpTokenDuration: config.Duration{Duration: 15 * time.Minute},
		},
		RateLimits: config.RateLimits{
			EmailOtpVerificationCooldown: config.Duration{Duration: 2 * time.Minute},
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
			name:        "successful OTP request",
			requestBody: `{"email":"test@example.com","password":"password123"}`,
			dbSetup: func(m *mock.Db) {
				m.GetUserByEmailFunc = func(email string) (*db.User, error) {
					return testUser, nil
				}
				m.InsertJobFunc = func(job db.Job) error {
					return nil
				}
			},
			wantStatus: http.StatusOK,
			wantCode:   CodeOkOtpTokenIssued,
		},
		{
			name:        "already verified user",
			requestBody: `{"email":"verified@example.com","password":"password123"}`,
			dbSetup: func(m *mock.Db) {
				m.GetUserByEmailFunc = func(email string) (*db.User, error) {
					return verifiedUser, nil
				}
			},
			wantStatus: http.StatusAccepted,
			wantCode:   CodeOkAlreadyVerified,
		},
		{
			name:        "user not found",
			requestBody: `{"email":"unknown@example.com","password":"password123"}`,
			dbSetup: func(m *mock.Db) {
				m.GetUserByEmailFunc = func(email string) (*db.User, error) {
					return nil, db.ErrUserNotFound
				}
			},
			wantStatus: http.StatusBadRequest,
			wantCode:   CodeErrorInvalidRequest,
		},
		{
			name:        "wrong password",
			requestBody: `{"email":"test@example.com","password":"wrongpassword"}`,
			dbSetup: func(m *mock.Db) {
				m.GetUserByEmailFunc = func(email string) (*db.User, error) {
					return testUser, nil
				}
			},
			wantStatus: http.StatusBadRequest,
			wantCode:   CodeErrorInvalidRequest,
		},
		{
			name:        "job cooldown (unique constraint) returns conflict",
			requestBody: `{"email":"test@example.com","password":"password123"}`,
			dbSetup: func(m *mock.Db) {
				m.GetUserByEmailFunc = func(email string) (*db.User, error) {
					return testUser, nil
				}
				m.InsertJobFunc = func(job db.Job) error {
					return db.ErrConstraintUnique
				}
			},
			wantStatus: http.StatusConflict,
			wantCode:   CodeErrorEmailOtpVerificationAlreadyRequested,
		},
		{
			name:        "successful request with whitespace trimming",
			requestBody: `{"email":"  test@example.com  ","password":"  password123  "}`,
			dbSetup: func(m *mock.Db) {
				m.GetUserByEmailFunc = func(email string) (*db.User, error) {
					if email != "test@example.com" {
						t.Errorf("expected email to be trimmed, got %q", email)
					}
					return testUser, nil
				}
				m.InsertJobFunc = func(job db.Job) error {
					return nil
				}
			},
			wantStatus: http.StatusOK,
			wantCode:   CodeOkOtpTokenIssued,
		},
		{
			name:        "successful request with normalization",
			requestBody: `{"email":"  NORMALIZED@Example.Com  ","password":"password123"}`,
			dbSetup: func(m *mock.Db) {
				m.GetUserByEmailFunc = func(email string) (*db.User, error) {
					if email != "normalized@example.com" {
						t.Errorf("expected email to be normalized to lowercase, got %q", email)
					}
					return testUser, nil
				}
				m.InsertJobFunc = func(job db.Job) error {
					return nil
				}
			},
			wantStatus: http.StatusOK,
			wantCode:   CodeOkOtpTokenIssued,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/request-email-otp-verification", strings.NewReader(tc.requestBody))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()

			mockDb := &mock.Db{}
			tc.dbSetup(mockDb)

			app := &App{
				validator:      &DefaultValidator{},
				dbAuth:         mockDb,
				dbQueue:        mockDb,
				configProvider: config.NewProvider(testConfig),
			}

			app.RequestEmailOtpVerificationHandler(rr, req)

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

			if tc.wantCode == CodeOkOtpTokenIssued {
				data, ok := body["data"].(map[string]interface{})
				if !ok {
					t.Fatal("expected 'data' field in successful OTP response")
				}
				if _, ok := data["verification_token"]; !ok {
					t.Error("successful OTP response missing 'verification_token'")
				}
			}
		})
	}
}

// TestRequestEmailOtpVerificationHandler_DependencyFailures tests how the
// request-email-otp-verification handler responds to failures in its dependencies
// such as OTP token generation and job queue insertion.
func TestRequestEmailOtpVerificationHandler_DependencyFailures(t *testing.T) {
	hashedPassword, _ := crypto.GenerateHash("password123")
	testUser := &db.User{
		ID:       "user123",
		Email:    "test@example.com",
		Password: string(hashedPassword),
		Verified: false,
	}

	testCases := []struct {
		name      string
		config    *config.Config
		dbSetup   func(*mock.Db)
		wantError jsonResponse
	}{
		{
			name: "OTP token generation failure (short secret)",
			config: &config.Config{
				Jwt: config.Jwt{
					VerificationEmailOtpSecret:       "short",
					VerificationEmailOtpTokenDuration: config.Duration{Duration: 15 * time.Minute},
				},
				RateLimits: config.RateLimits{
					EmailOtpVerificationCooldown: config.Duration{Duration: 2 * time.Minute},
				},
			},
			dbSetup: func(m *mock.Db) {
				m.GetUserByEmailFunc = func(email string) (*db.User, error) {
					return testUser, nil
				}
			},
			wantError: errorOtpFailed,
		},
		{
			name: "job insertion failure",
			config: &config.Config{
				Jwt: config.Jwt{
					VerificationEmailOtpSecret:       "test_secret_32_bytes_long_xxxxxx",
					VerificationEmailOtpTokenDuration: config.Duration{Duration: 15 * time.Minute},
				},
				RateLimits: config.RateLimits{
					EmailOtpVerificationCooldown: config.Duration{Duration: 2 * time.Minute},
				},
			},
			dbSetup: func(m *mock.Db) {
				m.GetUserByEmailFunc = func(email string) (*db.User, error) {
					return testUser, nil
				}
				m.InsertJobFunc = func(job db.Job) error {
					return errors.New("database connection failed")
				}
			},
			wantError: errorAuthDatabaseError,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			reqBody := `{"email":"test@example.com","password":"password123"}`
			req := httptest.NewRequest("POST", "/request-email-otp-verification", strings.NewReader(reqBody))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()

			mockDb := &mock.Db{}
			tc.dbSetup(mockDb)

			app := &App{
				validator:      &DefaultValidator{},
				dbAuth:         mockDb,
				dbQueue:        mockDb,
				configProvider: config.NewProvider(tc.config),
			}

			app.RequestEmailOtpVerificationHandler(rr, req)

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
	otp, validToken, err := crypto.NewJwtEmailOtpVerificationToken("test@example.com", secret, 15*time.Minute)
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
				m.GetUserByEmailFunc = func(email string) (*db.User, error) {
					return unverifiedUser, nil
				}
				m.VerifyEmailFunc = func(userId string) error {
					return nil
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
				m.GetUserByEmailFunc = func(email string) (*db.User, error) {
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
				m.GetUserByEmailFunc = func(email string) (*db.User, error) {
					return &db.User{
						ID:       "user456",
						Email:    "test@example.com",
						Password: string(hashedPassword),
						Verified: true,
					}, nil
				}
			},
			wantStatus: http.StatusUnauthorized,
			wantCode:   CodeErrorInvalidOtp,
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

	otp, validToken, err := crypto.NewJwtEmailOtpVerificationToken("test@example.com", secret, 15*time.Minute)
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
			name:   "database failure on VerifyEmail",
			config: baseConfig,
			dbSetup: func(m *mock.Db) {
				m.GetUserByEmailFunc = func(email string) (*db.User, error) {
					return &db.User{
						ID:       "user123",
						Email:    "test@example.com",
						Password: string(hashedPassword),
						Verified: false,
					}, nil
				}
				m.VerifyEmailFunc = func(userId string) error {
					return errors.New("db connection failed")
				}
			},
			wantError: errorAuthDatabaseError,
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
				m.GetUserByEmailFunc = func(email string) (*db.User, error) {
					return &db.User{
						ID:       "user123",
						Email:    "test@example.com",
						Password: string(hashedPassword),
						Verified: false,
					}, nil
				}
				m.VerifyEmailFunc = func(userId string) error {
					return nil
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
