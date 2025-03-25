package core

import (
	"encoding/json"
	"net/http"
)

func (a *App) ListEndpointsHandler(w http.ResponseWriter, r *http.Request) {
	endpoints := map[string]string{
		"auth_refresh":          "POST /api/auth-refresh",
		"auth_with_password":    "POST /api/auth-with-password",
		"auth_with_oauth2":      "POST /api/auth-with-oauth2",
		"request_verification":  "POST /api/request-verification",
		"register_with_password": "POST /api/register-with-password",
		"list_oauth2_providers": "GET /api/list-oauth2-providers",
		"confirm_verification":  "POST /api/confirm-verification",
		"list_endpoints":        "GET /api/list-endpoints",
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(endpoints); err != nil {
		http.Error(w, "Failed to encode endpoints", http.StatusInternalServerError)
	}
}
