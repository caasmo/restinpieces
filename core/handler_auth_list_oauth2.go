package core

import (
	"net/http"

	"github.com/caasmo/restinpieces/crypto"
	"golang.org/x/oauth2"
)

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
	if resp, err := a.Validator().ContentType(r, MimeTypeJSON); err != nil {
		WriteJsonError(w, resp)
		return
	}
	var providers []OAuth2ProviderInfo

	// Loop through configured providers
	cfg := a.Config() // Get the current config
	for name, provider := range cfg.OAuth2Providers {

		rUrl := redirectUrl(cfg.Server, provider)
		a.Logger().Debug("OAuth2 fields",
			"provider", name,
			"redirectURI", rUrl)

		oauth2Config := oauth2.Config{
			ClientID:     provider.ClientID,
			ClientSecret: provider.ClientSecret,
			RedirectURL:  rUrl,
			Scopes:       provider.Scopes,
			Endpoint: oauth2.Endpoint{
				AuthURL:  provider.AuthURL,
				TokenURL: provider.TokenURL,
			},
		}

		// 1. Unconditional Code Verifier Generation
		// We ALWAYS generate a high-entropy code_verifier (43-128 chars), regardless
		// of whether the provider natively supports PKCE. This acts as our backend-enforced
		// cryptographic nonce to protect our endpoint from CSRF and Confused Deputy attacks.
		codeVerifier := crypto.Oauth2CodeVerifier()

		// 2. Cryptographic State Binding
		// The JWT state is strictly bound to the codeVerifier. The client must return
		// BOTH the state and the exact codeVerifier to our backend to complete the login.
		state, err := crypto.NewJwtOauth2StateToken(codeVerifier, cfg.Jwt.Oauth2StateSecret, cfg.Jwt.Oauth2StateTokenDuration.Duration)
		if err != nil {
			a.Logger().Error("failed to generate oauth2 state token", "error", err)
			WriteJsonError(w, errorTokenGeneration)
			return
		}

		// Create base provider info
		info := OAuth2ProviderInfo{
			Name:         name,
			DisplayName:  provider.DisplayName,
			State:        state,
			RedirectURL:  rUrl,
			CodeVerifier: codeVerifier,
		}

		// 3. Provider-Specific PKCE Handling
		if provider.PKCE {
			// If the provider supports PKCE natively, we calculate the S256 challenge
			// and append it to the AuthURL so the provider can verify it on their end.
			codeChallenge := crypto.S256Challenge(codeVerifier)
			info.AuthURL = oauth2Config.AuthCodeURL(state,
				oauth2.SetAuthURLParam("code_challenge", codeChallenge),
				oauth2.SetAuthURLParam("code_challenge_method", crypto.PKCECodeChallengeMethod),
			)
			info.CodeChallenge = codeChallenge
			info.CodeChallengeMethod = crypto.PKCECodeChallengeMethod
		} else {
			// If the provider ignores PKCE, we simply omit the challenge from the AuthURL.
			// The provider won't verify it, but OUR backend will still verify the
			// state <-> code_verifier bind upon the POST return!
			info.AuthURL = oauth2Config.AuthCodeURL(state)
		}

		providers = append(providers, info)
	}

	if len(providers) == 0 {
		WriteJsonError(w, errorInvalidOAuth2Provider)
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
	WriteJsonWithData(w, response)
}

