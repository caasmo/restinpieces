package core

import (
	"net/http"
)

func (a *App) ListEndpointsHandler(w http.ResponseWriter, r *http.Request) {
	// Check if user is authenticated via context
	_, isAuthed := r.Context().Value(UserIDKey).(string)

	if isAuthed {
		writeJsonWithData(w, JsonWithData{
			JsonBasic: JsonBasic{
				Status:  http.StatusOK,
				Code:    CodeOkEndpoints,
				Message: "List of all available endpoints",
			},
			Data: a.config.Endpoints,
		})
	} else {
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
