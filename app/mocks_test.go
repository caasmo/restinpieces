package app

import (
	"net/http"

	"github.com/caasmo/restinpieces/db"
	"github.com/caasmo/restinpieces/queue"
	"github.com/caasmo/restinpieces/router"
)

// MockDB implements db.Db interface for testing
type MockDB struct {
	// Method-specific configurations
	GetUserByEmailConfig struct {
		User  *db.User
		Error error
	}
	GetUserByIdConfig struct {
		User  *db.User
		Error error
	}
	CreateUserConfig struct {
		User  *db.User
		Error error
	}
	// Add more method configs as needed
}

func (m *MockDB) Close()                     {}
func (m *MockDB) GetById(id int64) int       { return 0 }
func (m *MockDB) Insert(value int64)         {}
func (m *MockDB) InsertWithPool(value int64) {}
func (m *MockDB) InsertJob(job queue.Job) error {
	return nil
}
func (m *MockDB) GetUserByEmail(email string) (*db.User, error) {
	return m.GetUserByEmailConfig.User, m.GetUserByEmailConfig.Error
}

func (m *MockDB) CreateUser(user db.User) (*db.User, error) {
	return m.CreateUserConfig.User, m.CreateUserConfig.Error
}

func (m *MockDB) CreateUserWithPassword(user db.User) (*db.User, error) {
	return m.CreateUserConfig.User, m.CreateUserConfig.Error
}

func (m *MockDB) CreateUserWithOauth2(user db.User) (*db.User, error) {
	return m.CreateUserConfig.User, m.CreateUserConfig.Error
}

func (m *MockDB) GetUserById(id string) (*db.User, error) {
	return m.GetUserByIdConfig.User, m.GetUserByIdConfig.Error
}

func (m *MockDB) GetJobs(limit int) ([]*queue.Job, error) {
	return nil, nil
}

func (m *MockDB) Claim(limit int) ([]*queue.Job, error) {
	return nil, nil
}

// MockRouter implements router.Router interface for testing
type MockRouter struct{}

func (m *MockRouter) Handle(path string, handler http.Handler)                                 {}
func (m *MockRouter) HandleFunc(path string, handler func(http.ResponseWriter, *http.Request)) {}
func (m *MockRouter) ServeHTTP(w http.ResponseWriter, r *http.Request)                         {}
func (m *MockRouter) Param(req *http.Request, key string) string                               { return "" }
func (m *MockRouter) Register(routes ...*router.Route)                                         {}
