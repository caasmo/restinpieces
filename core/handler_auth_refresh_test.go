package core

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/caasmo/restinpieces/config"
	"github.com/caasmo/restinpieces/db"
)

func TestRefreshAuthHandlerValid(t *testing.T) {
	testUser := &db.User{
		ID:       "testuser123",
		Email:    "test@example.com",
		Password: "hashed_password",
	}

	testCases := []struct {
		name      string
		wantError jsonResponse
		authSetup func(*MockAuth)
		desc      string
	}{
		{
			name: "valid token refresh",
			authSetup: func(m *MockAuth) {
				m.AuthenticateFunc = func(r *http.Request) (*db.User, jsonResponse, error) {
					return testUser, jsonResponse{}, nil
				}
			},
			desc: "When authentication succeeds, should return new token",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockAuth := &MockAuth{}
			if tc.authSetup != nil {
				tc.authSetup(mockAuth)
			}

			mockValidator := &MockValidator{
				ContentTypeFunc: func(r *http.Request, allowedType string) (jsonResponse, error) {
					return jsonResponse{}, nil
				},
			}

			a := &App{
				authenticator: mockAuth,
				validator:     mockValidator,
				configProvider: config.NewProvider(&config.Config{
					Jwt: config.Jwt{
						AuthSecret:        "test_secret_32_bytes_long_xxxxxx",
						AuthTokenDuration: config.Duration{Duration: 15 * time.Minute},
					},
				}),
			}

			req := httptest.NewRequest("POST", "/auth-refresh", nil)
			rr := httptest.NewRecorder()

			a.RefreshAuthHandler(rr, req)

			var resp JsonWithData
			if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
				t.Fatalf("failed to decode response body: %v", err)
			}

			if resp.Status != http.StatusOK {
				t.Errorf("expected status %d, got %d", http.StatusOK, resp.Status)
			}
			if resp.Code != CodeOkAuthentication {
				t.Errorf("expected code %q, got %q", CodeOkAuthentication, resp.Code)
			}

			authData, ok := resp.Data.(map[string]interface{})
			if !ok {
				t.Fatalf("expected Data to be an AuthData map, got %T", resp.Data)
			}

			requiredFields := []string{"access_token", "token_type", "record"}
			for _, field := range requiredFields {
				if _, ok := authData[field]; !ok {
					t.Errorf("response data missing required field: %s", field)
				}
			}

			if authData["token_type"] != "Bearer" {
				t.Errorf("expected token_type Bearer, got %s", authData["token_type"])
			}
		})
	}
}

func TestRefreshAuthHandlerError(t *testing.T) {
	testCases := []struct {
		name      string
		wantError jsonResponse
		authSetup func(*MockAuth)
		desc      string
	}{
		{
			name:      "authentication error",
			wantError: errorInvalidCredentials,
			authSetup: func(m *MockAuth) {
				m.AuthenticateFunc = func(r *http.Request) (*db.User, jsonResponse, error) {
					return nil, errorInvalidCredentials, errors.New("auth error")
				}
			},
			desc: "When authentication fails, should return error",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockAuth := &MockAuth{}
			if tc.authSetup != nil {
				tc.authSetup(mockAuth)
			}

			mockValidator := &MockValidator{
				ContentTypeFunc: func(r *http.Request, allowedType string) (jsonResponse, error) {
					return jsonResponse{}, nil
				},
			}

			a := &App{
				authenticator: mockAuth,
				validator:     mockValidator,
				configProvider: config.NewProvider(&config.Config{
					Jwt: config.Jwt{
						AuthSecret:        "test_secret_32_bytes_long_xxxxxx",
						AuthTokenDuration: config.Duration{Duration: 15 * time.Minute},
					},
				}),
			}

			req := httptest.NewRequest("POST", "/auth-refresh", nil)
			rr := httptest.NewRecorder()

			a.RefreshAuthHandler(rr, req)

			var resp JsonBasic
			if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
				t.Fatalf("failed to decode error response: %v", err)
			}

			if rr.Code != tc.wantError.status {
				t.Errorf("expected status %d, got %d", tc.wantError.status, rr.Code)
			}

			var wantResp JsonBasic
			if err := json.Unmarshal(tc.wantError.body, &wantResp); err != nil {
				t.Fatalf("failed to unmarshal wantError: %v", err)
			}

			if resp.Code != wantResp.Code {
				t.Errorf("expected error code %q, got %q", wantResp.Code, resp.Code)
			}
		})
	}
}
