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
)

func TestRequestVerificationHandlerRequestValidation(t *testing.T) {
	testCases := []struct {
		name      string
		requestBody string
		wantError   jsonResponse
	}{
		{
			name:      "invalid email format",
			requestBody: `{"email":"not-an-email"}`,
			wantError: errorInvalidRequest,
		},
		{
			name:      "empty email",
			requestBody: `{"email":""}`,
			wantError: errorInvalidRequest,
		},
		{
			name:      "missing email field",
			requestBody: `{}`,
			wantError: errorInvalidRequest,
		},
		{
			name:      "invalid JSON",
			requestBody: `{"email": invalid`,
			wantError: errorInvalidRequest,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			reqBody := tc.requestBody
			req := httptest.NewRequest("POST", "/request-verification", strings.NewReader(reqBody))
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			a := &App{}

			a.RequestEmailVerificationHandler(rr, req)

			if rr.Code != tc.wantError.status {
				t.Errorf("expected status %d, got %d", tc.wantError.status, rr.Code)
			}

			// Compare response bodies
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

func TestRequestVerificationHandlerAuth(t *testing.T) {
	testCases := []struct {
		name        string
		requestBody string
		mockAuth    func(r *http.Request) (*db.User, error, jsonResponse)
		wantError   jsonResponse
	}{
		{
			name:        "authenticated user already verified",
			requestBody: `{"email":"verified@example.com"}`,
			mockAuth: func(r *http.Request) (*db.User, error, jsonResponse) {
				return &db.User{
					Email:    "verified@example.com",
					Verified: true,
				}, nil, jsonResponse{}
			},
			wantError: okAlreadyVerified,
		},
		{
			name:        "authenticated user email mismatch",
			requestBody: `{"email":"other@example.com"}`,
			mockAuth: func(r *http.Request) (*db.User, error, jsonResponse) {
				return &db.User{
					Email:    "verified@example.com",
					Verified: false,
				}, nil, jsonResponse{}
			},
			wantError: errorEmailConflict,
		},
		{
			name:        "unauthenticated request",
			requestBody: `{"email":"test@example.com"}`,
			mockAuth: func(r *http.Request) (*db.User, error, jsonResponse) {
				return nil, nil, errorInvalidCredentials
			},
			wantError: errorInvalidCredentials,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/request-verification", strings.NewReader(tc.requestBody))
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			
			mockAuth := &MockAuth{
				AuthenticateFunc: tc.mockAuth,
			}

			a := &App{
				authenticator: mockAuth,
				configProvider: config.NewProvider(&config.Config{
					Jwt: config.Jwt{
						AuthSecret:        "test_secret_32_bytes_long_xxxxxx",
						AuthTokenDuration: config.Duration{Duration: 15 * time.Minute},
					},
				}),
			}

			a.RequestEmailVerificationHandler(rr, req)

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

func TestRequestVerificationHandlerDatabase(t *testing.T) {
	testCases := []struct {
		name        string
		requestBody string
		dbSetup     func(*MockDB)
		wantError   jsonResponse
	}{
		{
			name: "database unique constraint error",
			requestBody: `{"email":"test@example.com"}`,
			dbSetup: func(mockDB *MockDB) {
				mockDB.GetUserByEmailFunc = func(email string) (*db.User, error) {
					return &db.User{
						ID:       "test123",
						Email:    "test@example.com",
						Verified: false,
					}, nil
				}
				mockDB.InsertJobFunc = func(job db.Job) error {
					return db.ErrConstraintUnique
				}
			},
			wantError: errorEmailVerificationAlreadyRequested,
		},
		{
			name: "database other error",
			requestBody: `{"email":"test@example.com"}`,
			dbSetup: func(mockDB *MockDB) {
				mockDB.GetUserByEmailFunc = func(email string) (*db.User, error) {
					return &db.User{
						ID:       "test123",
						Email:    "test@example.com",
						Verified: false,
					}, nil
				}
				mockDB.InsertJobFunc = func(job db.Job) error {
					return errors.New("database connection failed")
				}
			},
			wantError: errorServiceUnavailable,
		},
		{
			name: "successful job insertion",
			requestBody: `{"email":"test@example.com"}`,
			dbSetup: func(mockDB *MockDB) {
				mockDB.GetUserByEmailFunc = func(email string) (*db.User, error) {
					return &db.User{
						ID:       "test123",
						Email:    "test@example.com",
						Verified: false,
					}, nil
				}
				mockDB.InsertJobFunc = func(job db.Job) error {
					return nil
				}
			},
			wantError: okVerificationRequested,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/request-verification", strings.NewReader(tc.requestBody))
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			mockDB := &MockDB{}
			if tc.dbSetup != nil {
				tc.dbSetup(mockDB)
			}

			// Setup mock authenticator to return valid user
			mockAuth := &MockAuth{
				AuthenticateFunc: func(r *http.Request) (*db.User, error, jsonResponse) {
					return &db.User{
						Email:    "test@example.com",
						Verified: false,
					}, nil, jsonResponse{}
				},
			}

			a := &App{
				authenticator: mockAuth,
				configProvider: config.NewProvider(&config.Config{
					Jwt: config.Jwt{
						AuthSecret:        "test_secret_32_bytes_long_xxxxxx",
						AuthTokenDuration: config.Duration{Duration: 15 * time.Minute},
					},
				}),
				dbAuth:   mockDB,
				dbQueue:  mockDB,
				dbConfig: mockDB,
			}

			a.RequestEmailVerificationHandler(rr, req)

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
