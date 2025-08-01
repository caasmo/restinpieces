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

		state := crypto.Oauth2State()
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

		// Create base provider info
		info := OAuth2ProviderInfo{
			Name:        name,
			DisplayName: provider.DisplayName,
			State:       state,
			RedirectURL: rUrl,
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

