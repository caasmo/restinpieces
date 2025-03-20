package router_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	
	rtr "github.com/caasmo/restinpieces/router"
)

func TestRouteBasicHandler(t *testing.T) {
	route := rtr.NewRoute("GET /test").
		WithHandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		})
	
	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()
	
	route.Handler().ServeHTTP(rec, req)
	
	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if body := rec.Body.String(); body != "OK" {
		t.Errorf("expected body 'OK', got '%s'", body)
	}
}

func TestRouteMiddlewareChaining(t *testing.T) {
	var callOrder []string

	mw1 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callOrder = append(callOrder, "mw1")
			next.ServeHTTP(w, r)
		})
	}

	mw2 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callOrder = append(callOrder, "mw2")
			next.ServeHTTP(w, r)
		})
	}

	route := rtr.NewRoute("GET /test").
		WithHandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callOrder = append(callOrder, "handler")
			w.WriteHeader(http.StatusOK)
		}).
		WithMiddleware(mw1, mw2)

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()
	
	route.Handler().ServeHTTP(rec, req)

	expectedOrder := []string{"mw2", "mw1", "handler"}
	if len(callOrder) != len(expectedOrder) {
		t.Fatalf("expected %d calls, got %d", len(expectedOrder), len(callOrder))
	}
	for i, val := range expectedOrder {
		if callOrder[i] != val {
			t.Errorf("expected %s at position %d, got %s", val, i, callOrder[i])
		}
	}
}

func TestRouteObservers(t *testing.T) {
	var calledHandlers []string

	observer1 := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calledHandlers = append(calledHandlers, "observer1")
	})

	observer2 := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calledHandlers = append(calledHandlers, "observer2")
	})

	route := rtr.NewRoute("GET /test").
		WithHandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calledHandlers = append(calledHandlers, "handler")
			w.WriteHeader(http.StatusOK)
		}).
		WithObservers(observer1, observer2)

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()
	
	route.Handler().ServeHTTP(rec, req)

	expectedHandlers := []string{"handler", "observer1", "observer2"}
	if len(calledHandlers) != len(expectedHandlers) {
		t.Fatalf("expected %d calls, got %d", len(expectedHandlers), len(calledHandlers))
	}
	for i, val := range expectedHandlers {
		if calledHandlers[i] != val {
			t.Errorf("expected %s at position %d, got %s", val, i, calledHandlers[i])
		}
	}
}

func TestRouteNilHandler(t *testing.T) {
	route := rtr.NewRoute("GET /test")
	
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic with nil handler")
		}
	}()
	
	route.Handler() // Should panic
}

func TestRouteEmptyEndpoint(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic with empty endpoint")
		}
	}()
	
	rtr.NewRoute("") // Should panic
}
