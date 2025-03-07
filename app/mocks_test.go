package app

import (
	"net/http"
	"time"
	
	"github.com/caasmo/restinpieces/db"
)

// MockDB implements db.Db interface for testing
type MockDB struct{}

func (m *MockDB) Close()                     {}
func (m *MockDB) GetById(id int64) int       { return 0 }
func (m *MockDB) Insert(value int64)         {}
func (m *MockDB) InsertWithPool(value int64) {}
func (m *MockDB) CreateUser(email, hashedPassword, name string) (*db.User, error) {
	return &db.User{
		ID:        "mock-user",
		Email:     email,
		Name:      name,
		Password:  hashedPassword,
		Created:   "2024-01-01T00:00:00Z",
		Updated:   "2024-01-01T00:00:00Z",
	}, nil
}

// MockRouter implements router.Router interface for testing
type MockRouter struct{}

func (m *MockRouter) Handle(path string, handler http.Handler)                                 {}
func (m *MockRouter) HandleFunc(path string, handler func(http.ResponseWriter, *http.Request)) {}
func (m *MockRouter) ServeHTTP(w http.ResponseWriter, r *http.Request)                         {}
func (m *MockRouter) Param(req *http.Request, key string) string                               { return "" }
