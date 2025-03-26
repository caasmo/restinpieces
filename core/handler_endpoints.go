package core

import (
	"net/http"
)

func (a *App) ListEndpointsHandler(w http.ResponseWriter, r *http.Request) {
	writeJsonWithData(w, JsonWithData{
		JsonBasic: JsonBasic{
			Status:  http.StatusOK,
			Code:    CodeOkEndpoints,
			Message: "List of all available endpoints",
		},
		Data: a.config.Endpoints,
	})
}

