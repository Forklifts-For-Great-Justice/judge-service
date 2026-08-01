package models_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/forklifts-for-great-justice/judge-service/internal/models"
)

func TestShananigan_JSONMarshalBasic(t *testing.T) {
	cost := int64(100)
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	s := models.Shananigan{
		ID:          1,
		Name:        "Fireball",
		Description: "Shoots fireballs",
		RconPayload: "say hello",
		TargetType:  "team",
		Cost:        &cost,
		Metadata:    json.RawMessage(`{"key":"value"}`),
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	data, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	if result["id"].(float64) != 1 {
		t.Errorf("expected id 1, got %v", result["id"])
	}
	if result["name"] != "Fireball" {
		t.Errorf("expected name Fireball, got %v", result["name"])
	}
	if result["cost"].(float64) != 100 {
		t.Errorf("expected cost 100, got %v", result["cost"])
	}
}

func TestShananigan_JSONMarshalOmitEmpty(t *testing.T) {
	cost := int64(0)
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	s := models.Shananigan{
		ID:          42,
		Name:        "Thunder",
		Description: "Summons thunder",
		RconPayload: "say boom",
		TargetType:  "all",
		Cost:        &cost,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	data, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// cost=0 is not nil, should be present (only nil pointer is omitted)
	if _, ok := result["cost"]; !ok {
		t.Error("cost=0 should be present (only nil pointer is omitted)")
	}

	// metadata is nil, should be omitted
	if _, ok := result["metadata"]; ok {
		t.Error("nil metadata should be omitted with omitempty")
	}
}

func TestShananigan_JSONMarshalMetadata(t *testing.T) {
	s := models.Shananigan{
		ID:      1,
		Name:    "Test",
		CreatedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		Metadata:  json.RawMessage(`{"custom":"data"}`),
	}

	data, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	meta, ok := result["metadata"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected metadata to be object, got %T", result["metadata"])
	}
	if meta["custom"] != "data" {
		t.Errorf("expected metadata.custom=data, got %v", meta["custom"])
	}
}

func TestShananigan_JSONUnmarshal(t *testing.T) {
	data := []byte(`{
		"id": 5,
		"name": "Ice Wall",
		"description": "Builds an ice wall",
		"rcon_payload": "say ice",
		"target_type": "team",
		"cost": 250,
		"metadata": {"cooldown": 30},
		"created_at": "2025-06-15T10:30:00Z",
		"updated_at": "2025-06-16T08:00:00Z"
	}`)

	var s models.Shananigan
	if err := json.Unmarshal(data, &s); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if s.ID != 5 {
		t.Errorf("expected ID 5, got %d", s.ID)
	}
	if s.Name != "Ice Wall" {
		t.Errorf("expected name Ice Wall, got %s", s.Name)
	}
	if s.RconPayload != "say ice" {
		t.Errorf("expected rcon_payload say ice, got %s", s.RconPayload)
	}
	if s.Cost == nil || *s.Cost != 250 {
		t.Errorf("expected cost 250, got %v", s.Cost)
	}
	if m1, m2 := string(s.Metadata), string(json.RawMessage(`{"cooldown":30}`)); m1 != m2 {
		// Normalize by re-marshaling
		var v1, v2 interface{}
		json.Unmarshal([]byte(m1), &v1)
		json.Unmarshal([]byte(m2), &v2)
		b1, _ := json.Marshal(v1)
		b2, _ := json.Marshal(v2)
		if string(b1) != string(b2) {
			t.Errorf("expected metadata {\"cooldown\":30}, got %s", s.Metadata)
		}
	}
}

func TestShananigan_JSONUnmarshalOmitEmptyFields(t *testing.T) {
	data := []byte(`{
		"id": 10,
		"name": "Simple",
		"description": "A simple shenanigan",
		"rcon_payload": "simple",
		"target_type": "all",
		"created_at": "2025-01-01T00:00:00Z",
		"updated_at": "2025-01-01T00:00:00Z"
	}`)

	var s models.Shananigan
	if err := json.Unmarshal(data, &s); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if s.Cost != nil {
		t.Errorf("expected nil cost for missing field, got %v", s.Cost)
	}
	if s.Metadata != nil {
		t.Errorf("expected nil metadata for missing field, got %v", s.Metadata)
	}
}
