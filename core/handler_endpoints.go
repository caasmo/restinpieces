package core

import (
	"net/http"
)

type endpointsResponse struct {
	Endpoints interface{} `json:"endpoints"`
	Hash      string      `json:"hash"`
}

func (a *App) ListEndpointsHandler(w http.ResponseWriter, r *http.Request) {
	endpoints := a.Config().Endpoints
	WriteJsonWithData(w, JsonWithData{
		JsonBasic: JsonBasic{
			Status:  http.StatusOK,
			Code:    CodeOkEndpoints,
			Message: "List of all available endpoints",
		},
		Data: endpointsResponse{
			Endpoints: endpoints,
			Hash:      endpoints.Hash(),
		},
	})
}
