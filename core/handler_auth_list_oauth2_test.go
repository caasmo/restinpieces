package core

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"testing"

	"github.com/caasmo/restinpieces/config"
)

// TestListOAuth2ProvidersHandler_Success tests the happy path scenarios for the
// ListOAuth2ProvidersHandler. It verifies that for valid configurations, the handler
// returns a correct and structurally sound list of OAuth2 providers.
func TestListOAuth2ProvidersHandler_Success(t *testing.T) {
	testCases := []struct {
		name             string
		setupConfig      func() *config.Config
		validateResponse func(t *testing.T, rr *httptest.ResponseRecorder, cfg *config.Config)
	}{
		{
			name: "single provider without PKCE",
			setupConfig: func() *config.Config {
				cfg := config.NewDefaultConfig()
				cfg.OAuth2Providers = map[string]config.OAuth2Provider{
					"google": {
						DisplayName:  "Google",
						ClientID:     "google-client-id",
						ClientSecret: "google-client-secret",
						RedirectURL:  "https://app.example.com/oauth2/callback",
						Scopes:       []string{"email", "profile"},
						AuthURL:      "https://accounts.google.com/o/oauth2/v2/auth",
						TokenURL:     "https://oauth2.googleapis.com/token",
						PKCE:         false,
					},
				}
				return cfg
			},
			validateResponse: func(t *testing.T, rr *httptest.ResponseRecorder, cfg *config.Config) {
				var resp JsonWithData
				if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
					t.Fatalf("Failed to unmarshal response: %v", err)
				}

				if resp.Status != http.StatusOK {
					t.Errorf("expected status %d, got %d", http.StatusOK, resp.Status)
				}
				if resp.Code != CodeOkOAuth2ProvidersList {
					t.Errorf("expected code %s, got %s", CodeOkOAuth2ProvidersList, resp.Code)
				}

				data, ok := resp.Data.(map[string]interface{})
				if !ok {
					t.Fatal("Data field is not a map")
				}
				providersData, ok := data["providers"]
				if !ok {
					t.Fatal("Missing 'providers' key in data")
				}

				var providers []OAuth2ProviderInfo
				providersBytes, _ := json.Marshal(providersData)
				if err := json.Unmarshal(providersBytes, &providers); err != nil {
					t.Fatalf("Failed to unmarshal providers from response data: %v", err)
				}

				if len(providers) != 1 {
					t.Fatalf("expected 1 provider, got %d", len(providers))
				}

				pInfo := providers[0]
				pConfig := cfg.OAuth2Providers["google"]

				if pInfo.Name != "google" {
					t.Errorf("expected provider name 'google', got '%s'", pInfo.Name)
				}
				if pInfo.DisplayName != pConfig.DisplayName {
					t.Errorf("expected display name '%s', got '%s'", pConfig.DisplayName, pInfo.DisplayName)
				}
				if pInfo.State == "" {
					t.Error("State should not be empty")
				}
				if pInfo.CodeVerifier != "" {
					t.Error("CodeVerifier should be empty for non-PKCE")
				}

				authURL, err := url.Parse(pInfo.AuthURL)
				if err != nil {
					t.Fatalf("Failed to parse AuthURL: %v", err)
				}
				if authURL.Query().Get("state") != pInfo.State {
					t.Errorf("State in URL ('%s') must match state in response ('%s')", authURL.Query().Get("state"), pInfo.State)
				}
			},
		},
		{
			name: "single provider with PKCE",
			setupConfig: func() *config.Config {
				cfg := config.NewDefaultConfig()
				cfg.OAuth2Providers = map[string]config.OAuth2Provider{
					"github": {
						DisplayName: "GitHub",
						ClientID:    "github-client-id",
						PKCE:        true,
					},
				}
				return cfg
			},
			validateResponse: func(t *testing.T, rr *httptest.ResponseRecorder, cfg *config.Config) {
				var resp JsonWithData
				if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
					t.Fatalf("Failed to unmarshal response: %v", err)
				}

				data, ok := resp.Data.(map[string]interface{})
				if !ok {
					t.Fatal("Data field is not a map")
				}
				providersData, ok := data["providers"]
				if !ok {
					t.Fatal("Missing 'providers' key in data")
				}
				var providers []OAuth2ProviderInfo
				providersBytes, _ := json.Marshal(providersData)
				if err := json.Unmarshal(providersBytes, &providers); err != nil {
					t.Fatalf("Failed to unmarshal providers: %v", err)
				}

				if len(providers) != 1 {
					t.Fatalf("expected 1 provider, got %d", len(providers))
				}
				pInfo := providers[0]

				if pInfo.CodeVerifier == "" {
					t.Error("CodeVerifier should not be empty for PKCE")
				}
				if pInfo.CodeChallenge == "" {
					t.Error("CodeChallenge should not be empty for PKCE")
				}
				if pInfo.CodeChallengeMethod != "S256" {
					t.Errorf("expected CodeChallengeMethod 'S256', got '%s'", pInfo.CodeChallengeMethod)
				}

				authURL, err := url.Parse(pInfo.AuthURL)
				if err != nil {
					t.Fatalf("Failed to parse AuthURL: %v", err)
				}
				if authURL.Query().Get("code_challenge") != pInfo.CodeChallenge {
					t.Error("code_challenge in URL does not match CodeChallenge in response")
				}
			},
		},
		{
			name: "redirectUrl logic with path and absolute URL",
			setupConfig: func() *config.Config {
				cfg := config.NewDefaultConfig()
				cfg.Server.Addr = "test.com:443" // BaseURL derives from Addr
				cfg.Server.EnableTLS = true      // to get https scheme
				cfg.OAuth2Providers = map[string]config.OAuth2Provider{
					"providerWithPath": {
						RedirectURLPath: "/callback/path",
					},
					"providerWithURL": {
						RedirectURL: "https://absolute.com/callback",
					},
				}
				return cfg
			},
			validateResponse: func(t *testing.T, rr *httptest.ResponseRecorder, cfg *config.Config) {
				var resp JsonWithData
				if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
					t.Fatalf("Failed to unmarshal response: %v", err)
				}

				data, ok := resp.Data.(map[string]interface{})
				if !ok {
					t.Fatal("Data field is not a map")
				}
				providersData, ok := data["providers"]
				if !ok {
					t.Fatal("Missing 'providers' key in data")
				}
				var providers []OAuth2ProviderInfo
				providersBytes, _ := json.Marshal(providersData)
				if err := json.Unmarshal(providersBytes, &providers); err != nil {
					t.Fatalf("Failed to unmarshal providers: %v", err)
				}

				if len(providers) != 2 {
					t.Fatalf("expected 2 providers, got %d", len(providers))
				}

				providerMap := make(map[string]OAuth2ProviderInfo)
				for _, p := range providers {
					providerMap[p.Name] = p
				}

				pWithPath := providerMap["providerWithPath"]
				// BaseURL() will be https://test.com
				expectedPathURL := cfg.Server.BaseURL() + "/callback/path"
				if pWithPath.RedirectURL != expectedPathURL {
					t.Errorf("expected redirectURL '%s', got '%s'", expectedPathURL, pWithPath.RedirectURL)
				}

				pWithURL := providerMap["providerWithURL"]
				if pWithURL.RedirectURL != "https://absolute.com/callback" {
					t.Errorf("expected redirectURL '%s', got '%s'", "https://absolute.com/callback", pWithURL.RedirectURL)
				}
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := tc.setupConfig()
			provider := config.NewProvider(cfg)
			app := &App{
				configProvider: provider,
				logger:         slog.New(slog.NewTextHandler(io.Discard, nil)),
				validator:      &DefaultValidator{},
			}

			req := httptest.NewRequest("GET", "/list-oauth2-providers", nil)
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()

			app.ListOAuth2ProvidersHandler(rr, req)

			if rr.Code != http.StatusOK {
				t.Fatalf("expected status OK, got %d", rr.Code)
			}
			tc.validateResponse(t, rr, cfg)
		})
	}
}

