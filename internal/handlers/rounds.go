package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	chi "github.com/go-chi/chi/v5"
	"github.com/forklifts-for-great-justice/judge-service/internal/models"
	"github.com/forklifts-for-great-justice/judge-service/internal/openapi"
	"github.com/forklifts-for-great-justice/judge-service/internal/repository"
)

type RoundHandler struct {
	repo repository.RoundRepository
}

func NewRoundHandler(repo repository.RoundRepository) *RoundHandler {
	return &RoundHandler{repo: repo}
}

// isNilRoundRepo checks if the RoundRepository interface is nil or empty.
func isNilRoundRepo(r repository.RoundRepository) bool {
	if r == nil {
		return true
	}
	return r == (*repository.RoundRepo)(nil)
}

type RoundCreateRequest struct {
	RoundName string `json:"round_name"`
	TeamAID   int64  `json:"team_a_id"`
	TeamBID   int64  `json:"team_b_id"`
}

type RoundUpdateRequest struct {
	RoundName *string `json:"round_name,omitempty"`
	TeamAID   *int64  `json:"team_a_id,omitempty"`
	TeamBID   *int64  `json:"team_b_id,omitempty"`
}

// HandleList — GET /rounds
func (h *RoundHandler) HandleList(w http.ResponseWriter, req *http.Request) {
	if isNilRoundRepo(h.repo) {
		writeError(w, http.StatusServiceUnavailable, "database not available")
		return
	}
	rounds, err := h.repo.GetAll(req.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to list rounds: %v", err))
		return
	}
	if rounds == nil {
		rounds = []*models.Round{}
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{
		"rounds":             rounds,
		"game_state":         map[string]any{"live_round_id": nil, "current_match": map[string]any{}},
		"automation_state":   map[string]any{},
	})
}

