package core

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/caasmo/restinpieces/config"
	"github.com/caasmo/restinpieces/crypto"
	"github.com/caasmo/restinpieces/db"
	"github.com/caasmo/restinpieces/db/mock"
)

// TestRegisterWithPasswordHandler_Validation tests the initial validation steps of the
// registration handler. It covers scenarios like invalid request body, missing fields,
// password mismatches, and complexity failures, ensuring the handler rejects invalid
// requests early.
func TestRegisterWithPasswordHandler_Validation(t *testing.T) {
	testCases := []struct {
		name        string
		requestBody string
		wantError   jsonResponse
	}{
		{
			name:        "malformed json",
			requestBody: `{"identity":"test@example.com",`,
			wantError:   errorInvalidRequest,
		},
		{
			name:        "missing identity field",
			requestBody: `{"password":"password123", "password_confirm":"password123"}`,
			wantError:   errorMissingFields,
		},
		{
			name:        "missing password field",
			requestBody: `{"identity":"test@example.com", "password_confirm":"password123"}`,
			wantError:   errorMissingFields,
		},
		{
			name:        "missing password confirm field",
			requestBody: `{"identity":"test@example.com", "password":"password123"}`,
			wantError:   errorMissingFields,
		},
		{
			name:        "password mismatch",
			requestBody: `{"identity":"test@example.com", "password":"password123", "password_confirm":"password456"}`,
			wantError:   errorPasswordMismatch,
		},
		{
			name:        "password_complexity_failure",
			requestBody: `{"identity":"test@example.com", "password":"short", "password_confirm":"short"}`,
			wantError:   errorWeakPassword,
		},

		{
			name:        "invalid email format",
			requestBody: `{"identity":"not-an-email", "password":"password123", "password_confirm":"password123"}`,
			wantError:   errorInvalidRequest,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/register-with-password", strings.NewReader(tc.requestBody))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()

			app := &App{
				validator: &DefaultValidator{},
			}

			app.RegisterWithPasswordHandler(rr, req)

			if rr.Code != tc.wantError.status {
				t.Errorf("expected status %d, got %d", tc.wantError.status, rr.Code)
			}

			var gotBody, wantBody map[string]interface{}
			if err := json.NewDecoder(rr.Body).Decode(&gotBody); err != nil {
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

// TestRegisterWithPasswordHandler_RegistrationLogic tests the core business logic of the
// registration handler. It covers the happy path for a new user, the conflict case
// for an existing user with a password, and the scenario where an existing verified
// user (e.g., from OAuth2) adds a password.
func TestRegisterWithPasswordHandler_RegistrationLogic(t *testing.T) {
	hashedPassword, _ := crypto.GenerateHash("password123")
	newUser := db.User{
		ID:       "user123",
		Email:    "new@example.com",
		Password: string(hashedPassword),
		Verified: false,
	}

	existingUserWithPassword := db.User{
		ID:       "user456",
		Email:    "existing@example.com",
		Password: "different_hash",
		Verified: true,
	}

	existingVerifiedUserNoPassword := db.User{
		ID:       "user789",
		Email:    "verified@example.com",
		Password: string(hashedPassword), // This will be set by the handler
		Verified: true,
	}

	testCases := []struct {
		name              string
		requestBody       string
		dbSetup           func(*mock.Db, *bool)
		wantStatus        int
		wantCode          string
		expectJobInserted bool
	}{
		{
			name:        "successful new user registration",
			requestBody: `{"identity":"new@example.com", "password":"password123", "password_confirm":"password123"}`,
			dbSetup: func(m *mock.Db, jobInserted *bool) {
				m.CreateUserWithPasswordFunc = func(user db.User) (*db.User, error) {
					if !strings.HasPrefix(user.Password, "$2a$") {
						t.Error("password was not hashed before CreateUserWithPassword")
					}
					newUser.Password = user.Password
					return &newUser, nil
				}
			},
			wantStatus:        http.StatusAccepted,
			wantCode:          CodeOkPendingEmailOtpVerification,
			expectJobInserted: false,
		},
		{
			name:        "registration attempt with existing email",
			requestBody: `{"identity":"existing@example.com", "password":"password123", "password_confirm":"password123"}`,
			dbSetup: func(m *mock.Db, jobInserted *bool) {
				m.CreateUserWithPasswordFunc = func(user db.User) (*db.User, error) {
					return &existingUserWithPassword, nil
				}
			},
			wantStatus:        http.StatusAccepted,
			wantCode:          CodeOkPendingEmailOtpVerification,
			expectJobInserted: false,
		},
		{
			name:        "registration for existing verified user without password",
			requestBody: `{"identity":"verified@example.com", "password":"password123", "password_confirm":"password123"}`,
			dbSetup: func(m *mock.Db, jobInserted *bool) {
				m.CreateUserWithPasswordFunc = func(user db.User) (*db.User, error) {
					existingVerifiedUserNoPassword.Password = user.Password
					return &existingVerifiedUserNoPassword, nil
				}
			},
			wantStatus:        http.StatusAccepted,
			wantCode:          CodeOkPendingEmailOtpVerification,
			expectJobInserted: false,
		},
		{
			name:        "successful registration with whitespace trimming",
			requestBody: `{"identity":"  trimmed@example.com  ", "password":"  password123  ", "password_confirm":"  password123  "}`,
			dbSetup: func(m *mock.Db, jobInserted *bool) {
				m.CreateUserWithPasswordFunc = func(user db.User) (*db.User, error) {
					if user.Email != "trimmed@example.com" {
						t.Errorf("expected email to be trimmed, got %q", user.Email)
					}
					// We can't easily check the plaintext password because it's hashed,
					// but we can verify it hashes correctly against the trimmed version.
					if !crypto.CheckPassword("password123", user.Password) {
						t.Error("password was not trimmed correctly before hashing")
					}
					return &newUser, nil
				}
			},
			wantStatus:        http.StatusAccepted,
			wantCode:          CodeOkPendingEmailOtpVerification,
			expectJobInserted: false,
		},
		{
			name:        "successful registration with normalization",
			requestBody: `{"identity":"  NORMALIZED@Example.Com  ", "password":"password123", "password_confirm":"password123"}`,
			dbSetup: func(m *mock.Db, jobInserted *bool) {
				m.CreateUserWithPasswordFunc = func(user db.User) (*db.User, error) {
					if user.Email != "normalized@example.com" {
						t.Errorf("expected email to be normalized to lowercase, got %q", user.Email)
					}
					return &newUser, nil
				}
			},
			wantStatus:        http.StatusAccepted,
			wantCode:          CodeOkPendingEmailOtpVerification,
			expectJobInserted: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/register-with-password", strings.NewReader(tc.requestBody))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()

			mockDb := &mock.Db{}
			jobInserted := false
			tc.dbSetup(mockDb, &jobInserted)

			app := &App{
				validator: &DefaultValidator{},
				dbAuth:    mockDb,
				dbQueue:   mockDb,
			}

			app.RegisterWithPasswordHandler(rr, req)

			if rr.Code != tc.wantStatus {
				t.Errorf("expected status %d, got %d", tc.wantStatus, rr.Code)
			}

			var body map[string]interface{}
			if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
				t.Fatalf("failed to decode response body: %v", err)
			}

			if code, _ := body["code"].(string); code != tc.wantCode {
				t.Errorf("expected code %q, got %q", tc.wantCode, code)
			}

			if jobInserted != tc.expectJobInserted {
				t.Errorf("expected job insertion to be %v, but was %v", tc.expectJobInserted, jobInserted)
			}
		})
	}
}

// TestRegisterWithPasswordHandler_DependencyFailures tests how the handler responds to
// failures in its dependencies, such as the database.
func TestRegisterWithPasswordHandler_DependencyFailures(t *testing.T) {
	requestBody := `{"identity":"test@example.com", "password":"password123", "password_confirm":"password123"}`

	testCases := []struct {
		name      string
		dbSetup   func(*mock.Db)
		config    *config.Config
		wantError jsonResponse
	}{
		{
			name: "database failure on user creation",
			dbSetup: func(m *mock.Db) {
				m.CreateUserWithPasswordFunc = func(user db.User) (*db.User, error) {
					return nil, errors.New("db connection failed")
				}
			},
			config:    config.NewDefaultConfig(),
			wantError: errorAuthDatabaseError,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/register-with-password", strings.NewReader(requestBody))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()

			mockDb := &mock.Db{}
			tc.dbSetup(mockDb)

			app := &App{
				validator:      &DefaultValidator{},
				dbAuth:         mockDb,
				dbQueue:        mockDb,
				configProvider: config.NewProvider(tc.config),
			}

			app.RegisterWithPasswordHandler(rr, req)

			if rr.Code != tc.wantError.status {
				t.Errorf("expected status %d, got %d", tc.wantError.status, rr.Code)
			}

			var gotBody, wantBody map[string]interface{}
			if err := json.NewDecoder(rr.Body).Decode(&gotBody); err != nil {
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
