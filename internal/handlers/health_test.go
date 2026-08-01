package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/forklifts-for-great-justice/judge-service/internal/handlers"
	"github.com/forklifts-for-great-justice/judge-service/internal/openapi"
)

func TestHealth_StatusCode(t *testing.T) {
	router := chi.NewRouter()
	reg := openapi.NewRegistry()
	handlers.RegisterHealthRoute(router, reg)

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestHealth_Body(t *testing.T) {
	router := chi.NewRouter()
	reg := openapi.NewRegistry()
	handlers.RegisterHealthRoute(router, reg)

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var body map[string]string
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf("expected status ok, got %v", body["status"])
	}
}

func TestHealth_ContentType(t *testing.T) {
	router := chi.NewRouter()
	reg := openapi.NewRegistry()
	handlers.RegisterHealthRoute(router, reg)

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", ct)
	}
}

func TestRegisterHealthOpenAPI(t *testing.T) {
	reg := openapi.NewRegistry()
	handlers.RegisterHealthOpenAPI(reg)

	routes := reg.Routes()
	if len(routes) != 1 {
		t.Fatalf("expected 1 route, got %d", len(routes))
	}

	route := routes[0]
	if route.OperationID != "healthCheck" {
		t.Errorf("expected OperationID healthCheck, got %s", route.OperationID)
	}
	if route.Path != "/health" {
		t.Errorf("expected path /health, got %s", route.Path)
	}
	if route.Method != "GET" {
		t.Errorf("expected method GET, got %s", route.Method)
	}
}