// HandleCreate — POST /rounds
func (h *RoundHandler) HandleCreate(w http.ResponseWriter, req *http.Request) {
	var body RoundCreateRequest
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	r := &models.Round{RoundName: body.RoundName, TeamAID: body.TeamAID, TeamBID: body.TeamBID}
	if err := r.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.repo.Create(req.Context(), r); err != nil {
		if err == repository.ErrDuplicate {
			writeError(w, http.StatusConflict, "round already exists")
			return
		}
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to create round: %v", err))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]any{"round": r})
}

// HandleGet — GET /rounds/:id
func (h *RoundHandler) HandleGet(w http.ResponseWriter, req *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(req, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	r, err := h.repo.GetByID(req.Context(), id)
	if err != nil {
		if err == repository.ErrNotFound {
			writeError(w, http.StatusNotFound, "round not found")
			return
		}
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to get round: %v", err))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{"round": r})
}

// HandleUpdate — PUT /rounds/:id
func (h *RoundHandler) HandleUpdate(w http.ResponseWriter, req *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(req, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var body RoundUpdateRequest
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	updates := make(map[string]any)
	if body.RoundName != nil {
		updates["round_name"] = strings.TrimSpace(*body.RoundName)
	}
	if body.TeamAID != nil {
		updates["team_a_id"] = *body.TeamAID
	}
	if body.TeamBID != nil {
		updates["team_b_id"] = *body.TeamBID
	}
	if len(updates) == 0 {
		writeError(w, http.StatusBadRequest, "at least one field required")
		return
	}
	if err := h.repo.Update(req.Context(), id, updates); err != nil {
		if err == repository.ErrNotFound {
			writeError(w, http.StatusNotFound, "round not found")
			return
		}
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to update round: %v", err))
		return
	}
	r, err := h.repo.GetByID(req.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to get updated round: %v", err))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{"round": r})
}

// HandleDelete — DELETE /rounds/:id (soft-delete, returns 200 with body)
func (h *RoundHandler) HandleDelete(w http.ResponseWriter, req *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(req, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := h.repo.Disable(req.Context(), id); err != nil {
		if err == repository.ErrNotFound {
			writeError(w, http.StatusNotFound, "round not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to delete round")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{"round": map[string]any{"disabled": true}})
}

// HandleToggleReady — POST /rounds/:id/ready
func (h *RoundHandler) HandleToggleReady(w http.ResponseWriter, req *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(req, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := h.repo.ToggleReady(req.Context(), id); err != nil {
		if err == repository.ErrNotFound {
			writeError(w, http.StatusNotFound, "round not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to toggle ready")
		return
	}
	r, _ := h.repo.GetByID(req.Context(), id)
	w.Header().Set("Content-Type", "application/json")
	if r != nil {
		json.NewEncoder(w).Encode(map[string]any{
			"ready":  r.Ready,
			"status": r.Status,
		})
	} else {
		writeError(w, http.StatusInternalServerError, "failed to get updated round")
	}
}

// HandleToggleLive — POST /rounds/:id/live
func (h *RoundHandler) HandleToggleLive(w http.ResponseWriter, req *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(req, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	prevStatus, err := h.repo.SetLive(req.Context(), id)
	if err != nil {
		if err == repository.ErrNotFound {
			writeError(w, http.StatusNotFound, "round not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to set live")
		return
	}
	r, _ := h.repo.GetByID(req.Context(), id)
	w.Header().Set("Content-Type", "application/json")
	if r != nil {
		json.NewEncoder(w).Encode(map[string]any{
			"live":   r.Live,
			"status": r.Status,
		})
	} else {
		writeError(w, http.StatusInternalServerError, "failed to get updated round")
	}
	_ = prevStatus
}

// HandleGetCurrentTeams — GET /rounds/current/teams & GET /rounds/current
func (h *RoundHandler) HandleGetCurrentTeams(w http.ResponseWriter, req *http.Request) {
	if isNilRoundRepo(h.repo) {
		writeError(w, http.StatusServiceUnavailable, "database not available")
		return
	}

	ct, err := h.repo.GetCurrentTeams(req.Context())
	if err != nil {
		if err == repository.ErrNotFound {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{})
			return
		}
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to get current teams: %v", err))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(ct)
}

// HandleSetCurrentTeams — POST /rounds/current/teams & POST /rounds/current
func (h *RoundHandler) HandleSetCurrentTeams(w http.ResponseWriter, req *http.Request) {
	if isNilRoundRepo(h.repo) {
		writeError(w, http.StatusServiceUnavailable, "database not available")
		return
	}

	var body models.SetCurrentTeamsRequest
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	teamAID := body.GetTeamAID()
	teamBID := body.GetTeamBID()

	if teamAID == 0 || teamBID == 0 {
		writeError(w, http.StatusBadRequest, "team_a_id and team_b_id are required")
		return
	}
	if teamAID == teamBID {
		writeError(w, http.StatusBadRequest, "team_a_id and team_b_id must be different")
		return
	}

	ct, err := h.repo.SetCurrentTeams(req.Context(), teamAID, teamBID)
	if err != nil {
		if strings.Contains(err.Error(), "does not exist") {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to set current teams: %v", err))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(ct)
}

// RegisterOpenAPI registers all round routes with the OpenAPI registry.
func (h *RoundHandler) RegisterOpenAPI(reg *openapi.Registry) {
	reg.Register(
		openapi.Route{Method: "GET", Path: "/rounds", OperationID: "listRounds", Description: "List all rounds.", Responses: []openapi.Response{{Code: 200, Body: "json"}, {Code: 503, Body: "json"}}},
		openapi.Route{Method: "POST", Path: "/rounds", OperationID: "createRound", Description: "Create a new round.", RequestBody: "json", Responses: []openapi.Response{{Code: 201, Body: "json"}, {Code: 400, Body: "json"}, {Code: 503, Body: "json"}}},
		openapi.Route{Method: "GET", Path: "/rounds/current/teams", OperationID: "getCurrentTeams", Description: "Get active teams in the current round.", Responses: []openapi.Response{{Code: 200, Body: "json"}, {Code: 503, Body: "json"}}},
		openapi.Route{Method: "POST", Path: "/rounds/current/teams", OperationID: "setCurrentTeams", Description: "Set active teams in the current round.", RequestBody: "json", Responses: []openapi.Response{{Code: 200, Body: "json"}, {Code: 400, Body: "json"}, {Code: 503, Body: "json"}}},
		openapi.Route{Method: "GET", Path: "/rounds/{id}", OperationID: "getRound", Description: "Get a round by ID.", Responses: []openapi.Response{{Code: 200, Body: "json"}, {Code: 404, Body: "json"}, {Code: 503, Body: "json"}}},
		openapi.Route{Method: "PUT", Path: "/rounds/{id}", OperationID: "updateRound", Description: "Update a round.", RequestBody: "json", Responses: []openapi.Response{{Code: 200, Body: "json"}, {Code: 400, Body: "json"}, {Code: 404, Body: "json"}, {Code: 503, Body: "json"}}},
		openapi.Route{Method: "DELETE", Path: "/rounds/{id}", OperationID: "deleteRound", Description: "Soft-delete a round.", Responses: []openapi.Response{{Code: 200, Body: "json"}, {Code: 404, Body: "json"}, {Code: 503, Body: "json"}}},
		openapi.Route{Method: "POST", Path: "/rounds/{id}/ready", OperationID: "toggleRoundReady", Description: "Toggle ready state.", Responses: []openapi.Response{{Code: 200, Body: "json"}, {Code: 404, Body: "json"}, {Code: 503, Body: "json"}}},
		openapi.Route{Method: "POST", Path: "/rounds/{id}/live", OperationID: "toggleRoundLive", Description: "Toggle live state.", Responses: []openapi.Response{{Code: 200, Body: "json"}, {Code: 404, Body: "json"}, {Code: 503, Body: "json"}}},
	)
}

