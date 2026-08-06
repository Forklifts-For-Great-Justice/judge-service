package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/forklifts-for-great-justice/judge-service/internal/handlers"
	"github.com/forklifts-for-great-justice/judge-service/internal/models"
	"github.com/forklifts-for-great-justice/judge-service/internal/repository"
)

// --- mock implementations ---

type mockRoundRepo struct {
	rounds    map[int64]*models.Round
	nextID    int64
	returnErr error
}

func newMockRoundRepo() *mockRoundRepo {
	return &mockRoundRepo{
		rounds: make(map[int64]*models.Round),
		nextID: 1,
	}
}

func (m *mockRoundRepo) Create(_ context.Context, r *models.Round) error {
	if m.returnErr != nil {
		return m.returnErr
	}
	if r.CreatedAt.IsZero() {
		r.CreatedAt = r.UpdatedAt
	}
	r.ID = m.nextID
	m.rounds[m.nextID] = r
	m.nextID++
	return nil
}

func (m *mockRoundRepo) GetByID(_ context.Context, id int64) (*models.Round, error) {
	if m.returnErr != nil {
		return nil, m.returnErr
	}
	r, ok := m.rounds[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return r, nil
}

func (m *mockRoundRepo) GetAll(_ context.Context) ([]*models.Round, error) {
	if m.returnErr != nil {
		return nil, m.returnErr
	}
	result := make([]*models.Round, 0, len(m.rounds))
	for _, r := range m.rounds {
		result = append(result, r)
	}
	return result, nil
}

func (m *mockRoundRepo) Update(_ context.Context, id int64, updates map[string]any) error {
	if m.returnErr != nil {
		return m.returnErr
	}
	r, ok := m.rounds[id]
	if !ok {
		return repository.ErrNotFound
	}
	for k, v := range updates {
		switch k {
		case "round_name":
			r.RoundName = v.(string)
		case "team_a_id":
			r.TeamAID = v.(int64)
		case "team_b_id":
			r.TeamBID = v.(int64)
		}
	}
	return nil
}

func (m *mockRoundRepo) Disable(_ context.Context, id int64) error {
	if m.returnErr != nil {
		return m.returnErr
	}
	if _, ok := m.rounds[id]; !ok {
		return repository.ErrNotFound
	}
	delete(m.rounds, id)
	return nil
}

func (m *mockRoundRepo) ToggleReady(_ context.Context, id int64) error {
	if m.returnErr != nil {
		return m.returnErr
	}
	r, ok := m.rounds[id]
	if !ok {
		return repository.ErrNotFound
	}
	r.Ready = !r.Ready
	r.Status = "scheduled"
	return nil
}

func (m *mockRoundRepo) SetLive(_ context.Context, id int64) (string, error) {
	if m.returnErr != nil {
		return "", m.returnErr
	}
	r, ok := m.rounds[id]
	if !ok {
		return "", repository.ErrNotFound
	}
	prevStatus := r.Status
	r.Live = !r.Live
	if r.Live {
		r.Status = "in_progress"
	} else {
		r.Status = "scheduled"
	}
	return prevStatus, nil
}

func (m *mockRoundRepo) GetCurrentTeams(_ context.Context) (*models.CurrentTeams, error) {
	if m.returnErr != nil {
		return nil, m.returnErr
	}
	return &models.CurrentTeams{
		TeamAID: 1,
		TeamBID: 2,
		TeamA:   &models.Team{ID: 1, Name: "Red Team", ClanTag: "RED"},
		TeamB:   &models.Team{ID: 2, Name: "Blue Team", ClanTag: "BLUE"},
	}, nil
}

func (m *mockRoundRepo) SetCurrentTeams(_ context.Context, teamAID, teamBID int64) (*models.CurrentTeams, error) {
	if m.returnErr != nil {
		return nil, m.returnErr
	}
	return &models.CurrentTeams{
		TeamAID: teamAID,
		TeamBID: teamBID,
		TeamA:   &models.Team{ID: teamAID, Name: "Team A", ClanTag: "A"},
		TeamB:   &models.Team{ID: teamBID, Name: "Team B", ClanTag: "B"},
	}, nil
}

// ==================== List tests ====================

func TestRoundHandler_HandleList_Empty(t *testing.T) {
	h := handlers.NewRoundHandler(newMockRoundRepo())
	req := httptest.NewRequest("GET", "/rounds", nil)
	w := httptest.NewRecorder()

	h.HandleList(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, `"rounds"`) {
		t.Error("expected 'rounds' key in response body")
	}
	if !strings.Contains(body, `"game_state"`) {
		t.Error("expected 'game_state' key in response body")
	}
}

func TestRoundHandler_HandleList_WithRounds(t *testing.T) {
	repo := newMockRoundRepo()
	repo.Create(context.Background(), &models.Round{
		RoundName: "Opening Match",
		TeamAID:   1,
		TeamBID:   2,
	})

	h := handlers.NewRoundHandler(repo)
	req := httptest.NewRequest("GET", "/rounds", nil)
	w := httptest.NewRecorder()

	h.HandleList(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	rounds, ok := body["rounds"].([]interface{})
	if !ok {
		t.Fatal("expected 'rounds' array")
	}
	if len(rounds) != 1 {
		t.Fatalf("expected 1 round, got %d", len(rounds))
	}
}

// ==================== Create tests ====================

func TestRoundHandler_HandleCreate_Success(t *testing.T) {
	h := handlers.NewRoundHandler(newMockRoundRepo())

	payload, _ := json.Marshal(map[string]interface{}{
		"round_name": "Opening Match",
		"team_a_id":  float64(1),
		"team_b_id":  float64(2),
	})

	req := httptest.NewRequest("POST", "/rounds", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.HandleCreate(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	round, ok := resp["round"].(map[string]interface{})
	if !ok {
		t.Fatal("expected 'round' key in response")
	}
	if round["round_name"] != "Opening Match" {
		t.Errorf("expected round_name Opening Match, got %v", round["round_name"])
	}
}

func TestRoundHandler_HandleCreate_EmptyName(t *testing.T) {
	h := handlers.NewRoundHandler(newMockRoundRepo())

	payload, _ := json.Marshal(map[string]interface{}{
		"round_name": "  ",
		"team_a_id":  float64(1),
		"team_b_id":  float64(2),
	})

	req := httptest.NewRequest("POST", "/rounds", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.HandleCreate(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestRoundHandler_HandleCreate_SelfMatch(t *testing.T) {
	h := handlers.NewRoundHandler(newMockRoundRepo())

	payload, _ := json.Marshal(map[string]interface{}{
		"round_name": "Self Match",
		"team_a_id":  float64(1),
		"team_b_id":  float64(1),
	})

	req := httptest.NewRequest("POST", "/rounds", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.HandleCreate(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

// ==================== Get tests ====================

func TestRoundHandler_HandleGet_Success(t *testing.T) {
	repo := newMockRoundRepo()
	repo.Create(context.Background(), &models.Round{
		RoundName: "Get Me",
		TeamAID:   1,
		TeamBID:   2,
	})

	h := handlers.NewRoundHandler(repo)
	req := httptest.NewRequest("GET", "/rounds/1", nil)
	req = withURLParams(req, "1")
	w := httptest.NewRecorder()

	h.HandleGet(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	round, ok := resp["round"].(map[string]interface{})
	if !ok {
		t.Fatal("expected 'round' key")
	}
	if round["round_name"] != "Get Me" {
		t.Errorf("expected round_name Get Me, got %v", round["round_name"])
	}
}

func TestRoundHandler_HandleGet_NotFound(t *testing.T) {
	h := handlers.NewRoundHandler(newMockRoundRepo())
	req := httptest.NewRequest("GET", "/rounds/9999", nil)
	req = withURLParams(req, "9999")
	w := httptest.NewRecorder()

	h.HandleGet(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

// ==================== Update tests ====================

func TestRoundHandler_HandleUpdate_Success(t *testing.T) {
	repo := newMockRoundRepo()
	repo.Create(context.Background(), &models.Round{
		RoundName: "Old Name",
		TeamAID:   1,
		TeamBID:   2,
	})

	h := handlers.NewRoundHandler(repo)

	payload, _ := json.Marshal(map[string]interface{}{
		"round_name": "Updated Match",
	})

	req := httptest.NewRequest("PUT", "/rounds/1", bytes.NewReader(payload))
	req = withURLParams(req, "1")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.HandleUpdate(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	round, ok := resp["round"].(map[string]interface{})
	if !ok {
		t.Fatal("expected 'round' key")
	}
	if round["round_name"] != "Updated Match" {
		t.Errorf("expected updated round_name 'Updated Match', got %v", round["round_name"])
	}
}

// ==================== Delete tests ====================

func TestRoundHandler_HandleDelete_Success(t *testing.T) {
	repo := newMockRoundRepo()
	repo.Create(context.Background(), &models.Round{
		RoundName: "Delete Me",
		TeamAID:   1,
		TeamBID:   2,
	})

	h := handlers.NewRoundHandler(repo)
	req := httptest.NewRequest("DELETE", "/rounds/1", nil)
	req = withURLParams(req, "1")
	w := httptest.NewRecorder()

	h.HandleDelete(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	r, ok := resp["round"].(map[string]interface{})
	if !ok {
		t.Fatal("expected 'round' key")
	}
	if disabled, ok := r["disabled"]; !ok || disabled != true {
		t.Errorf("expected disabled=true, got %v", r["disabled"])
	}
}

func TestRoundHandler_HandleDelete_NotFound(t *testing.T) {
	h := handlers.NewRoundHandler(newMockRoundRepo())
	req := httptest.NewRequest("DELETE", "/rounds/9999", nil)
	req = withURLParams(req, "9999")
	w := httptest.NewRecorder()

	h.HandleDelete(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

// ==================== Toggle Ready tests ====================

func TestRoundHandler_HandleToggleReady_On(t *testing.T) {
	repo := newMockRoundRepo()
	repo.Create(context.Background(), &models.Round{
		RoundName: "Ready Round",
		TeamAID:   1,
		TeamBID:   2,
		Ready:     false,
	})

	h := handlers.NewRoundHandler(repo)
	req := httptest.NewRequest("POST", "/rounds/1/ready", nil)
	req = withURLParams(req, "1")
	w := httptest.NewRecorder()

	h.HandleToggleReady(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	ready, ok := resp["ready"].(bool)
	if !ok {
		t.Fatal("expected 'ready' bool")
	}
	if ready != true {
		t.Errorf("expected ready=true, got %v", ready)
	}
}

func TestRoundHandler_HandleToggleReady_Off(t *testing.T) {
	repo := newMockRoundRepo()
	repo.Create(context.Background(), &models.Round{
		RoundName: "Ready Round",
		TeamAID:   1,
		TeamBID:   2,
		Ready:     true,
	})

	h := handlers.NewRoundHandler(repo)
	req := httptest.NewRequest("POST", "/rounds/1/ready", nil)
	req = withURLParams(req, "1")
	w := httptest.NewRecorder()

	h.HandleToggleReady(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	ready, ok := resp["ready"].(bool)
	if !ok {
		t.Fatal("expected 'ready' bool")
	}
	if ready != false {
		t.Errorf("expected ready=false, got %v", ready)
	}
}

// ==================== Toggle Live tests ====================

func TestRoundHandler_HandleToggleLive_On(t *testing.T) {
	repo := newMockRoundRepo()
	repo.Create(context.Background(), &models.Round{
		RoundName: "Live Round",
		TeamAID:   1,
		TeamBID:   2,
		Live:      false,
		Status:    "scheduled",
	})

	h := handlers.NewRoundHandler(repo)
	req := httptest.NewRequest("POST", "/rounds/1/live", nil)
	req = withURLParams(req, "1")
	w := httptest.NewRecorder()

	h.HandleToggleLive(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	live, ok := resp["live"].(bool)
	if !ok {
		t.Fatal("expected 'live' bool")
	}
	if live != true {
		t.Errorf("expected live=true, got %v", live)
	}
	status, ok := resp["status"].(string)
	if !ok {
		t.Fatal("expected 'status' string")
	}
	if status != "in_progress" {
		t.Errorf("expected status 'in_progress', got %s", status)
	}
}

func TestRoundHandler_HandleToggleLive_Off(t *testing.T) {
	repo := newMockRoundRepo()
	repo.Create(context.Background(), &models.Round{
		RoundName: "Live Round",
		TeamAID:   1,
		TeamBID:   2,
		Live:      true,
		Status:    "in_progress",
	})

	h := handlers.NewRoundHandler(repo)
	req := httptest.NewRequest("POST", "/rounds/1/live", nil)
	req = withURLParams(req, "1")
	w := httptest.NewRecorder()

	h.HandleToggleLive(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	live, ok := resp["live"].(bool)
	if !ok {
		t.Fatal("expected 'live' bool")
	}
	if live != false {
		t.Errorf("expected live=false, got %v", live)
	}
}

func TestRoundHandler_HandleGetCurrentTeams_Success(t *testing.T) {
	h := handlers.NewRoundHandler(newMockRoundRepo())
	req := httptest.NewRequest("GET", "/rounds/current/teams", nil)
	w := httptest.NewRecorder()

	h.HandleGetCurrentTeams(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if resp["team_a_id"] != float64(1) || resp["team_b_id"] != float64(2) {
		t.Errorf("got team IDs %v, %v; want 1, 2", resp["team_a_id"], resp["team_b_id"])
	}
}

func TestRoundHandler_HandleSetCurrentTeams_Success(t *testing.T) {
	h := handlers.NewRoundHandler(newMockRoundRepo())

	payload, _ := json.Marshal(map[string]interface{}{
		"team_a_id": float64(10),
		"team_b_id": float64(20),
	})

	req := httptest.NewRequest("POST", "/rounds/current/teams", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.HandleSetCurrentTeams(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if resp["team_a_id"] != float64(10) || resp["team_b_id"] != float64(20) {
		t.Errorf("got team IDs %v, %v; want 10, 20", resp["team_a_id"], resp["team_b_id"])
	}
}


// ==================== helpers ====================
