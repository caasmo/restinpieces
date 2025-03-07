package app

import (
	"encoding/json"
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
	errorTokenGeneration    = jsonError{http.StatusInternalServerError, []byte(`{"error":"Failed to generate token"}`)}
	errorClaimsNotFound     = jsonError{http.StatusInternalServerError, []byte(`{"error":"Failed to generate token: Claims not found"}`)}
	errorInvalidRequest     = jsonError{http.StatusBadRequest, []byte(`{"error":"Invalid request format"}`)}
	errorInvalidCredentials = jsonError{http.StatusUnauthorized, []byte(`{"error":"Invalid credentials"}`)}
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

// AuthWithPasswordHandler handles password-based authentication
func (a *App) AuthWithPasswordHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Identity string `json:"identity"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, errorInvalidRequest)
		return
	}

	if req.Identity == "" || req.Password == "" {
		writeJSONError(w, errorInvalidRequest)
		return
	}

	// Get user from database
	userID, hashedPassword, err := a.db.GetUserByEmail(req.Identity)
	if err != nil || userID == "" {
		writeJSONError(w, errorInvalidCredentials)
		return
	}

	// Verify password hash
	if !checkPasswordHash(req.Password, hashedPassword) {
		writeJSONError(w, errorInvalidCredentials)
		return
	}

	// Generate JWT token
	token, _, err := jwt.Create(userID, a.config.JwtSecret, a.config.TokenDuration)
	if err != nil {
		writeJSONError(w, errorTokenGeneration)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"token": token,
		"record": map[string]string{
			"id": userID,
		},
	})
}

// checkPasswordHash verifies Argon2id hashed password using constant-time comparison
func checkPasswordHash(password, hash string) bool {
	// Decode stored hash
	p, salt, storedHash, err := decodeHash(hash)
	if err != nil {
		return false
	}

	// Generate hash with same parameters
	generatedHash := argon2.IDKey([]byte(password), salt, p.iterations, p.memory, p.parallelism, p.keyLength)

	// Constant-time comparison
	return bytes.Equal(storedHash, generatedHash)
}

// argon2Params represents the parameters used for Argon2id hashing
type argon2Params struct {
	memory      uint32
	iterations  uint32
	parallelism uint8
	saltLength  uint32
	keyLength   uint32
}

func decodeHash(encodedHash string) (*argon2Params, []byte, []byte, error) {
	// Hash format: $argon2id$v=19$m=65536,t=3,p=4$salt$hash
	vals := strings.Split(encodedHash, "$")
	if len(vals) != 6 || vals[1] != "argon2id" || vals[2] != "v=19" {
		return nil, nil, nil, fmt.Errorf("invalid hash format")
	}

	var p argon2Params
	_, err := fmt.Sscanf(vals[3], "m=%d,t=%d,p=%d", &p.memory, &p.iterations, &p.parallelism)
	if err != nil {
		return nil, nil, nil, err
	}

	salt, err := base64.RawStdEncoding.DecodeString(vals[4])
	if err != nil {
		return nil, nil, nil, err
	}
	p.saltLength = uint32(len(salt))

	storedHash, err := base64.RawStdEncoding.DecodeString(vals[5])
	if err != nil {
		return nil, nil, nil, err
	}
	p.keyLength = uint32(len(storedHash))

	return &p, salt, storedHash, nil
}
