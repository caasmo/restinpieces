package core

import (
	"net/http"
	"log/slog"
)

func (a *App) AllEndpointsHandler(w http.ResponseWriter, r *http.Request) {
	writeJsonWithData(w, JsonWithData{
		JsonBasic: JsonBasic{
			Status:  http.StatusOK,
			Code:    CodeOkEndpoints,
			Message: "List of all available endpoints",
		},
		Data: a.config.Endpoints,
	})
}

func (a *App) PublicEndpointsHandler(w http.ResponseWriter, r *http.Request) {
	writeJsonWithData(w, JsonWithData{
		JsonBasic: JsonBasic{
			Status:  http.StatusOK,
			Code:    CodeOkEndpointsWithoutAuth,
			Message: "List of endpoints available without authentication",
		},
		Data: a.config.Endpoints.EndpointsWithoutAuth,
	})
}
