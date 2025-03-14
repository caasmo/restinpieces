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
			
			authURL := oauth2Config.AuthCodeURL(state)
			providers = append(providers, providerInfo{
				Name:        name,
				DisplayName: provider.DisplayName,
				State:       state,
				AuthURL:     authURL,
			})
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

