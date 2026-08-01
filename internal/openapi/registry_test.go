package openapi_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/forklifts-for-great-justice/judge-service/internal/openapi"
)

func TestNewRegistry(t *testing.T) {
	reg := openapi.NewRegistry()
	if reg == nil {
		t.Fatal("NewRegistry returned nil")
	}
	if len(reg.Routes()) != 0 {
		t.Errorf("expected 0 routes, got %d", len(reg.Routes()))
	}
}

func TestRegister(t *testing.T) {
	reg := openapi.NewRegistry()
	reg.Register(openapi.Route{
		Method:      "GET",
		Path:        "/test",
		OperationID: "test",
		Description: "test",
	})
	if len(reg.Routes()) != 1 {
		t.Errorf("expected 1 route, got %d", len(reg.Routes()))
	}
}

func TestRegisterMultiple(t *testing.T) {
	reg := openapi.NewRegistry()
	reg.Register(
		openapi.Route{Method: "GET", Path: "/a"},
		openapi.Route{Method: "POST", Path: "/b"},
	)
	if len(reg.Routes()) != 2 {
		t.Fatalf("expected 2 routes, got %d", len(reg.Routes()))
	}
	// Verify independence (copy)
	_ = reg.Routes() // no panic
	_ = reg.Routes() // second call also works
}

func TestRegisterSchemaRoute(t *testing.T) {
	route := openapi.RegisterSchemaRoute(openapi.NewRegistry())
	if route.Path != "/openapi.json" {
		t.Errorf("expected path /openapi.json, got %s", route.Path)
	}
	if route.OperationID != "getOpenAPISpec" {
		t.Errorf("expected OperationID getOpenAPISpec, got %s", route.OperationID)
	}
}

func TestSpecBasic(t *testing.T) {
	reg := openapi.NewRegistry()
	reg.Register(
		openapi.Route{
			Method:      "GET",
			Path:        "/health",
			OperationID: "healthCheck",
			Description: "Health check.",
			Responses:   []openapi.Response{{Code: 200, Body: "json"}},
		},
		openapi.Route{
			Method:      "POST",
			Path:        "/health",
			OperationID: "createHealth",
			Description: "Create health.",
			RequestBody: "json",
			Responses:   []openapi.Response{{Code: 201, Body: "json"}, {Code: 400, Body: "json"}},
		},
	)

	spec := reg.Spec()
	info, ok := spec["info"].(map[string]interface{})
	if !ok {
		t.Fatal("missing info in spec")
	}
	if info["title"] != "JudgeService" {
		t.Errorf("expected title JudgeService, got %v", info["title"])
	}
	if spec["openapi"] != "3.0.0" {
		t.Errorf("expected openapi 3.0.0, got %v", spec["openapi"])
	}
}

func TestSpecPathsWithMultipleMethods(t *testing.T) {
	reg := openapi.NewRegistry()
	reg.Register(
		openapi.Route{Method: "GET", Path: "/items/{id}", OperationID: "get", Description: "Get", Responses: []openapi.Response{{Code: 200, Body: "json"}}},
		openapi.Route{Method: "PUT", Path: "/items/{id}", OperationID: "put", Description: "Put", RequestBody: "json", Responses: []openapi.Response{{Code: 200, Body: "json"}}},
		openapi.Route{Method: "DELETE", Path: "/items/{id}", OperationID: "delete", Description: "Delete", Responses: []openapi.Response{{Code: 204, Body: "none", Empty: true, EmptyBody: "No Content"}}},
	)

	spec := reg.Spec()
	paths, ok := spec["paths"].(map[string]interface{})
	if !ok {
		t.Fatal("missing paths")
	}
	items, ok := paths["/items/{id}"].(map[string]interface{})
	if !ok {
		t.Fatal("missing /items/{id}")
	}
	if _, ok := items["GET"]; !ok {
		t.Error("missing GET /items/{id}")
	}
	if _, ok := items["PUT"]; !ok {
		t.Error("missing PUT /items/{id}")
	}
	if _, ok := items["DELETE"]; !ok {
		t.Error("missing DELETE /items/{id}")
	}
}

func TestSpecNoRequestBodyForGet(t *testing.T) {
	reg := openapi.NewRegistry()
	reg.Register(openapi.Route{
		Method:      "GET",
		Path:        "/test",
		OperationID: "getTest",
		Description: "Get test.",
		Responses:   []openapi.Response{{Code: 200, Body: "json"}},
	})
	spec := reg.Spec()
	paths := spec["paths"].(map[string]interface{})
	getOp := paths["/test"].(map[string]interface{})["GET"].(map[string]interface{})
	if _, has := getOp["requestBody"]; has {
		t.Error("GET /test should not have requestBody")
	}
}

func TestSpecRequestBodyForPost(t *testing.T) {
	reg := openapi.NewRegistry()
	reg.Register(openapi.Route{
		Method:      "POST",
		Path:        "/test",
		OperationID: "createTest",
		Description: "Create test.",
		RequestBody: "json",
		Responses:   []openapi.Response{{Code: 201, Body: "json"}},
	})
	spec := reg.Spec()
	paths := spec["paths"].(map[string]interface{})
	postOp := paths["/test"].(map[string]interface{})["POST"].(map[string]interface{})
	if _, has := postOp["requestBody"]; !has {
		t.Error("POST /test should have requestBody")
	}
}

