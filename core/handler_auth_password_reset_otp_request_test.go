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
	"github.com/caasmo/restinpieces/db"
	"github.com/caasmo/restinpieces/db/mock"
	"github.com/caasmo/restinpieces/queue/handlers"
)

func TestRequestPasswordResetOtpHandler_Validation(t *testing.T) {
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
			requestBody: `{"email":"test@example.com"}`,
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
			name:        "empty email",
			contentType: "application/json",
			requestBody: `{"email":""}`,
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
			requestBody: `{"email":"not-an-email"}`,
			wantError:   errorInvalidRequest,
			setupValidator: func(m *MockValidator) {
				m.ContentTypeFunc = func(r *http.Request, allowedType string) (jsonResponse, error) {
					return jsonResponse{}, nil
				}
				m.EmailFunc = func(email string) error {
					return errors.New("invalid format")
				}
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/request-password-reset-otp", strings.NewReader(tc.requestBody))
			req.Header.Set("Content-Type", tc.contentType)
			rr := httptest.NewRecorder()

			mockValidator := &MockValidator{}
			tc.setupValidator(mockValidator)

			app := &App{
				validator: mockValidator,
				dbAuth:    &mock.Db{},
			}

			app.RequestPasswordResetOtpHandler(rr, req)

			if rr.Code != tc.wantError.status {
				t.Errorf("expected status %d, got %d", tc.wantError.status, rr.Code)
			}
		})
	}
}

func TestRequestPasswordResetOtpHandler_Logic(t *testing.T) {
	testConfig := &config.Config{
		Jwt: config.Jwt{
			PasswordResetSecret:        "test_secret_32_bytes_long_xxxxxx",
			PasswordResetTokenDuration: config.Duration{Duration: 15 * time.Minute},
		},
		RateLimits: config.RateLimits{
			PasswordResetCooldown: config.Duration{Duration: 5 * time.Minute},
		},
	}

	testCases := []struct {
		name        string
		email       string
		dbSetup     func(*mock.Db)
		wantJobType string
	}{
		{
			name:  "success - user found and verified",
			email: "test@example.com",
			dbSetup: func(m *mock.Db) {
				m.GetUserByEmailFunc = func(email string) (*db.User, error) {
					return &db.User{Email: email, Verified: true, Password: "hashed-password"}, nil
				}
			},
			wantJobType: handlers.JobTypePasswordResetOtp,
		},
		{
			name:  "success - user not found (silent path)",
			email: "unknown@example.com",
			dbSetup: func(m *mock.Db) {
				m.GetUserByEmailFunc = func(email string) (*db.User, error) {
					return nil, nil
				}
			},
			wantJobType: handlers.JobTypeDummy,
		},
		{
			name:  "success - user unverified (silent path)",
			email: "unverified@example.com",
			dbSetup: func(m *mock.Db) {
				m.GetUserByEmailFunc = func(email string) (*db.User, error) {
					return &db.User{Email: email, Verified: false}, nil
				}
			},
			wantJobType: handlers.JobTypeDummy,
		},
		{
			name:  "success - user has no password (silent path)",
			email: "nopass@example.com",
			dbSetup: func(m *mock.Db) {
				m.GetUserByEmailFunc = func(email string) (*db.User, error) {
					return &db.User{Email: email, Verified: true, Password: ""}, nil
				}
			},
			wantJobType: handlers.JobTypeDummy,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			reqBody := `{"email":"` + tc.email + `"}`
			req := httptest.NewRequest("POST", "/request-password-reset-otp", strings.NewReader(reqBody))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()

			mockDb := &mock.Db{}
			tc.dbSetup(mockDb)

			var capturedJob db.Job
			mockDb.InsertJobFunc = func(job db.Job) error {
				capturedJob = job
				return nil
			}

			app := &App{
				validator:      &DefaultValidator{},
				dbAuth:         mockDb,
				dbQueue:        mockDb,
				configProvider: config.NewProvider(testConfig),
			}

			app.RequestPasswordResetOtpHandler(rr, req)

			if rr.Code != http.StatusOK {
				t.Errorf("expected status 200, got %d", rr.Code)
			}

			var resp map[string]interface{}
			_ = json.NewDecoder(rr.Body).Decode(&resp)
			if resp["code"] != CodeOkOtpTokenIssued {
				t.Errorf("expected code %s, got %v", CodeOkOtpTokenIssued, resp["code"])
			}

			if capturedJob.JobType != tc.wantJobType {
				t.Errorf("expected job type %s, got %s", tc.wantJobType, capturedJob.JobType)
			}
		})
	}
}

func TestRequestPasswordResetOtpHandler_Failures(t *testing.T) {
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
					PasswordResetSecret: "short", // Causes failure
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
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			reqBody := `{"email":"test@example.com"}`
			req := httptest.NewRequest("POST", "/request-password-reset-otp", strings.NewReader(reqBody))
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

			app.RequestPasswordResetOtpHandler(rr, req)

			if rr.Code != tc.wantStatus {
				t.Errorf("expected status %d, got %d", tc.wantStatus, rr.Code)
			}
		})
	}
}
