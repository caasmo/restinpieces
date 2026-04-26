package core

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/caasmo/restinpieces/config"
	"github.com/caasmo/restinpieces/db"
	"github.com/caasmo/restinpieces/db/mock"
	"github.com/caasmo/restinpieces/queue/handlers"
)

func TestRequestPasswordResetOtpHandler(t *testing.T) {
	cfg := config.NewDefaultConfig()
	cfg.Jwt.PasswordResetSecret = "a_very_long_and_secure_secret_for_testing_purposes"
	provider := config.NewProvider(cfg)

	t.Run("success - user found and verified", func(t *testing.T) {
		mockDbAuth := &mock.Db{
			GetUserByEmailFunc: func(email string) (*db.User, error) {
				return &db.User{
					ID:       "user123",
					Email:    "test@example.com",
					Verified: true,
					Password: "hashed-password",
				}, nil
			},
		}

		var jobInserted bool
		var insertedJob db.Job
		mockDbQueue := &mock.Db{
			InsertJobFunc: func(job db.Job) error {
				jobInserted = true
				insertedJob = job
				return nil
			},
		}

		app := &App{
			configProvider: provider,
			validator:      &DefaultValidator{},
			dbAuth:         mockDbAuth,
			dbQueue:        mockDbQueue,
		}

		reqBody := `{"email":"test@example.com"}`
		req := httptest.NewRequest("POST", "/request-password-reset-otp", strings.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		app.RequestPasswordResetOtpHandler(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rr.Code)
		}

		var resp map[string]interface{}
		if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}
		if resp["code"] != CodeOkOtpTokenIssued {
			t.Errorf("expected code %s, got %v", CodeOkOtpTokenIssued, resp["code"])
		}

		if !jobInserted {
			t.Fatal("expected job to be inserted")
		}

		if insertedJob.JobType != handlers.JobTypePasswordResetOtp {
			t.Errorf("expected job type %s, got %s", handlers.JobTypePasswordResetOtp, insertedJob.JobType)
		}
	})

	t.Run("success - user not found (silent state machine)", func(t *testing.T) {
		mockDbAuth := &mock.Db{
			GetUserByEmailFunc: func(email string) (*db.User, error) {
				return nil, nil
			},
		}

		var jobInserted bool
		var insertedJob db.Job
		mockDbQueue := &mock.Db{
			InsertJobFunc: func(job db.Job) error {
				jobInserted = true
				insertedJob = job
				return nil
			},
		}

		app := &App{
			configProvider: provider,
			validator:      &DefaultValidator{},
			dbAuth:         mockDbAuth,
			dbQueue:        mockDbQueue,
		}

		reqBody := `{"email":"nonexistent@example.com"}`
		req := httptest.NewRequest("POST", "/request-password-reset-otp", strings.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		app.RequestPasswordResetOtpHandler(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rr.Code)
		}

		if !jobInserted {
			t.Fatal("expected job to be inserted even if user not found")
		}

		if insertedJob.JobType != handlers.JobTypeDummy {
			t.Errorf("expected job type %s, got %s", handlers.JobTypeDummy, insertedJob.JobType)
		}
	})

	t.Run("invalid email", func(t *testing.T) {
		app := &App{
			validator: &DefaultValidator{},
		}

		reqBody := `{"email":"invalid-email"}`
		req := httptest.NewRequest("POST", "/request-password-reset-otp", strings.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		app.RequestPasswordResetOtpHandler(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", rr.Code)
		}
	})
}
