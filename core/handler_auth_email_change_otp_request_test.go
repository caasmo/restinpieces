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
	"github.com/caasmo/restinpieces/queue/handlers"
)

func TestRequestEmailChangeOtpHandler_Validation(t *testing.T) {
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
			requestBody: `{"new_email":"new@example.com","password":"password123"}`,
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
			requestBody: `{"new_email":"new@example.com",`,
			wantError:   errorInvalidRequest,
			setupValidator: func(m *MockValidator) {
				m.ContentTypeFunc = func(r *http.Request, allowedType string) (jsonResponse, error) {
					return jsonResponse{}, nil
				}
			},
		},
		{
			name:        "missing new_email field",
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
			requestBody: `{"new_email":"new@example.com"}`,
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
			requestBody: `{"new_email":"not-an-email","password":"password123"}`,
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
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/api/request-email-change-otp", strings.NewReader(tc.requestBody))
			req.Header.Set("Content-Type", tc.contentType)
			rr := httptest.NewRecorder()

			mockValidator := &MockValidator{}
			tc.setupValidator(mockValidator)

			mockAuth := &MockAuth{
				AuthenticateFunc: func(r *http.Request) (*db.User, jsonResponse, error) {
					return &db.User{ID: "user123", Verified: true}, jsonResponse{}, nil
				},
			}

			app := &App{
				validator:     mockValidator,
				authenticator: mockAuth,
				dbAuth:         &mock.Db{},
			}

			app.RequestEmailChangeOtpHandler(rr, req)

			if rr.Code != tc.wantError.status {
				t.Errorf("expected status %d, got %d", tc.wantError.status, rr.Code)
			}
		})
	}
}

func TestRequestEmailChangeOtpHandler_Logic(t *testing.T) {
	hashedPassword, _ := crypto.GenerateHash("password123")
	testUser := &db.User{
		ID:       "user123",
		Email:    "old@example.com",
		Password: string(hashedPassword),
		Verified: true,
	}

	testConfig := &config.Config{
		Jwt: config.Jwt{
			EmailChangeOtpSecret:       "test_secret_32_bytes_long_xxxxxx",
			EmailChangeOtpTokenDuration: config.Duration{Duration: 15 * time.Minute},
		},
		RateLimits: config.RateLimits{
			EmailChangeCooldown: config.Duration{Duration: 5 * time.Minute},
		},
	}

	testCases := []struct {
		name        string
		requestBody string
		authSetup   func(*MockAuth)
		dbSetup     func(*mock.Db)
		wantStatus  int
		wantCode    string
		wantJobType string
	}{
		{
			name:        "success - new email available",
			requestBody: `{"new_email":"new@example.com","password":"password123"}`,
			authSetup: func(m *MockAuth) {
				m.AuthenticateFunc = func(r *http.Request) (*db.User, jsonResponse, error) {
					return testUser, jsonResponse{}, nil
				}
			},
			dbSetup: func(m *mock.Db) {
				m.GetUserByEmailFunc = func(email string) (*db.User, error) {
					return nil, nil // Not taken
				}
				m.InsertJobFunc = func(job db.Job) error {
					return nil
				}
			},
			wantStatus:  http.StatusOK,
			wantCode:    CodeOkOtpTokenIssued,
			wantJobType: handlers.JobTypeEmailChangeOtp,
		},
		{
			name:        "success - new email taken (silent path)",
			requestBody: `{"new_email":"taken@example.com","password":"password123"}`,
			authSetup: func(m *MockAuth) {
				m.AuthenticateFunc = func(r *http.Request) (*db.User, jsonResponse, error) {
					return testUser, jsonResponse{}, nil
				}
			},
			dbSetup: func(m *mock.Db) {
				m.GetUserByEmailFunc = func(email string) (*db.User, error) {
					return &db.User{ID: "other"}, nil // Taken
				}
				m.InsertJobFunc = func(job db.Job) error {
					return nil
				}
			},
			wantStatus:  http.StatusOK,
			wantCode:    CodeOkOtpTokenIssued,
			wantJobType: handlers.JobTypeDummy,
		},
		{
			name:        "unauthenticated",
			requestBody: `{"new_email":"new@example.com","password":"password123"}`,
			authSetup: func(m *MockAuth) {
				m.AuthenticateFunc = func(r *http.Request) (*db.User, jsonResponse, error) {
					return nil, errorJwtInvalidToken, errors.New("auth error")
				}
			},
			dbSetup:    func(m *mock.Db) {},
			wantStatus: http.StatusUnauthorized,
			wantCode:   CodeErrorJwtInvalidToken,
		},
		{
			name:        "unverified user cannot change email",
			requestBody: `{"new_email":"new@example.com","password":"password123"}`,
			authSetup: func(m *MockAuth) {
				m.AuthenticateFunc = func(r *http.Request) (*db.User, jsonResponse, error) {
					u := *testUser
					u.Verified = false
					return &u, jsonResponse{}, nil
				}
			},
			dbSetup:    func(m *mock.Db) {},
			wantStatus: http.StatusForbidden,
			wantCode:   CodeErrorUnverifiedEmail,
		},
		{
			name:        "wrong password",
			requestBody: `{"new_email":"new@example.com","password":"wrongpassword"}`,
			authSetup: func(m *MockAuth) {
				m.AuthenticateFunc = func(r *http.Request) (*db.User, jsonResponse, error) {
					return testUser, jsonResponse{}, nil
				}
			},
			dbSetup:    func(m *mock.Db) {},
			wantStatus: http.StatusUnauthorized,
			wantCode:   CodeErrorInvalidCredentials,
		},
		{
			name:        "same email conflict",
			requestBody: `{"new_email":"old@example.com","password":"password123"}`,
			authSetup: func(m *MockAuth) {
				m.AuthenticateFunc = func(r *http.Request) (*db.User, jsonResponse, error) {
					return testUser, jsonResponse{}, nil
				}
			},
			dbSetup:    func(m *mock.Db) {},
			wantStatus: http.StatusConflict,
			wantCode:   CodeErrorEmailConflict,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/api/request-email-change-otp", strings.NewReader(tc.requestBody))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()

			mockAuth := &MockAuth{}
			tc.authSetup(mockAuth)

			mockDb := &mock.Db{}
			tc.dbSetup(mockDb)

			var capturedJob db.Job
			mockDb.InsertJobFunc = func(job db.Job) error {
				capturedJob = job
				return nil
			}

			app := &App{
				validator:      &DefaultValidator{},
				authenticator:  mockAuth,
				dbAuth:         mockDb,
				dbQueue:        mockDb,
				configProvider: config.NewProvider(testConfig),
			}

			app.RequestEmailChangeOtpHandler(rr, req)

			if rr.Code != tc.wantStatus {
				t.Errorf("expected status %d, got %d. Body: %s", tc.wantStatus, rr.Code, rr.Body.String())
			}

			var body map[string]interface{}
			_ = json.NewDecoder(rr.Body).Decode(&body)
			if code, _ := body["code"].(string); code != tc.wantCode {
				t.Errorf("expected code %q, got %q", tc.wantCode, code)
			}

			if tc.wantJobType != "" {
				if capturedJob.JobType != tc.wantJobType {
					t.Errorf("expected job type %q, got %q", tc.wantJobType, capturedJob.JobType)
				}
			}
		})
	}
}

