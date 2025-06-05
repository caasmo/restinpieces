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
		name       string
		json       string
		wantStatus int
	}{
		{
			name:       "invalid email format",
			json:       `{"email":"not-an-email"}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "empty email",
			json:       `{"email":""}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "missing email field",
			json:       `{}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid JSON",
			json:       `{"email": invalid}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "empty email",
			json:       `{"email":""}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "missing email field",
			json:       `{}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid JSON",
			json:       `{"email": invalid}`,
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			reqBody := tc.json
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

			if rr.Code != tc.wantStatus {
				t.Errorf("expected status %d, got %d", tc.wantStatus, rr.Code)
			}

			// Validate error response body when expected
			if tc.wantStatus >= 400 {
				var resp map[string]interface{}
				if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
					t.Fatalf("failed to decode error response: %v", err)
				}

				if _, ok := resp["message"]; !ok {
					t.Error("error response missing 'message' field")
				}
			}
		})
	}
}

func TestRequestVerificationHandlerDatabase(t *testing.T) {
	testCases := []struct {
		name       string
		json       string
		dbSetup    func(*MockDB) // Configures mock DB behavior
		wantStatus int
		desc       string // Description of test case
	}{
		{
			name: "email exists but user is nil",
			json: `{"email":"niluser@example.com"}`,
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
			json: `{"email":"unverified@example.com"}`,
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
			json: `{"email":"verified@example.com"}`,
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
			json: `{"email":"error@example.com"}`,
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
			reqBody := tc.json
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
