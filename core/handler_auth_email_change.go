package core

import (
	"encoding/json"
	"net/http"

	"github.com/caasmo/restinpieces/crypto"
	"github.com/caasmo/restinpieces/queue"
)

// RequestEmailChangeHandler handles email change requests
// Endpoint: POST /api/request-email-change
// Authenticated: Yes (requires valid auth token)
// Allowed Mimetype: application/json
func (a *App) RequestEmailChangeHandler(w http.ResponseWriter, r *http.Request) {
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

	var req struct {
		NewEmail string `json:"new_email"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJsonError(w, errorInvalidRequest)
		return
	}

	// Validate new email
	if req.NewEmail == "" {
		writeJsonError(w, errorMissingFields)
		return
	}

	// Check if new email is same as current
	if req.NewEmail == user.Email {
		writeJsonError(w, errorEmailConflict)
		return
	}

	// Create email change token
	token, err := crypto.NewJwtEmailChangeToken(
		user.ID,
		user.Email,
		req.NewEmail,
		user.Password,
		a.config.Jwt.EmailChangeSecret,
		a.config.Jwt.EmailChangeTokenDuration,
	)
	if err != nil {
		writeJsonError(w, errorTokenGeneration)
		return
	}

	// Create queue payload
	payload := queue.PayloadEmailChange{
		OldEmail:       user.Email,
		NewEmail:       req.NewEmail,
		CooldownBucket: queue.CoolDownBucket(a.config.RateLimits.EmailChangeCooldown, time.Now()),
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		writeJsonError(w, errorInvalidRequest)
		return
	}

	// Insert job into queue
	err = a.db.InsertJob(queue.Job{
		JobType:  queue.JobTypeEmailChange,
		Payload:  payloadBytes,
		Status:   queue.StatusPending,
		Attempts: 0,
		MaxAttempts: 3,
	})
	if err != nil {
		writeJsonError(w, errorAuthDatabaseError)
		return
	}

	writeJsonOk(w, okEmailChangeRequested)
}
