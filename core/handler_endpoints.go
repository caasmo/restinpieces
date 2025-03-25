package core

import (
	"net/http"
)

func (a *App) ListEndpointsHandler(w http.ResponseWriter, r *http.Request) {
	// Check if user is authenticated via context
	_, isAuthed := r.Context().Value(UserIDKey).(string)

	// If authenticated, return full endpoints list
	if isAuthed {
		writeJsonWithData(w, okDataListEndpointsWithoutAuth)
		return
	}

	// If not authenticated, return limited endpoints list
	writeJsonWithData(w, okDataListEndpointsWithAuth)
}
