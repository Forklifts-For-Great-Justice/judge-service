package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/forklifts-for-great-justice/judge-service/internal/handlers"
	"github.com/forklifts-for-great-justice/judge-service/internal/models"
	"github.com/forklifts-for-great-justice/judge-service/internal/repository"
)

type mockPlayerRepo struct {
	challenges []*repository.PlayerChallengeItem
	shenanigans []*models.Shananigan
	submitResult bool
	submitPoints int
	submitErr error
	buyRecord *repository.PurchaseRecord
	buyRemaining int64
	buyErr error
}

func (m *mockPlayerRepo) GetChallengesForTeam(ctx context.Context, teamID int64) ([]*repository.PlayerChallengeItem, error) {
	return m.challenges, nil
}

func (m *mockPlayerRepo) SubmitFlag(ctx context.Context, challengeID int64, playerID string, teamID int64, submittedFlag string) (bool, int, error) {
	if m.submitErr != nil {
		return false, 0, m.submitErr
	}
	return m.submitResult, m.submitPoints, nil
}

func (m *mockPlayerRepo) GetEnabledPlayerShenanigans(ctx context.Context) ([]*models.Shananigan, error) {
	return m.shenanigans, nil
}

func (m *mockPlayerRepo) BuyShenanigan(ctx context.Context, shenaniganID int64, buyerID string, teamID int64) (*repository.PurchaseRecord, int64, error) {
	if m.buyErr != nil {
		return nil, m.buyRemaining, m.buyErr
	}
	return m.buyRecord, m.buyRemaining, nil
}

func TestPlayerHandlers(t *testing.T) {
	repo := &mockPlayerRepo{
		challenges: []*repository.PlayerChallengeItem{
			{
				Challenge: models.Challenge{
					ID: 1, Name: "Test Challenge", Description: "Desc", Points: 100, Flag: "flag{test}",
					CreatedAt: time.Now(), UpdatedAt: time.Now(),
				},
				Solved: true,
			},
		},
		shenanigans: []*models.Shananigan{
			{
				ID: 1, Name: "Flashbang", Description: "Blinds team", RconPayload: "say flash", TargetType: "team",
				CreatedAt: time.Now(), UpdatedAt: time.Now(),
			},
		},
		submitResult: true,
		submitPoints: 100,
		buyRecord: &repository.PurchaseRecord{
			PurchaseID: uuid.New(),
			RconPayload: "say flash",
		},
		buyRemaining: 500,
	}

	h := handlers.NewPlayerHandler(repo, nil, nil)
	r := chi.NewRouter()
	handlers.RegisterPlayerRoutes(r, h)

	t.Run("GET /player/challenges", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/player/challenges?team_id=1", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rec.Code)
		}

		var resp map[string][]repository.PlayerChallengeItem
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if len(resp["challenges"]) != 1 || !resp["challenges"][0].Solved {
			t.Errorf("unexpected challenge response: %+v", resp)
		}
	})

	t.Run("POST /player/challenges/submit", func(t *testing.T) {
		body := map[string]any{
			"challenge_id": 1,
			"flag": "flag{test}",
			"team_id": 1,
		}
		b, _ := json.Marshal(body)
		req := httptest.NewRequest("POST", "/player/challenges/submit", bytes.NewReader(b))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rec.Code)
		}

		var resp map[string]any
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp["correct"] != true || resp["points_awarded"] != float64(100) {
			t.Errorf("unexpected submit response: %+v", resp)
		}
	})

	t.Run("GET /player/shenanigans", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/player/shenanigans", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rec.Code)
		}

		var resp map[string][]models.Shananigan
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if len(resp["shenanigans"]) != 1 || resp["shenanigans"][0].Name != "Flashbang" {
			t.Errorf("unexpected shenanigans response: %+v", resp)
		}
	})

	t.Run("POST /player/shenanigans/buy", func(t *testing.T) {
		body := map[string]any{
			"shenanigan_id": 1,
			"team_id": 1,
		}
		b, _ := json.Marshal(body)
		req := httptest.NewRequest("POST", "/player/shenanigans/buy", bytes.NewReader(b))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rec.Code)
		}

		var resp map[string]any
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp["purchase_id"] == "" || resp["remaining_coins"] != float64(500) {
			t.Errorf("unexpected buy response: %+v", resp)
		}
	})

	t.Run("POST /player/shenanigans/buy insufficient funds", func(t *testing.T) {
		repo.buyErr = fmt.Errorf("insufficient hackcoins: team has 0, required 50")
		defer func() { repo.buyErr = nil }()

		body := map[string]any{
			"shenanigan_id": 1,
			"team_id": 1,
		}
		b, _ := json.Marshal(body)
		req := httptest.NewRequest("POST", "/player/shenanigans/buy", bytes.NewReader(b))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rec.Code)
		}

		var resp map[string]string
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp["error"] != "you have no money" {
			t.Errorf("expected error 'you have no money', got '%s'", resp["error"])
		}
	})

	t.Run("POST /player/shenanigans/buy team not in round", func(t *testing.T) {
		repo.buyErr = repository.ErrTeamNotInMatch
		defer func() { repo.buyErr = nil }()

		body := map[string]any{
			"shenanigan_id": 1,
			"team_id": 99,
		}
		b, _ := json.Marshal(body)
		req := httptest.NewRequest("POST", "/player/shenanigans/buy", bytes.NewReader(b))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rec.Code)
		}

		var resp map[string]string
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp["error"] != "your team is not in this round, WTF do you think you're doing" {
			t.Errorf("unexpected error message: %s", resp["error"])
		}
	})
}
