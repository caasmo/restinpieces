package prerouter

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/caasmo/restinpieces/config"
	"github.com/caasmo/restinpieces/core"
)

func TestBlockEndpointsMismatchDeactivated(t *testing.T) {
	mockApp := &core.App{}
	cfg := &config.Config{
		EndpointsBlockMismatch: config.EndpointsBlockMismatch{
			Activated: false,
		},
	}
	mockApp.SetConfigProvider(config.NewProvider(cfg))

	middleware := NewBlockEndpointsMismatch(mockApp)
	req := httptest.NewRequest("POST", "/api/auth-with-password", nil)
	req.Header.Set(core.HeaderEndpointsHash, "stale-hash")
	rr := httptest.NewRecorder()
	next := &mockNextHandler{}

	handler := middleware.Execute(next)
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
	if !next.called {
		t.Error("Expected next handler to be called")
	}
}

func TestBlockEndpointsMismatchNoHeader(t *testing.T) {
	endpoints := config.Endpoints{
		ListEndpoints:    "GET /api/list-endpoints",
		AuthWithPassword: "POST /api/auth-with-password",
	}
	mockApp := &core.App{}
	cfg := &config.Config{
		Endpoints: endpoints,
		EndpointsBlockMismatch: config.EndpointsBlockMismatch{
			Activated: true,
		},
	}
	mockApp.SetConfigProvider(config.NewProvider(cfg))

	middleware := NewBlockEndpointsMismatch(mockApp)
	req := httptest.NewRequest("POST", "/api/auth-with-password", nil)
	rr := httptest.NewRecorder()
	next := &mockNextHandler{}

	handler := middleware.Execute(next)
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
	if !next.called {
		t.Error("Expected next handler to be called")
	}
}

func TestBlockEndpointsMismatchMatchingHash(t *testing.T) {
	endpoints := config.Endpoints{
		ListEndpoints:    "GET /api/list-endpoints",
		AuthWithPassword: "POST /api/auth-with-password",
	}
	mockApp := &core.App{}
	cfg := &config.Config{
		Endpoints: endpoints,
		EndpointsBlockMismatch: config.EndpointsBlockMismatch{
			Activated: true,
		},
	}
	mockApp.SetConfigProvider(config.NewProvider(cfg))

	middleware := NewBlockEndpointsMismatch(mockApp)
	req := httptest.NewRequest("POST", "/api/auth-with-password", nil)
	req.Header.Set(core.HeaderEndpointsHash, endpoints.Hash())
	rr := httptest.NewRecorder()
	next := &mockNextHandler{}

	handler := middleware.Execute(next)
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
	if !next.called {
		t.Error("Expected next handler to be called")
	}
}

func TestBlockEndpointsMismatchStaleHash(t *testing.T) {
	endpoints := config.Endpoints{
		ListEndpoints:    "GET /api/list-endpoints",
		AuthWithPassword: "POST /api/auth-with-password",
	}
	mockApp := &core.App{}
	cfg := &config.Config{
		Endpoints: endpoints,
		EndpointsBlockMismatch: config.EndpointsBlockMismatch{
			Activated: true,
		},
	}
	mockApp.SetConfigProvider(config.NewProvider(cfg))

	middleware := NewBlockEndpointsMismatch(mockApp)
	req := httptest.NewRequest("POST", "/api/auth-with-password", nil)
	req.Header.Set(core.HeaderEndpointsHash, "stale-hash-value")
	rr := httptest.NewRecorder()
	next := &mockNextHandler{}

	handler := middleware.Execute(next)
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusConflict {
		t.Errorf("Expected status %d, got %d", http.StatusConflict, rr.Code)
	}
	if next.called {
		t.Error("Expected next handler to NOT be called")
	}
}

func TestBlockEndpointsMismatchListEndpointsExempt(t *testing.T) {
	endpoints := config.Endpoints{
		ListEndpoints:    "GET /api/list-endpoints",
		AuthWithPassword: "POST /api/auth-with-password",
	}
	mockApp := &core.App{}
	cfg := &config.Config{
		Endpoints: endpoints,
		EndpointsBlockMismatch: config.EndpointsBlockMismatch{
			Activated: true,
		},
	}
	mockApp.SetConfigProvider(config.NewProvider(cfg))

	middleware := NewBlockEndpointsMismatch(mockApp)
	req := httptest.NewRequest("GET", "/api/list-endpoints", nil)
	req.Header.Set(core.HeaderEndpointsHash, "stale-hash-value")
	rr := httptest.NewRecorder()
	next := &mockNextHandler{}

	handler := middleware.Execute(next)
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
	if !next.called {
		t.Error("Expected next handler to be called")
	}
}
