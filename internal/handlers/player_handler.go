// Package handlers provides HTTP handlers for player-facing endpoints.
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
	"github.com/forklifts-for-great-justice/judge-service/internal/rabbitmq"
	"github.com/forklifts-for-great-justice/judge-service/internal/repository"
)

// PlayerHandler handles HTTP requests for player-facing endpoints (/player/*)
type PlayerHandler struct {
	repo      repository.PlayerRepository
	publisher Publisher
	metrics   ShenaniganMetrics
}

// NewPlayerHandler creates a new PlayerHandler.
func NewPlayerHandler(repo repository.PlayerRepository, publisher Publisher, metrics ShenaniganMetrics) *PlayerHandler {
	return &PlayerHandler{repo: repo, publisher: publisher, metrics: metrics}
}

func isNilPlayerRepo(r repository.PlayerRepository) bool {
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

// RegisterPlayerRoutes registers /player routes on the chi router.
func RegisterPlayerRoutes(r chi.Router, h *PlayerHandler) {
	r.Get("/player/challenges", h.HandleListChallenges)
	r.Post("/player/challenges/submit", h.HandleSubmitChallenge)
	r.Get("/player/shenanigans", h.HandleListShenanigans)
	r.Post("/player/shenanigans/buy", h.HandleBuyShenanigan)
}

// RegisterOpenAPI registers player routes with the OpenAPI registry.
func (h *PlayerHandler) RegisterOpenAPI(reg *openapi.Registry) {
	reg.Register(
		openapi.Route{
			Method:      "GET",
			Path:        "/player/challenges",
			OperationID: "listPlayerChallenges",
			Description: "Shows available challenges with solved flag for the team.",
			Responses: []openapi.Response{
				{Code: 200, Body: "json"},
			},
		},
		openapi.Route{
			Method:      "POST",
			Path:        "/player/challenges/submit",
			OperationID: "submitPlayerChallenge",
			Description: "Submit a flag for a challenge.",
			RequestBody: "json",
			Responses: []openapi.Response{
				{Code: 200, Body: "json"},
				{Code: 400, Body: "json"},
				{Code: 404, Body: "json"},
			},
		},
		openapi.Route{
			Method:      "GET",
			Path:        "/player/shenanigans",
			OperationID: "listPlayerShenanigans",
			Description: "Shows all enabled player shenanigans.",
			Responses: []openapi.Response{
				{Code: 200, Body: "json"},
			},
		},
		openapi.Route{
			Method:      "POST",
			Path:        "/player/shenanigans/buy",
			OperationID: "buyPlayerShenanigan",
			Description: "Spend hack_coins to buy a requested shenanigan.",
			RequestBody: "json",
			Responses: []openapi.Response{
				{Code: 200, Body: "json"},
				{Code: 400, Body: "json"},
				{Code: 404, Body: "json"},
			},
		},
	)
}

// writePlayerError writes JSON error responses for player endpoints using HTTP 200 OK.
func writePlayerError(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// HandleListChallenges serves GET /player/challenges
// Shows available challenges; challenges the team has solved are marked solved: true.
// Optional team_id via header or query param.
func (h *PlayerHandler) HandleListChallenges(w http.ResponseWriter, req *http.Request) {
	if isNilPlayerRepo(h.repo) {
		writePlayerError(w, "database not available")
		return
	}

	ctx := req.Context()
	teamID := parseTeamID(req)

	challenges, err := h.repo.GetChallengesForTeam(ctx, teamID)
	if err != nil {
		writePlayerError(w, fmt.Sprintf("failed to list challenges: %v", err))
		return
	}

	if challenges == nil {
		challenges = []*repository.PlayerChallengeItem{}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{
		"challenges": challenges,
	})
}

// HandleSubmitChallenge serves POST /player/challenges/submit
// Players submit flag for challenge_id.
type SubmitChallengeRequest struct {
	ChallengeID int64  `json:"challenge_id"`
	Flag        string `json:"flag"`
	PlayerID    string `json:"player_id,omitempty"`
	TeamID      int64  `json:"team_id,omitempty"`
}

func (h *PlayerHandler) HandleSubmitChallenge(w http.ResponseWriter, req *http.Request) {
	if isNilPlayerRepo(h.repo) {
		writePlayerError(w, "database not available")
		return
	}

	ctx := req.Context()

	var body SubmitChallengeRequest
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		writePlayerError(w, "invalid request body")
		return
	}

	if body.ChallengeID <= 0 {
		writePlayerError(w, "challenge_id is required")
		return
	}

	submittedFlag := strings.TrimSpace(body.Flag)
	if submittedFlag == "" {
		writePlayerError(w, "flag is required")
		return
	}

	playerID := body.PlayerID
	if playerID == "" {
		playerID = req.Header.Get("x-auth-user")
		if playerID == "" {
			playerID = "anonymous"
		}
	}

	teamID := body.TeamID
	if teamID <= 0 {
		teamID = parseTeamID(req)
	}

	correct, pointsAwarded, err := h.repo.SubmitFlag(ctx, body.ChallengeID, playerID, teamID, submittedFlag)
	if err != nil {
		if err == repository.ErrTeamNotInMatch || strings.Contains(err.Error(), "WTF do you think") {
			writePlayerError(w, "your team is not in this round, WTF do you think you're doing")
			return
		}
		if err == repository.ErrNotFound || err == repository.ErrChallengeNotFound {
			writePlayerError(w, "challenge not found")
			return
		}
		writePlayerError(w, fmt.Sprintf("failed to submit flag: %v", err))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{
		"correct":        correct,
		"points_awarded": pointsAwarded,
		"message":        map[bool]string{true: "Flag accepted!", false: "Incorrect flag"}[correct],
	})
}

// HandleListShenanigans serves GET /player/shenanigans
// Shows all enabled player shenanigans.
func (h *PlayerHandler) HandleListShenanigans(w http.ResponseWriter, req *http.Request) {
	if isNilPlayerRepo(h.repo) {
		writePlayerError(w, "database not available")
		return
	}

	ctx := req.Context()
	shenaniganList, err := h.repo.GetEnabledPlayerShenanigans(ctx)
	if err != nil {
		writePlayerError(w, fmt.Sprintf("failed to list shenanigans: %v", err))
		return
	}

	if shenaniganList == nil {
		shenaniganList = []*models.Shananigan{}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{
		"shenanigans": shenaniganList,
	})
}

// HandleBuyShenanigan serves POST /player/shenanigans/buy
// Spend hack_coins to buy the requested shenanigan.
type BuyShenaniganRequest struct {
	ShenaniganID int64           `json:"shenanigan_id"`
	BuyerID      string          `json:"buyer_id,omitempty"`
	TeamID       int64           `json:"team_id,omitempty"`
	TargetTeam   string          `json:"target_team,omitempty"`
	Metadata     json.RawMessage `json:"metadata,omitempty"`
}

func (h *PlayerHandler) HandleBuyShenanigan(w http.ResponseWriter, req *http.Request) {
	if isNilPlayerRepo(h.repo) {
		writePlayerError(w, "database not available")
		return
	}

	ctx := req.Context()

	var body BuyShenaniganRequest
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		writePlayerError(w, "invalid request body")
		return
	}

	if body.ShenaniganID <= 0 {
		writePlayerError(w, "shenanigan_id is required")
		return
	}

	buyerID := body.BuyerID
	if buyerID == "" {
		buyerID = req.Header.Get("x-auth-user")
		if buyerID == "" {
			buyerID = "anonymous"
		}
	}

	teamID := body.TeamID
	if teamID <= 0 {
		teamID = parseTeamID(req)
	}

	record, remainingCoins, err := h.repo.BuyShenanigan(ctx, body.ShenaniganID, buyerID, teamID)
	if err != nil {
		if err == repository.ErrTeamNotInMatch || strings.Contains(err.Error(), "WTF do you think") {
			writePlayerError(w, "your team is not in this round, WTF do you think you're doing")
			return
		}
		if err == repository.ErrNotFound {
			writePlayerError(w, "shenanigan not found")
			return
		}
		if strings.Contains(err.Error(), "insufficient") {
			writePlayerError(w, "you have no money")
			return
		}
		writePlayerError(w, fmt.Sprintf("failed to buy shenanigan: %v", err))
		return
	}

	if h.metrics != nil {
		h.metrics.IncrementActivations()
	}

	published := false
	status := "ok"
	if h.publisher != nil {
		msg := rabbitmq.ShenaniganMessage{
			PurchaseID:   record.PurchaseID.String(),
			ShenaniganID: fmt.Sprintf("%d", body.ShenaniganID),
			RconPayload:  record.RconPayload,
			Metadata:     body.Metadata,
		}

		published, err = h.publisher.Publish(ctx, msg)
		if err != nil {
			if h.metrics != nil {
				h.metrics.IncrementPublishFailures()
			}
			status = "error"
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{
		"purchase_id":     record.PurchaseID.String(),
		"status":          status,
		"published":       published,
		"remaining_coins": remainingCoins,
	})
}

// parseTeamID attempts to parse team_id from header (x-team-id) or query parameter (team_id).
func parseTeamID(req *http.Request) int64 {
	if t := req.Header.Get("x-team-id"); t != "" {
		if id, err := strconv.ParseInt(t, 10, 64); err == nil && id > 0 {
			return id
		}
	}
	if t := req.URL.Query().Get("team_id"); t != "" {
		if id, err := strconv.ParseInt(t, 10, 64); err == nil && id > 0 {
			return id
		}
	}
	return 0
}
