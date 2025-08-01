package core

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/caasmo/restinpieces/crypto"
	"github.com/caasmo/restinpieces/db"
	"github.com/caasmo/restinpieces/queue/handlers"
)

// RegisterWithPasswordHandler handles password-based user registration with validation
// Endpoint: POST /register-with-password
// Authenticated: No
// Allowed Mimetype: application/json
// TODO we allow register with password after the user has oauth, we just
// update the password and do not require validated email as we trust the oauth2
// provider
// if password exist CreateUserWithPassword will succeed but the password will be not updated.
func (a *App) RegisterWithPasswordHandler(w http.ResponseWriter, r *http.Request) {
	if resp, err := a.Validator().ContentType(r, MimeTypeJSON); err != nil {
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
		payload, _ := json.Marshal(handlers.PayloadEmailVerification{Email: retrievedUser.Email})
		job := db.Job{
			JobType: handlers.JobTypeEmailVerification,
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

