package core

import (
	"encoding/json"
	"net/http"
)

const (
	CodeOkEndpoints            = "ok_endpoints"
	CodeOkEndpointsWithoutAuth = "ok_endpoints_without_auth"
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
	setHeaders(w, apiJsonDefaultHeaders)
	w.WriteHeader(resp.Status)
	json.NewEncoder(w).Encode(resp)
}
