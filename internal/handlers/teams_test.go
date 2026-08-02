package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/forklifts-for-great-justice/judge-service/internal/handlers"
	"github.com/forklifts-for-great-justice/judge-service/internal/models"
	"github.com/forklifts-for-great-justice/judge-service/internal/repository"
)

// --- mock implementations ---

type mockTeamRepo struct {
	teams       map[int64]*models.Team
	nextID      int64
	returnErr   error
	duplicate   bool
}

func newMockTeamRepo() *mockTeamRepo {
	return &mockTeamRepo{
		teams:  make(map[int64]*models.Team),
		nextID: 1,
	}
}

func (m *mockTeamRepo) Create(_ context.Context, t *models.Team) error {
	if m.duplicate {
		return repository.ErrDuplicate
	}
	if m.returnErr != nil {
		return m.returnErr
	}
	now := time.Now().UTC()
	t.ID = m.nextID
	t.CreatedAt = now
	t.UpdatedAt = now
	m.teams[m.nextID] = t
	m.nextID++
	return nil
}

func (m *mockTeamRepo) GetByID(_ context.Context, id int64) (*models.Team, error) {
	t, ok := m.teams[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return t, nil
}

func (m *mockTeamRepo) Update(_ context.Context, id int64, updates map[string]any) error {
	t, ok := m.teams[id]
	if !ok {
		return repository.ErrNotFound
	}
	if v, ok := updates["name"]; ok {
		t.Name = v.(string)
	}
	if v, ok := updates["slug"]; ok {
		t.Slug = v.(string)
	}
	if v, ok := updates["alt_name"]; ok {
		t.AltName = v.(string)
	}
	if v, ok := updates["clan_tag"]; ok {
		t.ClanTag = v.(string)
	}
	t.UpdatedAt = time.Now().UTC()
	return nil
}

func (m *mockTeamRepo) Delete(_ context.Context, id int64) error {
	if _, ok := m.teams[id]; !ok {
		return repository.ErrNotFound
	}
	delete(m.teams, id)
	return nil
}

func (m *mockTeamRepo) GetAll(_ context.Context) ([]*models.Team, error) {
	result := make([]*models.Team, 0, len(m.teams))
	for _, t := range m.teams {
		result = append(result, t)
	}
	return result, nil
}

// --- tests ---

func TestTeamHandleList_Empty(t *testing.T) {
	h := handlers.NewTeamHandler(newMockTeamRepo())
	req := httptest.NewRequest("GET", "/teams", nil)
	w := httptest.NewRecorder()

	h.HandleList(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	teams, ok := body["teams"].([]interface{})
	if !ok {
		t.Fatal("expected teams array")
	}
	if len(teams) != 0 {
		t.Errorf("expected empty array, got %d items", len(teams))
	}
}

func TestHandleList_ReturnsTeam(t *testing.T) {
	repo := newMockTeamRepo()
	_ = repo.Create(context.Background(), &models.Team{Slug: "red", Name: "Red Team", AltName: "Reds", ClanTag: "RED"})

	h := handlers.NewTeamHandler(repo)
	req := httptest.NewRequest("GET", "/teams", nil)
	w := httptest.NewRecorder()

	h.HandleList(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	teams, ok := body["teams"].([]interface{})
	if !ok {
		t.Fatal("expected teams array")
	}
	if len(teams) != 1 {
		t.Fatalf("expected 1 team, got %d", len(teams))
	}
}

func TestHandleCreate_Success(t *testing.T) {
	h := handlers.NewTeamHandler(newMockTeamRepo())

	payload, _ := json.Marshal(map[string]string{
		"slug":     "blue-crew",
		"name":     "Blue Crew",
		"alt_name": "Blue Crew Alternative",
		"clan_tag": "BLU",
	})

	req := httptest.NewRequest("POST", "/teams", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.HandleCreate(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", w.Code)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	team, ok := body["team"].(map[string]interface{})
	if !ok {
		t.Fatal("expected team envelope in response")
	}
	if team["slug"] != "blue-crew" {
		t.Errorf("expected slug blue-crew, got %v", team["slug"])
	}
	if team["id"] == nil {
		t.Error("expected team to have an ID")
	}
}

func TestHandleCreate_MissingSlug(t *testing.T) {
	h := handlers.NewTeamHandler(newMockTeamRepo())

	payload, _ := json.Marshal(map[string]string{
		"name":     "No Slug Team",
		"alt_name": "Alt",
		"clan_tag": "NSL",
	})

	req := httptest.NewRequest("POST", "/teams", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.HandleCreate(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing slug, got %d", w.Code)
	}
}

func TestHandleCreate_BadSlugFormat(t *testing.T) {
	h := handlers.NewTeamHandler(newMockTeamRepo())

	payload, _ := json.Marshal(map[string]string{
		"slug":     "INVALID",
		"name":     "Bad Slug Team",
		"alt_name": "Alt",
		"clan_tag": "BAD",
	})

	req := httptest.NewRequest("POST", "/teams", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.HandleCreate(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for bad slug, got %d", w.Code)
	}
}

func TestHandleCreate_EmptyName(t *testing.T) {
	h := handlers.NewTeamHandler(newMockTeamRepo())

	payload, _ := json.Marshal(map[string]string{
		"slug":     "no-name",
		"name":     "",
		"alt_name": "Alt",
		"clan_tag": "NON",
	})

	req := httptest.NewRequest("POST", "/teams", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.HandleCreate(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for empty name, got %d", w.Code)
	}
}

func TestHandleGet_Success(t *testing.T) {
	repo := newMockTeamRepo()
	_ = repo.Create(context.Background(), &models.Team{Slug: "getme", Name: "Get Me", AltName: "G", ClanTag: "GM"})

	h := handlers.NewTeamHandler(repo)
	req := httptest.NewRequest("GET", "/teams/1", nil)
	req = withURLParams(req, "1")
	w := httptest.NewRecorder()

	h.HandleGet(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	team, ok := body["team"].(map[string]interface{})
	if !ok {
		t.Fatal("expected team object")
	}
	if team["name"] != "Get Me" {
		t.Errorf("expected name Get Me, got %v", team["name"])
	}
}

func TestTeamHandleGet_NotFound(t *testing.T) {
	h := handlers.NewTeamHandler(newMockTeamRepo())
	req := httptest.NewRequest("GET", "/teams/999", nil)
	req = withURLParams(req, "999")
	w := httptest.NewRecorder()

	h.HandleGet(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestHandleUpdate_Success(t *testing.T) {
	repo := newMockTeamRepo()
	_ = repo.Create(context.Background(), &models.Team{
		Slug:    "update-me",
		Name:    "Original Name",
		AltName: "Original Alt",
		ClanTag: "ORG",
	})

	h := handlers.NewTeamHandler(repo)

	payload, _ := json.Marshal(map[string]string{
		"name": "Updated Name",
	})

	req := httptest.NewRequest("PUT", "/teams/1", bytes.NewReader(payload))
	req = withURLParams(req, "1")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.HandleUpdate(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	team, ok := body["team"].(map[string]interface{})
	if !ok {
		t.Fatal("expected team object")
	}
	if team["name"] != "Updated Name" {
		t.Errorf("expected name Updated Name, got %v", team["name"])
	}
	// Verify other fields preserved
	if team["slug"] != "update-me" {
		t.Errorf("expected slug preserved, got %v", team["slug"])
	}
}

func TestHandleUpdate_ClanTagOnly(t *testing.T) {
	repo := newMockTeamRepo()
	_ = repo.Create(context.Background(), &models.Team{
		Slug:    "clantag-test",
		Name:    "Team",
		AltName: "Alt",
		ClanTag: "OLD",
	})

	h := handlers.NewTeamHandler(repo)

	payload, _ := json.Marshal(map[string]string{
		"clan_tag": "NEW",
	})

	req := httptest.NewRequest("PUT", "/teams/1", bytes.NewReader(payload))
	req = withURLParams(req, "1")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.HandleUpdate(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	team, ok := body["team"].(map[string]interface{})
	if !ok {
		t.Fatal("expected team object")
	}
	if team["clan_tag"] != "NEW" {
		t.Errorf("expected clan_tag NEW, got %v", team["clan_tag"])
	}
	if team["name"] != "Team" {
		t.Errorf("expected name preserved, got %v", team["name"])
	}
}

func TestHandleDelete_Success(t *testing.T) {
	repo := newMockTeamRepo()
	_ = repo.Create(context.Background(), &models.Team{Slug: "delete-me", Name: "Delete Me", AltName: "D", ClanTag: "DM"})

	h := handlers.NewTeamHandler(repo)
	req := httptest.NewRequest("DELETE", "/teams/1", nil)
	req = withURLParams(req, "1")
	w := httptest.NewRecorder()

	h.HandleDelete(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", w.Code)
	}
}

func TestHandleList_NilRepo(t *testing.T) {
	h := handlers.NewTeamHandler(nil)
	req := httptest.NewRequest("GET", "/teams", nil)
	w := httptest.NewRecorder()

	h.HandleList(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503 for nil repo, got %d", w.Code)
	}
}
