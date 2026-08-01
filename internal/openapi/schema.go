package openapi

import "net/http"

// SchemaHandlerMiddleware wraps an existing handler and prepends the OpenAPI
// spec route at GET /openapi.json. Call this from main() to wire up the
// dynamic /openapi.json endpoint.
func SchemaHandlerMiddleware(r *Registry, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path == "/openapi.json" {
			SchemaHandler(r)(w, req)
			return
		}
		next.ServeHTTP(w, req)
	})
}

// RegisterSchemaRoute adds the /openapi.json route to the router and the
// registry so it appears in the spec.
func RegisterSchemaRoute(reg *Registry) Route {
	return Route{
		Method:      "GET",
		Path:        "/openapi.json",
		OperationID: "getOpenAPISpec",
		Description: "Serves a machine-readable OpenAPI 3.0.0 specification for the API in JSON format.",
		Responses: []Response{
			{Code: 200, Body: "json", Empty: false, EmptyBody: ""},
		},
	}
}
