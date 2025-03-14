package app

import (
	"encoding/json"
	"net/http"

	"github.com/caasmo/restinpieces/crypto"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
	"golang.org/x/oauth2/google"
)

type providerInfo struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	State       string `json:"state"`
	AuthURL     string `json:"authURL"`
}

func (a *App) OAuth2ProvidersHandler(w http.ResponseWriter, r *http.Request) {
	var providers []providerInfo
	
	// Google OAuth2
	if a.config.OAuth2Google.ClientID != "" && a.config.OAuth2Google.ClientSecret != "" {
		state := crypto.Oauth2State()
		googleConfig := oauth2.Config{
			ClientID:     a.config.OAuth2Google.ClientID,
			ClientSecret: a.config.OAuth2Google.ClientSecret,
			RedirectURL:  a.config.OAuth2Google.RedirectURL,
			Scopes:       a.config.OAuth2Google.Scopes,
			Endpoint: oauth2.Endpoint{
				AuthURL:  a.config.OAuth2Google.AuthURL,
				TokenURL: a.config.OAuth2Google.TokenURL,
			},
		}
		
		authURL := googleConfig.AuthCodeURL(state)
		providers = append(providers, providerInfo{
			Name:        "google",
			DisplayName: a.config.OAuth2Google.DisplayName,
			State:       state,
			AuthURL:     authURL,
		})
	}

	// GitHub OAuth2
	if a.config.OAuth2Github.ClientID != "" && a.config.OAuth2Github.ClientSecret != "" {
		state := crypto.Oauth2State()
		githubConfig := oauth2.Config{
			ClientID:     a.config.OAuth2Github.ClientID,
			ClientSecret: a.config.OAuth2Github.ClientSecret,
			RedirectURL:  a.config.OAuth2Github.RedirectURL,
			Scopes:       a.config.OAuth2Github.Scopes,
			Endpoint: oauth2.Endpoint{
				AuthURL:  a.config.OAuth2Github.AuthURL,
				TokenURL: a.config.OAuth2Github.TokenURL,
			},
		}
		
		authURL := githubConfig.AuthCodeURL(state)
		providers = append(providers, providerInfo{
			Name:        "github",
			DisplayName: a.config.OAuth2Github.DisplayName,
			State:       state,
			AuthURL:     authURL,
		})
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

