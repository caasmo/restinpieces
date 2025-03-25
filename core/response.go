package core

import (
	"encoding/json"
	"net/http"

	"github.com/caasmo/restinpieces/db"
)

type jsonResponse struct {
	status int
	body   []byte
}

// JsonBasic contains the basic response fields. All responses must have them
type JsonBasic struct {
	Status  int    `json:"status"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// JsonWithData is used for structured JSON responses with data
type JsonWithData struct {
	JsonBasic
	Data interface{} `json:"data,omitempty"`
}

// NewJsonWithData creates a new JsonWithData instance
func NewJsonWithData(status int, code, message string, data interface{}) *JsonWithData {
	return &JsonWithData{
		JsonBasic: JsonBasic{
			Status:  status,
			Code:    code,
			Message: message,
		},
		Data: data,
	}
}


// writeJsonWithData writes a structured JSON response with the provided data
func writeJsonWithData(w http.ResponseWriter, resp JsonWithData) {
	w.WriteHeader(resp.Status)
	setHeaders(w, apiJsonDefaultHeaders)
	json.NewEncoder(w).Encode(resp)
}


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
	response := NewJsonWithData(
		http.StatusOK,
		CodeOkAuthentication,
		"Authentication successful",
		authData,
	)
	writeJsonWithData(w, *response)
}
