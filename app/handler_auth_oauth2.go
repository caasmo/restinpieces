package app

import (
	"encoding/json"
	"net/http"

	"github.com/caasmo/restinpieces/crypto"
	"golang.org/x/oauth2"
)

type providerInfo struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	State       string `json:"state"`
	AuthURL     string `json:"authURL"`
}

func (a *App) OAuth2ProvidersHandler(w http.ResponseWriter, r *http.Request) {
	var providers []providerInfo
	
	// Loop through configured providers
	for name, provider := range a.config.OAuth2Providers {
		if provider.ClientID != "" && provider.ClientSecret != "" {
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
			info := providerInfo{
				Name:        name,
				DisplayName: provider.DisplayName,
				State:       state,
			}

			// Handle PKCE if enabled
			if provider.PKCE {
				codeVerifier := crypto.Oauth2CodeVerifier()
				codeChallenge := crypto.S256Challenge(codeVerifier)
				info.AuthURL = oauth2Config.AuthCodeURL(state, 
					oauth2.SetAuthURLParam("code_challenge", codeChallenge),
					oauth2.SetAuthURLParam("code_challenge_method", "S256"),
				)
				info.CodeVerifier = codeVerifier
				info.CodeChallenge = codeChallenge
				info.CodeChallengeMethod = "S256"
			} else {
				info.AuthURL = oauth2Config.AuthCodeURL(state)
			}

			providers = append(providers, info)
		}
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

