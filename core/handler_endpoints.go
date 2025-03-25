package core

import (
	"encoding/json"
	"net/http"
)

func (a *App) ListEndpointsHandler(w http.ResponseWriter, r *http.Request) {
	endpoints := NewEndpointsData(&a.config.Endpoints)
	response := NewJsonWithData(
		http.StatusOK, 
		"ok_endpoints_list",
		"List of available endpoints",
		endpoints,
	)
	writeJsonWithData(w, *response)
}
