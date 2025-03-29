package core

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/caasmo/restinpieces/crypto"
	oauth2provider "github.com/caasmo/restinpieces/oauth2"
	"golang.org/x/oauth2"
)

// oauth2TokenExchangeTimeout defines the maximum duration for OAuth2 token exchange operations.
// This timeout prevents hanging if the OAuth2 provider is unresponsive.
const oauth2TokenExchangeTimeout = 10 * time.Second

// OAuth2ProviderInfo contains the provider details needed for client-side OAuth2 flow
type OAuth2ProviderInfo struct {
	Name                string `json:"name"`
	DisplayName         string `json:"displayName"`
	State               string `json:"state"`
	AuthURL             string `json:"authURL"`
	RedirectURL         string `json:"redirectURL"`
	CodeVerifier        string `json:"codeVerifier,omitempty"`
	CodeChallenge       string `json:"codeChallenge,omitempty"`
	CodeChallengeMethod string `json:"codeChallengeMethod,omitempty"`
}

// OAuth2ProviderListData wraps the list of providers for standardized response
type OAuth2ProviderListData struct {
	Providers []OAuth2ProviderInfo `json:"providers"`
}

// TODO move to request
type oauth2Request struct {
	Provider     string `json:"provider"`
	Code         string `json:"code"`
	CodeVerifier string `json:"code_verifier"`
	RedirectURI  string `json:"redirect_uri"`
}

// AuthWithOAuth2Handler handles OAuth2 authentication
// Endpoint: POST /auth-with-oauth2
// Authenticated: No
// Allowed Mimetype: application/json
func (a *App) AuthWithOAuth2Handler(w http.ResponseWriter, r *http.Request) {
	if err, resp := a.ValidateContentType(r, MimeTypeJSON); err != nil {
		writeJsonError(w, resp)
		return
	}
	var req oauth2Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJsonError(w, errorInvalidRequest)
		return
	}

	slog.Debug("OAuth2 fields",
		"provider", req.Provider,
		"code", req.Code,
		"codeVerifier", req.CodeVerifier,
		"redirectURI", req.RedirectURI)
	// Validate required fields
	if req.Provider == "" || req.Code == "" || req.CodeVerifier == "" || req.RedirectURI == "" {
		writeJsonError(w, errorMissingFields)
		return
	}

	// Get provider config
	provider, ok := a.config.OAuth2Providers[req.Provider]
	if !ok {
		writeJsonError(w, errorInvalidOAuth2Provider)
		return
	}

	// Create OAuth2 config
	slog.Debug("Creating OAuth2 config", "provider", req.Provider, "scopes", provider.Scopes)
	oauth2Config := oauth2.Config{
		ClientID:     provider.ClientID,
		ClientSecret: provider.ClientSecret,
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
		writeJsonError(w, errorOAuth2TokenExchangeFailed)
		return
	}

	// Get user info
	client := oauth2Config.Client(ctx, token)
	resp, err := client.Get(provider.UserInfoURL)
	if err != nil {
		writeJsonError(w, errorOAuth2UserInfoFailed)
		return
	}
	defer resp.Body.Close()

	oauthUser, err := oauth2provider.UserFromUserInfoURL(resp, provider.Name)
	if err != nil {
		slog.Debug("Failed to map provider user info", "error", err)
		writeJsonError(w, errorOAuth2UserInfoProcessingFailed)
		return
	}

	if oauthUser.Email == "" {
		writeJsonError(w, errorInvalidRequest)
		return
	}
	if err := ValidateEmail(oauthUser.Email); err != nil {
		writeJsonError(w, errorInvalidRequest)
		return
	}

	// At this point we can say the user has registered from the point of the provider adn the user.
	// - if the user exists and have ExternalAuth oauth2, we do not need to
	//   create: the user registered before with this or another auth provider
	// - if the user exists and have ExternalAuth "", the user has already register with another method, like password.
	// - if the user does not exists, we create record with auth
	//
	// Below we try to read from the Users table and then potentially write.
	// We coudl choose not to read but given that oauth2 distintion between
	// login and register is minimal, we want to minimize the number of writes.
	//
	// This could be a source of race condition and as result data inconsistency.
	//
	// we want though avoid transactions, and make a design that allow no
	// transactions while keeping data integrity.
	// the potential race condition could be two goroutines trying to write in
	// the same row(same email):
	// - user login/register with two different oauths providers at the same time
	// - user login/register one oauth2 provider, one with password
	// - user login/register with two different passwords, ?????TODO
	//
	// with sqlite beeing just "one writer at at time" we can avoid transactions by using simply a UNIQUE in the table.
	// If the two goroutines write, the last will lose an the goroutine will
	// provide a error: that is the same as we would have a transaction.
	//
	// But we go one step further:
	//
	// with the design of the methods CreateUserWithPassword and
	// CreateUserWithOauth2, we allow both competing goroutines to both always
	// succeed (on email conflict): ON CONFLICT updates the record by modifying fields that do not create
	// data inconsistency: In the case of two oauth2 registers, the first
	// will succeed and the second will just not write any new fields. the second user can
	// be informed that the user already exist by seeing the difference in its intended
	// avatar and the present (or other fields).
	// in the case of two conflicting auth methods, each one will write its
	// relevant fields (password, ExternalAuth), and the looser gorotuine can
	// also inform the user of existing user.
	user, err := a.db.GetUserByEmail(oauthUser.Email)
	if err != nil {
		writeJsonError(w, errorOAuth2DatabaseError)
		return
	}

	// Create or update user with OAuth2 info if user doesn't exist or has false Oauth2
	if user == nil || !user.Oauth2 {
		user, err = a.db.CreateUserWithOauth2(*oauthUser)
		if err != nil {
			writeJsonError(w, errorOAuth2DatabaseError)
			return
		}
	}

	// Generate JWT session token
	slog.Debug("Generating JWT for user", "userID", user.ID)
	jwtToken, err := crypto.NewJwtSessionToken(user.ID, user.Email, "", a.config.Jwt.AuthSecret, a.config.Jwt.AuthTokenDuration)
	if err != nil {
		writeJsonError(w, errorTokenGeneration)
		return
	}

	// Return standardized authentication response
	slog.Debug("Preparing successful authentication response")
	writeAuthResponse(w, jwtToken, user)
}

