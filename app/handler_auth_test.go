package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/caasmo/restinpieces/config"
	"github.com/caasmo/restinpieces/db"
)

func TestRequestVerificationHandler(t *testing.T) {
	testCases := []struct {
		name       string
		email      string
		wantStatus int
	}{
		{
			name:       "valid email request",
			email:      "test@example.com",
			wantStatus: http.StatusAccepted,
		},
		{
			name:       "invalid email format",
			email:      "not-an-email",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "empty email",
			email:      "",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "non-existent email",
			email:      "nonexistent@example.com",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "already verified email",
			email:      "verified@example.com",
			wantStatus: http.StatusConflict,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			reqBody := fmt.Sprintf(`{"email":"%s"}`, tc.email)
			req := httptest.NewRequest("POST", "/request-verification", strings.NewReader(reqBody))
			req.Header.Set("Content-Type", "application/json")
			
			rr := httptest.NewRecorder()
			mockDB := &MockDB{
                GetUserByEmailConfig: struct {
                    User  *db.User
                    Error error
                }{
                    User: &db.User{
                        ID:       "test456",
                        Email:    tc.email,
                        Name:     "Test User",
                        Password: "hash123",
                        Created:  time.Time{},
                        Updated:  time.Time{},
                        Verified: tc.email == "verified@example.com",
                    },
                },
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
