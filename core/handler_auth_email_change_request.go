package core

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/caasmo/restinpieces/db"
	"github.com/caasmo/restinpieces/queue"
	"github.com/caasmo/restinpieces/queue/handlers"
)

// RequestEmailChangeHandler handles email change requests
// Endpoint: POST /api/request-email-change
// Authenticated: Yes (requires valid auth token)
// Allowed Mimetype: application/json
func (a *App) RequestEmailChangeHandler(w http.ResponseWriter, r *http.Request) {
	if resp, err := a.Validator().ContentType(r, MimeTypeJSON); err != nil {
		WriteJsonError(w, resp)
		return
	}

	// Authenticate the user using the token from the request
	user, authResp, err := a.Auth().Authenticate(r)
	if err != nil {
		WriteJsonError(w, authResp)
		return
	}

	if !user.Verified {
		WriteJsonError(w, errorUnverifiedEmail)
		return
	}

	var req struct {
		NewEmail string `json:"new_email"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteJsonError(w, errorInvalidRequest)
		return
	}

	// Validate new email
	if req.NewEmail == "" {
		WriteJsonError(w, errorMissingFields)
		return
	}

	// Check if new email is same as current
	if req.NewEmail == user.Email {
		WriteJsonError(w, errorEmailConflict)
		return
	}

	// Validate email format
	if err := ValidateEmail(req.NewEmail); err != nil {
		WriteJsonError(w, errorInvalidRequest)
		return
	}

	// Create queue payload
	// this is for uniqueness
	// use one request per bucket.
	cfg := a.Config() // Get the current config
	payload := handlers.PayloadEmailChange{
		UserID:         user.ID,
		CooldownBucket: queue.CoolDownBucket(cfg.RateLimits.EmailChangeCooldown.Duration, time.Now()),
	}

	payloadExtra := handlers.PayloadEmailChangeExtra{
		NewEmail: req.NewEmail,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		WriteJsonError(w, errorInvalidRequest)
		return
	}

	payloadExtraBytes, err := json.Marshal(payloadExtra)
	if err != nil {
		WriteJsonError(w, errorInvalidRequest)
		return
	}

	err = a.DbQueue().InsertJob(db.Job{
		JobType:      handlers.JobTypeEmailChange,
		Payload:      payloadBytes,
		PayloadExtra: payloadExtraBytes,
		Status:       queue.StatusPending,
		Attempts:     0,
		MaxAttempts:  3,
	})
	if err != nil {
		if err == db.ErrConstraintUnique {
			WriteJsonError(w, errorEmailChangeAlreadyRequested)
			return
		}
		WriteJsonError(w, errorAuthDatabaseError)
		return
	}

	WriteJsonOk(w, okEmailChangeRequested)
}


