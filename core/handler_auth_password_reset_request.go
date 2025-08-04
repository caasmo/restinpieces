package core

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/caasmo/restinpieces/db"
	"github.com/caasmo/restinpieces/queue"
	"github.com/caasmo/restinpieces/queue/handlers"
)

// RequestPasswordResetHandler handles password reset requests
// Endpoint: POST /request-password-reset
// Authenticated: No
// Allowed Mimetype: application/json
//
// Important Security Notes:
// - Sending emails is an expensive operation and potential spam vector
// - Rate limiting is enforced via cooldown buckets
// - Email enumeration is prevented by uniform success responses
// - Email verification check prevents password reset on unverified accounts
func (a *App) RequestPasswordResetHandler(w http.ResponseWriter, r *http.Request) {
	if resp, err := a.Validator().ContentType(r, MimeTypeJSON); err != nil {
		WriteJsonError(w, resp)
		return
	}

	var req struct {
		Email string `json:"email"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteJsonError(w, errorInvalidRequest)
		return
	}

	req.Email = strings.TrimSpace(req.Email)
	if req.Email == "" {
		WriteJsonError(w, errorInvalidRequest)
		return
	}
	if err := ValidateEmail(req.Email); err != nil {
		WriteJsonError(w, errorInvalidRequest)
		return
	}

	// Check if email exists in system
	// this is error of db, return internal server error
	// TODO
	user, err := a.DbAuth().GetUserByEmail(req.Email)
	if err != nil {
		WriteJsonError(w, errorNotFound)
		return
	}

	// user not found
	if user == nil {
		// Return success even if email doesn't exist to prevent email enumeration
		WriteJsonOk(w, okPasswordResetRequested)
		return
	}

	// Check if email is verified before allowing password reset
	if !user.Verified {
		WriteJsonError(w, errorUnverifiedEmail)
		return
	}

	// Check if user has no password (oauth2 only)
	if user.Password == "" {
		WriteJsonOk(w, okPasswordNotRequired)
		return
	}

	// Calculate cooldown bucket for rate limiting
	cfg := a.Config() // Get the current config
	cooldownBucket := queue.CoolDownBucket(cfg.RateLimits.PasswordResetCooldown.Duration, time.Now())

	// Create queue job with cooldown bucket. Second insertion in same bucket
	// will fail because unique
	payload, _ := json.Marshal(handlers.PayloadPasswordReset{
		UserID:         user.ID,
		CooldownBucket: cooldownBucket,
	})
	payloadExtra, _ := json.Marshal(handlers.PayloadPasswordResetExtra{
		Email: req.Email,
	})
	job := db.Job{
		JobType:      handlers.JobTypePasswordReset,
		Payload:      payload,
		PayloadExtra: payloadExtra,
	}

	// Insert into job queue with deduplication
	err = a.DbQueue().InsertJob(job)
	if err != nil {
		if err == db.ErrConstraintUnique {
			WriteJsonError(w, errorPasswordResetAlreadyRequested)
			return
		}
		WriteJsonError(w, errorServiceUnavailable)
		return
	}

	WriteJsonOk(w, okPasswordResetRequested)
}

