package core

import (
	"encoding/json"
	"errors"
	
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/caasmo/restinpieces/config"
	"github.com/caasmo/restinpieces/db"
	"github.com/caasmo/restinpieces/db/mock"
)

// TestRequestPasswordResetHandler_Validation tests the initial input validation logic.
// It ensures the handler rejects requests that are malformed, have an incorrect
// content type, or are missing required/valid fields, all before attempting
// any database interaction.
func TestRequestPasswordResetHandler_Validation(t *testing.T) {
	testCases := []struct {
		name        string
		contentType string
		requestBody string
		wantError   jsonResponse
	}{
		{
			name:        "invalid content type",
			contentType: "text/plain",
			requestBody: `{"email":"a@b.com"}`,
			wantError:   errorInvalidContentType,
		},
		{
			name:        "malformed json",
			contentType: "application/json",
			requestBody: `{"email":`,
			wantError:   errorInvalidRequest,
		},
		{
			name:        "missing email field",
			contentType: "application/json",
			requestBody: `{}`,
			wantError:   errorInvalidRequest,
		},
		{
			name:        "empty email field",
			contentType: "application/json",
			requestBody: `{"email": " "}`,
			wantError:   errorInvalidRequest,
		},
		{
			name:        "invalid email format",
			contentType: "application/json",
			requestBody: `{"email": "not-an-email"}`,
			wantError:   errorInvalidRequest,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/request-password-reset", strings.NewReader(tc.requestBody))
			req.Header.Set("Content-Type", tc.contentType)
			rr := httptest.NewRecorder()

			// Setup mock app with only the validator
			app := &App{
				validator: &DefaultValidator{},
			}

			app.RequestPasswordResetHandler(rr, req)

			if rr.Code != tc.wantError.status {
				t.Errorf("expected status %d, got %d", tc.wantError.status, rr.Code)
			}

			var gotBody, wantBody map[string]interface{}
			if err := json.Unmarshal(rr.Body.Bytes(), &gotBody); err != nil {
				t.Fatalf("failed to decode response body: %v", err)
			}
			if err := json.Unmarshal(tc.wantError.body, &wantBody); err != nil {
				t.Fatalf("failed to decode wantError body: %v", err)
			}

			if gotBody["code"] != wantBody["code"] {
				t.Errorf("expected error code %q, got %q", wantBody["code"], gotBody["code"])
			}
		})
	}
}

// TestRequestPasswordResetHandler_UserLookup tests how the handler behaves based on
// the state of the user found in the database.
func TestRequestPasswordResetHandler_UserLookup(t *testing.T) {
	testCases := []struct {
		name      string
		dbSetup   func(*mock.Db)
		wantError jsonResponse
	}{
		{
			name: "user not found (email enumeration prevention)",
			dbSetup: func(m *mock.Db) {
				m.GetUserByEmailFunc = func(email string) (*db.User, error) {
					return nil, nil
				}
			},
			wantError: okPasswordResetRequested,
		},
		{
			name: "database error on user lookup",
			dbSetup: func(m *mock.Db) {
				m.GetUserByEmailFunc = func(email string) (*db.User, error) {
					return nil, errors.New("db connection failed")
				}
			},
			wantError: errorNotFound,
		},
		{
			name: "user not verified",
			dbSetup: func(m *mock.Db) {
				m.GetUserByEmailFunc = func(email string) (*db.User, error) {
					return &db.User{Email: "test@example.com", Verified: false}, nil
				}
			},
			wantError: errorUnverifiedEmail,
		},
		{
			name: "user is oauth2-only (no password)",
			dbSetup: func(m *mock.Db) {
				m.GetUserByEmailFunc = func(email string) (*db.User, error) {
					return &db.User{Email: "test@example.com", Verified: true, Password: ""}, nil
				}
			},
			wantError: okPasswordNotRequired,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			reqBody := `{"email": "test@example.com"}`
			req := httptest.NewRequest("POST", "/request-password-reset", strings.NewReader(reqBody))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()

			mockDb := &mock.Db{}
			tc.dbSetup(mockDb)

			app := &App{
				validator: &DefaultValidator{},
				dbAuth:    mockDb,
			}

			app.RequestPasswordResetHandler(rr, req)

			if rr.Code != tc.wantError.status {
				t.Errorf("expected status %d, got %d", tc.wantError.status, rr.Code)
			}

			var gotBody, wantBody map[string]interface{}
			if err := json.Unmarshal(rr.Body.Bytes(), &gotBody); err != nil {
				t.Fatalf("failed to decode response body: %v", err)
			}
			if err := json.Unmarshal(tc.wantError.body, &wantBody); err != nil {
				t.Fatalf("failed to decode wantError body: %v", err)
			}

			if gotBody["code"] != wantBody["code"] {
				t.Errorf("expected error code %q, got %q", wantBody["code"], gotBody["code"])
			}
		})
	}
}

// TestRequestPasswordResetHandler_QueueInsertion tests the interaction with the database queue.
// It assumes a valid, verified user with a password is making the request.
func TestRequestPasswordResetHandler_QueueInsertion(t *testing.T) {
	// This user is the result of a successful lookup for an eligible user
	mockUser := &db.User{ID: "user123", Email: "current@example.com", Verified: true, Password: "some_hash"}

	testCases := []struct {
		name      string
		dbSetup   func(*mock.Db)
		wantError jsonResponse
	}{
		{
			name: "successful job insertion",
			dbSetup: func(m *mock.Db) {
				m.InsertJobFunc = func(job db.Job) error {
					return nil
				}
			},
			wantError: okPasswordResetRequested,
		},
		{
			name: "unique constraint violation",
			dbSetup: func(m *mock.Db) {
				m.InsertJobFunc = func(job db.Job) error {
					return db.ErrConstraintUnique
				}
			},
			wantError: errorPasswordResetAlreadyRequested,
		},
		{
			name: "generic database error",
			dbSetup: func(m *mock.Db) {
				m.InsertJobFunc = func(job db.Job) error {
					return errors.New("db connection failed")
				}
			},
			wantError: errorServiceUnavailable,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			reqBody := `{"email": "current@example.com"}`
			req := httptest.NewRequest("POST", "/request-password-reset", strings.NewReader(reqBody))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()

			mockDb := &mock.Db{
				GetUserByEmailFunc: func(email string) (*db.User, error) {
					return mockUser, nil
				},
			}
			tc.dbSetup(mockDb)

			// Create a test config with rate limits for cooldown bucket calculation
			testConfig := &config.Config{
				RateLimits: config.RateLimits{
					PasswordResetCooldown: config.Duration{Duration: 5 * time.Minute},
				},
			}

			app := &App{
				validator:      &DefaultValidator{},
				dbAuth:         mockDb,
				dbQueue:        mockDb,
				configProvider: config.NewProvider(testConfig),
			}

			app.RequestPasswordResetHandler(rr, req)

			if rr.Code != tc.wantError.status {
				t.Errorf("expected status %d, got %d", tc.wantError.status, rr.Code)
			}

			var gotBody, wantBody map[string]interface{}
			if err := json.Unmarshal(rr.Body.Bytes(), &gotBody); err != nil {
				t.Fatalf("failed to decode response body: %v", err)
			}
			if err := json.Unmarshal(tc.wantError.body, &wantBody); err != nil {
				t.Fatalf("failed to decode wantError body: %v", err)
			}

			if gotBody["code"] != wantBody["code"] {
				t.Errorf("expected error code %q, got %q", wantBody["code"], gotBody["code"])
			}
		})
	}
}