// ListOAuth2ProvidersHandler returns available OAuth2 providers
// Authenticated: No
// Allowed Mimetype: application/json
// Example OAuth2 Providers List Response:
//
//	{
//	  "status": 200,
//	  "code": "ok_oauth2_providers_list",
//	  "message": "OAuth2 providers list",
//	  "data": {
//	    "providers": [
//	      {
//	        "name": "google",
//	        "displayName": "Google",
//	        "state": "random-state-string",
//	        "authURL": "https://..."
//	      }
//	    ]
//	  }
//	}
//
// Endpoint: GET /list-oauth2-providers
func (a *App) ListOAuth2ProvidersHandler(w http.ResponseWriter, r *http.Request) {
	if err, resp := a.ValidateContentType(r, MimeTypeJSON); err != nil {
		writeJsonError(w, resp)
		return
	}
	var providers []OAuth2ProviderInfo

	// Loop through configured providers
	for name, provider := range a.config.OAuth2Providers {

		state := crypto.Oauth2State()
		oauth2Config := oauth2.Config{
			ClientID:     provider.ClientID,
			ClientSecret: provider.ClientSecret,
			RedirectURL:  provider.RedirectURL,
			Scopes:       provider.Scopes,
			Endpoint: oauth2.Endpoint{
				AuthURL:  provider.AuthURL,
				TokenURL: provider.TokenURL,
			},
		}

		// Create base provider info
		info := OAuth2ProviderInfo{
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
		writeJsonError(w, errorInvalidOAuth2Provider)
		return
	}

	response := JsonWithData{
		JsonBasic: JsonBasic{
			Status:  http.StatusOK,
			Code:    CodeOkOAuth2ProvidersList,
			Message: "OAuth2 providers list",
		},
		Data: OAuth2ProviderListData{Providers: providers},
	}
	writeJsonWithData(w, response)
}
