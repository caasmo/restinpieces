package core

import (
	"encoding/json"
	"net/http"

)

const (
    // oks for non precomputed, dynamic responses
    // no precomputed
	CodeOkAuthentication              = "ok_authentication" // Standard success code for auth
	CodeOkOAuth2ProvidersList         = "ok_oauth2_providers_list" // Success code for OAuth2 providers list
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

// writeJsonWithData writes a structured JSON response with the provided data
func writeJsonWithData(w http.ResponseWriter, resp JsonWithData) {
	w.WriteHeader(resp.Status)
	setHeaders(w, apiJsonDefaultHeaders)
	json.NewEncoder(w).Encode(resp)
}