// TestListOAuth2ProvidersHandler_Errors tests failure scenarios for the handler.
func TestListOAuth2ProvidersHandler_Errors(t *testing.T) {
	testCases := []struct {
		name         string
		setupConfig  func() *config.Config
		setupRequest func(*http.Request)
		wantError    jsonResponse
	}{
		{
			name: "no providers configured",
			setupConfig: func() *config.Config {
				cfg := config.NewDefaultConfig()
				cfg.OAuth2Providers = map[string]config.OAuth2Provider{} // Empty map
				return cfg
			},
			setupRequest: func(r *http.Request) {
				r.Header.Set("Content-Type", "application/json")
			},
			wantError: errorInvalidOAuth2Provider,
		},
		{
			name: "invalid content type",
			setupConfig: func() *config.Config {
				return config.NewDefaultConfig()
			},
			setupRequest: func(r *http.Request) {
				r.Header.Set("Content-Type", "text/plain")
			},
			wantError: errorInvalidContentType,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := tc.setupConfig()
			provider := config.NewProvider(cfg)
			app := &App{
				configProvider: provider,
				logger:         slog.New(slog.NewTextHandler(io.Discard, nil)),
				validator:      &DefaultValidator{},
			}

			req := httptest.NewRequest("GET", "/list-oauth2-providers", nil)
			tc.setupRequest(req)
			rr := httptest.NewRecorder()

			app.ListOAuth2ProvidersHandler(rr, req)

			if rr.Code != tc.wantError.status {
				t.Errorf("expected status %d, got %d", tc.wantError.status, rr.Code)
			}

			var gotBody, wantBody map[string]interface{}
			if err := json.Unmarshal(rr.Body.Bytes(), &gotBody); err != nil {
				t.Fatalf("failed to decode response body: %v", err)
			}
			if err := json.Unmarshal(tc.wantError.body, &wantBody); err != nil {
				t.Fatalf("failed to decode wantError body: %v", err)
			}

			if !reflect.DeepEqual(gotBody, wantBody) {
				t.Errorf("handler returned unexpected body:\ngot:  %+v\nwant: %+v", gotBody, wantBody)
			}
		})
	}
}
