package core

import (
	"net/http"
	"log/slog"
)

func (a *App) ListEndpointsHandler(w http.ResponseWriter, r *http.Request) {
	// Check if user is authenticated via context
	_, isAuthed := r.Context().Value(UserIDKey).(string)

	if isAuthed {
		slog.Info("user authenticated, showing all endpoints")
		writeJsonWithData(w, JsonWithData{
			JsonBasic: JsonBasic{
				Status:  http.StatusOK,
				Code:    CodeOkEndpoints,
				Message: "List of all available endpoints",
			},
			Data: a.config.Endpoints,
		})
	} else {
		slog.Info("unauthenticated user, showing public endpoints only")
		writeJsonWithData(w, JsonWithData{
			JsonBasic: JsonBasic{
				Status:  http.StatusOK,
				Code:    CodeOkEndpointsWithoutAuth,
				Message: "List of endpoints available without authentication",
			},
			Data: a.config.Endpoints.EndpointsWithoutAuth,
		})
	}
}
