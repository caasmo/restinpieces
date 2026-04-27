package core

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/caasmo/restinpieces/config"
	"github.com/caasmo/restinpieces/crypto"
	"github.com/caasmo/restinpieces/db"
	"github.com/caasmo/restinpieces/db/mock"
	"github.com/caasmo/restinpieces/queue/handlers"
)

func TestRequestEmailChangeOtpHandler(t *testing.T) {
	cfg := config.NewDefaultConfig()
	cfg.Jwt.EmailChangeOtpSecret = "a_very_long_and_secure_secret_for_testing_purposes"
	provider := config.NewProvider(cfg)

	hashedPassword, _ := crypto.GenerateHash("password123")

	t.Run("success - user found and password correct", func(t *testing.T) {
		mockDbAuth := &mock.Db{
			GetUserByEmailFunc: func(email string) (*db.User, error) {
				// New email not taken
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

		mockAuth := &MockAuth{
			AuthenticateFunc: func(r *http.Request) (*db.User, jsonResponse, error) {
				return &db.User{
					ID:       "user123",
					Email:    "old@example.com",
					Verified: true,
					Password: string(hashedPassword),
				}, jsonResponse{}, nil
			},
		}

		app := &App{
			configProvider: provider,
			validator:      &DefaultValidator{},
			dbAuth:         mockDbAuth,
			dbQueue:        mockDbQueue,
			authenticator:  mockAuth,
		}

		reqBody := `{"new_email":"new@example.com", "password":"password123"}`
		req := httptest.NewRequest("POST", "/api/request-email-change-otp", strings.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		app.RequestEmailChangeOtpHandler(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d. Body: %s", rr.Code, rr.Body.String())
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

		if insertedJob.JobType != handlers.JobTypeEmailChangeOtp {
			t.Errorf("expected job type %s, got %s", handlers.JobTypeEmailChangeOtp, insertedJob.JobType)
		}
	})

	t.Run("success - new email taken (silent state machine)", func(t *testing.T) {
		mockDbAuth := &mock.Db{
			GetUserByEmailFunc: func(email string) (*db.User, error) {
				// New email IS taken
				return &db.User{ID: "other-user"}, nil
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

		mockAuth := &MockAuth{
			AuthenticateFunc: func(r *http.Request) (*db.User, jsonResponse, error) {
				return &db.User{
					ID:       "user123",
					Email:    "old@example.com",
					Verified: true,
					Password: string(hashedPassword),
				}, jsonResponse{}, nil
			},
		}

		app := &App{
			configProvider: provider,
			validator:      &DefaultValidator{},
			dbAuth:         mockDbAuth,
			dbQueue:        mockDbQueue,
			authenticator:  mockAuth,
		}

		reqBody := `{"new_email":"taken@example.com", "password":"password123"}`
		req := httptest.NewRequest("POST", "/api/request-email-change-otp", strings.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		app.RequestEmailChangeOtpHandler(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rr.Code)
		}

		if !jobInserted {
			t.Fatal("expected job to be inserted even if email taken")
		}

		if insertedJob.JobType != handlers.JobTypeDummy {
			t.Errorf("expected job type %s, got %s", handlers.JobTypeDummy, insertedJob.JobType)
		}
	})

	t.Run("unauthorized", func(t *testing.T) {
		mockAuth := &MockAuth{
			AuthenticateFunc: func(r *http.Request) (*db.User, jsonResponse, error) {
				return nil, errorJwtInvalidToken, http.ErrAbortHandler
			},
		}

		app := &App{
			validator:     &DefaultValidator{},
			authenticator: mockAuth,
		}

		reqBody := `{"new_email":"new@example.com", "password":"password123"}`
		req := httptest.NewRequest("POST", "/api/request-email-change-otp", strings.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		app.RequestEmailChangeOtpHandler(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d", rr.Code)
		}
	})

	t.Run("invalid credentials (wrong password)", func(t *testing.T) {
		mockAuth := &MockAuth{
			AuthenticateFunc: func(r *http.Request) (*db.User, jsonResponse, error) {
				return &db.User{
					ID:       "user123",
					Email:    "old@example.com",
					Verified: true,
					Password: string(hashedPassword),
				}, jsonResponse{}, nil
			},
		}

		app := &App{
			validator:     &DefaultValidator{},
			authenticator: mockAuth,
		}

		reqBody := `{"new_email":"new@example.com", "password":"wrong-password"}`
		req := httptest.NewRequest("POST", "/api/request-email-change-otp", strings.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		app.RequestEmailChangeOtpHandler(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d", rr.Code)
		}
	})

	t.Run("email conflict (same email)", func(t *testing.T) {
		mockAuth := &MockAuth{
			AuthenticateFunc: func(r *http.Request) (*db.User, jsonResponse, error) {
				return &db.User{
					ID:       "user123",
					Email:    "same@example.com",
					Verified: true,
					Password: string(hashedPassword),
				}, jsonResponse{}, nil
			},
		}

		app := &App{
			validator:     &DefaultValidator{},
			authenticator: mockAuth,
		}

		reqBody := `{"new_email":"same@example.com", "password":"password123"}`
		req := httptest.NewRequest("POST", "/api/request-email-change-otp", strings.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		app.RequestEmailChangeOtpHandler(rr, req)

		if rr.Code != http.StatusConflict {
			t.Errorf("expected status 409, got %d", rr.Code)
		}
	})
}
