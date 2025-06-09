package core

import (
	"net/http"

	"github.com/caasmo/restinpieces/db"
	"github.com/caasmo/restinpieces/router"
)

// Compile-time check to ensure MockDB implements the DbApp interface
var _ db.DbApp = (*MockDB)(nil)

// MockDB implements db.DbApp for testing purposes.
// Use function fields to allow overriding behavior in specific tests.
type MockDB struct {
	// --- Mock DbAuth Methods ---
	GetUserByEmailFunc         func(email string) (*db.User, error)
	GetUserByIdFunc            func(id string) (*db.User, error)
	CreateUserWithPasswordFunc func(user db.User) (*db.User, error)
	CreateUserWithOauth2Func   func(user db.User) (*db.User, error)
	VerifyEmailFunc            func(userId string) error
	UpdatePasswordFunc         func(userId string, newPassword string) error
	UpdateEmailFunc            func(userId string, newEmail string) error

	// --- Mock DbQueue Methods ---
	InsertJobFunc            func(job db.Job) error
	ClaimFunc                func(limit int) ([]*db.Job, error)
	MarkCompletedFunc        func(jobID int64) error
	MarkFailedFunc           func(jobID int64, errMsg string) error
	MarkRecurrentCompletedFunc func(completedJobID int64, newJob db.Job) error

	// --- Mock DbConfig Methods ---
	GetConfigFunc    func(scope string, generation int) ([]byte, string, error)
	InsertConfigFunc func(scope string, contentData []byte, format string, description string) error

	// DbLifecycle methods removed
}

// --- Implement DbAuth ---
func (m *MockDB) GetUserByEmail(email string) (*db.User, error) {
	if m.GetUserByEmailFunc != nil {
		return m.GetUserByEmailFunc(email)
	}
	return nil, db.ErrUserNotFound // Default: Not found
}
func (m *MockDB) GetUserById(id string) (*db.User, error) {
	if m.GetUserByIdFunc != nil {
		return m.GetUserByIdFunc(id)
	}
	return nil, db.ErrUserNotFound // Default: Not found
}
func (m *MockDB) CreateUserWithPassword(user db.User) (*db.User, error) {
	if m.CreateUserWithPasswordFunc != nil {
		return m.CreateUserWithPasswordFunc(user)
	}
	// Default: Return the user passed in, assuming success
	user.ID = "mock-pw-user-id" // Assign a mock ID
	return &user, nil
}
func (m *MockDB) CreateUserWithOauth2(user db.User) (*db.User, error) {
	if m.CreateUserWithOauth2Func != nil {
		return m.CreateUserWithOauth2Func(user)
	}
	// Default: Return the user passed in, assuming success
	user.ID = "mock-oauth-user-id" // Assign a mock ID
	return &user, nil
}
func (m *MockDB) VerifyEmail(userId string) error {
	if m.VerifyEmailFunc != nil {
		return m.VerifyEmailFunc(userId)
	}
	return nil // Default: Success
}
func (m *MockDB) UpdatePassword(userId string, newPassword string) error {
	if m.UpdatePasswordFunc != nil {
		return m.UpdatePasswordFunc(userId, newPassword)
	}
	return nil // Default: Success
}
func (m *MockDB) UpdateEmail(userId string, newEmail string) error {
	if m.UpdateEmailFunc != nil {
		return m.UpdateEmailFunc(userId, newEmail)
	}
	return nil // Default: Success
}

// --- Implement DbQueue ---
func (m *MockDB) InsertJob(job db.Job) error {
	if m.InsertJobFunc != nil {
		return m.InsertJobFunc(job)
	}
	return nil // Default: Success
}
func (m *MockDB) Claim(limit int) ([]*db.Job, error) {
	if m.ClaimFunc != nil {
		return m.ClaimFunc(limit)
	}
	return []*db.Job{}, nil // Default: No jobs claimed
}
func (m *MockDB) MarkCompleted(jobID int64) error {
	if m.MarkCompletedFunc != nil {
		return m.MarkCompletedFunc(jobID)
	}
	return nil // Default: Success
}
func (m *MockDB) MarkFailed(jobID int64, errMsg string) error {
	if m.MarkFailedFunc != nil {
		return m.MarkFailedFunc(jobID, errMsg)
	}
	return nil // Default: Success
}
func (m *MockDB) MarkRecurrentCompleted(completedJobID int64, newJob db.Job) error {
	if m.MarkRecurrentCompletedFunc != nil {
		return m.MarkRecurrentCompletedFunc(completedJobID, newJob)
	}
	return nil // Default: Success
}

// --- Implement DbConfig ---
func (m *MockDB) GetConfig(scope string, generation int) ([]byte, string, error) {
	if m.GetConfigFunc != nil {
		return m.GetConfigFunc(scope, generation)
	}
	return nil, "", nil // Default: No config found, no error
}

func (m *MockDB) InsertConfig(scope string, contentData []byte, format string, description string) error {
	if m.InsertConfigFunc != nil {
		return m.InsertConfigFunc(scope, contentData, format, description)
	}
	return nil // Default: Success
}

// --- DbLifecycle methods removed ---

// MockAuthenticator implements the Authenticator interface for testing
type MockAuth struct {
	// AuthenticateFunc allows customizing the authentication behavior
	AuthenticateFunc func(r *http.Request) (*db.User, error, jsonResponse)
}

func (m *MockAuth) Authenticate(r *http.Request) (*db.User, error, jsonResponse) {
	if m.AuthenticateFunc != nil {
		return m.AuthenticateFunc(r)
	}
	// Default: Return nil user with no error (unauthenticated)
	return nil, nil, jsonResponse{}
}

// MockRouter implements router.Router interface for testing
type MockRouter struct{}

func (m *MockRouter) Handle(path string, handler http.Handler)                                 {}
func (m *MockRouter) HandleFunc(path string, handler func(http.ResponseWriter, *http.Request)) {}
func (m *MockRouter) ServeHTTP(w http.ResponseWriter, r *http.Request)                         {}
func (m *MockRouter) Param(req *http.Request, key string) string                               { return "" }
func (m *MockRouter) Register(chains router.Chains)                                            {}
