package core

import (
	"net/http"

	"github.com/caasmo/restinpieces/db"
)

// This file defines the standardized response formats for authentication-related API endpoints.
// It ensures consistent response structures.
//
// Two main response types standardized here:
// 1. Authentication responses - used for successful login, token refresh, registration
// 2. OAuth2 providers list - used for the OAuth2 provider discovery endpoint
//
// Example Authentication Response (successful login or token refresh):
// {
//   "status": 200,
//   "code": "ok_authentication",
//   "message": "Authentication successful",
//   "data": {
//     "token_type": "Bearer",
//     "access_token": "eyJhbGciOiJIUzI...",
//     "expires_in": 3600,
//     "record": {
//       "id": "user123",
//       "email": "user@example.com",
//       "name": "John Doe",
//       "verified": true
//     }
//   }
// }
//

const (
	// oks for non precomputed, dynamic auth responses
	CodeOkAuthentication      = "ok_authentication"        // Standard success code for auth
	CodeOkOAuth2ProvidersList = "ok_oauth2_providers_list" // Success code for OAuth2 providers list
)

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
