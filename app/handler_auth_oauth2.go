package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
	"log/slog"

	"github.com/caasmo/restinpieces/crypto"
	oauth2provider "github.com/caasmo/restinpieces/oauth2"
	"golang.org/x/oauth2"
)

// oauth2TokenExchangeTimeout defines the maximum duration for OAuth2 token exchange operations.
// This timeout prevents hanging if the OAuth2 provider is unresponsive.
// The value of 10 seconds is chosen as a balance between:
// - Allowing enough time for network latency and provider processing
// - Providing a reasonable user experience
// - Preventing resource exhaustion from hanging requests
const oauth2TokenExchangeTimeout = 10 * time.Second

type responseProviderInfo struct {
	Name                string `json:"name"`
	DisplayName         string `json:"displayName"`
	State               string `json:"state"`
	AuthURL             string `json:"authURL"`
	RedirectURL         string `json:"redirectURL"`
	CodeVerifier        string `json:"codeVerifier,omitempty"`
	CodeChallenge       string `json:"codeChallenge,omitempty"`
	CodeChallengeMethod string `json:"codeChallengeMethod,omitempty"`
}

type oauth2Request struct {
	Provider     string `json:"provider"`
	Code         string `json:"code"`
	CodeVerifier string `json:"code_verifier"`
	RedirectURI  string `json:"redirect_uri"`
}

// TODO more fields, provider dependent?
type oauth2UserInfo struct {
	Email string `json:"email"`
	Name  string `json:"name"`
}

// AuthWithOAuth2Handler handles OAuth2 authentication
// Endpoint: POST /auth-with-oauth2
func (a *App) AuthWithOAuth2Handler(w http.ResponseWriter, r *http.Request) {
	var req oauth2Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, errorInvalidRequest)
		return
	}

		slog.Debug("OAuth2 fields", 
			"provider", req.Provider,
			"code", req.Code,
			"codeVerifier", req.CodeVerifier,
			"redirectURI", req.RedirectURI)
	// Validate required fields
	if req.Provider == "" || req.Code == "" || req.CodeVerifier == "" || req.RedirectURI == "" {
		writeJSONError(w, errorMissingFields)
		return
	}

	// Get provider config
	provider, ok := a.config.OAuth2Providers[req.Provider]
	if !ok {
		writeJSONError(w, jsonError{http.StatusBadRequest, []byte(`{"error":"Invalid OAuth2 provider"}`)})
		return
	}

	// Create OAuth2 config
	slog.Debug("Creating OAuth2 config", "provider", req.Provider, "scopes", provider.Scopes)
	oauth2Config := oauth2.Config{
		ClientID:     provider.ClientID.Value,
		ClientSecret: provider.ClientSecret.Value,
		RedirectURL:  req.RedirectURI,
		Scopes:       provider.Scopes,
		Endpoint: oauth2.Endpoint{
			AuthURL:  provider.AuthURL,
			TokenURL: provider.TokenURL,
		},
	}

	// Exchange code for token with timeout
	// Using a timeout prevents hanging if the OAuth2 provider is unresponsive
	slog.Debug("Setting up context with timeout for token exchange")
	ctx, cancel := context.WithTimeout(r.Context(), oauth2TokenExchangeTimeout)
	defer cancel()

	slog.Debug("Exchanging OAuth2 code for token", "provider", req.Provider)
	token, err := oauth2Config.Exchange(
		ctx,
		req.Code,
		oauth2.SetAuthURLParam("code_verifier", req.CodeVerifier),
	)
	slog.Debug("OAuth2 token exchange completed", "provider", req.Provider, "token", token != nil)
	if err != nil {
		writeJSONError(w, jsonError{http.StatusBadRequest, []byte(fmt.Sprintf(`{"error":"Failed to exchange token: %s"}`, err.Error()))})
		return
	}

	// Get user info
	slog.Debug("Creating OAuth2 client with token")
	client := oauth2Config.Client(ctx, token)
	slog.Debug("Fetching user info from OAuth2 provider", "url", provider.UserInfoURL)
	resp, err := client.Get(provider.UserInfoURL)
	slog.Debug("Received user info response", "status", resp.StatusCode)
	if err != nil {
		writeJSONError(w, jsonError{http.StatusBadRequest, []byte(fmt.Sprintf(`{"error":"Failed to get user info: %s"}`, err.Error()))})
		return
	}
	defer resp.Body.Close()

	oauthUser, err := oauth2provider.UserFromInfoResponse(resp, req.Provider)
	if err != nil {
		slog.Debug("Failed to map provider user info", "error", err)
		writeJSONError(w, jsonError{http.StatusBadRequest, []byte(fmt.Sprintf(`{"error":"Failed to process user info: %s"}`, err.Error()))})
		return
	}
	slog.Debug("Successfully mapped provider user info", "user", oauthUser)

	if oauthUser.Email == "" {
		slog.Debug("OAuth2 provider did not return email")
		writeJSONError(w, jsonError{http.StatusBadRequest, []byte(`{"error":"OAuth2 provider did not return email"}`)})
		return
	}

	// Check if user exists or create new
	slog.Debug("Looking up user by email", "email", oauthUser.Email)
	user, err := a.db.GetUserByEmail(oauthUser.Email)
	slog.Debug("User lookup result", "found", user != nil, "error", err)
	if err != nil {
		writeJSONError(w, jsonError{http.StatusInternalServerError, []byte(fmt.Sprintf(`{"error":"Database error: %s"}`, err.Error()))})
		return
	}

	if user == nil {
		// Create new user from OAuth2 info
		slog.Debug("Creating new user in users")
		user, err = a.db.CreateUser(*oauthUser)
		slog.Debug("New user created", "user", user)
		if err != nil {
			writeJSONError(w, jsonError{http.StatusInternalServerError, []byte(fmt.Sprintf(`{"error":"Failed to create user: %s"}`, err.Error()))})
			return
		}
	}

	// Generate JWT session token
	slog.Debug("Generating JWT for user", "userID", user.ID)
	jwtToken, _, err := crypto.NewJwtSession(user.ID, user.Email, a.config.JwtSecret, a.config.TokenDuration)
	slog.Debug("JWT generation completed", "success", err == nil)
	if err != nil {
		writeJSONError(w, errorTokenGeneration)
		return
	}

	// Return same response format as password auth
	slog.Debug("Preparing successful authentication response")
	writeAuthOkResponse(w, jwtToken, user)
}

