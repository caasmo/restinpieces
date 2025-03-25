package core

import (
	"net/http"

	"github.com/caasmo/restinpieces/db"
)


//auth response. Separated here because is the response not of one, but many handlers

// AuthRecord represents the user record in authentication responses
type AuthRecord struct {
	ID       string `json:"id"`
	Email    string `json:"email"`
	Name     string `json:"name"`
	Verified bool   `json:"verified"`
}

// AuthData represents the authentication response structure
type AuthData struct {
	TokenType   string     `json:"token_type"`
	AccessToken string     `json:"access_token"`
	ExpiresIn   int        `json:"expires_in"`
	Record      AuthRecord `json:"record"`
}

// NewAuthData creates a new AuthData instance
func NewAuthData(token string, expiresIn int, user *db.User) *AuthData {
	return &AuthData{
		TokenType:   "Bearer",
		AccessToken: token,
		ExpiresIn:   expiresIn,
		Record: AuthRecord{
			ID:       user.ID,
			Email:    user.Email,
			Name:     user.Name,
			Verified: user.Verified,
		},
	}
}

// writeAuthResponse writes a standardized authentication response
func writeAuthResponse(w http.ResponseWriter, token string, expiresIn int, user *db.User) {
	authData := NewAuthData(token, expiresIn, user)
	response := JsonWithData{
		JsonBasic: JsonBasic{
			Status:  http.StatusOK,
			Code:    CodeOkAuthentication,
			Message: "Authentication successful",
		},
		Data: authData,
	}
	writeJsonWithData(w, response)
}
