package app

import (
	"fmt"
	"net/http"
	"time"

	"github.com/caasmo/restinpieces/db"
	"github.com/caasmo/restinpieces/queue"
)

// MockDB implements db.Db interface for testing
type MockDB struct{}

func (m *MockDB) Close()                     {}
func (m *MockDB) GetById(id int64) int       { return 0 }
func (m *MockDB) Insert(value int64)         {}
func (m *MockDB) InsertWithPool(value int64) {}
func (m *MockDB) InsertQueueJob(job queue.QueueJob) error {
	return nil
}
func (m *MockDB) CreateUser(user db.User) (*db.User, error) {
	return &db.User{
		ID:       "mock-user",
		Email:    user.Email,
		Name:     user.Name,
		Password: user.Password,
		Created:  time.Time{},
		Updated:  time.Time{},
		TokenKey: user.TokenKey,
	}, nil
}

func (m *MockDB) GetUserByEmail(email string) (*db.User, error) {
	switch email {
	case "existing@test.com":
		return &db.User{
			ID:       "test123",
			Email:    email,
			Name:     "Test User",
			Password: "hash123",
			Created:  time.Time{},
			Updated:  time.Time{},
		}, nil
	case "test@example.com":
		return &db.User{
			ID:       "test456",
			Email:    email,
			Name:     "Test Example",
			Password: "hash456",
			Created:  time.Time{},
			Updated:  time.Time{},
			Verified: false,
		}, nil
	default:
		return nil, fmt.Errorf("user not found")
	}
}

// MockRouter implements router.Router interface for testing
type MockRouter struct{}

func (m *MockRouter) Handle(path string, handler http.Handler)                                 {}
func (m *MockRouter) HandleFunc(path string, handler func(http.ResponseWriter, *http.Request)) {}
func (m *MockRouter) ServeHTTP(w http.ResponseWriter, r *http.Request)                         {}
func (m *MockRouter) Param(req *http.Request, key string) string                               { return "" }
