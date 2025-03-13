package app

import (
	"context"
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
			a, _ := New(
				WithConfig(&config.Config{
					JwtSecret:     []byte("test_secret_32_bytes_long_xxxxxx"),
					TokenDuration: 15 * time.Minute,
				}),
				WithRouter(&MockRouter{}),
			)

			a.RequestVerificationHandler(rr, req)

			if rr.Code != tc.wantStatus {
				t.Errorf("expected status %d, got %d", tc.wantStatus, rr.Code)
			}

			// Validate error response body when expected
			if tc.wantStatus >= 400 {
				var resp map[string]interface{}
				if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
					t.Fatalf("failed to decode error response: %v", err)
				}

				if _, ok := resp["error"]; !ok {
					t.Error("error response missing 'error' field")
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
				mockDB.GetUserByEmailConfig.User = nil
			},
			wantStatus: http.StatusNotFound,
			desc:       "When email exists but GetUserByEmail returns nil user, should return 404",
		},
		{
			name: "email exists but user not verified",
			json: `{"email":"unverified@example.com"}`,
			dbSetup: func(mockDB *MockDB) {
				mockDB.GetUserByEmailConfig.User = &db.User{
					ID:       "test123",
					Email:    "unverified@example.com",
					Verified: false,
				}
			},
			wantStatus: http.StatusAccepted,
			desc:       "When email exists and user is not verified, should return 202 Accepted",
		},
		{
			name: "email exists and user is verified",
			json: `{"email":"verified@example.com"}`,
			dbSetup: func(mockDB *MockDB) {
				mockDB.GetUserByEmailConfig.User = &db.User{
					ID:       "test456",
					Email:    "verified@example.com",
					Verified: true,
				}
			},
			wantStatus: http.StatusConflict,
			desc:       "When email exists and user is already verified, should return 409 Conflict",
		},
		{
			name: "database error",
			json: `{"email":"error@example.com"}`,
			dbSetup: func(mockDB *MockDB) {
				mockDB.GetUserByEmailConfig.Error = errors.New("database connection failed")
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

			a, _ := New(
				WithConfig(&config.Config{
					JwtSecret:     []byte("test_secret_32_bytes_long_xxxxxx"),
					TokenDuration: 15 * time.Minute,
				}),
				WithDB(mockDB),
				WithRouter(&MockRouter{}),
			)

			a.RequestVerificationHandler(rr, req)

			if rr.Code != tc.wantStatus {
				t.Errorf("expected status %d, got %d", tc.wantStatus, rr.Code)
			}
		})
	}
}

func TestRefreshAuthHandler(t *testing.T) {
	testCases := []struct {
		name       string
		userID     string
		wantStatus int
	}{
		{
			name:       "valid token refresh",
			userID:     "testuser123",
			wantStatus: http.StatusOK,
		},
		{
			name:       "missing user claims",
			userID:     "",
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/auth-refresh", nil)
			rr := httptest.NewRecorder()
			a, _ := New(
				WithConfig(&config.Config{
					JwtSecret:     []byte("test_secret_32_bytes_long_xxxxxx"), // 32-byte secret
					TokenDuration: 15 * time.Minute,
				}),
				WithDB(&MockDB{}),
				WithRouter(&MockRouter{}),
			)

			// Add user ID to context directly since middleware would normally handle this
			ctx := context.WithValue(req.Context(), UserIDKey, tc.userID)
			a.RefreshAuthHandler(rr, req.WithContext(ctx))

			if rr.Code != tc.wantStatus {
				t.Errorf("expected status %d, got %d", tc.wantStatus, rr.Code)
			}

			if tc.wantStatus == http.StatusOK {
				var body map[string]interface{}
				if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
					t.Fatalf("failed to decode response body: %v", err)
				}

				if _, ok := body["access_token"]; !ok {
					t.Error("response missing access_token")
				}
				if body["token_type"] != "Bearer" {
					t.Errorf("expected token_type Bearer, got %s", body["token_type"])
				}
			}
		})
	}
}
