package servemux

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestServeMuxRouter(t *testing.T) {
	mux := New()

	// Register handlers for testing
	// Note: Go's new ServeMux requires method specification in the path for registration.
	mux.Handle("GET /hello", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "Hello, World!")
	}))
	mux.Handle("POST /data", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		fmt.Fprint(w, "Data created")
	}))
	mux.Handle("GET /users/{id}", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The custom Param method is not used directly in Handle, but we test the underlying mechanism
		id := r.PathValue("id")
		fmt.Fprintf(w, "User ID: %s", id)
	}))
	mux.Handle("GET /users/new", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "New User Form")
	}))

	testCases := []struct {
		name           string
		method         string
		path           string
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "Simple GET",
			method:         "GET",
			path:           "/hello",
			expectedStatus: http.StatusOK,
			expectedBody:   "Hello, World!",
		},
		{
			name:           "Simple POST",
			method:         "POST",
			path:           "/data",
			expectedStatus: http.StatusCreated,
			expectedBody:   "Data created",
		},
		{
			name:           "Not Found",
			method:         "GET",
			path:           "/not/found",
			expectedStatus: http.StatusNotFound,
			expectedBody:   "404 page not found\n",
		},
		{
			name:           "Method Not Allowed",
			method:         "GET",
			path:           "/data",
			expectedStatus: http.StatusMethodNotAllowed,
			expectedBody:   "Method Not Allowed\n",
		},
		{
			name:           "Path Parameter",
			method:         "GET",
			path:           "/users/123",
			expectedStatus: http.StatusOK,
			expectedBody:   "User ID: 123",
		},
		{
			name:           "Static Route Before Param Route",
			method:         "GET",
			path:           "/users/new",
			expectedStatus: http.StatusOK,
			expectedBody:   "New User Form",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			rr := httptest.NewRecorder()

			mux.ServeHTTP(rr, req)

			if rr.Code != tc.expectedStatus {
				t.Errorf("handler returned wrong status code: got %v want %v",
					rr.Code, tc.expectedStatus)
			}

			body, err := io.ReadAll(rr.Body)
			if err != nil {
				t.Fatalf("could not read response body: %v", err)
			}
			if string(body) != tc.expectedBody {
				t.Errorf("handler returned unexpected body: got %q want %q",
					string(body), tc.expectedBody)
			}
		})
	}
}