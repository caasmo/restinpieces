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
			requestBody: `{"email": invalid}`,
			wantError: errorInvalidRequest,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			reqBody := tc.requestBody
			req := httptest.NewRequest("POST", "/request-verification", strings.NewReader(reqBody))
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			a := &App{
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

func TestRequestVerificationHandlerDatabase(t *testing.T) {
	testCases := []struct {
		name       string
		requestBody string
		dbSetup    func(*MockDB) // Configures mock DB behavior
		wantStatus int
		desc       string // Description of test case
	}{
		{
			name: "email exists but user is nil",
			requestBody: `{"email":"niluser@example.com"}`,
			dbSetup: func(mockDB *MockDB) {
				mockDB.GetUserByEmailFunc = func(email string) (*db.User, error) {
					return nil, nil
				}
			},
			wantStatus: http.StatusNotFound,
			desc:       "When email exists but GetUserByEmail returns nil user, should return 404",
		},
		{
			name: "email exists but user not verified",
			requestBody: `{"email":"unverified@example.com"}`,
			dbSetup: func(mockDB *MockDB) {
				mockDB.GetUserByEmailFunc = func(email string) (*db.User, error) {
					return &db.User{
						ID:       "test123",
						Email:    "unverified@example.com",
						Verified: false,
					}, nil
				}
			},
			wantStatus: http.StatusAccepted,
			desc:       "When email exists and user is not verified, should return 202 Accepted",
		},
		{
			name: "email exists and user is verified",
			requestBody: `{"email":"verified@example.com"}`,
			dbSetup: func(mockDB *MockDB) {
				mockDB.GetUserByEmailFunc = func(email string) (*db.User, error) {
					return &db.User{
						ID:       "test456",
						Email:    "verified@example.com",
						Verified: true,
					}, nil
				}
			},
			wantStatus: http.StatusConflict,
			desc:       "When email exists and user is already verified, should return 409 Conflict",
		},
		{
			name: "database error",
			requestBody: `{"email":"error@example.com"}`,
			dbSetup: func(mockDB *MockDB) {
				mockDB.GetUserByEmailFunc = func(email string) (*db.User, error) {
					return nil, errors.New("database connection failed")
				}
			},
			wantStatus: http.StatusInternalServerError,
			desc:       "When database query fails, should return 500 Internal Server Error",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			reqBody := tc.requestBody
			req := httptest.NewRequest("POST", "/request-verification", strings.NewReader(reqBody))
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			mockDB := &MockDB{}
			if tc.dbSetup != nil {
				tc.dbSetup(mockDB)
			}

			a := &App{
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

			if rr.Code != tc.wantStatus {
				t.Errorf("expected status %d, got %d", tc.wantStatus, rr.Code)
			}
		})
	}
}
