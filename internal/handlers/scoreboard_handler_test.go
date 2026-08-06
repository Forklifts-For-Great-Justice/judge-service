package handlers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/forklifts-for-great-justice/judge-service/internal/handlers"
	"github.com/forklifts-for-great-justice/judge-service/internal/repository"
)

type mockScoreboardRepo struct {
	data *repository.ScoreboardData
	err  error
}

func (m *mockScoreboardRepo) GetScoreboard(ctx context.Context) (*repository.ScoreboardData, error) {
	return m.data, m.err
}

func TestScoreboardHandler_HandleGet_Success(t *testing.T) {
	repo := &mockScoreboardRepo{
		data: &repository.ScoreboardData{
			TeamAName:       "foo",
			TeamBName:       "bar",
			TeamAPoints:     20,
			TeamBPoints:     0,
			TeamAHackPoints: 40,
			TeamBHackPoints: 200,
			TeamAHackCoins:  50,
			TeamBHackCoins:  200,
		},
	}
	h := handlers.NewScoreboardHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/scoreboard", nil)
	rr := httptest.NewRecorder()

	h.HandleGet(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Fatalf("expected status 200, got %d", status)
	}

	var res map[string]map[string]int
	if err := json.Unmarshal(rr.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}

	foo, ok := res["foo"]
	if !ok {
		t.Fatalf("expected key 'foo' in response")
	}
	if foo["quake_points"] != 20 || foo["hack_points"] != 40 || foo["hack_coins"] != 50 {
		t.Errorf("foo stats mismatch: %v", foo)
	}

	bar, ok := res["bar"]
	if !ok {
		t.Fatalf("expected key 'bar' in response")
	}
	if bar["quake_points"] != 0 || bar["hack_points"] != 200 || bar["hack_coins"] != 200 {
		t.Errorf("bar stats mismatch: %v", bar)
	}
}

func TestScoreboardHandler_HandleGet_NoTeams(t *testing.T) {
	repo := &mockScoreboardRepo{
		data: nil,
		err:  repository.ErrNotFound,
	}
	h := handlers.NewScoreboardHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/scoreboard", nil)
	rr := httptest.NewRecorder()

	h.HandleGet(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Fatalf("expected status 200, got %d", status)
	}

	var res map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}

	if len(res) != 0 {
		t.Errorf("expected empty map {}, got %v", res)
	}
}
