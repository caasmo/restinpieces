package core

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



func TestRefreshAuthHandler(t *testing.T) {
	testCases := []struct {
		name       string
		userID     string
		wantStatus int
		dbSetup    func(*MockDB)
		desc       string
	}{
		{
			name:       "valid token refresh",
			userID:     "testuser123",
			wantStatus: http.StatusOK,
			dbSetup: func(mockDB *MockDB) {
				mockDB.GetUserByIdFunc = func(id string) (*db.User, error) {
					return &db.User{
						ID:    "testuser123",
						Email: "test@example.com",
					}, nil
				}
			},
			desc: "When valid user ID is present in context and user exists in database, should return new token",
		},
		{
			name:       "missing user claims",
			userID:     "",
			wantStatus: http.StatusInternalServerError,
			desc:       "When user ID is missing from context, should return 500 error",
		},
		{
			name:       "user not found",
			userID:     "nonexistent",
			wantStatus: http.StatusUnauthorized,
			dbSetup: func(mockDB *MockDB) {
				mockDB.GetUserByIdFunc = func(id string) (*db.User, error) {
					return nil, nil
				}
			},
			desc: "When user ID exists but user is not found in database, should return 401 unauthorized",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup request with context
			req := httptest.NewRequest("POST", "/auth-refresh", nil)
			ctx := context.WithValue(req.Context(), UserIDKey, tc.userID)
			req = req.WithContext(ctx)

			// Setup response recorder
			rr := httptest.NewRecorder()

			// Configure mock DB if needed
			mockDB := &MockDB{}
			if tc.dbSetup != nil {
				tc.dbSetup(mockDB)
			}

			// Create app with test config
			a, _ := New(
				WithConfig(&config.Config{
					Jwt: config.Jwt{
						AuthSecret:        []byte("test_secret_32_bytes_long_xxxxxx"),
						AuthTokenDuration: 15 * time.Minute,
					},
				}),
				WithDB(mockDB),
				WithRouter(&MockRouter{}),
			)

			// Execute handler
			a.RefreshAuthHandler(rr, req)

			// Verify status code
			if rr.Code != tc.wantStatus {
				t.Errorf("expected status %d, got %d", tc.wantStatus, rr.Code)
			}

			// For successful responses, verify token format
			if tc.wantStatus == http.StatusOK {
				var body map[string]interface{}
				if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
					t.Fatalf("failed to decode response body: %v", err)
				}

				// Verify required token fields
				requiredFields := []string{"access_token", "token_type", "expires_in"}
				for _, field := range requiredFields {
					if _, ok := body[field]; !ok {
						t.Errorf("response missing required field: %s", field)
					}
				}

				// Verify token type
				if body["token_type"] != "Bearer" {
					t.Errorf("expected token_type Bearer, got %s", body["token_type"])
				}
			}
		})
	}
}
