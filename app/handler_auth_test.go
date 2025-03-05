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
		// Add minimal required dependencies
	}
}

func TestRefreshAuthHandler(t *testing.T) {
	validSecret := []byte("your_jwt_secret_here")
	now := time.Now()

	testCases := []struct {
		name       string
		tokenFn    func() string
		wantStatus int
		wantBody   map[string]interface{}
	}{
		{
			name: "valid token refresh",
			tokenFn: func() string {
				token, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
					ExpiresAt: jwt.NewNumericDate(now.Add(1 * time.Hour)),
					IssuedAt:  jwt.NewNumericDate(now),
				}).SignedString(validSecret)
				return token
			},
			wantStatus: http.StatusOK,
			wantBody: map[string]interface{}{
				"token_type": "Bearer",
				"expires_in": float64(21600),
			},
		},
		{
			name: "expired token",
			tokenFn: func() string {
				token, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
					ExpiresAt: jwt.NewNumericDate(now.Add(-1 * time.Hour)),
				}).SignedString(validSecret)
				return token
			},
			wantStatus: http.StatusUnauthorized,
			wantBody: map[string]interface{}{
				"error": "Invalid token: token has invalid claims: token is expired",
			},
		},
		{
			name:       "missing authorization header",
			tokenFn:    func() string { return "" },
			wantStatus: http.StatusUnauthorized,
			wantBody: map[string]interface{}{
				"error": "Authorization header required",
			},
		},
		{
			name: "invalid token format",
			tokenFn: func() string {
				return "invalidtoken"
			},
			wantStatus: http.StatusUnauthorized,
			wantBody: map[string]interface{}{
				"error": "Invalid token: token is malformed: token contains an invalid number of segments",
			},
		},
		{
			name: "tampered token",
			tokenFn: func() string {
				token, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{}).SignedString([]byte("wrong_secret"))
				return token
			},
			wantStatus: http.StatusUnauthorized,
			wantBody: map[string]interface{}{
				"error": "Invalid token:",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/auth-refresh", nil)
			if token := tc.tokenFn(); token != "" {
				req.Header.Set("Authorization", "Bearer "+token)
			}

			rr := httptest.NewRecorder()
			a := testApp()
			a.RefreshAuthHandler(rr, req)

			if rr.Code != tc.wantStatus {
				t.Errorf("expected status %d, got %d", tc.wantStatus, rr.Code)
			}

			var body map[string]interface{}
			if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
				t.Fatalf("failed to decode response body: %v", err)
			}

			// Special case for error messages with dynamic content
			if tc.wantStatus != http.StatusOK {
				wantErr := tc.wantBody["error"].(string)
				gotErr := body["error"].(string)
				if !bytes.Contains([]byte(gotErr), []byte(wantErr)) {
					t.Errorf("expected error to contain %q, got %q", wantErr, gotErr)
				}
			} else {
				// Validate token response structure
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
