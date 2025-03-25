package core

import (
	"net/http"
	"log/slog"
)

func (a *App) ListAllEndpointsHandler(w http.ResponseWriter, r *http.Request) {
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

func (a *App) ListPublicEndpointsHandler(w http.ResponseWriter, r *http.Request) {
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
