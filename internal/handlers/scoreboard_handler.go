// Package handlers provides HTTP handlers for the judge service.
package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/forklifts-for-great-justice/judge-service/internal/openapi"
	"github.com/forklifts-for-great-justice/judge-service/internal/repository"
)

// ScoreboardHandler handles GET /scoreboard requests.
type ScoreboardHandler struct {
	repo repository.ScoreboardRepository
}

// NewScoreboardHandler creates a new ScoreboardHandler instance.
func NewScoreboardHandler(repo repository.ScoreboardRepository) *ScoreboardHandler {
	return &ScoreboardHandler{repo: repo}
}

// HandleGet serves GET /scoreboard.
func (h *ScoreboardHandler) HandleGet(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if h == nil || h.repo == nil {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{})
		return
	}

	sb, err := h.repo.GetScoreboard(r.Context())
	if err != nil || sb == nil {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{})
		return
	}

	resp := make(map[string]any)

	if sb.TeamAName != "" {
		resp[sb.TeamAName] = map[string]int{
			"quake_points": sb.TeamAPoints,
			"hack_points":  sb.TeamAHackPoints,
			"hack_coins":   sb.TeamAHackCoins,
		}
	}

	if sb.TeamBName != "" {
		resp[sb.TeamBName] = map[string]int{
			"quake_points": sb.TeamBPoints,
			"hack_points":  sb.TeamBHackPoints,
			"hack_coins":   sb.TeamBHackCoins,
		}
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

// RegisterOpenAPI registers the /scoreboard route in the OpenAPI spec.
func (h *ScoreboardHandler) RegisterOpenAPI(reg *openapi.Registry) {
	reg.Register(
		openapi.Route{
			Method:      "GET",
			Path:        "/scoreboard",
			OperationID: "getScoreboard",
			Description: "Returns current match scores by team name.",
			Responses: []openapi.Response{
				{Code: 200, Body: "json"},
			},
		},
	)
}
