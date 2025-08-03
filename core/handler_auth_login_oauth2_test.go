package core

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/caasmo/restinpieces/config"
	"github.com/caasmo/restinpieces/db"
	"github.com/caasmo/restinpieces/db/mock"
	"golang.org/x/oauth2"
)

// mockOAuth2Server sets up a mock HTTP server to simulate an OAuth2 provider's
// token and user info endpoints. This allows testing the handler's interaction
// with an external provider without making actual network calls.
func mockOAuth2Server(t *testing.T, tokenHandler http.HandlerFunc, userInfoHandler http.HandlerFunc) (*httptest.Server, string, string) {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/token", tokenHandler)
	mux.HandleFunc("/userinfo", userInfoHandler)

	server := httptest.NewTLSServer(mux)
	t.Cleanup(server.Close)

	return server, server.URL + "/token", server.URL + "/userinfo"
}

// TestAuthWithOAuth2Handler_Validation tests the initial input validation logic of the
// AuthWithOAuth2Handler. It ensures that the handler correctly rejects requests that
// are malformed, have an incorrect content type, are missing required fields, or
// specify an unknown provider, all before attempting any external communication.
func TestAuthWithOAuth2Handler_Validation(t *testing.T) {
	testCases := []struct {
		name          string
		contentType   string
		requestBody   string
		providerInCfg bool
		wantError     jsonResponse
	}{
		{
			name:          "invalid content type",
			contentType:   "text/plain",
			requestBody:   `{} `,
			providerInCfg: true,
			wantError:     errorInvalidContentType,
		},
		{
			name:          "malformed json",
			contentType:   "application/json",
			requestBody:   `{"provider": "google",`,
			providerInCfg: true,
			wantError:     errorInvalidRequest,
		},
		{
			name:          "missing provider field",
			contentType:   "application/json",
			requestBody:   `{"code": "c", "code_verifier": "cv", "redirect_uri": "ru"}`,
			providerInCfg: true,
			wantError:     errorMissingFields,
		},
		{
			name:          "missing code field",
			contentType:   "application/json",
			requestBody:   `{"provider": "p", "code_verifier": "cv", "redirect_uri": "ru"}`,
			providerInCfg: true,
			wantError:     errorMissingFields,
		},
		{
			name:          "unknown provider",
			contentType:   "application/json",
			requestBody:   `{"provider": "unknown", "code": "c", "code_verifier": "cv", "redirect_uri": "ru"}`,
			providerInCfg: false, // The key for this test
			wantError:     errorInvalidOAuth2Provider,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := config.NewDefaultConfig()
			if tc.providerInCfg {
				cfg.OAuth2Providers = map[string]config.OAuth2Provider{"google": {Name: config.OAuth2ProviderGoogle}}
			}

			app := &App{
				configProvider: config.NewProvider(cfg),
				validator:      &DefaultValidator{},
				logger:         slog.New(slog.NewTextHandler(io.Discard, nil)),
			}

			req := httptest.NewRequest("POST", "/auth-with-oauth2", strings.NewReader(tc.requestBody))
			req.Header.Set("Content-Type", tc.contentType)
			rr := httptest.NewRecorder()

			app.AuthWithOAuth2Handler(rr, req)

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

// TestAuthWithOAuth2Handler_Flow tests the core authentication flow, including interactions
// with the mocked OAuth2 provider and the mock database. It covers scenarios for new user
// registration, login for existing users, and failures during the external API calls.
func TestAuthWithOAuth2Handler_Flow(t *testing.T) {
	testUser := db.User{
		ID:    "user123",
		Email: "test@example.com",
		Name:  "Test User",
	}

	testCases := []struct {
		name            string
		dbSetup         func(*mock.Db)
		tokenHandler    http.HandlerFunc
		userInfoHandler http.HandlerFunc
		wantStatus      int
		wantCode        string
		expectCreate    bool // whether CreateUserWithOauth2 should be called
	}{
		{
			name: "successful login - new user",
			dbSetup: func(m *mock.Db) {
				m.GetUserByEmailFunc = func(email string) (*db.User, error) {
					return nil, nil // User does not exist
				}
				m.CreateUserWithOauth2Func = func(user db.User) (*db.User, error) {
					t.Logf("CreateUserWithOauth2Func called with user: %+v", user)
					return &testUser, nil
				}
			},
			tokenHandler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if err := json.NewEncoder(w).Encode(map[string]string{"access_token": "mock_access_token", "token_type": "Bearer"}); err != nil {
					t.Fatalf("failed to write mock token response: %v", err)
				}
			},
			userInfoHandler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if err := json.NewEncoder(w).Encode(map[string]interface{}{
					"sub": "user123", "name": "Test User", "picture": "", "email": "test@example.com", "email_verified": true,
				}); err != nil {
					t.Fatalf("failed to write mock user info response: %v", err)
				}
			},
			wantStatus:   http.StatusOK,
			wantCode:     CodeOkAuthentication,
			expectCreate: true,
		},
		{
			name: "successful login - existing oauth2 user",
			dbSetup: func(m *mock.Db) {
				existingUser := testUser
				existingUser.Oauth2 = true
				m.GetUserByEmailFunc = func(email string) (*db.User, error) {
					return &existingUser, nil
				}
			},
			tokenHandler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if err := json.NewEncoder(w).Encode(map[string]string{"access_token": "mock_access_token", "token_type": "Bearer"}); err != nil {
					t.Fatalf("failed to write mock token response: %v", err)
				}
			},
			userInfoHandler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if err := json.NewEncoder(w).Encode(map[string]interface{}{
					"sub": "user123", "email": "test@example.com", "email_verified": true,
				}); err != nil {
					t.Fatalf("failed to write mock user info response: %v", err)
				}
			},
			wantStatus:   http.StatusOK,
			wantCode:     CodeOkAuthentication,
			expectCreate: false, // Should not create a new user
		},
		{
			name: "oauth2 token exchange fails",
			dbSetup: func(m *mock.Db) {},
			tokenHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
				if err := json.NewEncoder(w).Encode(map[string]string{"error": "invalid_grant"}); err != nil {
					t.Fatalf("failed to write mock token response: %v", err)
				}
			},
			userInfoHandler: func(w http.ResponseWriter, r *http.Request) {},
			wantStatus:      http.StatusBadRequest,
			wantCode:        CodeErrorOAuth2TokenExchangeFailed,
		},
		{
			name:    "fetch user info fails",
			dbSetup: func(m *mock.Db) {},
			tokenHandler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if err := json.NewEncoder(w).Encode(map[string]string{"access_token": "mock_access_token", "token_type": "Bearer"}); err != nil {
					t.Fatalf("failed to write mock token response: %v", err)
				}
			},
			userInfoHandler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				if err := json.NewEncoder(w).Encode(map[string]string{"error": "server error"}); err != nil {
					t.Fatalf("failed to write mock user info response: %v", err)
				}
			},
			wantStatus: http.StatusBadRequest,
			wantCode:   CodeErrorOAuth2UserInfoProcessingFailed,
		},
		{
			name:    "user info lacks email",
			dbSetup: func(m *mock.Db) {},
			tokenHandler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if err := json.NewEncoder(w).Encode(map[string]string{"access_token": "mock_access_token", "token_type": "Bearer"}); err != nil {
					t.Fatalf("failed to write mock token response: %v", err)
				}
			},
			userInfoHandler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if err := json.NewEncoder(w).Encode(map[string]interface{}{
					"sub": "user123", "name": "Test User", "email_verified": true,
				}); err != nil {
					t.Fatalf("failed to write mock user info response: %v", err)
				}
			},
			wantStatus: http.StatusBadRequest,
			wantCode:   CodeErrorInvalidRequest,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			server, tokenURL, userInfoURL := mockOAuth2Server(t, tc.tokenHandler, tc.userInfoHandler)

			cfg := config.NewDefaultConfig()
			cfg.Jwt.AuthSecret = "test_secret_that_is_long_enough_for_hs256"
			cfg.OAuth2Providers = map[string]config.OAuth2Provider{
				config.OAuth2ProviderGoogle: {TokenURL: tokenURL, UserInfoURL: userInfoURL, Name: config.OAuth2ProviderGoogle},
			}

			mockDb := &mock.Db{}
			tc.dbSetup(mockDb)
			createCalled := false
			mockDb.CreateUserWithOauth2Func = func(user db.User) (*db.User, error) {
				createCalled = true
				return &testUser, nil
			}

			app := &App{
				configProvider: config.NewProvider(cfg),
				validator:      &DefaultValidator{},
				logger:         slog.New(slog.NewTextHandler(io.Discard, nil)),
				dbAuth:         mockDb,
			}

			body := `{"provider": "google", "code": "c", "code_verifier": "cv", "redirect_uri": "ru"}`
			req := httptest.NewRequest("POST", "/auth-with-oauth2", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			// The oauth2 library's Exchange function uses its own http.Client. To make it
			// trust our httptest server's self-signed certificate, we must create a new
			// client that is configured with the server's certificate and inject it
			// into the request's context.
			certPool := x509.NewCertPool()
			certPool.AddCert(server.Certificate())
			sslTransport := &http.Transport{
				TLSClientConfig: &tls.Config{RootCAs: certPool},
			}
			client := &http.Client{Transport: sslTransport}
			ctx := context.WithValue(req.Context(), oauth2.HTTPClient, client)
			req = req.WithContext(ctx)

			rr := httptest.NewRecorder()
			app.AuthWithOAuth2Handler(rr, req)

			if rr.Code != tc.wantStatus {
				t.Errorf("expected status %d, got %d", tc.wantStatus, rr.Code)
			}

			var respBody map[string]interface{}
			if err := json.Unmarshal(rr.Body.Bytes(), &respBody); err != nil {
				t.Fatalf("failed to decode response body: %v", err)
			}

			if code, _ := respBody["code"].(string); code != tc.wantCode {
				t.Errorf("expected code %q, got %q", tc.wantCode, code)
			}

			if tc.expectCreate != createCalled {
				t.Errorf("expected CreateUserWithOauth2 called to be %v, but was %v", tc.expectCreate, createCalled)
			}
		})
	}
}

