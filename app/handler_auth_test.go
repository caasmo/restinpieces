package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// MockDB implements db.Db interface for testing
type MockDB struct{}

func (m *MockDB) Close()                     {}
func (m *MockDB) GetById(id int64) int       { return 0 }
func (m *MockDB) Insert(value int64)         {}
func (m *MockDB) InsertWithPool(value int64) {}

// MockRouter implements router.Router interface for testing
type MockRouter struct{}

func (m *MockRouter) Handle(path string, handler http.Handler)                                 {}
func (m *MockRouter) HandleFunc(path string, handler func(http.ResponseWriter, *http.Request)) {}
func (m *MockRouter) ServeHTTP(w http.ResponseWriter, r *http.Request)                         {}
func (m *MockRouter) Param(req *http.Request, key string) string                               { return "" }

func TestRefreshAuthHandler(t *testing.T) {
	testCases := []struct {
		name       string
		userID     string
		wantStatus int
	}{
		{
			name:       "valid token refresh",
			userID:     "testuser123",
			wantStatus: http.StatusOK,
		},
		{
			name:       "missing user claims",
			userID:     "",
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/auth-refresh", nil)
			rr := httptest.NewRecorder()
			a, _ := New(
				WithConfig(&Config{
					JwtSecret:     []byte("test_secret"),
					TokenDuration: 15 * time.Minute,
				}),
				WithDB(&MockDB{}),
				WithRouter(&MockRouter{}),
			)

			// Add user ID to context directly since middleware would normally handle this
			ctx := context.WithValue(req.Context(), UserIDKey, tc.userID)
			a.RefreshAuthHandler(rr, req.WithContext(ctx))

			if rr.Code != tc.wantStatus {
				t.Errorf("expected status %d, got %d", tc.wantStatus, rr.Code)
			}

			if tc.wantStatus == http.StatusOK {
				var body map[string]interface{}
				if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
					t.Fatalf("failed to decode response body: %v", err)
				}

				if _, ok := body["access_token"]; !ok {
					t.Error("response missing access_token")
				}
				if body["token_type"] != "Bearer" {
					t.Errorf("expected token_type Bearer, got %s", body["token_type"])
				}
			}
		})
	}
}
