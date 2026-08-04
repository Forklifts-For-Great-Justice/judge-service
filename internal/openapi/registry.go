// Package openapi provides a dynamic OpenAPI 3.0.0 spec registry used to serve
// the API specification at GET /openapi.json.
//
// Each handler registers its routes via Route structs. The handler's openapi
// registration function is called from main() so the spec is built from the
// same source of truth as the router configuration.
package openapi

import (
	"encoding/json"
	"net/http"
	"strconv"
)

// Route describes a single API endpoint and its OpenAPI metadata.
type Route struct {
	Method      string
	Path        string
	OperationID string
	Description string
	RequestBody string // "none", "json", "form", etc.
	Responses   []Response
	Notes       string // implementation notes or caveats
}

// Response describes a single HTTP response code.
type Response struct {
	Code      int
	Body      string // "none", "json", "binary", etc.
	Empty     bool   // true for 204 and similar responses
	EmptyBody string // "No Content"
}

// Registry collects all Route descriptions for the API.
type Registry struct {
	routes []Route
}

// NewRegistry creates an empty route registry ready for handler registration.
func NewRegistry() *Registry {
	return &Registry{routes: make([]Route, 0)}
}

// Register adds a Route to the registry.
func (r *Registry) Register(rts ...Route) {
	r.routes = append(r.routes, rts...)
}

// Routes returns a copy of all registered routes.
func (r *Registry) Routes() []Route {
	out := make([]Route, len(r.routes))
	copy(out, r.routes)
	return out
}

// Handle returns an http.HandlerFunc that serves the OpenAPI 3.0.0 spec as
// JSON at GET /openapi.json. It calls Spec() on the registry and marshals
// the result on each request so the spec always reflects the latest routes.
func SchemaHandler(reg *Registry) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		spec := reg.Spec()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(spec)
	}
}

// Spec builds an OpenAPI 3.0.0 specification map from all registered routes.
func (r *Registry) Spec() map[string]interface{} {
	openAPI := map[string]interface{}{
		"openapi": "3.0.0",
		"info": map[string]interface{}{
			"title":       "JudgeService",
			"description": "HackFortress judges web API: shenanigans, challenges, teams, and rounds.",
			"version":     "0.1.0",
		},
		"servers": []interface{}{
			map[string]interface{}{
				"url": "https://judge.hackfortress.net",
			},
		},
		"paths": make(map[string]interface{}),
		"components": map[string]interface{}{
			"schemas": buildSchemas(r.routes),
		},
	}

	for _, rt := range r.routes {
		pathMap, ok := openAPI["paths"].(map[string]interface{})
		if !ok {
			pathMap = make(map[string]interface{})
			openAPI["paths"] = pathMap
		}
		pathItems, ok := pathMap[rt.Path].(map[string]interface{})
		if !ok {
			pathItems = make(map[string]interface{})
			pathMap[rt.Path] = pathItems
		}
		pathItems[rt.Method] = buildOp(rt)
	}

	return openAPI
}

func buildOp(rt Route) map[string]interface{} {
	op := map[string]interface{}{
		"operationId": rt.OperationID,
		"description": rt.Description,
		"responses":   buildResponses(rt.Responses),
	}

	if rt.RequestBody == "json" {
		op["requestBody"] = map[string]interface{}{
			"content": map[string]interface{}{
				"application/json": map[string]interface{}{
					"schema": map[string]interface{}{
						"type": "object",
					},
				},
			},
			"required": true,
		}
	}

	if rt.Notes != "" {
		op["x-notes"] = rt.Notes
	}

	return op
}

func buildResponses(resps []Response) map[string]interface{} {
	out := make(map[string]interface{})
	for _, resp := range resps {
		entry := map[string]interface{}{"description": http.StatusText(resp.Code)}
		if resp.Code == 204 || resp.Empty {
			entry["description"] = resp.EmptyBody
		}
		out[strconv.Itoa(resp.Code)] = entry
	}
	return out
}

