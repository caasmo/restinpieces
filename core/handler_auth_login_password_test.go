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

// TestAuthWithPasswordHandler_Validation tests input validation scenarios for the
// password login handler. It covers cases like invalid content type, malformed JSON,
// missing fields, and invalid email formats.
func TestAuthWithPasswordHandler_Validation(t *testing.T) {
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
			requestBody: `{"identity":"test@example.com", "password":"password123"}`,
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
			requestBody: `{"identity":"test@example.com",`,
			wantError:   errorInvalidRequest,
			setupValidator: func(m *MockValidator) {
				m.ContentTypeFunc = func(r *http.Request, allowedType string) (jsonResponse, error) {
					return jsonResponse{}, nil
				}
			},
		},
		{
			name:        "missing identity field",
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
			requestBody: `{"identity":"test@example.com"}`,
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
			requestBody: `{"identity":"not-an-email", "password":"password123"}`,
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
			req := httptest.NewRequest("POST", "/auth-with-password", strings.NewReader(tc.requestBody))
			req.Header.Set("Content-Type", tc.contentType)
			rr := httptest.NewRecorder()

			mockValidator := &MockValidator{}
			tc.setupValidator(mockValidator)

			app := &App{
				validator: mockValidator,
			}

			app.AuthWithPasswordHandler(rr, req)

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

// TestAuthWithPasswordHandler_Authentication tests the core authentication logic,
// including successful login, user not found, and incorrect password scenarios.
func TestAuthWithPasswordHandler_Authentication(t *testing.T) {
	hashedPassword, _ := crypto.GenerateHash("password123")
	testUser := &db.User{
		ID:       "user123",
		Email:    "test@example.com",
		Password: string(hashedPassword),
		Verified: true,
	}

	testCases := []struct {
		name        string
		requestBody string
		dbSetup     func(*mock.Db)
		wantStatus  int
		wantCode    string
	}{
		{
			name:        "successful login",
			requestBody: `{"identity":"test@example.com", "password":"password123"}`,
			dbSetup: func(m *mock.Db) {
				m.GetUserByEmailFunc = func(email string) (*db.User, error) {
					return testUser, nil
				}
			},
			wantStatus: http.StatusOK,
			wantCode:   CodeOkAuthentication,
		},
		{
			name:        "user not found",
			requestBody: `{"identity":"notfound@example.com", "password":"password123"}`,
			dbSetup: func(m *mock.Db) {
				m.GetUserByEmailFunc = func(email string) (*db.User, error) {
					return nil, db.ErrUserNotFound
				}
			},
			wantStatus: http.StatusUnauthorized,
			wantCode:   CodeErrorInvalidCredentials,
		},
		{
			name:        "incorrect password",
			requestBody: `{"identity":"test@example.com", "password":"wrongpassword"}`,
			dbSetup: func(m *mock.Db) {
				m.GetUserByEmailFunc = func(email string) (*db.User, error) {
					return testUser, nil
				}
			},
			wantStatus: http.StatusUnauthorized,
			wantCode:   CodeErrorInvalidCredentials,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/auth-with-password", strings.NewReader(tc.requestBody))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()

			mockDb := &mock.Db{}
			tc.dbSetup(mockDb)

			app := &App{
				validator: &DefaultValidator{},
				dbAuth:    mockDb,
				configProvider: config.NewProvider(&config.Config{
					Jwt: config.Jwt{
						AuthSecret:        "test_secret_32_bytes_long_xxxxxx",
						AuthTokenDuration: config.Duration{Duration: 15 * time.Minute},
					},
				}),
			}

			app.AuthWithPasswordHandler(rr, req)

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

			if tc.wantStatus == http.StatusOK {
				data, ok := body["data"].(map[string]interface{})
				if !ok {
					t.Fatal("expected 'data' field in successful response")
				}
				if _, ok := data["access_token"]; !ok {
					t.Error("successful response missing 'access_token'")
				}
			}
		})
	}
}

// TestAuthWithPasswordHandler_DependencyFailures tests how the handler behaves when
// its dependencies, such as the database or token generation, fail.
func TestAuthWithPasswordHandler_DependencyFailures(t *testing.T) {
	testCases := []struct {
		name        string
		dbSetup     func(*mock.Db)
		config      *config.Config
		wantError   jsonResponse
	}{
		{
			name: "database failure on user lookup",
			dbSetup: func(m *mock.Db) {
				m.GetUserByEmailFunc = func(email string) (*db.User, error) {
					return nil, errors.New("db connection failed")
				}
			},
			config: &config.Config{
				Jwt: config.Jwt{
					AuthSecret: "test_secret_32_bytes_long_xxxxxx",
				},
			},
			wantError: errorInvalidCredentials,
		},
		{
			name: "jwt generation failure",
			dbSetup: func(m *mock.Db) {
				hashedPassword, _ := crypto.GenerateHash("password123")
				m.GetUserByEmailFunc = func(email string) (*db.User, error) {
					return &db.User{
						ID: "user123",
						Email: "test@example.com",
						Password: string(hashedPassword),
					}, nil
				}
			},
			config: &config.Config{
				Jwt: config.Jwt{
					AuthSecret: "short", // Invalid secret to cause error
				},
			},
			wantError: errorTokenGeneration,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			reqBody := `{"identity":"test@example.com", "password":"password123"}`
			req := httptest.NewRequest("POST", "/auth-with-password", strings.NewReader(reqBody))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()

			mockDb := &mock.Db{}
			tc.dbSetup(mockDb)

			app := &App{
				validator:      &DefaultValidator{},
				dbAuth:         mockDb,
				configProvider: config.NewProvider(tc.config),
			}

			app.AuthWithPasswordHandler(rr, req)

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
