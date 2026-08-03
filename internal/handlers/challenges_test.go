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

type mockChallengeRepo struct {
	challenges map[int64]*models.Challenge
	nextID     int64
	returnErr  error
	duplicate  bool
}

func newMockChallengeRepo() *mockChallengeRepo {
	return &mockChallengeRepo{
		challenges: make(map[int64]*models.Challenge),
		nextID:     1,
	}
}

func (m *mockChallengeRepo) Create(_ context.Context, c *models.Challenge) error {
	if m.duplicate {
		return repository.ErrDuplicate
	}
	if m.returnErr != nil {
		return m.returnErr
	}
	now := time.Now().UTC()
	c.ID = m.nextID
	c.CreatedAt = now
	c.UpdatedAt = now
	m.challenges[m.nextID] = c
	m.nextID++
	return nil
}

func (m *mockChallengeRepo) GetByID(_ context.Context, id int64) (*models.Challenge, error) {
	c, ok := m.challenges[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return c, nil
}

func (m *mockChallengeRepo) Update(_ context.Context, id int64, updates map[string]any) error {
	c, ok := m.challenges[id]
	if !ok {
		return repository.ErrNotFound
	}
	if v, ok := updates["name"]; ok {
		c.Name = v.(string)
	}
	if v, ok := updates["description"]; ok {
		c.Description = v.(string)
	}
	if v, ok := updates["challenge_type"]; ok {
		s := v.(string)
		c.ChallengeType = &s
	}
	if v, ok := updates["location"]; ok {
		s := v.(string)
		c.Location = &s
	}
	if v, ok := updates["points"]; ok {
		c.Points = v.(int)
	}
	if v, ok := updates["disabled"]; ok {
		c.Disabled = v.(bool)
	}
	if v, ok := updates["flag"]; ok {
		c.Flag = v.(string)
	}
	c.UpdatedAt = time.Now().UTC()
	return nil
}

func (m *mockChallengeRepo) SoftDelete(_ context.Context, id int64) error {
	c, ok := m.challenges[id]
	if !ok || c.Disabled {
		return repository.ErrNotFound
	}
	c.Disabled = true
	return nil
}

func (m *mockChallengeRepo) GetAll(_ context.Context) ([]*models.Challenge, error) {
	result := make([]*models.Challenge, 0, len(m.challenges))
	for _, c := range m.challenges {
		result = append(result, c)
	}
	return result, nil
}

// --- tests ---

func TestChallengeHandleList_Empty(t *testing.T) {
	h := handlers.NewChallengeHandler(newMockChallengeRepo())
	req := httptest.NewRequest("GET", "/challenges", nil)
	w := httptest.NewRecorder()

	h.HandleList(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	challenges, ok := body["challenges"].([]interface{})
	if !ok {
		t.Fatal("expected challenges array")
	}
	if len(challenges) != 0 {
		t.Errorf("expected empty array, got %d items", len(challenges))
	}
}

func TestChallengeHandleList_ReturnsChallenge(t *testing.T) {
	repo := newMockChallengeRepo()
	_ = repo.Create(context.Background(), &models.Challenge{
		Name:        "SQL Injection",
		Description: "Find the SQL injection",
		ChallengeType: func() *string { s := "web"; return &s }(),
		Points:      100,
		Flag:        "FLAG{test}",
	})

	h := handlers.NewChallengeHandler(repo)
	req := httptest.NewRequest("GET", "/challenges", nil)
	w := httptest.NewRecorder()

	h.HandleList(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	challenges, ok := body["challenges"].([]interface{})
	if !ok {
		t.Fatal("expected challenges array")
	}
	if len(challenges) != 1 {
		t.Fatalf("expected 1 challenge, got %d", len(challenges))
	}
}

func TestChallengeHandleCreate_Success(t *testing.T) {
	h := handlers.NewChallengeHandler(newMockChallengeRepo())

	payload, _ := json.Marshal(map[string]interface{}{
		"name":          "Web Exploitation",
		"description":   "Find the XSS vulnerability",
		"challenge_type": "web",
		"points":        150,
		"flag":          "FLAG{web}",
		"disabled":      false,
	})

	req := httptest.NewRequest("POST", "/challenges", bytes.NewReader(payload))
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

	challenge, ok := body["challenge"].(map[string]interface{})
	if !ok {
		t.Fatal("expected challenge envelope in response")
	}
	if challenge["name"] != "Web Exploitation" {
		t.Errorf("expected name Web Exploitation, got %v", challenge["name"])
	}
	if challenge["id"] == nil {
		t.Error("expected challenge to have an ID")
	}
	if challenge["challenge_type"] != "web" {
		t.Errorf("expected challenge_type web, got %v", challenge["challenge_type"])
	}
}

func TestChallengeHandleCreate_MissingName(t *testing.T) {
	h := handlers.NewChallengeHandler(newMockChallengeRepo())

	payload, _ := json.Marshal(map[string]interface{}{
		"description": "No name!",
		"flag":        "FLAG{test}",
		"points":      100,
	})

	req := httptest.NewRequest("POST", "/challenges", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.HandleCreate(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing name, got %d", w.Code)
	}
}

func TestChallengeHandleCreate_EmptyFlag(t *testing.T) {
	h := handlers.NewChallengeHandler(newMockChallengeRepo())

	payload, _ := json.Marshal(map[string]interface{}{
		"name":     "No Flag",
		"flag":     "",
		"points":   100,
	})

	req := httptest.NewRequest("POST", "/challenges", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.HandleCreate(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for empty flag, got %d", w.Code)
	}
}

func TestChallengeHandleCreate_ZeroPoints(t *testing.T) {
	h := handlers.NewChallengeHandler(newMockChallengeRepo())

	payload, _ := json.Marshal(map[string]interface{}{
		"name":     "Free Points",
		"points":   0,
		"flag":     "FLAG{test}",
	})

	req := httptest.NewRequest("POST", "/challenges", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.HandleCreate(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for zero points, got %d", w.Code)
	}
}

func TestChallengeHandleGet_Success(t *testing.T) {
	repo := newMockChallengeRepo()
	challenge := &models.Challenge{
		Name:        "Get Me",
		Description: "Find me",
		ChallengeType: func() *string { s := "binary"; return &s }(),
		Points:      200,
		Flag:        "FLAG{get}",
	}
	_ = repo.Create(context.Background(), challenge)

	h := handlers.NewChallengeHandler(repo)
	req := httptest.NewRequest("GET", "/challenges/1", nil)
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

	got, ok := body["challenge"].(map[string]interface{})
	if !ok {
		t.Fatal("expected challenge object")
	}
	if got["name"] != "Get Me" {
		t.Errorf("expected name Get Me, got %v", got["name"])
	}
	if got["id"] != float64(1) {
		t.Errorf("expected id 1, got %v", got["id"])
	}
	if got["challenge_type"] != "binary" {
		t.Errorf("expected challenge_type binary, got %v", got["challenge_type"])
	}
}

func TestChallengeHandleGet_NotFound(t *testing.T) {
	h := handlers.NewChallengeHandler(newMockChallengeRepo())
	req := httptest.NewRequest("GET", "/challenges/999", nil)
	req = withURLParams(req, "999")
	w := httptest.NewRecorder()

	h.HandleGet(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestChallengeHandleUpdate_Success(t *testing.T) {
	repo := newMockChallengeRepo()
	_ = repo.Create(context.Background(), &models.Challenge{
		Name:        "Original Name",
		Description: "Original Desc",
		ChallengeType: func() *string { s := "crypto"; return &s }(),
		Points:      100,
		Flag:        "FLAG{original}",
	})

	h := handlers.NewChallengeHandler(repo)

	payload, _ := json.Marshal(map[string]string{
		"name": "Updated Name",
	})

	req := httptest.NewRequest("PUT", "/challenges/1", bytes.NewReader(payload))
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

	challenge, ok := body["challenge"].(map[string]interface{})
	if !ok {
		t.Fatal("expected challenge object")
	}
	if challenge["name"] != "Updated Name" {
		t.Errorf("expected name Updated Name, got %v", challenge["name"])
	}
	// Verify other fields preserved
	if challenge["description"] != "Original Desc" {
		t.Errorf("expected description preserved, got %v", challenge["description"])
	}
	if challenge["points"] != float64(100) {
		t.Errorf("expected points preserved, got %v", challenge["points"])
	}
}

func TestChallengeHandleUpdate_DisabledOnly(t *testing.T) {
	repo := newMockChallengeRepo()
	_ = repo.Create(context.Background(), &models.Challenge{
		Name:        "Keep Me",
		Description: "Unchanged",
		ChallengeType: func() *string { s := "web"; return &s }(),
		Points:      50,
		Flag:        "FLAG{keep}",
	})

	h := handlers.NewChallengeHandler(repo)

	disabled := true
	payload, _ := json.Marshal(map[string]bool{
		"disabled": disabled,
	})

	req := httptest.NewRequest("PUT", "/challenges/1", bytes.NewReader(payload))
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

	challenge, ok := body["challenge"].(map[string]interface{})
	if !ok {
		t.Fatal("expected challenge object")
	}
	if challenge["disabled"] != true {
		t.Errorf("expected disabled true, got %v", challenge["disabled"])
	}
	if challenge["name"] != "Keep Me" {
		t.Errorf("expected name preserved, got %v", challenge["name"])
	}
}

func TestChallengeHandleUpdate_EmptyBody(t *testing.T) {
	repo := newMockChallengeRepo()
	_ = repo.Create(context.Background(), &models.Challenge{
		Name:     "No Change",
		Points:   100,
		Flag:     "FLAG{n}",
	})

	h := handlers.NewChallengeHandler(repo)

	payload, _ := json.Marshal(map[string]interface{}{})

	req := httptest.NewRequest("PUT", "/challenges/1", bytes.NewReader(payload))
	req = withURLParams(req, "1")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.HandleUpdate(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for empty update, got %d", w.Code)
	}
}

func TestChallengeHandleSoftDelete_Success(t *testing.T) {
	repo := newMockChallengeRepo()
	_ = repo.Create(context.Background(), &models.Challenge{
		Name:        "Delete Me",
		Description: "Gone",
		Points:      100,
		Flag:        "FLAG{del}",
	})

	h := handlers.NewChallengeHandler(repo)
	req := httptest.NewRequest("DELETE", "/challenges/1", nil)
	req = withURLParams(req, "1")
	w := httptest.NewRecorder()

	h.HandleDelete(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", w.Code)
	}
}

func TestChallengeHandleSoftDelete_NilRepo(t *testing.T) {
	h := handlers.NewChallengeHandler(nil)
	req := httptest.NewRequest("DELETE", "/challenges/1", nil)
	req = withURLParams(req, "1")
	w := httptest.NewRecorder()

	h.HandleDelete(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503 for nil repo, got %d", w.Code)
	}
}

func TestChallengeHandleCreate_Duplicate(t *testing.T) {
	repo := newMockChallengeRepo()
	repo.duplicate = true
	h := handlers.NewChallengeHandler(repo)

	payload, _ := json.Marshal(map[string]interface{}{
		"name":          "Duplicate",
		"description":   "Will duplicate",
		"points":        100,
		"flag":          "FLAG{dup}",
	})

	req := httptest.NewRequest("POST", "/challenges", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.HandleCreate(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("expected 409 for duplicate, got %d", w.Code)
	}
}

func TestChallengeHandleList_NilRepo(t *testing.T) {
	h := handlers.NewChallengeHandler(nil)
	req := httptest.NewRequest("GET", "/challenges", nil)
	w := httptest.NewRecorder()

	h.HandleList(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503 for nil repo, got %d", w.Code)
	}
}

func TestChallengeHandleGet_NilRepo(t *testing.T) {
	h := handlers.NewChallengeHandler(nil)
	req := httptest.NewRequest("GET", "/challenges/1", nil)
	req = withURLParams(req, "1")
	w := httptest.NewRecorder()

	h.HandleGet(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503 for nil repo, got %d", w.Code)
	}
}
