package core

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/caasmo/restinpieces/crypto"
	"github.com/caasmo/restinpieces/db"
	"github.com/caasmo/restinpieces/queue"
)

// RefreshAuthHandler handles explicit JWT token refresh requests
// Endpoint: POST /auth-refresh
// Authenticated: Yes
// Allowed Mimetype: application/json
func (a *App) RefreshAuthHandler(w http.ResponseWriter, r *http.Request) {
	if err, resp := a.ValidateContentType(r, MimeTypeJSON); err != nil {
		writeJsonError(w, resp)
		return
	}
	// Authenticate the user using the token from the request
	user, err, authResp := a.Authenticate(r)
	if err != nil {
		writeJsonError(w, authResp)
		return
	}

	// If authentication is successful, 'user' is the authenticated user object.
	// No need to fetch the user again.

	// Generate new token with fresh expiration using NewJwtSession
	newToken, err := crypto.NewJwtSessionToken(user.ID, user.Email, user.Password, a.config.Jwt.AuthSecret, a.config.Jwt.AuthTokenDuration)
	if err != nil {
		slog.Error("Failed to generate new token", "error", err)
		writeJsonError(w, errorTokenGeneration)
		return
	}
	slog.Debug("New token generated", "token_length", len(newToken))

	// Return standardized authentication response
	writeAuthResponse(w, newToken, user)

}

// AuthWithPasswordHandler handles password-based authentication (login)
// Endpoint: POST /auth-with-password
// Authenticated: No
// Allowed Mimetype: application/json
func (a *App) AuthWithPasswordHandler(w http.ResponseWriter, r *http.Request) {
	if err, resp := a.ValidateContentType(r, MimeTypeJSON); err != nil {
		writeJsonError(w, resp)
		return
	}

	var req struct {
		Identity string `json:"identity"` // username or email, only mail implemented
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJsonError(w, errorInvalidRequest)
		return
	}

	if req.Identity == "" || req.Password == "" {
		writeJsonError(w, errorInvalidRequest)
		return
	}

	// Validate email format
	if err := ValidateEmail(req.Identity); err != nil {
		writeJsonError(w, errorInvalidRequest)
		return
	}

	// Get user from database
	user, err := a.db.GetUserByEmail(req.Identity)
	if err != nil || user == nil {
		writeJsonError(w, errorInvalidCredentials)
		return
	}

	// Verify password hash
	if !crypto.CheckPassword(req.Password, user.Password) {
		writeJsonError(w, errorInvalidCredentials)
		return
	}

	// Generate JWT session token
	token, err := crypto.NewJwtSessionToken(user.ID, user.Email, user.Password, a.config.Jwt.AuthSecret, a.config.Jwt.AuthTokenDuration)
	if err != nil {
		writeJsonError(w, errorTokenGeneration)
		return
	}

	// Return standardized authentication response
	writeAuthResponse(w, token, user)
}


// RegisterWithPasswordHandler handles password-based user registration with validation
// Endpoint: POST /register-with-password
// Authenticated: No
// Allowed Mimetype: application/json
// TODO we allow register with password after the user has oauth, we just
// update the password and do not require validated email as we trust the oauth2
// provider
// if password exist CreateUserWithPassword will succeed but the password will be not updated.
func (a *App) RegisterWithPasswordHandler(w http.ResponseWriter, r *http.Request) {
	if err, resp := a.ValidateContentType(r, MimeTypeJSON); err != nil {
		writeJsonError(w, resp)
		return
	}

	var req struct {
		Identity        string `json:"identity"`
		Password        string `json:"password"`
		PasswordConfirm string `json:"password_confirm"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJsonError(w, errorInvalidRequest)
		return
	}

	// Validate required fields
	req.Identity = strings.TrimSpace(req.Identity)
	req.Password = strings.TrimSpace(req.Password)
	if req.Identity == "" || req.Password == "" || req.PasswordConfirm == "" {
		writeJsonError(w, errorMissingFields)
		return
	}

	// Validate password match
	if req.Password != req.PasswordConfirm {
		writeJsonError(w, errorPasswordMismatch)
		return
	}

	// Validate password complexity TODO
	if len(req.Password) < 8 {
		writeJsonError(w, errorPasswordComplexity)
		return
	}

	// Hash password before storage
	hashedPassword, err := crypto.GenerateHash(req.Password)
	if err != nil {
		writeJsonError(w, errorTokenGeneration)
		return
	}

	// Prepare user data
	newUser := db.User{
		Email:           req.Identity,
		Password:        string(hashedPassword),
		Name:            "", // Optional field TODO
		Verified:        false,
		Oauth2:          false,
		EmailVisibility: false,
	}

	// Create user with password authentication
	retrievedUser, err := a.db.CreateUserWithPassword(newUser)
	if err != nil {
		writeJsonError(w, errorAuthDatabaseError)
		return
	}

	// If passwords are different CreateUserWithPassword did not write the new
	// password on conflict because the user had already a password.
	if retrievedUser.Password != newUser.Password {
		writeJsonError(w, errorEmailConflict)
		return
	}

	// If user is not verified, add verification job to queue
	if !retrievedUser.Verified {
		payload, _ := json.Marshal(queue.PayloadEmailVerification{Email: retrievedUser.Email})
		job := queue.Job{
			JobType: queue.JobTypeEmailVerification,
			Payload: payload,
		}

		err = a.db.InsertJob(job)
		if err != nil {
			slog.Error("Failed to insert verification job", "error", err, "job", job)
			writeJsonError(w, errorServiceUnavailable)
			return
		}
	}

	// Generate JWT session token for immediate authentication
	token, err := crypto.NewJwtSessionToken(retrievedUser.ID, retrievedUser.Email, retrievedUser.Password, a.config.Jwt.AuthSecret, a.config.Jwt.AuthTokenDuration)
	if err != nil {
		writeJsonError(w, errorTokenGeneration)
		return
	}

	// Return standardized authentication response
	writeAuthResponse(w, token, retrievedUser)
}

