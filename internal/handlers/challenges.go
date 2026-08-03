// Package handlers provides HTTP request handlers for the Challenge resource.
package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/forklifts-for-great-justice/judge-service/internal/models"
	"github.com/forklifts-for-great-justice/judge-service/internal/openapi"
	"github.com/forklifts-for-great-justice/judge-service/internal/repository"
)

// CreateChallengeRequest represents the JSON body for creating a challenge.
type CreateChallengeRequest struct {
	Name          string  `json:"name"`
	Description   string  `json:"description"`
	ChallengeType *string `json:"challenge_type,omitempty"`
	Location      *string `json:"location,omitempty"`
	Points        int     `json:"points"`
	Disabled      *bool   `json:"disabled,omitempty"`
	Flag          string  `json:"flag"`
}

// UpdateChallengeRequest represents the JSON body for updating a challenge.
// All fields are optional; only non-nil fields are applied.
type UpdateChallengeRequest struct {
	Name          *string `json:"name,omitempty"`
	Description   *string `json:"description,omitempty"`
	ChallengeType *string `json:"challenge_type,omitempty"`
	Location      *string `json:"location,omitempty"`
	Points        *int    `json:"points,omitempty"`
	Disabled      *bool   `json:"disabled,omitempty"`
	Flag          *string `json:"flag,omitempty"`
}

// ChallengeHandler handles HTTP requests for /challenges
type ChallengeHandler struct {
	repo repository.ChallengeRepository
}

// NewChallengeHandler creates a new ChallengeHandler with the given repository.
func NewChallengeHandler(repo repository.ChallengeRepository) *ChallengeHandler {
	return &ChallengeHandler{repo: repo}
}

// isNilChallengeRepo determines whether a ChallengeRepository is nil or contains a nil db field.
func isNilChallengeRepo(r repository.ChallengeRepository) bool {
	if r == nil {
		return true
	}
	v := reflect.ValueOf(r)
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return true
		}
		if elem := v.Elem(); elem.Kind() == reflect.Struct {
			dbField := elem.FieldByName("db")
			if dbField.IsValid() && dbField.Kind() == reflect.Ptr && dbField.IsNil() {
				return true
			}
		}
	}
	return false
}

// HandleList serves GET /challenges — list all challenges.
func (h *ChallengeHandler) HandleList(w http.ResponseWriter, req *http.Request) {
	if isNilChallengeRepo(h.repo) {
		writeError(w, http.StatusServiceUnavailable, "database not available")
		return
	}

	ctx := req.Context()

	challenges, err := h.repo.GetAll(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to list challenges: %v", err))
		return
	}

	if challenges == nil {
		challenges = []*models.Challenge{}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{
		"challenges": challenges,
	})
}

// HandleGet serves GET /challenges/{id} — get a single challenge by ID.
func (h *ChallengeHandler) HandleGet(w http.ResponseWriter, req *http.Request) {
	if isNilChallengeRepo(h.repo) {
		writeError(w, http.StatusServiceUnavailable, "database not available")
		return
	}

	ctx := req.Context()
	id, err := strconv.ParseInt(chi.URLParam(req, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	c, err := h.repo.GetByID(ctx, id)
	if err != nil {
		if err == repository.ErrNotFound {
			writeError(w, http.StatusNotFound, "challenge not found")
			return
		}
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to get challenge: %v", err))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{
		"challenge": c,
	})
}

// HandleCreate serves POST /challenges — create a new challenge.
func (h *ChallengeHandler) HandleCreate(w http.ResponseWriter, req *http.Request) {
	if isNilChallengeRepo(h.repo) {
		writeError(w, http.StatusServiceUnavailable, "database not available")
		return
	}

	ctx := req.Context()

	var body CreateChallengeRequest
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	t := &models.Challenge{}
	if body.Disabled != nil {
		t.Disabled = *body.Disabled
	}
	t.ChallengeType = body.ChallengeType
	t.Location = body.Location
	t.Name = strings.TrimSpace(body.Name)
	t.Description = strings.TrimSpace(body.Description)
	t.Points = body.Points
	t.Flag = strings.TrimSpace(body.Flag)

	if err := t.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.repo.Create(ctx, t); err != nil {
		if repository.IsDuplicatePostgres(err) {
			writeError(w, http.StatusConflict, "challenge already exists")
			return
		}
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to create challenge: %v", err))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]any{
		"challenge": t,
	})
}

