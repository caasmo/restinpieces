package mock

import (
	"github.com/caasmo/restinpieces/db"
)

// Compile-time check to ensure Db implements the DbApp interface
var _ db.DbApp = (*Db)(nil)

// Db implements db.DbApp for testing purposes.
// Use function fields to allow overriding behavior in specific tests.
type Db struct {
	// --- Mock DbAuth Methods ---
	GetUserByEmailFunc         func(email string) (*db.User, error)
	GetUserByIdFunc            func(id string) (*db.User, error)
	CreateUserWithPasswordFunc func(user db.User) (*db.User, error)
	CreateUserWithOauth2Func   func(user db.User) (*db.User, error)
	VerifyEmailFunc            func(userId string) error
	UpdatePasswordFunc         func(userId string, newPassword string) error
	UpdateEmailFunc            func(userId string, newEmail string) error

	// --- Mock DbQueue Methods ---
	InsertJobFunc              func(job db.Job) error
	ClaimFunc                  func(limit int) ([]*db.Job, error)
	MarkCompletedFunc          func(jobID int64) error
	MarkFailedFunc             func(jobID int64, errMsg string) error
	MarkRecurrentCompletedFunc func(completedJobID int64, newJob db.Job) error

	// --- Mock DbConfig Methods ---
	GetConfigFunc    func(scope string, generation int) ([]byte, string, error)
	InsertConfigFunc func(scope string, contentData []byte, format string, description string) error
	PathFunc         func() string

	// DbLifecycle methods removed
}

// --- Implement DbAuth ---
func (m *Db) GetUserByEmail(email string) (*db.User, error) {
	if m.GetUserByEmailFunc != nil {
		return m.GetUserByEmailFunc(email)
	}
	return nil, db.ErrUserNotFound // Default: Not found
}
func (m *Db) GetUserById(id string) (*db.User, error) {
	if m.GetUserByIdFunc != nil {
		return m.GetUserByIdFunc(id)
	}
	return nil, db.ErrUserNotFound // Default: Not found
}
func (m *Db) CreateUserWithPassword(user db.User) (*db.User, error) {
	if m.CreateUserWithPasswordFunc != nil {
		return m.CreateUserWithPasswordFunc(user)
	}
	// Default: Return the user passed in, assuming success
	user.ID = "mock-pw-user-id" // Assign a mock ID
	return &user, nil
}
func (m *Db) CreateUserWithOauth2(user db.User) (*db.User, error) {
	if m.CreateUserWithOauth2Func != nil {
		return m.CreateUserWithOauth2Func(user)
	}
	// Default: Return the user passed in, assuming success
	user.ID = "mock-oauth-user-id" // Assign a mock ID
	return &user, nil
}
func (m *Db) VerifyEmail(userId string) error {
	if m.VerifyEmailFunc != nil {
		return m.VerifyEmailFunc(userId)
	}
	return nil // Default: Success
}
func (m *Db) UpdatePassword(userId string, newPassword string) error {
	if m.UpdatePasswordFunc != nil {
		return m.UpdatePasswordFunc(userId, newPassword)
	}
	return nil // Default: Success
}
func (m *Db) UpdateEmail(userId string, newEmail string) error {
	if m.UpdateEmailFunc != nil {
		return m.UpdateEmailFunc(userId, newEmail)
	}
	return nil // Default: Success
}

// --- Implement DbQueue ---
func (m *Db) InsertJob(job db.Job) error {
	if m.InsertJobFunc != nil {
		return m.InsertJobFunc(job)
	}
	return nil // Default: Success
}
func (m *Db) Claim(limit int) ([]*db.Job, error) {
	if m.ClaimFunc != nil {
		return m.ClaimFunc(limit)
	}
	return []*db.Job{}, nil // Default: No jobs claimed
}
func (m *Db) MarkCompleted(jobID int64) error {
	if m.MarkCompletedFunc != nil {
		return m.MarkCompletedFunc(jobID)
	}
	return nil // Default: Success
}
func (m *Db) MarkFailed(jobID int64, errMsg string) error {
	if m.MarkFailedFunc != nil {
		return m.MarkFailedFunc(jobID, errMsg)
	}
	return nil // Default: Success
}
func (m *Db) MarkRecurrentCompleted(completedJobID int64, newJob db.Job) error {
	if m.MarkRecurrentCompletedFunc != nil {
		return m.MarkRecurrentCompletedFunc(completedJobID, newJob)
	}
	return nil // Default: Success
}

// --- Implement DbConfig ---
func (m *Db) GetConfig(scope string, generation int) ([]byte, string, error) {
	if m.GetConfigFunc != nil {
		return m.GetConfigFunc(scope, generation)
	}
	return nil, "", nil // Default: No config found, no error
}

func (m *Db) InsertConfig(scope string, contentData []byte, format string, description string) error {
	if m.InsertConfigFunc != nil {
		return m.InsertConfigFunc(scope, contentData, format, description)
	}
	return nil // Default: Success
}

// Path implements db.DbConfig for testing purposes.
func (m *Db) Path() string {
	if m.PathFunc != nil {
		return m.PathFunc()
	}
	return "/tmp/mock.db"
}

