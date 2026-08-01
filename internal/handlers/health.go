// Package handlers provides the /health endpoint.
package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/forklifts-for-great-justice/judge-service/internal/openapi"
)

// RegisterHealthRoute wires the health endpoint to the chi router.
func RegisterHealthRoute(r chi.Router, reg *openapi.Registry) {
	r.Get("/health", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
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
