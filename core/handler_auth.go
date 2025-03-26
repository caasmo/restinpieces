package core

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/caasmo/restinpieces/crypto"
	"github.com/caasmo/restinpieces/db"
	"github.com/caasmo/restinpieces/queue"
)

// RefreshAuthHandler handles explicit JWT token refresh requests
// Endpoint: POST /auth-refresh
func (a *App) RefreshAuthHandler(w http.ResponseWriter, r *http.Request) {
	// Authenticate the user using the token from the request
	user, err, authResp := a.Authenticate(r)
	if err != nil {
		writeJsonError(w, authResp)
		return
	}

	// If authentication is successful, 'user' is the authenticated user object.
	// No need to fetch the user again.

	// Generate new token with fresh expiration using NewJwtSession
	newToken, expiry, err := crypto.NewJwtSessionToken(user.ID, user.Email, user.Password, a.config.Jwt.AuthSecret, a.config.Jwt.AuthTokenDuration)
	if err != nil {
		slog.Error("Failed to generate new token", "error", err)
		writeJsonError(w, errorTokenGeneration)
		return
	}
	slog.Debug("New token generated",
		"expiry", expiry,
		"token_length", len(newToken))

	// Calculate seconds until expiry
	expiresIn := int(time.Until(expiry).Seconds())

	// TODO move to response standard Ok response.
	// Return standardized authentication response
	writeAuthResponse(w, newToken, expiresIn, user)

}

// AuthWithPasswordHandler handles password-based authentication (login)
// Endpoint: POST /auth-with-password
func (a *App) AuthWithPasswordHandler(w http.ResponseWriter, r *http.Request) {

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
	token, _, err := crypto.NewJwtSessionToken(user.ID, user.Email, user.Password, a.config.Jwt.AuthSecret, a.config.Jwt.AuthTokenDuration)
	if err != nil {
		writeJsonError(w, errorTokenGeneration)
		return
	}

	// Return standardized authentication response
	writeAuthResponse(w, token, int(a.config.Jwt.AuthTokenDuration.Seconds()), user)
}

// todo already verified.
// TODO do we need this endpoint? register endpoint already makes a job to send email
// Yes: for the: if you do not have received email, click here, can be a simple botton
// goroutine generates token
// RequestVerificationHandler handles email verification requests
// Endpoint: POST /request-verification
func (a *App) RequestVerificationHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email string `json:"email"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJsonError(w, errorInvalidRequest)
		return
	}

	req.Email = strings.TrimSpace(req.Email)
	if req.Email == "" {
		writeJsonError(w, errorInvalidRequest)
		return
	}
	if err := ValidateEmail(req.Email); err != nil {
		writeJsonError(w, errorInvalidRequest)
		return
	}

	// Check if email exists in system
	user, err := a.db.GetUserByEmail(req.Email)
	if err != nil {
		writeJsonError(w, errorNotFound)
		return
	}
	if user == nil {
		writeJsonError(w, errorNotFound)
		return
	}

	// Check if user is already verified
	if user.Verified {
		writeJsonOk(w, okAlreadyVerified)
		return
	}

	// Create queue job
	payload, _ := json.Marshal(queue.PayloadEmailVerification{Email: req.Email})
	job := queue.Job{
		JobType: queue.JobTypeEmailVerification,
		Payload: payload,
	}

	// Insert into job queue with deduplication
	err = a.db.InsertJob(job)
	if err != nil {
		if err == db.ErrConstraintUnique {
			writeJsonError(w, errorConflict)
			return
		}
		writeJsonError(w, errorServiceUnavailable)
		return
	}

	writeJsonOk(w, okVerificationRequested)
}

// confirm-
//
//	user created per email, requires validation of email, we have already emaila dn user id in table
//
// queue job creates token like this:
//
//	{
//	 "email": "lipo@goole.com",
//	 "exp": 1736630179,
//	 "id": "m648zm0q421yfc0",
//	 "type": "verification"
//	}
//
// with a new verification secret, create method in crypto
// with map claim in a good place with signing key email, passwordhash
// receives token
// parse unverified, should have all fiedls above reject if no
// validate all fields
// we can not write in the db yet
// we get user password from table. build signed key and try to verify the message if verified
// run VerifyEmail, document no race conditions, why
// get id, builds sig key with verification email secret
// jwt validate signed
// set verified
// key := (m.TokenKey() + m.Collection().VerificationToken.Secret)
// is a jwt
//
//	{
//	 "collectionId": "_pb_users_auth_",
//	 "email": "lipo@google.com",
//	 "exp": 1736630179,
//	 "id": "m648zm0q421yfc0",
//	 "type": "verification"
//	}
func (a *App) ConfirmVerificationHandler(w http.ResponseWriter, r *http.Request) {
	type request struct {
		Token string `json:"token"`
	}

	var req request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJsonError(w, errorInvalidRequest)
		return
	}

	// Parse unverified claims to discrd fast
	claims, err := crypto.ParseJwtUnverified(req.Token)
	if err != nil {
		writeJsonError(w, errorJwtInvalidVerificationToken)
		return
	}

	// Validate all required claims exist and have correct values
	if err := crypto.ValidateVerificationClaims(claims); err != nil {
		writeJsonError(w, errorJwtInvalidVerificationToken)
		return
	}

	// Get user from database to get password hash for signing key
	user, err := a.db.GetUserById(claims[crypto.ClaimUserID].(string))
	if err != nil || user == nil {
		writeJsonError(w, errorNotFound)
		return
	}

	// Verify token signature using verification email secret
	signingKey, err := crypto.NewJwtSigningKeyWithCredentials(
		claims[crypto.ClaimEmail].(string),
		user.Password,
		a.config.Jwt.VerificationEmailSecret,
	)
	if err != nil {
		writeJsonError(w, errorEmailVerificationFailed)
		return
	}

	// Fully verify token signature and claims
	_, err = crypto.ParseJwt(req.Token, signingKey)
	if err != nil {
		writeJsonError(w, errorJwtInvalidVerificationToken)
		return
	}

	// Check if user is already verified
	if user.Verified {
		writeJsonOk(w, okAlreadyVerified)
		return
	}

	// Mark user as verified
	err = a.db.VerifyEmail(user.ID)
	if err != nil {
		writeJsonError(w, errorServiceUnavailable)
		return
	}

	writeJsonOk(w, okEmailVerified)
}

// RegisterWithPasswordHandler handles password-based user registration with validation
// Endpoint: POST /register-with-password
// TODO we allow register with password after the user has oauth, we just
// update the password and do not require validated email as we trust the oauth2
// provider
// if password exist CreateUserWithPassword will succeed but the password will be not updated.
func (a *App) RegisterWithPasswordHandler(w http.ResponseWriter, r *http.Request) {

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
	token, _, err := crypto.NewJwtSessionToken(retrievedUser.ID, retrievedUser.Email, retrievedUser.Password, a.config.Jwt.AuthSecret, a.config.Jwt.AuthTokenDuration)
	if err != nil {
		writeJsonError(w, errorTokenGeneration)
		return
	}

	// Return standardized authentication response
	writeAuthResponse(w, token, int(a.config.Jwt.AuthTokenDuration.Seconds()), retrievedUser)
}