func buildSchemas(routes []Route) map[string]interface{} {
	schemas := map[string]interface{}{
		"Shananigan": map[string]interface{}{
			"type":        "object",
			"description": "A shenanigan catalogue entry representing an activatable event.",
			"properties": map[string]interface{}{
				"id":           map[string]interface{}{"type": "integer", "format": "int64"},
				"name":         map[string]interface{}{"type": "string"},
				"description":  map[string]interface{}{"type": "string"},
				"rcon_payload": map[string]interface{}{"type": "string"},
				"target_type": map[string]interface{}{
					"type": "string",
					"enum": []string{"team", "all"},
				},
				"cost": map[string]interface{}{"type": "integer", "format": "int64"},
				"metadata": map[string]interface{}{
					"type":        "object",
					"description": "Arbitrary JSON metadata attached to this shenanigan.",
				},
				"created_at": map[string]interface{}{"type": "string", "format": "date-time"},
				"updated_at": map[string]interface{}{"type": "string", "format": "date-time"},
			},
			"required": []interface{}{"id", "name", "description", "rcon_payload", "target_type", "created_at"},
		},
		"CreateShananiganRequest": map[string]interface{}{
			"type":        "object",
			"description": "Payload for creating a new shenanigan catalogue entry.",
			"properties": map[string]interface{}{
				"name":         map[string]interface{}{"type": "string"},
				"description":  map[string]interface{}{"type": "string"},
				"rcon_payload": map[string]interface{}{"type": "string"},
				"target_type": map[string]interface{}{
					"type": "string",
					"enum": []string{"team", "all"},
				},
				"cost": map[string]interface{}{"type": "integer", "format": "int64"},
				"metadata": map[string]interface{}{
					"type": "object",
				},
			},
			"required": []interface{}{"name", "description", "rcon_payload", "target_type"},
		},
		"ActivateShananiganRequest": map[string]interface{}{
			"type":        "object",
			"description": "Payload for activating (triggering) a shenanigan.",
			"properties": map[string]interface{}{
				"team":     map[string]interface{}{"type": "string"},
				"metadata": map[string]interface{}{"type": "object"},
			},
			"required": []interface{}{"team"},
		},
		"ActivateShananiganResponse": map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"purchase_id": map[string]interface{}{"type": "string", "format": "uuid"},
				"status":      map[string]interface{}{"type": "string", "enum": []string{"ok"}},
				"published":   map[string]interface{}{"type": "boolean"},
			},
		},
	}

	// Add a list envelope that wraps each shenanigan.
	schemas["ShananiganListEnvelope"] = map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"shenanigans": map[string]interface{}{
				"type":  "array",
				"items": map[string]interface{}{"$ref": "#/components/schemas/Shananigan"},
			},
		},
	}
	schemas["ActivateShananiganEnvelope"] = map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"shenanigans": map[string]interface{}{
				"type": "array",
			},
		},
	}

	// Team schemas
	schemas["Team"] = map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"id":       map[string]interface{}{"type": "integer", "format": "int64"},
			"slug":     map[string]interface{}{"type": "string", "pattern": "^[a-z0-9-]+$", "minLength": 2, "maxLength": 64},
			"name":     map[string]interface{}{"type": "string"},
			"alt_name": map[string]interface{}{"type": "string"},
			"clan_tag": map[string]interface{}{"type": "string"},
			"created_at": map[string]interface{}{"type": "string", "format": "date-time"},
			"updated_at": map[string]interface{}{"type": "string", "format": "date-time"},
		},
		"required": []interface{}{"id", "slug", "name", "alt_name", "clan_tag", "created_at", "updated_at"},
	}
	schemas["TeamListEnvelope"] = map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"teams": map[string]interface{}{
				"type":  "array",
				"items": map[string]interface{}{"$ref": "#/components/schemas/Team"},
			},
		},
	}
	schemas["CreateTeamRequest"] = map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"slug":     map[string]interface{}{"type": "string"},
			"name":     map[string]interface{}{"type": "string"},
			"alt_name": map[string]interface{}{"type": "string"},
			"clan_tag": map[string]interface{}{"type": "string"},
		},
		"required": []interface{}{"slug", "name", "alt_name", "clan_tag"},
	}
	schemas["UpdateTeamRequest"] = map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"slug":     map[string]interface{}{"type": "string"},
			"name":     map[string]interface{}{"type": "string"},
			"alt_name": map[string]interface{}{"type": "string"},
			"clan_tag": map[string]interface{}{"type": "string"},
		},
	}
	schemas["ErrorResponse"] = map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"error": map[string]interface{}{"type": "string"},
		},
		"required": []interface{}{"error"},
	}

	// Challenge schemas
	schemas["Challenge"] = map[string]interface{}{
		"type":        "object",
		"description": "A challenge (puzzle) definition in the competition.",
		"properties": map[string]interface{}{
			"id":             map[string]interface{}{"type": "integer", "format": "int64"},
			"name":           map[string]interface{}{"type": "string"},
			"description":    map[string]interface{}{"type": "string"},
			"challenge_type": map[string]interface{}{"type": "string"},
			"location":       map[string]interface{}{"type": "string"},
			"points":         map[string]interface{}{"type": "integer"},
			"disabled":       map[string]interface{}{"type": "boolean"},
			"flag":           map[string]interface{}{"type": "string"},
			"created_at":     map[string]interface{}{"type": "string", "format": "date-time"},
			"updated_at":     map[string]interface{}{"type": "string", "format": "date-time"},
		},
		"required": []interface{}{"id", "name", "description", "points", "flag", "created_at", "updated_at"},
	}

	schemas["ChallengeListEnvelope"] = map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"challenges": map[string]interface{}{
				"type":  "array",
				"items": map[string]interface{}{"$ref": "#/components/schemas/Challenge"},
			},
		},
	}

	schemas["CreateChallengeRequest"] = map[string]interface{}{
		"type":        "object",
		"description": "Payload for creating a new challenge.",
		"properties": map[string]interface{}{
			"name":             map[string]interface{}{"type": "string"},
			"description":      map[string]interface{}{"type": "string"},
			"challenge_type":   map[string]interface{}{"type": "string"},
			"location":         map[string]interface{}{"type": "string"},
			"points":           map[string]interface{}{"type": "integer"},
			"disabled":         map[string]interface{}{"type": "boolean"},
			"flag":             map[string]interface{}{"type": "string"},
		},
		"required": []interface{}{"name", "description", "points", "flag"},
	}

	schemas["UpdateChallengeRequest"] = map[string]interface{}{
		"type":        "object",
		"description": "Payload for updating a challenge. Any field is optional.",
		"properties": map[string]interface{}{
			"name":             map[string]interface{}{"type": "string"},
			"description":      map[string]interface{}{"type": "string"},
			"challenge_type":   map[string]interface{}{"type": "string"},
			"location":         map[string]interface{}{"type": "string"},
			"points":           map[string]interface{}{"type": "integer"},
			"disabled":         map[string]interface{}{"type": "boolean"},
			"flag":             map[string]interface{}{"type": "string"},
		},
	}

	// Round schemas
	schemas["Round"] = map[string]interface{}{
		"type":        "object",
		"description": "A round (match) in the competition.",
		"properties": map[string]interface{}{
			"id":                 map[string]interface{}{"type": "integer", "format": "int64"},
			"team_a_id":          map[string]interface{}{"type": "integer", "format": "int64"},
			"team_b_id":          map[string]interface{}{"type": "integer", "format": "int64"},
			"round_name":         map[string]interface{}{"type": "string"},
			"team_a_points":      map[string]interface{}{"type": "integer"},
			"team_b_points":      map[string]interface{}{"type": "integer"},
			"team_a_hack_points": map[string]interface{}{"type": "integer"},
			"team_b_hack_points": map[string]interface{}{"type": "integer"},
			"team_a_hackcoins":   map[string]interface{}{"type": "integer"},
			"team_b_hackcoins":   map[string]interface{}{"type": "integer"},
			"status":             map[string]interface{}{"type": "string", "enum": []interface{}{"scheduled", "in_progress", "completed", "cancelled"}},
			"ready":              map[string]interface{}{"type": "boolean"},
			"live":               map[string]interface{}{"type": "boolean"},
			"disabled":           map[string]interface{}{"type": "boolean"},
			"live_at":            map[string]interface{}{"type": "string", "format": "date-time", "nullable": true},
			"ready_at":           map[string]interface{}{"type": "string", "format": "date-time", "nullable": true},
			"created_at":         map[string]interface{}{"type": "string", "format": "date-time"},
			"updated_at":         map[string]interface{}{"type": "string", "format": "date-time"},
		},
		"required": []interface{}{"id", "round_name", "team_a_id", "team_b_id", "status", "created_at"},
	}

	schemas["RoundListEnvelope"] = map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"rounds": map[string]interface{}{
				"type":  "array",
				"items": map[string]interface{}{"$ref": "#/components/schemas/Round"},
			},
			"game_state":       map[string]interface{}{"type": "object"},
			"automation_state": map[string]interface{}{"type": "object"},
		},
	}

	schemas["RoundCreateRequest"] = map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"round_name": map[string]interface{}{"type": "string"},
			"team_a_id":  map[string]interface{}{"type": "integer", "format": "int64"},
			"team_b_id":  map[string]interface{}{"type": "integer", "format": "int64"},
		},
		"required": []interface{}{"round_name", "team_a_id", "team_b_id"},
	}

	schemas["RoundUpdateRequest"] = map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"round_name": map[string]interface{}{"type": "string"},
			"team_a_id":  map[string]interface{}{"type": "integer", "format": "int64"},
			"team_b_id":  map[string]interface{}{"type": "integer", "format": "int64"},
		},
	}

	schemas["ReadyToggleResponse"] = map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"ready":  map[string]interface{}{"type": "boolean"},
			"status": map[string]interface{}{"type": "string"},
		},
	}

	schemas["LiveToggleResponse"] = map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"live":   map[string]interface{}{"type": "boolean"},
			"status": map[string]interface{}{"type": "string"},
		},
	}

	return schemas
}
