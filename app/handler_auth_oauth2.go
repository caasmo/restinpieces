package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/caasmo/restinpieces/crypto"
	"github.com/caasmo/restinpieces/db"
	"golang.org/x/oauth2"
)

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

func (a *App) AuthWithOAuth2Handler(w http.ResponseWriter, r *http.Request) {
	var req oauth2Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		slog.Debug("Failed to decode OAuth2 request", "error", err)
		writeJSONError(w, errorInvalidRequest)
		return
	}
	slog.Debug("Decoded OAuth2 request", "provider", req.Provider, "code", req.Code)

	// Validate required fields
	if req.Provider == "" || req.Code == "" || req.CodeVerifier == "" || req.RedirectURI == "" {
		slog.Debug("Missing required OAuth2 fields", 
			"provider", req.Provider,
			"code", req.Code,
			"codeVerifier", req.CodeVerifier,
			"redirectURI", req.RedirectURI)
		writeJSONError(w, errorMissingFields)
		return
	}

	// Get provider config
	provider, ok := a.config.OAuth2Providers[req.Provider]
	if !ok {
		slog.Debug("Invalid OAuth2 provider", "provider", req.Provider)
		writeJSONError(w, jsonError{http.StatusBadRequest, []byte(`{"error":"Invalid OAuth2 provider"}`)})
		return
	}

	// Create OAuth2 config
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
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
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
	client := oauth2Config.Client(ctx, token)
	slog.Debug("Fetching user info from OAuth2 provider", "url", provider.UserInfoURL)
	resp, err := client.Get(provider.UserInfoURL)
	slog.Debug("Received user info response", "status", resp.StatusCode)
	if err != nil {
		writeJSONError(w, jsonError{http.StatusBadRequest, []byte(fmt.Sprintf(`{"error":"Failed to get user info: %s"}`, err.Error()))})
		return
	}
	defer resp.Body.Close()

	var userInfo oauth2UserInfo
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		writeJSONError(w, jsonError{http.StatusBadRequest, []byte(fmt.Sprintf(`{"error":"Failed to decode user info: %s"}`, err.Error()))})
		return
	}
	// TODO each provider has own fields, we need a traslation from raw response to our stanrdat user. 
	// See BaseProvider pocketbase, FetchRawUser  

	slog.Debug("Received user info", "email", userInfo.Email, "name", userInfo.Name)
	if userInfo.Email == "" {
		slog.Debug("OAuth2 provider did not return email")
		writeJSONError(w, jsonError{http.StatusBadRequest, []byte(`{"error":"OAuth2 provider did not return email"}`)})
		return
	}

	// Check if user exists or create new
	slog.Debug("Looking up user by email", "email", userInfo.Email)
	user, err := a.db.GetUserByEmail(userInfo.Email)
	slog.Debug("User lookup result", "found", user != nil, "error", err)
	if err != nil {
		writeJSONError(w, jsonError{http.StatusInternalServerError, []byte(fmt.Sprintf(`{"error":"Database error: %s"}`, err.Error()))})
		return
	}

	now := time.Now()
	if user == nil {
		// Create new user
		slog.Debug("Creating new user from OAuth2 info")
		user, err = a.db.CreateUser(db.User{
			Email:    userInfo.Email,
			Name:     userInfo.Name,
			Created:  now,
			Updated:  now,
			Verified: true, // OAuth2 users are considered verified
		})
		if err != nil {
			writeJSONError(w, jsonError{http.StatusInternalServerError, []byte(fmt.Sprintf(`{"error":"Failed to create user: %s"}`, err.Error()))})
			return
		}
	}

	// Generate JWT token
	claims := map[string]any{crypto.ClaimUserID: user.ID}
	slog.Debug("Generating JWT for authenticated user", "userID", user.ID)
	jwtToken, _, err := crypto.NewJwt(claims, a.config.JwtSecret, a.config.TokenDuration)
	slog.Debug("JWT generation completed", "success", err == nil)
	if err != nil {
		writeJSONError(w, errorTokenGeneration)
		return
	}

	// Return same response format as password auth
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"token": jwtToken,
		"record": map[string]interface{}{
			"id":       user.ID,
			"email":    user.Email,
			"name":     user.Name,
			"verified": user.Verified,
		},
	})
}

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
