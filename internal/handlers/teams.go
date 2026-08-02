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

// CreateTeamRequest represents the JSON body for creating a team.
type CreateTeamRequest struct {
	Slug    string `json:"slug"`
	Name    string `json:"name"`
	AltName string `json:"alt_name"`
	ClanTag string `json:"clan_tag"`
}

// UpdateTeamRequest represents the JSON body for updating a team.
// All fields are optional; only non-nil fields are applied.
type UpdateTeamRequest struct {
	Slug    *string `json:"slug,omitempty"`
	Name    *string `json:"name,omitempty"`
	AltName *string `json:"alt_name,omitempty"`
	ClanTag *string `json:"clan_tag,omitempty"`
}

// TeamHandler handles HTTP requests for /teams
type TeamHandler struct {
	repo repository.TeamRepository
}

// NewTeamHandler creates a new TeamHandler with the given repository.
func NewTeamHandler(repo repository.TeamRepository) *TeamHandler {
	return &TeamHandler{repo: repo}
}

// isNilTeamRepo determines whether a TeamRepository is nil or contains a nil db field.
// We can't use isNilRepo from shenanigans.go because it takes a repository.Repository
// (the shenanigan interface), not a TeamRepository.
func isNilTeamRepo(r repository.TeamRepository) bool {
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

// HandleList serves GET /teams — list all teams.
func (h *TeamHandler) HandleList(w http.ResponseWriter, req *http.Request) {
	if isNilTeamRepo(h.repo) {
		writeError(w, http.StatusServiceUnavailable, "database not available")
		return
	}

	ctx := req.Context()

	teams, err := h.repo.GetAll(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to list teams: %v", err))
		return
	}

	if teams == nil {
		teams = []*models.Team{}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{
		"teams": teams,
	})
}

// HandleGet serves GET /teams/{id} — get a single team by ID.
func (h *TeamHandler) HandleGet(w http.ResponseWriter, req *http.Request) {
	if isNilTeamRepo(h.repo) {
		writeError(w, http.StatusServiceUnavailable, "database not available")
		return
	}

	ctx := req.Context()
	id, err := strconv.ParseInt(chi.URLParam(req, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	t, err := h.repo.GetByID(ctx, id)
	if err != nil {
		if err == repository.ErrNotFound {
			writeError(w, http.StatusNotFound, "team not found")
			return
		}
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to get team: %v", err))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{
		"team": t,
	})
}

// HandleCreate serves POST /teams — create a new team.
func (h *TeamHandler) HandleCreate(w http.ResponseWriter, req *http.Request) {
	if isNilTeamRepo(h.repo) {
		writeError(w, http.StatusServiceUnavailable, "database not available")
		return
	}

	ctx := req.Context()

	var body CreateTeamRequest
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	t := &models.Team{
		Slug:    strings.TrimSpace(body.Slug),
		Name:    strings.TrimSpace(body.Name),
		AltName: strings.TrimSpace(body.AltName),
		ClanTag: strings.TrimSpace(body.ClanTag),
	}

	if err := t.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.repo.Create(ctx, t); err != nil {
		if err == repository.ErrDuplicate || strings.Contains(err.Error(), "duplicate") {
			writeError(w, http.StatusConflict, "team already exists")
			return
		}
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to create team: %v", err))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]any{
		"team": t,
	})
}

// HandleUpdate serves PUT /teams/{id} — update an existing team.
func (h *TeamHandler) HandleUpdate(w http.ResponseWriter, req *http.Request) {
	if isNilTeamRepo(h.repo) {
		writeError(w, http.StatusServiceUnavailable, "database not available")
		return
	}

	ctx := req.Context()
	id, err := strconv.ParseInt(chi.URLParam(req, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	var body UpdateTeamRequest
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	updates := make(map[string]any)
	if body.Slug != nil {
		updates["slug"] = strings.TrimSpace(*body.Slug)
	}
	if body.Name != nil {
		updates["name"] = strings.TrimSpace(*body.Name)
	}
	if body.AltName != nil {
		updates["alt_name"] = strings.TrimSpace(*body.AltName)
	}
	if body.ClanTag != nil {
		updates["clan_tag"] = strings.TrimSpace(*body.ClanTag)
	}

	if len(updates) == 0 {
		writeError(w, http.StatusBadRequest, "at least one field required")
		return
	}

	if err := h.repo.Update(ctx, id, updates); err != nil {
		if err == repository.ErrNotFound {
			writeError(w, http.StatusNotFound, "team not found")
			return
		}
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to update team: %v", err))
		return
	}

	t, err := h.repo.GetByID(ctx, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to get updated team: %v", err))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{
		"team": t,
	})
}

// HandleDelete serves DELETE /teams/{id} — delete a team by ID.
func (h *TeamHandler) HandleDelete(w http.ResponseWriter, req *http.Request) {
	if isNilTeamRepo(h.repo) {
		writeError(w, http.StatusServiceUnavailable, "database not available")
		return
	}

	ctx := req.Context()
	id, err := strconv.ParseInt(chi.URLParam(req, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	if err := h.repo.Delete(ctx, id); err != nil {
		if err == repository.ErrNotFound {
			writeError(w, http.StatusNotFound, "team not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to delete team")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// RegisterOpenAPI registers the team routes with the OpenAPI registry.
func (h *TeamHandler) RegisterOpenAPI(reg *openapi.Registry) {
	reg.Register(
		openapi.Route{
			Method:      "GET",
			Path:        "/teams",
			OperationID: "listTeams",
			Description: "List all teams.",
			Responses: []openapi.Response{
				{Code: 200, Body: "json"},
			},
		},
		openapi.Route{
			Method:      "POST",
			Path:        "/teams",
			OperationID: "createTeam",
			Description: "Create a new team.",
			RequestBody: "json",
			Responses: []openapi.Response{
				{Code: 201, Body: "json"},
			},
		},
		openapi.Route{
			Method:      "GET",
			Path:        "/teams/{id}",
			OperationID: "getTeam",
			Description: "Get a team by ID.",
			Responses: []openapi.Response{
				{Code: 200, Body: "json"},
				{Code: 404, Body: "json", Empty: false, EmptyBody: "Not Found"},
			},
		},
		openapi.Route{
			Method:      "PUT",
			Path:        "/teams/{id}",
			OperationID: "updateTeam",
			Description: "Update a team.",
			RequestBody: "json",
			Responses: []openapi.Response{
				{Code: 200, Body: "json"},
				{Code: 404, Body: "json", Empty: false, EmptyBody: "Not Found"},
			},
		},
		openapi.Route{
			Method:      "DELETE",
			Path:        "/teams/{id}",
			OperationID: "deleteTeam",
			Description: "Delete a team.",
			Responses: []openapi.Response{
				{Code: 204, Empty: true, EmptyBody: "No Content"},
			},
		},
	)
}
