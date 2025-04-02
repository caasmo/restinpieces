package core

import (
	"encoding/json"
	"net/http"
	"time"

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

	// Check if user is verified
	if !user.Verified {
		writeJsonError(w, errorUnverifiedUser)
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

	// Validate email format
	if err := ValidateEmail(req.NewEmail); err != nil {
		writeJsonError(w, errorInvalidRequest)
		return
	}

	// Create queue payload
    // this is for uniqueness
	payload := queue.PayloadEmailChange{
		Email:       user.Email,
		CooldownBucket: queue.CoolDownBucket(a.config.RateLimits.EmailChangeCooldown, time.Now()),
	}

	payloadExtra := queue.PayloadEmailChangeExtra{
		NewEmail:       req.NewEmail,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		writeJsonError(w, errorInvalidRequest)
		return
	}

	payloadExtraBytes, err := json.Marshal(payloadExtra)
	if err != nil {
		writeJsonError(w, errorInvalidRequest)
		return
	}

	// Insert job into queue
	err = a.db.InsertJob(queue.Job{
		JobType:     queue.JobTypeEmailChange,
		Payload:     payloadBytes,
		PayloadExtra: payloadExtraBytes,
		Status:      queue.StatusPending,
		Attempts:    0,
		MaxAttempts: 3,
	})
	if err != nil {
		writeJsonError(w, errorAuthDatabaseError)
		return
	}

	writeJsonOk(w, okEmailChangeRequested)
}

func (a *App) ConfirmEmailChangeHandler(w http.ResponseWriter, r *http.Request) {
		writeJsonError(w, errorAuthDatabaseError)
}