func TestSpecXNotes(t *testing.T) {
	reg := openapi.NewRegistry()
	reg.Register(openapi.Route{
		Method:      "POST",
		Path:        "/activate",
		OperationID: "activate",
		Description: "Activate.",
		RequestBody: "json",
		Responses:   []openapi.Response{{Code: 200, Body: "json"}},
		Notes:       "This is a note.",
	})
	spec := reg.Spec()
	paths := spec["paths"].(map[string]interface{})
	postOp := paths["/activate"].(map[string]interface{})["POST"].(map[string]interface{})
	notes, ok := postOp["x-notes"].(string)
	if !ok {
		t.Fatal("missing x-notes")
	}
	if notes != "This is a note." {
		t.Errorf("expected x-notes 'This is a note.', got %q", notes)
	}
}

func TestSpecSchemas(t *testing.T) {
	reg := openapi.NewRegistry()
	reg.Register(openapi.Route{
		Method:      "GET",
		Path:        "/shenanigans",
		OperationID: "list",
		Description: "List.",
		Responses:   []openapi.Response{{Code: 200, Body: "json"}},
	})
	spec := reg.Spec()
	components := spec["components"].(map[string]interface{})
	schemas := components["schemas"].(map[string]interface{})

	expectedSchemas := []string{"Shananigan", "CreateShananiganRequest", "ActivateShananiganRequest", "ActivateShananiganResponse"}
	for _, name := range expectedSchemas {
		if _, ok := schemas[name]; !ok {
			t.Errorf("missing schema %s", name)
		}
	}
}

func TestSpecSchemasShananigan(t *testing.T) {
	reg := openapi.NewRegistry()
	spec := reg.Spec()
	components := spec["components"].(map[string]interface{})
	schemas := components["schemas"].(map[string]interface{})
	shananigan := schemas["Shananigan"].(map[string]interface{})

	if shananigan["type"] != "object" {
		t.Errorf("expected Shananigan type object, got %v", shananigan["type"])
	}
	props := shananigan["properties"].(map[string]interface{})
	if _, ok := props["id"]; !ok {
		t.Error("Shananigan missing id property")
	}
	if _, ok := props["target_type"]; !ok {
		t.Error("Shananigan missing target_type property")
	}
	targetType := props["target_type"].(map[string]interface{})
	raw := targetType["enum"]
	var enums []string
	switch v := raw.(type) {
	case []string:
		enums = v
	case []interface{}:
		for _, item := range v {
			enums = append(enums, item.(string))
		}
	}
	if len(enums) != 2 {
		t.Errorf("expected 2 target_type enums, got %d", len(enums))
	}
}

func TestSchemaHandler(t *testing.T) {
	reg := openapi.NewRegistry()
	reg.Register(
		openapi.Route{
			Method: "GET", Path: "/test", OperationID: "getTest", Description: "Get test.",
			Responses: []openapi.Response{{Code: 200, Body: "json"}},
		},
	)

	req := httptest.NewRequest("GET", "/openapi.json", nil)
	w := httptest.NewRecorder()
	openapi.SchemaHandler(reg)(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", ct)
	}

	var spec map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&spec); err != nil {
		t.Fatalf("failed to decode spec: %v", err)
	}
	if spec["openapi"] != "3.0.0" {
		t.Error("spec missing openapi field")
	}
}

func TestSchemaHandlerWithMultipleRoutes(t *testing.T) {
	reg := openapi.NewRegistry()
	reg.Register(
		openapi.Route{Method: "GET", Path: "/a", OperationID: "getA", Description: "A.", Responses: []openapi.Response{{Code: 200, Body: "json"}}},
		openapi.Route{Method: "POST", Path: "/a", OperationID: "createA", Description: "Create A.", RequestBody: "json", Responses: []openapi.Response{{Code: 201, Body: "json"}}},
		openapi.Route{Method: "GET", Path: "/b", OperationID: "getB", Description: "B.", Responses: []openapi.Response{{Code: 200, Body: "json"}}},
	)

	req := httptest.NewRequest("GET", "/openapi.json", nil)
	w := httptest.NewRecorder()
	openapi.SchemaHandler(reg)(w, req)

	spec := decodeSpec(t, w)
	paths := spec["paths"].(map[string]interface{})
	if _, ok := paths["/a"]; !ok {
		t.Error("missing /a in paths")
	}
	if _, ok := paths["/b"]; !ok {
		t.Error("missing /b in paths")
	}
}

func TestSchemaHandlerServers(t *testing.T) {
	reg := openapi.NewRegistry()
	spec := reg.Spec()
	servers := spec["servers"].([]interface{})
	if len(servers) == 0 {
		t.Error("expected at least one server")
	}
	first := servers[0].(map[string]interface{})
	if first["url"] != "https://judge.hackfortress.net" {
		t.Errorf("expected server URL judge.hackfortress.net, got %v", first["url"])
	}
}

func TestSchemaHandlerMiddleware(t *testing.T) {
	reg := openapi.NewRegistry()
	reg.Register(
		openapi.Route{Method: "GET", Path: "/test", OperationID: "test", Description: "Test.",
			Responses: []openapi.Response{{Code: 200, Body: "json"}}},
	)

	wrapped := openapi.SchemaHandlerMiddleware(reg, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))

	// Request to /openapi.json should be handled by middleware
	req := httptest.NewRequest("GET", "/openapi.json", nil)
	w := httptest.NewRecorder()
	wrapped.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("middleware /openapi.json: expected 200, got %d", w.Code)
	}

	// Request to /test should pass through to the next handler
	req = httptest.NewRequest("GET", "/test", nil)
	w = httptest.NewRecorder()
	wrapped.ServeHTTP(w, req)
	if w.Code != http.StatusTeapot {
		t.Errorf("middleware /test passthrough: expected 418, got %d", w.Code)
	}
}

func decodeSpec(t *testing.T, w *httptest.ResponseRecorder) map[string]interface{} {
	var spec map[string]interface{}
	if err := json.NewDecoder(w.Result().Body).Decode(&spec); err != nil {
		t.Fatalf("decode spec: %v", err)
	}
	return spec
}
