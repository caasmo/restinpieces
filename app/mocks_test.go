package app_test

import (
	"net/http"
	
	"github.com/caasmo/restinpieces/db"
	"github.com/caasmo/restinpieces/router"
)

// MockDB implements the Db interface for testing
type MockDB struct{}

func (m *MockDB) Close()                     {}
func (m *MockDB) GetById(id int64) int       { return 0 }
func (m *MockDB) Insert(value int64)         {}
func (m *MockDB) InsertWithPool(value int64) {}

// MockRouter implements the Router interface for testing
type MockRouter struct{}

func (m *MockRouter) Handle(path string, handler http.Handler)          {}
func (m *MockRouter) HandleFunc(path string, handler func(http.ResponseWriter, *http.Request)) {}
func (m *MockRouter) ServeHTTP(w http.ResponseWriter, r *http.Request)  {}
func (m *MockRouter) Param(req *http.Request, key string) string       { return "" }
