package core

import (
	"net/http"
)

func (a *App) ListEndpointsHandler(w http.ResponseWriter, r *http.Request) {
	writeJsonError(w, errorNotFound)
}
