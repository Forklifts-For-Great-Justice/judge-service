// Package handlers provides the /health endpoint.
package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/forklifts-for-great-justice/judge-service/internal/openapi"
)

// RegisterHealthRoute wires the health and root endpoints to the chi router.
func RegisterHealthRoute(r chi.Router, reg *openapi.Registry) {
	r.Get("/health", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	r.Get("/", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"service": "judge-service",
			"status":  "ok",
			"m2m_connection": map[string]string{
				"auth_server":    "https://auth.hackfortress.net",
				"token_endpoint": "https://auth.hackfortress.net/api/oidc/token",
				"grant_type":     "client_credentials",
				"client_id":       "judge-client",
				"scope":          "profile groups",
				"usage_example":  "curl -X POST https://auth.hackfortress.net/api/oidc/token -d 'grant_type=client_credentials&client_id=judge-client&client_secret=<SECRET>&scope=profile groups'",
			},
			"openapi": "https://judge.hackfortress.net/openapi.json",
		})
	})
}

// RegisterHealthOpenAPI adds the health check route to the OpenAPI registry.
func RegisterHealthOpenAPI(reg *openapi.Registry) {
	reg.Register(
		openapi.Route{
			Method:      "GET",
			Path:        "/health",
			OperationID: "healthCheck",
			Description: "Health check. Returns {\"status\":\"ok\"}.",
			Responses: []openapi.Response{
				{Code: 200, Body: "json"},
			},
		},
	)
}
