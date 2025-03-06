package app_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/caasmo/restinpieces/app"
	"github.com/golang-jwt/jwt/v5"
)

func testApp() *app.App {
	return &app.App{
		config: &app.Config{
			JwtSecret:     []byte("test_secret"),
			TokenDuration: 15 * time.Minute,
		},
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
			a := testApp()
			
			// Add user ID to context directly since middleware would normally handle this
			ctx := context.WithValue(req.Context(), app.UserIDKey, tc.userID)
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
				if expiresIn := body["expires_in"].(float64); expiresIn != 900 {
					t.Errorf("expected expires_in 900, got %v", expiresIn)
				}
			}
		})
	}
}
