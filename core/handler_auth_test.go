package core

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/caasmo/restinpieces/config"
	"github.com/caasmo/restinpieces/db"
)



func TestRefreshAuthHandler(t *testing.T) {
	testUser := &db.User{
		ID:       "testuser123",
		Email:    "test@example.com",
		Password: "hashed_password",
	}

	testCases := []struct {
		name       string
		wantError  jsonResponse
		authSetup  func(*MockAuth)
		desc       string
	}{
		{
			name:      "valid token refresh",
			wantError: jsonResponse{},
			authSetup: func(m *MockAuth) {
				m.AuthenticateFunc = func(r *http.Request) (*db.User, error, jsonResponse) {
					return testUser, nil, jsonResponse{}
				}
			},
			desc: "When authentication succeeds, should return new token",
		},
		{
			name:      "authentication error",
			wantError: errorInvalidCredentials,
			authSetup: func(m *MockAuth) {
				m.AuthenticateFunc = func(r *http.Request) (*db.User, error, jsonResponse) {
					return nil, errors.New("auth error"), errorInvalidCredentials
				}
			},
			desc: "When authentication fails, should return error",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup mocks
			mockAuth := &MockAuth{}
			if tc.authSetup != nil {
				tc.authSetup(mockAuth)
			}

			mockValidator := &MockValidator{
				ContentTypeFunc: func(r *http.Request, allowedType string) (error, jsonResponse) {
					return nil, jsonResponse{}
				},
			}

			// Create app with test config
			a := &App{
				auth:      mockAuth,
				validator: mockValidator,
				configProvider: config.NewProvider(&config.Config{
					Jwt: config.Jwt{
						AuthSecret:        "test_secret_32_bytes_long_xxxxxx",
						AuthTokenDuration: config.Duration{Duration: 15 * time.Minute},
					},
				}),
			}

			// Setup request
			req := httptest.NewRequest("POST", "/auth-refresh", nil)
			rr := httptest.NewRecorder()

			// Execute handler
			a.RefreshAuthHandler(rr, req)

			// Check for expected error response
			if tc.wantError.status != 0 {
				var resp JsonBasic
				if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
					t.Fatalf("failed to decode error response: %v", err)
				}

				if rr.Code != tc.wantError.status {
					t.Errorf("expected status %d, got %d", tc.wantError.status, rr.Code)
				}

				// Compare with expected error code
				var wantResp JsonBasic
				if err := json.Unmarshal(tc.wantError.body, &wantResp); err != nil {
					t.Fatalf("failed to unmarshal wantError: %v", err)
				}

				if resp.Code != wantResp.Code {
					t.Errorf("expected error code %q, got %q", wantResp.Code, resp.Code)
				}
			} else {
				// Verify successful token response
				var body map[string]interface{}
				if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
					t.Fatalf("failed to decode response body: %v", err)
				}

				requiredFields := []string{"access_token", "token_type", "expires_in"}
				for _, field := range requiredFields {
					if _, ok := body[field]; !ok {
						t.Errorf("response missing required field: %s", field)
					}
				}

				if body["token_type"] != "Bearer" {
					t.Errorf("expected token_type Bearer, got %s", body["token_type"])
				}
			}
		})
	}
}
