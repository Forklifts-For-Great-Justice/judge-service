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
	"github.com/forklifts-for-great-justice/judge-service/internal/openapi"
	"github.com/forklifts-for-great-justice/judge-service/internal/repository"
	"github.com/forklifts-for-great-justice/judge-service/internal/rabbitmq"
)

// --- mock implementations ---

type mockRepo struct {
	records  map[int64]*models.Shananigan
	nextID   int64
}

func newMockRepo() *mockRepo {
	return &mockRepo{
		records: make(map[int64]*models.Shananigan),
		nextID:  1,
	}
}

func (m *mockRepo) Create(_ context.Context, s *models.Shananigan) error {
	now := time.Now().UTC()
	s.CreatedAt = now
	s.UpdatedAt = now
	s.ID = m.nextID
	m.records[m.nextID] = s
	m.nextID++
	return nil
}

func (m *mockRepo) GetByID(_ context.Context, id int64) (*models.Shananigan, error) {
	s, ok := m.records[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return s, nil
}

func (m *mockRepo) GetAll(_ context.Context) ([]*models.Shananigan, error) {
	result := make([]*models.Shananigan, 0, len(m.records))
	for _, s := range m.records {
		result = append(result, s)
	}
	return result, nil
}

func (m *mockRepo) Update(_ context.Context, id int64, updates map[string]any) error {
	s, ok := m.records[id]
	if !ok {
		return repository.ErrNotFound
	}
	if v, ok := updates["name"]; ok {
		s.Name = v.(string)
	}
	if v, ok := updates["description"]; ok {
		s.Description = v.(string)
	}
	if v, ok := updates["rcon_payload"]; ok {
		s.RconPayload = v.(string)
	}
	if v, ok := updates["target_type"]; ok {
		s.TargetType = v.(string)
	}
	if v, ok := updates["cost"]; ok {
		s.Cost = v.(*int64)
	}
	if v, ok := updates["metadata"]; ok {
		s.Metadata = v.(json.RawMessage)
	}
	return nil
}

func (m *mockRepo) Delete(_ context.Context, id int64) error {
	if _, ok := m.records[id]; !ok {
		return repository.ErrNotFound
	}
	delete(m.records, id)
	return nil
}

func (m *mockRepo) Activate(_ context.Context, shenaniganID int64) (*repository.PurchaseRecord, error) {
	s, ok := m.records[shenaniganID]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return &repository.PurchaseRecord{
		RconPayload: s.RconPayload,
		PurchaseID:  uuid.New(),
	}, nil
}

// mockPublisher implements the publisher interface for Activate tests.
type mockPublisher struct {
	calls     int
	lastMsg   rabbitmq.ShenaniganMessage
	returnErr error
}

func (m *mockPublisher) Publish(_ context.Context, msg rabbitmq.ShenaniganMessage) (bool, error) {
	m.calls++
	m.lastMsg = msg
	if m.returnErr != nil {
		return false, m.returnErr
	}
	return true, nil
}

// withURLParams wraps an httptest request with chi URL params for the "id" key.
func withURLParams(req *http.Request, id string) *http.Request {
	rc := chi.NewRouteContext()
	rc.URLParams.Add("id", id)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rc))
}

// --- test cases ---

