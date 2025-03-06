package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/caasmo/restinpieces/jwt"
)

func TestJwtValidateMiddleware(t *testing.T) {
	testCases := []struct {
		name           string
		authHeader     string
		wantStatus     int
		wantError      string
		expectUserID   bool
	}{
		{
			name:         "valid token",
			authHeader:   "Bearer " + generateTestToken(t, "testuser123"),
			wantStatus:   http.StatusOK,
			expectUserID: true,
		},
		{
			name:       "missing authorization header",
			authHeader: "",
			wantStatus: http.StatusUnauthorized,
			wantError:  "Authorization header required",
		},
		{
			name:       "invalid token format",
			authHeader: "InvalidToken",
			wantStatus: http.StatusUnauthorized,
			wantError:  "Invalid authorization format",
		},
		{
			name:       "expired token",
			authHeader: "Bearer " + generateExpiredTestToken(t, "testuser123"),
			wantStatus: http.StatusUnauthorized,
			wantError:  "Token expired",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/protected", nil)
			if tc.authHeader != "" {
				req.Header.Set("Authorization", tc.authHeader)
			}

			rr := httptest.NewRecorder()
			a, _ := New(
				WithConfig(&Config{
					JwtSecret:     []byte("test_secret"),
					TokenDuration: 15 * time.Minute,
				}),
				WithDB(&MockDB{}),
				WithRouter(&MockRouter{}),
			)

			// Create a test handler that checks for user ID in context
			testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				userID, ok := r.Context().Value(UserIDKey).(string)
				if tc.expectUserID && !ok {
					t.Error("Expected user ID in context but none found")
				}
				_ = userID // Silence unused var check
				w.WriteHeader(http.StatusOK)
			})

			// Apply the middleware and serve the request
			middleware := a.JwtValidate(testHandler)
			middleware.ServeHTTP(rr, req)

			if rr.Code != tc.wantStatus {
				t.Errorf("expected status %d, got %d", tc.wantStatus, rr.Code)
			}

			if tc.wantError != "" {
				var body map[string]interface{}
				if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
					t.Fatalf("failed to decode response body: %v", err)
				}

				if body["error"] != tc.wantError {
					t.Errorf("expected error %q, got %q", tc.wantError, body["error"])
				}
			}
		})
	}
}

func generateTestToken(t *testing.T, userID string) string {
	t.Helper()
	token, _, err := jwt.Create(userID, []byte("test_secret"), 15*time.Minute)
	if err != nil {
		t.Fatalf("failed to generate test token: %v", err)
	}
	return token
}

func generateExpiredTestToken(t *testing.T, userID string) string {
	t.Helper()
	token, _, err := jwt.Create(userID, []byte("test_secret"), -15*time.Minute) // Negative duration for expired token
	if err != nil {
		t.Fatalf("failed to generate expired test token: %v", err)
	}
	return token
}