// HandleUpdate serves PUT /challenges/{id} — update an existing challenge.
func (h *ChallengeHandler) HandleUpdate(w http.ResponseWriter, req *http.Request) {
	if isNilChallengeRepo(h.repo) {
		writeError(w, http.StatusServiceUnavailable, "database not available")
		return
	}

	ctx := req.Context()
	id, err := strconv.ParseInt(chi.URLParam(req, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	var body UpdateChallengeRequest
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	updates := make(map[string]any)
	if body.Name != nil {
		updates["name"] = strings.TrimSpace(*body.Name)
	}
	if body.Description != nil {
		updates["description"] = strings.TrimSpace(*body.Description)
	}
	if body.ChallengeType != nil {
		updates["challenge_type"] = strings.TrimSpace(*body.ChallengeType)
	}
	if body.Location != nil {
		updates["location"] = strings.TrimSpace(*body.Location)
	}
	if body.Points != nil {
		updates["points"] = *body.Points
	}
	if body.Disabled != nil {
		updates["disabled"] = *body.Disabled
	}
	if body.Flag != nil {
		updates["flag"] = strings.TrimSpace(*body.Flag)
	}

	if len(updates) == 0 {
		writeError(w, http.StatusBadRequest, "at least one field required")
		return
	}

	if err := h.repo.Update(ctx, id, updates); err != nil {
		if err == repository.ErrNotFound {
			writeError(w, http.StatusNotFound, "challenge not found")
			return
		}
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to update challenge: %v", err))
		return
	}

	c, err := h.repo.GetByID(ctx, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to get updated challenge: %v", err))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{
		"challenge": c,
	})
}

// HandleDelete serves DELETE /challenges/{id} — delete a challenge by ID.
func (h *ChallengeHandler) HandleDelete(w http.ResponseWriter, req *http.Request) {
	if isNilChallengeRepo(h.repo) {
		writeError(w, http.StatusServiceUnavailable, "database not available")
		return
	}

	ctx := req.Context()
	id, err := strconv.ParseInt(chi.URLParam(req, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	if err := h.repo.SoftDelete(ctx, id); err != nil {
		if err.Error() == "challenge not found" {
			writeError(w, http.StatusNotFound, "challenge not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to delete challenge")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// RegisterOpenAPI registers the challenge routes with the OpenAPI registry.
func (h *ChallengeHandler) RegisterOpenAPI(reg *openapi.Registry) {
	reg.Register(
		openapi.Route{
			Method:      "GET",
			Path:        "/challenges",
			OperationID: "listChallenges",
			Description: "List all challenges.",
			Responses: []openapi.Response{
				{Code: 200, Body: "json"},
			},
		},
		openapi.Route{
			Method:      "POST",
			Path:        "/challenges",
			OperationID: "createChallenge",
			Description: "Create a new challenge.",
			RequestBody: "json",
			Responses: []openapi.Response{
				{Code: 201, Body: "json"},
			},
		},
		openapi.Route{
			Method:      "GET",
			Path:        "/challenges/{id}",
			OperationID: "getChallenge",
			Description: "Get a challenge by ID.",
			Responses: []openapi.Response{
				{Code: 200, Body: "json"},
				{Code: 404, Body: "json", Empty: false, EmptyBody: "Not Found"},
			},
		},
		openapi.Route{
			Method:      "PUT",
			Path:        "/challenges/{id}",
			OperationID: "updateChallenge",
			Description: "Update a challenge.",
			RequestBody: "json",
			Responses: []openapi.Response{
				{Code: 200, Body: "json"},
				{Code: 404, Body: "json", Empty: false, EmptyBody: "Not Found"},
			},
		},
		openapi.Route{
			Method:      "DELETE",
			Path:        "/challenges/{id}",
			OperationID: "deleteChallenge",
			Description: "Delete a challenge.",
			Responses: []openapi.Response{
				{Code: 204, Empty: true, EmptyBody: "No Content"},
			},
		},
	)
}