func TestHandleList(t *testing.T) {
	repo := newMockRepo()
	_ = repo.Create(context.Background(), &models.Shananigan{Name: "Fireball", Description: "Blast", RconPayload: "say fire", TargetType: "team"})
	_ = repo.Create(context.Background(), &models.Shananigan{Name: "Thunder", Description: "Crash", RconPayload: "say boom", TargetType: "all"})

	h := handlers.NewShenaniganHandler(repo, nil)
	req := httptest.NewRequest("GET", "/shenanigans", nil)
	w := httptest.NewRecorder()

	h.HandleList(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	shenanigans, ok := body["shenanigans"].([]interface{})
	if !ok {
		t.Fatal("expected shenanigans array")
	}
	if len(shenanigans) != 2 {
		t.Fatalf("expected 2 shenanigans, got %d", len(shenanigans))
	}
}

func TestHandleList_Empty(t *testing.T) {
	h := handlers.NewShenaniganHandler(newMockRepo(), nil)
	req := httptest.NewRequest("GET", "/shenanigans", nil)
	w := httptest.NewRecorder()

	h.HandleList(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	shenanigans := body["shenanigans"].([]interface{})
	if len(shenanigans) != 0 {
		t.Errorf("expected empty array, got %d items", len(shenanigans))
	}
}

func TestHandleCreate(t *testing.T) {
	h := handlers.NewShenaniganHandler(newMockRepo(), nil)

	payload, _ := json.Marshal(map[string]string{
		"name":         "Meteor",
		"description":  "Calls meteor",
		"rcon_payload": "call meteor 5",
		"target_type":  "all",
	})

	req := httptest.NewRequest("POST", "/shenanigans", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.HandleCreate(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", w.Code)
	}

	var created models.Shananigan
	if err := json.NewDecoder(w.Body).Decode(&created); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if created.ID != 1 {
		t.Errorf("expected ID 1, got %d", created.ID)
	}
	if created.Name != "Meteor" {
		t.Errorf("expected name Meteor, got %s", created.Name)
	}
}

func TestHandleCreate_BadJSON(t *testing.T) {
	h := handlers.NewShenaniganHandler(newMockRepo(), nil)
	req := httptest.NewRequest("POST", "/shenanigans", bytes.NewReader([]byte("not json")))
	w := httptest.NewRecorder()

	h.HandleCreate(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for bad JSON, got %d", w.Code)
	}
}

func TestHandleCreate_WithCost(t *testing.T) {
	h := handlers.NewShenaniganHandler(newMockRepo(), nil)

	payload, _ := json.Marshal(map[string]interface{}{
		"name":         "Test",
		"description":  "Has cost",
		"rcon_payload": "t",
		"target_type":  "team",
		"cost":         100,
	})

	req := httptest.NewRequest("POST", "/shenanigans", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.HandleCreate(w, req)

	var created models.Shananigan
	if err := json.NewDecoder(w.Body).Decode(&created); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if created.Cost == nil || *created.Cost != 100 {
		t.Errorf("expected cost 100, got %v", created.Cost)
	}
}

func TestHandleCreate_WithoutCost(t *testing.T) {
	h := handlers.NewShenaniganHandler(newMockRepo(), nil)

	payload, _ := json.Marshal(map[string]string{
		"name":         "No Cost",
		"description":  "free for judges",
		"rcon_payload": "t",
		"target_type":  "team",
	})

	req := httptest.NewRequest("POST", "/shenanigans", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.HandleCreate(w, req)

	var created models.Shananigan
	if err := json.NewDecoder(w.Body).Decode(&created); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if created.Cost != nil {
		t.Errorf("expected nil cost when not provided, got %v", created.Cost)
	}
}

func TestHandleCreate_InvalidCost(t *testing.T) {
	h := handlers.NewShenaniganHandler(newMockRepo(), nil)

	payload, _ := json.Marshal(map[string]interface{}{
		"name":         "Bad Cost",
		"description":  "Invalid cost",
		"rcon_payload": "t",
		"target_type":  "team",
		"cost":         "not-a-number",
	})

	req := httptest.NewRequest("POST", "/shenanigans", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.HandleCreate(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid cost, got %d", w.Code)
	}
}

func TestHandleGet(t *testing.T) {
	repo := newMockRepo()
	_ = repo.Create(context.Background(), &models.Shananigan{Name: "Get Me", Description: "Find me", RconPayload: "find", TargetType: "team"})

	h := handlers.NewShenaniganHandler(repo, nil)
	req := httptest.NewRequest("GET", "/shenanigans/1", nil)
	req = withURLParams(req, "1")
	w := httptest.NewRecorder()

	h.HandleGet(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var got models.Shananigan
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Name != "Get Me" {
		t.Errorf("expected name Get Me, got %s", got.Name)
	}
}

func TestHandleGet_NotFound(t *testing.T) {
	h := handlers.NewShenaniganHandler(newMockRepo(), nil)
	req := httptest.NewRequest("GET", "/shenanigans/999", nil)
	req = withURLParams(req, "999")
	w := httptest.NewRecorder()

	h.HandleGet(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestHandleGet_InvalidID(t *testing.T) {
	h := handlers.NewShenaniganHandler(newMockRepo(), nil)
	req := httptest.NewRequest("GET", "/shenanigans/abc", nil)
	req = withURLParams(req, "abc")
	w := httptest.NewRecorder()

	h.HandleGet(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid ID, got %d", w.Code)
	}
}

func TestHandleUpdate(t *testing.T) {
	repo := newMockRepo()
	_ = repo.Create(context.Background(), &models.Shananigan{Name: "Before", Description: "Initial", RconPayload: "cmd", TargetType: "team"})

	h := handlers.NewShenaniganHandler(repo, nil)

	payload, _ := json.Marshal(map[string]string{
		"name":        "Updated",
		"description": "Modified",
	})

	req := httptest.NewRequest("PUT", "/shenanigans/1", bytes.NewReader(payload))
	req = withURLParams(req, "1")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.HandleUpdate(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var updated models.Shananigan
	if err := json.NewDecoder(w.Body).Decode(&updated); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if updated.Name != "Updated" {
		t.Errorf("expected name Updated, got %s", updated.Name)
	}
	if updated.Description != "Modified" {
		t.Errorf("expected desc Modified, got %s", updated.Description)
	}
	if updated.RconPayload != "cmd" {
		t.Errorf("expected unchanged rcon_payload cmd, got %s", updated.RconPayload)
	}
}

func TestHandleUpdate_NotFound(t *testing.T) {
	h := handlers.NewShenaniganHandler(newMockRepo(), nil)

	payload, _ := json.Marshal(map[string]string{"name": "Gone"})
	req := httptest.NewRequest("PUT", "/shenanigans/999", bytes.NewReader(payload))
	req = withURLParams(req, "999")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.HandleUpdate(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestHandleUpdate_InvalidID(t *testing.T) {
	h := handlers.NewShenaniganHandler(newMockRepo(), nil)

	payload, _ := json.Marshal(map[string]string{"name": "Nope"})
	req := httptest.NewRequest("PUT", "/shenanigans/abc", bytes.NewReader(payload))
	req = withURLParams(req, "abc")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.HandleUpdate(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid ID, got %d", w.Code)
	}
}

func TestHandleUpdate_EmptyBody(t *testing.T) {
	repo := newMockRepo()
	_ = repo.Create(context.Background(), &models.Shananigan{Name: "No Change", Description: "unchanged", RconPayload: "c", TargetType: "team"})

	h := handlers.NewShenaniganHandler(repo, nil)

	payload, _ := json.Marshal(map[string]interface{}{})
	req := httptest.NewRequest("PUT", "/shenanigans/1", bytes.NewReader(payload))
	req = withURLParams(req, "1")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.HandleUpdate(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for empty update, got %d", w.Code)
	}
}

func TestHandleDelete(t *testing.T) {
	repo := newMockRepo()
	_ = repo.Create(context.Background(), &models.Shananigan{Name: "Delete Me", Description: "Gone", RconPayload: "del", TargetType: "team"})

	h := handlers.NewShenaniganHandler(repo, nil)
	req := httptest.NewRequest("DELETE", "/shenanigans/1", nil)
	req = withURLParams(req, "1")
	w := httptest.NewRecorder()

	h.HandleDelete(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", w.Code)
	}

	// Verify it's gone
	_, err := repo.GetByID(context.Background(), 1)
	if err != repository.ErrNotFound {
		t.Error("expected record to be deleted")
	}
}

func TestHandleDelete_NotFound(t *testing.T) {
	h := handlers.NewShenaniganHandler(newMockRepo(), nil)
	req := httptest.NewRequest("DELETE", "/shenanigans/999", nil)
	req = withURLParams(req, "999")
	w := httptest.NewRecorder()

	h.HandleDelete(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestHandleDelete_InvalidID(t *testing.T) {
	h := handlers.NewShenaniganHandler(newMockRepo(), nil)
	req := httptest.NewRequest("DELETE", "/shenanigans/abc", nil)
	req = withURLParams(req, "abc")
	w := httptest.NewRecorder()

	h.HandleDelete(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid ID, got %d", w.Code)
	}
}

func TestHandleActivate_WithPublisher(t *testing.T) {
	repo := newMockRepo()
	_ = repo.Create(context.Background(), &models.Shananigan{Name: "Activate Me", Description: "Active", RconPayload: "say active", TargetType: "team"})

	pub := &mockPublisher{}
	h := handlers.NewShenaniganHandler(repo, pub)

	payload, _ := json.Marshal(map[string]string{"team": "red"})

	req := httptest.NewRequest("POST", "/shenanigans/1/activate", bytes.NewReader(payload))
	req = withURLParams(req, "1")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.HandleActivate(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if resp["published"] != true {
		t.Error("expected published=true")
	}
	if resp["status"] != "ok" {
		t.Errorf("expected status ok, got %v", resp["status"])
	}
	purchaseID, ok := resp["purchase_id"].(string)
	if !ok || purchaseID == "" {
		t.Error("expected non-empty purchase_id")
	}

	if pub.calls != 1 {
		t.Errorf("expected 1 publish call, got %d", pub.calls)
	}
	if pub.lastMsg.RconPayload != "say active" {
		t.Errorf("expected rcon_payload 'say active', got %s", pub.lastMsg.RconPayload)
	}
	if pub.lastMsg.ShenaniganID != "1" {
		t.Errorf("expected shenanigan_id 1, got %s", pub.lastMsg.ShenaniganID)
	}
}

func TestHandleActivate_WithoutPublisher(t *testing.T) {
	repo := newMockRepo()
	_ = repo.Create(context.Background(), &models.Shananigan{Name: "No MQ", Description: "Offline", RconPayload: "offline", TargetType: "team"})

	h := handlers.NewShenaniganHandler(repo, nil)

	payload, _ := json.Marshal(map[string]string{"team": "blue"})
	req := httptest.NewRequest("POST", "/shenanigans/1/activate", bytes.NewReader(payload))
	req = withURLParams(req, "1")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.HandleActivate(w, req)

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["published"] != false {
		t.Error("expected published=false when no publisher")
	}
}

func TestHandleActivate_PublishError(t *testing.T) {
	repo := newMockRepo()
	_ = repo.Create(context.Background(), &models.Shananigan{Name: "PB Fail", Description: "Error", RconPayload: "err", TargetType: "team"})

	pub := &mockPublisher{returnErr: fmt.Errorf("connection lost")}
	h := handlers.NewShenaniganHandler(repo, pub)

	payload, _ := json.Marshal(map[string]string{"team": "red"})
	req := httptest.NewRequest("POST", "/shenanigans/1/activate", bytes.NewReader(payload))
	req = withURLParams(req, "1")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.HandleActivate(w, req)

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["status"] != "error" {
		t.Errorf("expected status error on publish failure, got %v", resp["status"])
	}
	if resp["published"] != false {
		t.Errorf("expected published=false on error, got %v", resp["published"])
	}
}

func TestHandleActivate_NotFound(t *testing.T) {
	h := handlers.NewShenaniganHandler(newMockRepo(), nil)

	payload, _ := json.Marshal(map[string]string{"team": "red"})
	req := httptest.NewRequest("POST", "/shenanigans/999/activate", bytes.NewReader(payload))
	req = withURLParams(req, "999")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.HandleActivate(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestHandleActivate_InvalidID(t *testing.T) {
	h := handlers.NewShenaniganHandler(newMockRepo(), nil)

	payload, _ := json.Marshal(map[string]string{"team": "red"})
	req := httptest.NewRequest("POST", "/shenanigans/abc/activate", bytes.NewReader(payload))
	req = withURLParams(req, "abc")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.HandleActivate(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid ID, got %d", w.Code)
	}
}

func TestHandleActivate_BadJSON(t *testing.T) {
	h := handlers.NewShenaniganHandler(newMockRepo(), nil)
	req := httptest.NewRequest("POST", "/shenanigans/1/activate", bytes.NewReader([]byte("bad")))
	req = withURLParams(req, "1")
	w := httptest.NewRecorder()

	h.HandleActivate(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for bad JSON, got %d", w.Code)
	}
}

func TestHandleActivate_WithMetadata(t *testing.T) {
	repo := newMockRepo()
	_ = repo.Create(context.Background(), &models.Shananigan{Name: "Meta Active", Description: "Active", RconPayload: "meta", TargetType: "team"})

	pub := &mockPublisher{}
	h := handlers.NewShenaniganHandler(repo, pub)

	payload, _ := json.Marshal(map[string]interface{}{
		"team":     "red",
		"metadata": map[string]interface{}{"bonus": true, "round": 3},
	})

	req := httptest.NewRequest("POST", "/shenanigans/1/activate", bytes.NewReader(payload))
	req = withURLParams(req, "1")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.HandleActivate(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if pub.calls != 1 {
		t.Errorf("expected 1 call, got %d", pub.calls)
	}
}

func TestTestErrorResponses(t *testing.T) {
	h := handlers.NewShenaniganHandler(newMockRepo(), nil)

	// Bad ID returns 400
	req := httptest.NewRequest("GET", "/shenanigans/abc", nil)
	req = withURLParams(req, "abc")
	w := httptest.NewRecorder()
	h.HandleGet(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for bad ID, got %d", w.Code)
	}
}

func TestRegisterRoutes_NoPanic(t *testing.T) {
	repo := newMockRepo()
	h := handlers.NewShenaniganHandler(repo, nil)

	r := chi.NewRouter()
	handlers.RegisterRoutes(r, h)

	req := httptest.NewRequest("GET", "/shenanigans", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestRegisterOpenAPI_NoPanic(t *testing.T) {
	reg := openapi.NewRegistry()
	handlers.RegisterOpenAPI(reg)

	routes := reg.Routes()
	if len(routes) == 0 {
		t.Error("expected routes from RegisterOpenAPI")
	}

	expectedOps := map[string]bool{
		"listShenanigans":      false,
		"createShenanigan":     false,
		"getShenanigan":        false,
		"updateShenanigan":     false,
		"deleteShenanigan":     false,
		"activateShenanigan":   false,
	}

	for _, route := range routes {
		if _, ok := expectedOps[route.OperationID]; ok {
			expectedOps[route.OperationID] = true
		}
	}

	for opID, found := range expectedOps {
		if !found {
			t.Errorf("missing operationId %s", opID)
		}
	}
}
