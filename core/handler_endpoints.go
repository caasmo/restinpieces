package core

import (
	"net/http"
	"log/slog"
)

func (a *App) AllEndpointsHandler(w http.ResponseWriter, r *http.Request) {
	slog.Info("user authenticated, showing all endpoints")
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
	slog.Info("showing public endpoints only")
	writeJsonWithData(w, JsonWithData{
		JsonBasic: JsonBasic{
			Status:  http.StatusOK,
			Code:    CodeOkEndpointsWithoutAuth,
			Message: "List of endpoints available without authentication",
		},
		Data: a.config.Endpoints.EndpointsWithoutAuth,
	})
}
