package app

import (
	"fmt"
	"net/http"
	"time"

	"github.com/caasmo/restinpieces/jwt"
)

//	export JWT_SECRET=$(openssl rand -base64 32)
//
//	First get a valid JWT token (replace JWT_SECRET with your actual secret)
//	This is a test token generation command using jwt-cli (install via 'go install github.com/matiaskorhonen/jwt-cli@latest')
//	JWT_TOKEN=$(jwt encode --secret "${JWT_SECRET}" --claim user_id=testuser123 --exp +5m)
//
//	# Test valid token refresh
//	curl -v -X POST http://localhost:8080/auth-refresh \
//	  -H "Authorization: Bearer $JWT_TOKEN"
//
//	# Test invalid token
//	curl -v -X POST http://localhost:8080/auth-refresh \
//	  -H "Authorization: Bearer invalid.token.here"
//
//	# Test missing header
//	curl -v -X POST http://localhost:8080/auth-refresh
//
// Precomputed error responses with status codes
var (
	errorTokenGeneration = jsonError{http.StatusInternalServerError, []byte(`{"error":"Failed to generate token"}`)}
	errorClaimsNotFound  = jsonError{http.StatusInternalServerError, []byte(`{"error":"Failed to generate token: Claims not found"}`)}
)

// RefreshAuthHandler handles explicit token refresh requests
func (a *App) RefreshAuthHandler(w http.ResponseWriter, r *http.Request) {
	// Get claims from context (added by JwtValidate middleware)
	userId, ok := r.Context().Value(UserIDKey).(string)
	if !ok || userId == "" {
		writeJSONError(w, errorClaimsNotFound)
		return
	}

	// Generate new token with fresh expiration
	newToken, expiry, err := jwt.Create(userId, a.config.JwtSecret, a.config.TokenDuration)
	if err != nil {
		writeJSONError(w, errorTokenGeneration)
		return
	}

	// Calculate seconds until expiry
	expiresIn := int(time.Until(expiry).Seconds())

	// Return new token in response following OAuth2 token exchange format
	w.Header()["Content-Type"] = jsonHeader

	// Standard OAuth2 token response format
	fmt.Fprintf(w, `{
		"token_type": "Bearer",
		"expires_in": %d,
		"access_token": "%s"
	}`, expiresIn, newToken)

}
