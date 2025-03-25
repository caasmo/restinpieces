package core

import (
	"net/http"
)

func (a *App) ListEndpointsHandler(w http.ResponseWriter, r *http.Request) {
	// Check if user is authenticated via context
	_, isAuthed := r.Context().Value(UserIDKey).(string)

	var endpoints interface{}
	if isAuthed {
		// Return all endpoints for authenticated users
		endpoints = a.config.Endpoints
	} else {
		// Return only endpoints that don't require auth
		endpoints = a.config.Endpoints.EndpointsWithoutAuth
	}

	writeJsonWithData(w, JsonWithData{
		JsonBasic: JsonBasic{
			Status:  http.StatusOK,
			Code:    "ok_endpoints_list",
			Message: "List of available endpoints",
		},
		Data: endpoints,
	})
}
