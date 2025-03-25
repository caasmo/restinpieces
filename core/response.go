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


