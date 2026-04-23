package core

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/caasmo/restinpieces/config"
	"github.com/caasmo/restinpieces/crypto"
	"github.com/caasmo/restinpieces/db"
)

var (
	errParseUserID = errors.New("parse user id error")
	errInvalidMac  = errors.New("invalid user id mac")
)

// Authenticator defines the interface for authentication operations
type Authenticator interface {
	Authenticate(r *http.Request) (*db.User, jsonResponse, error)
}

// DefaultAuthenticator implements Authenticator using the standard authentication flow
type DefaultAuthenticator struct {
	dbAuth         db.DbAuth
	logger         *slog.Logger
	configProvider *config.Provider
}

// fastJwtPayload is used to quickly unmarshal only the fields we care about
// from the unverified JWT payload.
type fastJwtPayload struct {
	UserID string `json:"user_id"`
	UidMac string `json:"uid_mac"`
}

// NewDefaultAuthenticator creates a new DefaultAuthenticator instance
func NewDefaultAuthenticator(dbAuth db.DbAuth, logger *slog.Logger, configProvider *config.Provider) *DefaultAuthenticator {
	return &DefaultAuthenticator{
		dbAuth:         dbAuth,
		logger:         logger,
		configProvider: configProvider,
	}
}

// Authenticate implements the Authenticator interface
func (a *DefaultAuthenticator) Authenticate(r *http.Request) (*db.User, jsonResponse, error) {
	errAuth := errors.New("Auth error")
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
		return nil, errorNoAuthHeader, errAuth
	}

	tokenString := strings.TrimPrefix(authHeader, "Bearer ")
	cfg := a.configProvider.Get()

	// STEP 1: Fast, stateless gatekeeper (CPU only, Nanoseconds/Microseconds)
	// We extract the user_id and verify the MAC. If an attacker forged the user_id, 
	// they won't have the correct MAC. The request dies here, saving a database hit.
	userId, err := extractAndVerifyUserID(tokenString, cfg.Jwt.AuthSecret)
	if err != nil {
		// Timing attack prevented: Forged IDs fail here in constant time
		return nil, errorJwtInvalidToken, errAuth
	}

	// STEP 2: Stateful Verification (Database, Microseconds/Milliseconds)
	// We are now cryptographically certain the user_id was issued by us.
	user, err := a.dbAuth.GetUserById(userId)
	if err != nil || user == nil {
		return nil, errorJwtInvalidToken, errors.New("Auth error")
	}

	// STEP 3: Full Signature Verification using user credentials
	signingKey, err := crypto.NewJwtSigningKeyWithCredentials(user.Email, user.Password, cfg.Jwt.AuthSecret)
	if err != nil {
		return nil, errorTokenGeneration, errAuth
	}

	claims, err := crypto.ParseJwt(tokenString, signingKey)
	if err != nil {
		if errors.Is(err, crypto.ErrJwtTokenExpired) {
			return nil, errorJwtTokenExpired, errAuth
		}
		if errors.Is(err, crypto.ErrJwtInvalidSigningMethod) {
			return nil, errorJwtInvalidSignMethod, errAuth
		}
		// Treat all other verification errors as an invalid token
		return nil, errorJwtInvalidToken, errAuth
	}

	if err := crypto.ValidateSessionClaims(claims); err != nil {
		return nil, errorJwtInvalidToken, errAuth
	}

	return user, jsonResponse{}, nil
}

// extractAndVerifyUserID base64-decodes the payload, extracts the ID and MAC,
// and cryptographically verifies them before allowing the process to continue.
func extractAndVerifyUserID(tokenString, serverSecret string) (string, error) {
	parts := strings.SplitN(tokenString, ".", 3)
	if len(parts) != 3 {
		return "", errParseUserID
	}

	// Decode the payload
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", errParseUserID
	}

	// Use fast struct unmarshaling (only pulls the 2 fields we need)
	var payload fastJwtPayload
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return "", errParseUserID
	}

	if payload.UserID == "" || payload.UidMac == "" {
		return "", errParseUserID
	}

	// SECURE VERIFICATION: Constant-time MAC check
	if !crypto.VerifyUserMac(payload.UserID, payload.UidMac, serverSecret) {
		return "", errInvalidMac
	}

	return payload.UserID, nil
}