func TestRequestEmailChangeOtpHandler_Failures(t *testing.T) {
	hashedPassword, _ := crypto.GenerateHash("password123")
	testUser := &db.User{
		ID:       "user123",
		Email:    "old@example.com",
		Password: string(hashedPassword),
		Verified: true,
	}

	testCases := []struct {
		name       string
		config     *config.Config
		dbSetup    func(*mock.Db)
		wantStatus int
		wantCode   string
	}{
		{
			name: "OTP token generation failure",
			config: &config.Config{
				Jwt: config.Jwt{
					EmailChangeOtpSecret: "short", // Causes failure
				},
			},
			dbSetup: func(m *mock.Db) {
				m.GetUserByEmailFunc = func(email string) (*db.User, error) {
					return nil, nil
				}
			},
			wantStatus: http.StatusInternalServerError,
			wantCode:   CodeErrorOtpFailed,
		},
		{
			name: "queue insertion failure (logged but doesn't change response)",
			config: &config.Config{
				Jwt: config.Jwt{
					EmailChangeOtpSecret:       "test_secret_32_bytes_long_xxxxxx",
					EmailChangeOtpTokenDuration: config.Duration{Duration: 15 * time.Minute},
				},
				RateLimits: config.RateLimits{
					EmailChangeCooldown: config.Duration{Duration: 5 * time.Minute},
				},
			},
			dbSetup: func(m *mock.Db) {
				m.GetUserByEmailFunc = func(email string) (*db.User, error) {
					return nil, nil
				}
				m.InsertJobFunc = func(job db.Job) error {
					return errors.New("db error")
				}
			},
			wantStatus: http.StatusOK,
			wantCode:   CodeOkOtpTokenIssued,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			reqBody := `{"new_email":"new@example.com","password":"password123"}`
			req := httptest.NewRequest("POST", "/api/request-email-change-otp", strings.NewReader(reqBody))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()

			mockAuth := &MockAuth{
				AuthenticateFunc: func(r *http.Request) (*db.User, jsonResponse, error) {
					return testUser, jsonResponse{}, nil
				},
			}

			mockDb := &mock.Db{}
			tc.dbSetup(mockDb)

			app := &App{
				validator:      &DefaultValidator{},
				authenticator:  mockAuth,
				dbAuth:         mockDb,
				dbQueue:        mockDb,
				configProvider: config.NewProvider(tc.config),
			}

			app.RequestEmailChangeOtpHandler(rr, req)

			if rr.Code != tc.wantStatus {
				t.Errorf("expected status %d, got %d", tc.wantStatus, rr.Code)
			}

			var body map[string]interface{}
			_ = json.NewDecoder(rr.Body).Decode(&body)
			if code, _ := body["code"].(string); code != tc.wantCode {
				t.Errorf("expected code %q, got %q", tc.wantCode, code)
			}
		})
	}
}
