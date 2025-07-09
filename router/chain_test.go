package router_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	rtr "github.com/caasmo/restinpieces/router"
)

func TestChainBasicHandler(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})
	chain := rtr.NewChain(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	chain.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if body := rec.Body.String(); body != "OK" {
		t.Errorf("expected body 'OK', got '%s'", body)
	}
}

func TestChainMiddlewareChaining(t *testing.T) {
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

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callOrder = append(callOrder, "handler")
		w.WriteHeader(http.StatusOK)
	})
	chain := rtr.NewChain(handler).
		WithMiddleware(mw1, mw2)

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	chain.Handler().ServeHTTP(rec, req)

	expectedOrder := []string{"mw1", "mw2", "handler"}
	if len(callOrder) != len(expectedOrder) {
		t.Fatalf("expected %d calls, got %d", len(expectedOrder), len(callOrder))
	}
	for i, val := range expectedOrder {
		if callOrder[i] != val {
			t.Errorf("expected %s at position %d, got %s", val, i, callOrder[i])
		}
	}
}

func TestChainObservers(t *testing.T) {
	var calledHandlers []string

	observer1 := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calledHandlers = append(calledHandlers, "observer1")
	})

	observer2 := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calledHandlers = append(calledHandlers, "observer2")
	})

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calledHandlers = append(calledHandlers, "handler")
		w.WriteHeader(http.StatusOK)
	})
	chain := rtr.NewChain(handler).
		WithObservers(observer1, observer2)

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	chain.Handler().ServeHTTP(rec, req)

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

// TestNewChainNilHandler verifies that NewChain panics if a nil handler is provided.
func TestNewChainNilHandler(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic when creating chain with nil handler")
		}
	}()
	_ = rtr.NewChain(nil) // Should panic
}

func TestChainMiddlewareChain(t *testing.T) {
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

	// Create middleware chain
	middlewareChain := []func(http.Handler) http.Handler{mw1, mw2}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callOrder = append(callOrder, "handler")
		w.WriteHeader(http.StatusOK)
	})
	chain := rtr.NewChain(handler).
		WithMiddlewareChain(middlewareChain)

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	chain.Handler().ServeHTTP(rec, req)

	expectedOrder := []string{"mw1", "mw2", "handler"}
	if len(callOrder) != len(expectedOrder) {
		t.Fatalf("expected %d calls, got %d", len(expectedOrder), len(callOrder))
	}
	for i, val := range expectedOrder {
		if callOrder[i] != val {
			t.Errorf("expected %s at position %d, got %s", val, i, callOrder[i])
		}
	}
}

func TestChainChainedWithMiddleware(t *testing.T) {
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

	mw3 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callOrder = append(callOrder, "mw3")
			next.ServeHTTP(w, r)
		})
	}

	mw4 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callOrder = append(callOrder, "mw4")
			next.ServeHTTP(w, r)
		})
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callOrder = append(callOrder, "handler")
		w.WriteHeader(http.StatusOK)
	})
	chain := rtr.NewChain(handler).
		WithMiddleware(mw1, mw2). // First middlewares
		WithMiddleware(mw3, mw4)  // Second middlewares

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	chain.Handler().ServeHTTP(rec, req)

	expectedOrder := []string{"mw1", "mw2", "mw3", "mw4", "handler"}
	if len(callOrder) != len(expectedOrder) {
		t.Fatalf("expected %d calls, got %d", len(expectedOrder), len(callOrder))
	}
	for i, val := range expectedOrder {
		if callOrder[i] != val {
			t.Errorf("expected %s at position %d, got %s", val, i, callOrder[i])
		}
	}
}

func TestChainMixedMiddlewareChaining(t *testing.T) {
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

	mw3 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callOrder = append(callOrder, "mw3")
			next.ServeHTTP(w, r)
		})
	}

	mw4 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callOrder = append(callOrder, "mw4")
			next.ServeHTTP(w, r)
		})
	}

	// Create initial middleware chain
	middlewareChain := []func(http.Handler) http.Handler{mw1, mw2}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callOrder = append(callOrder, "handler")
		w.WriteHeader(http.StatusOK)
	})
	chain := rtr.NewChain(handler).
		WithMiddlewareChain(middlewareChain). // First chain
		WithMiddleware(mw3, mw4)              // Additional middlewares

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	chain.Handler().ServeHTTP(rec, req)

	expectedOrder := []string{"mw1", "mw2", "mw3", "mw4", "handler"}
	if len(callOrder) != len(expectedOrder) {
		t.Fatalf("expected %d calls, got %d", len(expectedOrder), len(callOrder))
	}
	for i, val := range expectedOrder {
		if callOrder[i] != val {
			t.Errorf("expected %s at position %d, got %s", val, i, callOrder[i])
		}
	}
}

func TestChainFullChain(t *testing.T) {
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

	// Create observers
	observer1 := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callOrder = append(callOrder, "observer1")
	})

	observer2 := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callOrder = append(callOrder, "observer2")
	})

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callOrder = append(callOrder, "handler")
		w.WriteHeader(http.StatusOK)
	})
	chain := rtr.NewChain(handler).
		WithMiddleware(mw1, mw2).
		WithObservers(observer1, observer2)

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	chain.Handler().ServeHTTP(rec, req)

	// Verify execution order
	expectedOrder := []string{"mw1", "mw2", "handler", "observer1", "observer2"}
	if len(callOrder) != len(expectedOrder) {
		t.Fatalf("expected %d calls, got %d", len(expectedOrder), len(callOrder))
	}
	for i, val := range expectedOrder {
		if callOrder[i] != val {
			t.Errorf("expected %s at position %d, got %s", val, i, callOrder[i])
		}
	}

	// Verify response status
	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestChainMiddlewareReturnEarly(t *testing.T) {
	var calledHandlers []string

	authMiddleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calledHandlers = append(calledHandlers, "authMiddleware")
			w.WriteHeader(http.StatusUnauthorized)
			// Do not call next.ServeHTTP() to simulate failed auth
		})
	}

	observer1 := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calledHandlers = append(calledHandlers, "observer1")
	})

	observer2 := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calledHandlers = append(calledHandlers, "observer2")
	})

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calledHandlers = append(calledHandlers, "handler")
		w.WriteHeader(http.StatusOK)
	})
	chain := rtr.NewChain(handler).
		WithMiddleware(authMiddleware).
		WithObservers(observer1, observer2)

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	chain.Handler().ServeHTTP(rec, req)

	// Verify status code
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}

	// Verify execution order - observers should still run even though middleware returned early
	expectedHandlers := []string{"authMiddleware", "observer1", "observer2"}
	if len(calledHandlers) != len(expectedHandlers) {
		t.Fatalf("expected %d calls, got %d", len(expectedHandlers), len(calledHandlers))
	}
	for i, val := range expectedHandlers {
		if calledHandlers[i] != val {
			t.Errorf("expected %s at position %d, got %s", val, i, calledHandlers[i])
		}
	}
}
