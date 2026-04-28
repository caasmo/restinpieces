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

func TestConfirmEmailChangeOtpHandler_Validation(t *testing.T) {
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
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/api/confirm-email-change-otp", strings.NewReader(tc.requestBody))
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

			app.ConfirmEmailChangeOtpHandler(rr, req)

			if rr.Code != tc.wantError.status {
				t.Errorf("expected status %d, got %d", tc.wantError.status, rr.Code)
			}
		})
	}
}

func TestConfirmEmailChangeOtpHandler_Logic(t *testing.T) {
	secret := "test_secret_32_bytes_long_xxxxxx"
	newEmail := "new@example.com"
	oldEmail := "old@example.com"
	userID := "user123"

	otp, token, _ := crypto.NewJwtEmailOtpToken(newEmail, secret, time.Minute)

	testConfig := &config.Config{
		Jwt: config.Jwt{
			EmailChangeOtpSecret: secret,
		},
	}

	testCases := []struct {
		name       string
		otp        string
		token      string
		authSetup  func(*MockAuth)
		dbSetup    func(*mock.Db)
		wantStatus int
		wantCode   string
	}{
		{
			name:  "success",
			otp:   otp,
			token: token,
			authSetup: func(m *MockAuth) {
				m.AuthenticateFunc = func(r *http.Request) (*db.User, jsonResponse, error) {
					return &db.User{ID: userID, Email: oldEmail, Verified: true}, jsonResponse{}, nil
				}
			},
			dbSetup: func(m *mock.Db) {
				m.UpdateEmailFunc = func(id, email string) error {
					return nil
				}
				m.InsertJobFunc = func(job db.Job) error {
					return nil
				}
			},
			wantStatus: http.StatusOK,
			wantCode:   CodeOkEmailChange,
		},
		{
			name:  "unauthenticated",
			otp:   otp,
			token: token,
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
			name:  "invalid otp",
			otp:   "wrong",
			token: token,
			authSetup: func(m *MockAuth) {
				m.AuthenticateFunc = func(r *http.Request) (*db.User, jsonResponse, error) {
					return &db.User{ID: userID, Email: oldEmail, Verified: true}, jsonResponse{}, nil
				}
			},
			dbSetup:    func(m *mock.Db) {},
			wantStatus: http.StatusUnauthorized,
			wantCode:   CodeErrorInvalidOtp,
		},
		{
			name:  "invalid token",
			otp:   otp,
			token: "bogus",
			authSetup: func(m *MockAuth) {
				m.AuthenticateFunc = func(r *http.Request) (*db.User, jsonResponse, error) {
					return &db.User{ID: userID, Email: oldEmail, Verified: true}, jsonResponse{}, nil
				}
			},
			dbSetup:    func(m *mock.Db) {},
			wantStatus: http.StatusUnauthorized,
			wantCode:   CodeErrorInvalidOtp,
		},
		{
			name:  "same email as old email",
			otp:   otp,
			token: token,
			authSetup: func(m *MockAuth) {
				m.AuthenticateFunc = func(r *http.Request) (*db.User, jsonResponse, error) {
					return &db.User{ID: userID, Email: newEmail, Verified: true}, jsonResponse{}, nil
				}
			},
			dbSetup:    func(m *mock.Db) {},
			wantStatus: http.StatusUnauthorized,
			wantCode:   CodeErrorInvalidOtp,
		},
		{
			name:  "database error on update",
			otp:   otp,
			token: token,
			authSetup: func(m *MockAuth) {
				m.AuthenticateFunc = func(r *http.Request) (*db.User, jsonResponse, error) {
					return &db.User{ID: userID, Email: oldEmail, Verified: true}, jsonResponse{}, nil
				}
			},
			dbSetup: func(m *mock.Db) {
				m.UpdateEmailFunc = func(id, email string) error {
					return errors.New("db error")
				}
			},
			wantStatus: http.StatusServiceUnavailable,
			wantCode:   CodeErrorServiceUnavailable,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			reqBody := `{"otp":"` + tc.otp + `","verification_token":"` + tc.token + `"}`
			req := httptest.NewRequest("POST", "/api/confirm-email-change-otp", strings.NewReader(reqBody))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()

			mockAuth := &MockAuth{}
			tc.authSetup(mockAuth)

			mockDb := &mock.Db{}
			tc.dbSetup(mockDb)

			app := &App{
				validator:      &DefaultValidator{},
				authenticator:  mockAuth,
				dbAuth:         mockDb,
				dbQueue:        mockDb,
				configProvider: config.NewProvider(testConfig),
			}

			app.ConfirmEmailChangeOtpHandler(rr, req)

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

func TestConfirmEmailChangeOtpHandler_InvalidEmailInToken(t *testing.T) {
	secret := "test_secret_32_bytes_long_xxxxxx"
	invalidEmail := "not-an-email"
	otp, token, _ := crypto.NewJwtEmailOtpToken(invalidEmail, secret, time.Minute)

	testConfig := &config.Config{
		Jwt: config.Jwt{
			EmailChangeOtpSecret: secret,
		},
	}

	reqBody := `{"otp":"` + otp + `","verification_token":"` + token + `"}`
	req := httptest.NewRequest("POST", "/api/confirm-email-change-otp", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	mockAuth := &MockAuth{
		AuthenticateFunc: func(r *http.Request) (*db.User, jsonResponse, error) {
			return &db.User{ID: "user123", Email: "old@example.com", Verified: true}, jsonResponse{}, nil
		},
	}

	app := &App{
		validator:      &DefaultValidator{},
		authenticator:  mockAuth,
		dbAuth:         &mock.Db{},
		dbQueue:        &mock.Db{},
		configProvider: config.NewProvider(testConfig),
	}

	app.ConfirmEmailChangeOtpHandler(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rr.Code)
	}

	var body map[string]interface{}
	_ = json.NewDecoder(rr.Body).Decode(&body)
	if body["code"] != CodeErrorInvalidOtp {
		t.Errorf("expected code %q, got %v", CodeErrorInvalidOtp, body["code"])
	}
}