// TestAuthWithOAuth2Handler_DependencyFailures tests how the handler responds to internal
// failures, such as the database being unavailable or JWT token generation failing.
func TestAuthWithOAuth2Handler_DependencyFailures(t *testing.T) {
	server, tokenURL, userInfoURL := mockOAuth2Server(t,
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(map[string]string{"access_token": "mock_access_token", "token_type": "Bearer"}); err != nil {
				t.Fatalf("failed to write mock token response: %v", err)
			}
		},
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(map[string]interface{}{"sub": "user123", "email": "test@example.com", "email_verified": true}); err != nil {
				t.Fatalf("failed to write mock user info response: %v", err)
			}
		},
	)

	testCases := []struct {
		name      string
		dbSetup   func(*mock.Db)
		jwtSecret string
		wantError jsonResponse
	}{
		{
			name: "db fails on GetUserByEmail",
			dbSetup: func(m *mock.Db) {
				m.GetUserByEmailFunc = func(email string) (*db.User, error) {
					return nil, errors.New("db connection failed")
				}
			},
			jwtSecret: "a_valid_secret_that_is_long_enough",
			wantError: errorOAuth2DatabaseError,
		},
		{
			name: "db fails on CreateUserWithOauth2",
			dbSetup: func(m *mock.Db) {
				m.GetUserByEmailFunc = func(email string) (*db.User, error) {
					return nil, nil // User not found
				}
				m.CreateUserWithOauth2Func = func(user db.User) (*db.User, error) {
					return nil, errors.New("db write failed")
				}
			},
			jwtSecret: "a_valid_secret_that_is_long_enough",
			wantError: errorOAuth2DatabaseError,
		},
		{
			name: "jwt generation fails",
			dbSetup: func(m *mock.Db) {
				m.GetUserByEmailFunc = func(email string) (*db.User, error) {
					return &db.User{Email: "test@example.com", Oauth2: true}, nil
				}
			},
			jwtSecret: "short", // Invalid secret
			wantError: errorTokenGeneration,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := config.NewDefaultConfig()
			cfg.Jwt.AuthSecret = tc.jwtSecret
			cfg.Jwt.AuthTokenDuration = config.Duration{Duration: 15 * time.Minute}
			cfg.OAuth2Providers = map[string]config.OAuth2Provider{
				config.OAuth2ProviderGoogle: {TokenURL: tokenURL, UserInfoURL: userInfoURL, Name: config.OAuth2ProviderGoogle},
			}

			mockDb := &mock.Db{}
			tc.dbSetup(mockDb)

			app := &App{
				configProvider: config.NewProvider(cfg),
				validator:      &DefaultValidator{},
				logger:         slog.New(slog.NewTextHandler(io.Discard, nil)),
				dbAuth:         mockDb,
			}

			body := `{"provider": "google", "code": "c", "code_verifier": "cv", "redirect_uri": "ru"}`
			req := httptest.NewRequest("POST", "/auth-with-oauth2", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			client := server.Client()
			ctx := context.WithValue(req.Context(), oauth2.HTTPClient, client)
			req = req.WithContext(ctx)

			rr := httptest.NewRecorder()
			app.AuthWithOAuth2Handler(rr, req)

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