// OAuth2ProvidersHandler returns available OAuth2 providers
// Endpoint: GET /oauth2-providers
func (a *App) OAuth2ProvidersHandler(w http.ResponseWriter, r *http.Request) {
	var providers []responseProviderInfo

	// Loop through configured providers
	for name, provider := range a.config.OAuth2Providers {

		state := crypto.Oauth2State()
		oauth2Config := oauth2.Config{
			ClientID:     provider.ClientID.Value,
			ClientSecret: provider.ClientSecret.Value,
			RedirectURL:  provider.RedirectURL,
			Scopes:       provider.Scopes,
			Endpoint: oauth2.Endpoint{
				AuthURL:  provider.AuthURL,
				TokenURL: provider.TokenURL,
			},
		}

		// Create base provider info
		info := responseProviderInfo{
			Name:        name,
			DisplayName: provider.DisplayName,
			State:       state,
			RedirectURL: provider.RedirectURL,
		}

		// Handle PKCE if enabled
		if provider.PKCE {
			codeVerifier := crypto.Oauth2CodeVerifier()
			codeChallenge := crypto.S256Challenge(codeVerifier)
			info.AuthURL = oauth2Config.AuthCodeURL(state,
				oauth2.SetAuthURLParam("code_challenge", codeChallenge),
				oauth2.SetAuthURLParam("code_challenge_method", crypto.PKCECodeChallengeMethod),
			)
			info.CodeVerifier = codeVerifier
			info.CodeChallenge = codeChallenge
			info.CodeChallengeMethod = crypto.PKCECodeChallengeMethod
		} else {
			info.AuthURL = oauth2Config.AuthCodeURL(state)
		}

		providers = append(providers, info)
	}

	if len(providers) == 0 {
		writeJSONError(w, jsonError{http.StatusBadRequest, []byte(`{"error":"No OAuth2 providers configured"}`)})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(providers); err != nil {
		writeJSONError(w, jsonError{http.StatusInternalServerError, []byte(`{"error":"Failed to encode providers"}`)})
		return
	}
}
