package core

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/caasmo/restinpieces/config"
	"github.com/caasmo/restinpieces/db"
	"github.com/caasmo/restinpieces/db/mock"
)

// TestRequestEmailChangeHandler_Validation tests the initial input validation logic.
// It ensures the handler rejects requests that are malformed, have an incorrect
// content type, or are missing required/valid fields, all before attempting
// any significant business logic.
func TestRequestEmailChangeHandler_Validation(t *testing.T) {
	// A mock user is needed for the validation case that checks if the new email is the same as the old one.
	mockUser := &db.User{Email: "current@example.com", Verified: true}

	testCases := []struct {
		name        string
		contentType string
		requestBody string
		mockAuth    *MockAuth // Auth mock is needed to pass the auth check for some validation cases
		wantError   jsonResponse
	}{
		{
			name:        "invalid content type",
			contentType: "text/plain",
			requestBody: `{"new_email":"a@b.com"}`,
			mockAuth:    &MockAuth{AuthenticateFunc: func(r *http.Request) (*db.User, jsonResponse, error) { return mockUser, jsonResponse{}, nil }},
			wantError:   errorInvalidContentType,
		},
		{
			name:        "malformed json",
			contentType: "application/json",
			requestBody: `{"new_email":`,
			mockAuth:    &MockAuth{AuthenticateFunc: func(r *http.Request) (*db.User, jsonResponse, error) { return mockUser, jsonResponse{}, nil }},
			wantError:   errorInvalidRequest,
		},
		{
			name:        "missing new_email field",
			contentType: "application/json",
			requestBody: `{}`,
			mockAuth:    &MockAuth{AuthenticateFunc: func(r *http.Request) (*db.User, jsonResponse, error) { return mockUser, jsonResponse{}, nil }},
			wantError:   errorMissingFields,
		},
		{
			name:        "empty new_email field",
			contentType: "application/json",
			requestBody: `{"new_email": ""}`,
			mockAuth:    &MockAuth{AuthenticateFunc: func(r *http.Request) (*db.User, jsonResponse, error) { return mockUser, jsonResponse{}, nil }},
			wantError:   errorMissingFields,
		},
		{
			name:        "invalid email format",
			contentType: "application/json",
			requestBody: `{"new_email": "not-an-email"}`,
			mockAuth:    &MockAuth{AuthenticateFunc: func(r *http.Request) (*db.User, jsonResponse, error) { return mockUser, jsonResponse{}, nil }},
			wantError:   errorInvalidRequest,
		},
		{
			name:        "email same as current",
			contentType: "application/json",
			requestBody: `{"new_email": "current@example.com"}`,
			mockAuth:    &MockAuth{AuthenticateFunc: func(r *http.Request) (*db.User, jsonResponse, error) { return mockUser, jsonResponse{}, nil }},
			wantError:   errorEmailConflict,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/request-email-change", strings.NewReader(tc.requestBody))
			req.Header.Set("Content-Type", tc.contentType)
			rr := httptest.NewRecorder()

			// Setup mock app with only the necessary components for validation
			app := &App{
				validator:     &DefaultValidator{},
				authenticator: tc.mockAuth,
			}

			app.RequestEmailChangeHandler(rr, req)

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

// TestRequestEmailChangeHandler_Auth tests the authentication and authorization logic.
// It ensures that only authenticated and verified users can proceed.
func TestRequestEmailChangeHandler_Auth(t *testing.T) {
	testCases := []struct {
		name      string
		mockAuth  *MockAuth
		wantError jsonResponse
	}{
		{
			name: "unauthenticated request",
			mockAuth: &MockAuth{
				AuthenticateFunc: func(r *http.Request) (*db.User, jsonResponse, error) {
					return nil, errorInvalidCredentials, errors.New("auth error")
				},
			},
			wantError: errorInvalidCredentials,
		},
		{
			name: "unverified user",
			mockAuth: &MockAuth{
				AuthenticateFunc: func(r *http.Request) (*db.User, jsonResponse, error) {
					return &db.User{Email: "unverified@example.com", Verified: false}, jsonResponse{}, nil
				},
			},
			wantError: errorUnverifiedEmail,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Request body is valid, as we are not testing validation here
			reqBody := `{"new_email": "new@example.com"}`
			req := httptest.NewRequest("POST", "/request-email-change", strings.NewReader(reqBody))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()

			app := &App{
				validator:     &DefaultValidator{},
				authenticator: tc.mockAuth,
			}

			app.RequestEmailChangeHandler(rr, req)

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

// TestRequestEmailChangeHandler_DB tests the interaction with the database queue.
// It assumes a valid, authenticated, and verified user is making the request.
func TestRequestEmailChangeHandler_DB(t *testing.T) {
	// This user is assumed to be the result of a successful authentication
	mockUser := &db.User{ID: "user123", Email: "current@example.com", Verified: true}

	testCases := []struct {
		name      string
		dbSetup   func(*mock.Db)
		wantError jsonResponse
	}{
		{
			name: "successful job insertion",
			dbSetup: func(m *mock.Db) {
				m.InsertJobFunc = func(job db.Job) error {
					// Optional: assert job properties here if needed
					return nil
				}
			},
			wantError: okEmailChangeRequested,
		},
		{
			name: "unique constraint violation",
			dbSetup: func(m *mock.Db) {
				m.InsertJobFunc = func(job db.Job) error {
					return db.ErrConstraintUnique
				}
			},
			wantError: errorEmailChangeAlreadyRequested,
		},
		{
			name: "generic database error",
			dbSetup: func(m *mock.Db) {
				m.InsertJobFunc = func(job db.Job) error {
					return errors.New("db connection failed")
				}
			},
			wantError: errorAuthDatabaseError,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			reqBody := `{"new_email": "new@example.com"}`
			req := httptest.NewRequest("POST", "/request-email-change", strings.NewReader(reqBody))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()

			mockDb := &mock.Db{}
			tc.dbSetup(mockDb)

			// Mock authenticator to always return the valid user
			mockAuth := &MockAuth{
				AuthenticateFunc: func(r *http.Request) (*db.User, jsonResponse, error) {
					return mockUser, jsonResponse{}, nil
				},
			}

			// Create a test config with rate limits for cooldown bucket calculation
			testConfig := &config.Config{
				RateLimits: config.RateLimits{
					EmailChangeCooldown: config.Duration{Duration: 5 * time.Minute},
				},
			}

			app := &App{
				validator:      &DefaultValidator{},
				authenticator:  mockAuth,
				dbQueue:        mockDb,
				configProvider: config.NewProvider(testConfig),
			}

			app.RequestEmailChangeHandler(rr, req)

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
