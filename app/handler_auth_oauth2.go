package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

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
	if a.config.OAuth2GoogleClientID != "" && a.config.OAuth2GoogleClientSecret != "" {
		state := crypto.Oauth2State()
		googleConfig := oauth2.Config{
			ClientID:     a.config.OAuth2GoogleClientID,
			ClientSecret: a.config.OAuth2GoogleClientSecret,
			RedirectURL:  a.config.CallbackURL + "/callback/google",
			Scopes:       []string{"https://www.googleapis.com/auth/userinfo.email"},
			Endpoint:     google.Endpoint,
		}
		
		authURL := googleConfig.AuthCodeURL(state)
		providers = append(providers, providerInfo{
			Name:        "google",
			DisplayName: "Google",
			State:       state,
			AuthURL:     authURL,
		})
	}

	// GitHub OAuth2
	if a.config.OAuth2GithubClientID != "" && a.config.OAuth2GithubClientSecret != "" {
		state := crypto.Oauth2State()
		githubConfig := oauth2.Config{
			ClientID:     a.config.OAuth2GithubClientID,
			ClientSecret: a.config.OAuth2GithubClientSecret,
			RedirectURL:  a.config.CallbackURL + "/callback/github",
			Scopes:       []string{"user:email"},
			Endpoint:     github.Endpoint,
		}
		
		authURL := githubConfig.AuthCodeURL(state)
		providers = append(providers, providerInfo{
			Name:        "github",
			DisplayName: "GitHub",
			State:       state,
			AuthURL:     authURL,
		})
	}

	if len(providers) == 0 {
		writeJSONError(w, errorBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(providers); err != nil {
		writeJSONError(w, errorInternalServer)
		return
	}
}

