package core

import (
	"net/http"
)

func (a *App) ListEndpointsHandler(w http.ResponseWriter, r *http.Request) {
	WriteJsonWithData(w, JsonWithData{
		JsonBasic: JsonBasic{
			Status:  http.StatusOK,
			Code:    CodeOkEndpoints,
			Message: "List of all available endpoints",
		},
		Data: a.Config().Endpoints, // Use Config() method
	})
}
