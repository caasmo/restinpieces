package core

import (
	"encoding/json"
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
	if err, resp := a.Validator().ContentType(r, MimeTypeJSON); err != nil {
		WriteJsonError(w, resp)
		return
	}
	// Authenticate the user using the token from the request
	user, err, authResp := a.Auth().Authenticate(r)
	if err != nil {
		WriteJsonError(w, authResp)
		return
	}

	// If authentication is successful, 'user' is the authenticated user object.
	// No need to fetch the user again.

	// Generate new token with fresh expiration using NewJwtSession
	cfg := a.Config() // Get the current config
	newToken, err := crypto.NewJwtSessionToken(user.ID, user.Email, user.Password, cfg.Jwt.AuthSecret, cfg.Jwt.AuthTokenDuration.Duration)
	if err != nil {
		a.Logger().Error("Failed to generate new token", "error", err)
		WriteJsonError(w, errorTokenGeneration)
		return
	}

	// Return standardized authentication response
	writeAuthResponse(w, newToken, user)

}

// AuthWithPasswordHandler handles password-based authentication (login)
// Endpoint: POST /auth-with-password
// Authenticated: No
// Allowed Mimetype: application/json
func (a *App) AuthWithPasswordHandler(w http.ResponseWriter, r *http.Request) {
	if err, resp := a.Validator().ContentType(r, MimeTypeJSON); err != nil {
		WriteJsonError(w, resp)
		return
	}

	var req struct {
		Identity string `json:"identity"` // username or email, only mail implemented
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteJsonError(w, errorInvalidRequest)
		return
	}

	if req.Identity == "" || req.Password == "" {
		WriteJsonError(w, errorInvalidRequest)
		return
	}

	// Validate email format
	if err := ValidateEmail(req.Identity); err != nil {
		WriteJsonError(w, errorInvalidRequest)
		return
	}

	user, err := a.DbAuth().GetUserByEmail(req.Identity)
	if err != nil || user == nil {
		WriteJsonError(w, errorInvalidCredentials)
		return
	}

	// Verify password hash
	if !crypto.CheckPassword(req.Password, user.Password) {
		WriteJsonError(w, errorInvalidCredentials)
		return
	}

	// Generate JWT session token
	cfg := a.Config() // Get the current config
	token, err := crypto.NewJwtSessionToken(user.ID, user.Email, user.Password, cfg.Jwt.AuthSecret, cfg.Jwt.AuthTokenDuration.Duration)
	if err != nil {
		WriteJsonError(w, errorTokenGeneration)
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
	if err, resp := a.Validator().ContentType(r, MimeTypeJSON); err != nil {
		WriteJsonError(w, resp)
		return
	}

	var req struct {
		Identity        string `json:"identity"`
		Password        string `json:"password"`
		PasswordConfirm string `json:"password_confirm"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteJsonError(w, errorInvalidRequest)
		return
	}

	// Validate required fields
	req.Identity = strings.TrimSpace(req.Identity)
	req.Password = strings.TrimSpace(req.Password)
	if req.Identity == "" || req.Password == "" || req.PasswordConfirm == "" {
		WriteJsonError(w, errorMissingFields)
		return
	}

	// Validate password match
	if req.Password != req.PasswordConfirm {
		WriteJsonError(w, errorPasswordMismatch)
		return
	}

	// Validate password complexity TODO
	if len(req.Password) < 8 {
		WriteJsonError(w, errorPasswordComplexity)
		return
	}

	// Hash password before storage
	hashedPassword, err := crypto.GenerateHash(req.Password)
	if err != nil {
		WriteJsonError(w, errorTokenGeneration)
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

	retrievedUser, err := a.DbAuth().CreateUserWithPassword(newUser)
	if err != nil {
		WriteJsonError(w, errorAuthDatabaseError)
		return
	}

	// If passwords are different CreateUserWithPassword did not write the new
	// password on conflict because the user had already a password.
	if retrievedUser.Password != newUser.Password {
		WriteJsonError(w, errorEmailConflict)
		return
	}

	// If user is not verified, add verification job to queue
	if !retrievedUser.Verified {
		payload, _ := json.Marshal(queue.PayloadEmailVerification{Email: retrievedUser.Email})
		job := db.Job{
			JobType: queue.JobTypeEmailVerification,
			Payload: payload,
		}

		err = a.DbQueue().InsertJob(job)
		if err != nil {
			a.Logger().Error("Failed to insert verification job", "error", err, "job", job)
			WriteJsonError(w, errorServiceUnavailable)
			return
		}
	}

	// Generate JWT session token for immediate authentication
	cfg := a.Config() // Get the current config
	token, err := crypto.NewJwtSessionToken(retrievedUser.ID, retrievedUser.Email, retrievedUser.Password, cfg.Jwt.AuthSecret, cfg.Jwt.AuthTokenDuration.Duration)
	if err != nil {
		WriteJsonError(w, errorTokenGeneration)
		return
	}

	// Return standardized authentication response
	writeAuthResponse(w, token, retrievedUser)
}
