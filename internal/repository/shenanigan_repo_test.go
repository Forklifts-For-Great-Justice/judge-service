package repository_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"github.com/forklifts-for-great-justice/judge-service/internal/models"
	"github.com/forklifts-for-great-justice/judge-service/internal/repository"
)

func setupDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	createTable := `
	CREATE TABLE shenanigans (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		description TEXT NOT NULL,
		rcon_payload TEXT NOT NULL,
		target_type TEXT NOT NULL,
		cost INTEGER,
		metadata TEXT,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL
	);`

	if _, err := db.Exec(createTable); err != nil {
		t.Fatalf("create table: %v", err)
	}
	return db
}

func TestCreate(t *testing.T) {
	db := setupDB(t)
	repo := repository.NewShananiganRepo(db)

	cost := int64(50)
	s := &models.Shananigan{
		Name:        "Fireball",
		Description: "Shoots a big fireball",
		RconPayload: "say fireball",
		TargetType:  "team",
		Cost:        &cost,
	}

	ctx := context.Background()
	if err := repo.Create(ctx, s); err != nil {
		t.Fatalf("create: %v", err)
	}
	if s.ID == 0 {
		t.Error("expected non-zero ID after create")
	}
	if s.CreatedAt.IsZero() {
		t.Error("expected non-zero CreatedAt")
	}
	if s.UpdatedAt.IsZero() {
		t.Error("expected non-zero UpdatedAt")
	}
}

func TestGetByID(t *testing.T) {
	db := setupDB(t)
	repo := repository.NewShananiganRepo(db)

	cost := int64(50)
	s := &models.Shananigan{
		Name:        "Thunder",
		Description: "Calls thunder",
		RconPayload: "say thunder",
		TargetType:  "all",
		Cost:        &cost,
	}
	if err := repo.Create(context.Background(), s); err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := repo.GetByID(context.Background(), s.ID)
	if err != nil {
		t.Fatalf("get by id: %v", err)
	}
	if got.ID != s.ID {
		t.Errorf("expected ID %d, got %d", s.ID, got.ID)
	}
	if got.Name != "Thunder" {
		t.Errorf("expected name Thunder, got %s", got.Name)
	}
	if got.RconPayload != "say thunder" {
		t.Errorf("expected rcon_payload say thunder, got %s", got.RconPayload)
	}
	if got.TargetType != "all" {
		t.Errorf("expected target_type all, got %s", got.TargetType)
	}
}

func TestGetByIDNotFound(t *testing.T) {
	db := setupDB(t)
	repo := repository.NewShananiganRepo(db)

	_, err := repo.GetByID(context.Background(), 9999)
	if err != repository.ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestGetAll(t *testing.T) {
	db := setupDB(t)
	repo := repository.NewShananiganRepo(db)

	s1 := &models.Shananigan{Name: "A", Description: "First", RconPayload: "a", TargetType: "team"}
	s2 := &models.Shananigan{Name: "B", Description: "Second", RconPayload: "b", TargetType: "all"}
	if err := repo.Create(context.Background(), s1); err != nil {
		t.Fatalf("create s1: %v", err)
	}
	if err := repo.Create(context.Background(), s2); err != nil {
		t.Fatalf("create s2: %v", err)
	}

	all, err := repo.GetAll(context.Background())
	if err != nil {
		t.Fatalf("get all: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2 shenanigans, got %d", len(all))
	}
	if all[0].Name != "A" {
		t.Errorf("expected first name A, got %s", all[0].Name)
	}
	if all[1].Name != "B" {
		t.Errorf("expected second name B, got %s", all[1].Name)
	}
}

func TestGetAllEmpty(t *testing.T) {
	db := setupDB(t)
	repo := repository.NewShananiganRepo(db)

	all, err := repo.GetAll(context.Background())
	if err != nil {
		t.Fatalf("get all: %v", err)
	}
	if len(all) != 0 {
		t.Errorf("expected 0 shenanigans, got %d", len(all))
	}
}

func TestUpdate(t *testing.T) {
	db := setupDB(t)
	repo := repository.NewShananiganRepo(db)

	s := &models.Shananigan{Name: "Before", Description: "Old", RconPayload: "old", TargetType: "team"}
	if err := repo.Create(context.Background(), s); err != nil {
		t.Fatalf("create: %v", err)
	}

	updates := map[string]interface{}{
		"name":        "After",
		"description": "New description",
	}
	if err := repo.Update(context.Background(), s.ID, updates); err != nil {
		t.Fatalf("update: %v", err)
	}

	got, err := repo.GetByID(context.Background(), s.ID)
	if err != nil {
		t.Fatalf("get after update: %v", err)
	}
	if got.Name != "After" {
		t.Errorf("expected updated name After, got %s", got.Name)
	}
	if got.Description != "New description" {
		t.Errorf("expected updated description, got %s", got.Description)
	}
	// Unchanged fields should remain the same
	if got.RconPayload != "old" {
		t.Errorf("expected unchanged rcon_payload old, got %s", got.RconPayload)
	}
	if got.TargetType != "team" {
		t.Errorf("expected unchanged target_type team, got %s", got.TargetType)
	}
}

func TestUpdateNotFound(t *testing.T) {
	db := setupDB(t)
	repo := repository.NewShananiganRepo(db)

	err := repo.Update(context.Background(), 9999, map[string]interface{}{"name": "nope"})
	if err != repository.ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestUpdateCost(t *testing.T) {
	db := setupDB(t)
	repo := repository.NewShananiganRepo(db)

	s := &models.Shananigan{Name: "Test", Description: "Test", RconPayload: "t", TargetType: "team"}
	if err := repo.Create(context.Background(), s); err != nil {
		t.Fatalf("create: %v", err)
	}

	newCost := int64(75)
	err := repo.Update(context.Background(), s.ID, map[string]interface{}{"cost": &newCost})
	if err != nil {
		t.Fatalf("update cost: %v", err)
	}

	got, err := repo.GetByID(context.Background(), s.ID)
	if err != nil {
		t.Fatalf("get after update: %v", err)
	}
	if got.Cost == nil || *got.Cost != 75 {
		t.Errorf("expected cost 75, got %v", got.Cost)
	}
}

func TestUpdateMetadata(t *testing.T) {
	db := setupDB(t)
	repo := repository.NewShananiganRepo(db)

	s := &models.Shananigan{Name: "Meta", Description: "Metadata test", RconPayload: "m", TargetType: "team"}
	if err := repo.Create(context.Background(), s); err != nil {
		t.Fatalf("create: %v", err)
	}

	meta := []byte(`{"bonus": true}`)
	err := repo.Update(context.Background(), s.ID, map[string]interface{}{"metadata": meta})
	if err != nil {
		t.Fatalf("update metadata: %v", err)
	}

	got, err := repo.GetByID(context.Background(), s.ID)
	if err != nil {
		t.Fatalf("get after update: %v", err)
	}
	var m1, m2 any
	json.Unmarshal(got.Metadata, &m1)
	json.Unmarshal([]byte(`{"bonus":true}`), &m2)
	b1, _ := json.Marshal(m1)
	b2, _ := json.Marshal(m2)
	if string(b1) != string(b2) {
		t.Errorf("expected metadata {\"bonus\":true}, got %s", got.Metadata)
	}
}

func TestDelete(t *testing.T) {
	db := setupDB(t)
	repo := repository.NewShananiganRepo(db)

	s := &models.Shananigan{Name: "ToDelete", Description: "Gone", RconPayload: "g", TargetType: "team"}
	if err := repo.Create(context.Background(), s); err != nil {
		t.Fatalf("create: %v", err)
	}

	if err := repo.Delete(context.Background(), s.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	_, err := repo.GetByID(context.Background(), s.ID)
	if err != repository.ErrNotFound {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestDeleteNotFound(t *testing.T) {
	db := setupDB(t)
	repo := repository.NewShananiganRepo(db)

	err := repo.Delete(context.Background(), 9999)
	if err != repository.ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestActivate_PreservesPayload(t *testing.T) {
	db := setupDB(t)
	repo := repository.NewShananiganRepo(db)

	s := &models.Shananigan{
		Name:        "Meteor",
		Description: "Calls meteor",
		RconPayload: "call meteor 5",
		TargetType:  "all",
	}
	if err := repo.Create(context.Background(), s); err != nil {
		t.Fatalf("create: %v", err)
	}

	record, err := repo.Activate(context.Background(), s.ID)
	if err != nil {
		t.Fatalf("activate: %v", err)
	}
	if record.RconPayload != "call meteor 5" {
		t.Errorf("expected rcon_payload 'call meteor 5', got %q", record.RconPayload)
	}
	// PurchaseID should be a valid UUID
	if record.PurchaseID.String() == "00000000-0000-0000-0000-000000000000" {
		t.Error("expected non-zero UUID")
	}
}

func TestActivate_NonExistent(t *testing.T) {
	db := setupDB(t)
	repo := repository.NewShananiganRepo(db)

	_, err := repo.Activate(context.Background(), 9999)
	if err == nil {
		t.Fatal("expected error for non-existent shenanigan")
	}
}

func TestCreate_VerifyRoundsTrip(t *testing.T) {
	db := setupDB(t)
	repo := repository.NewShananiganRepo(db)

	original := &models.Shananigan{
		Name:        "Complete Test",
		Description: "Full CRUD round-trip validation",
		RconPayload: "say complete",
		TargetType:  "all",
		Cost:        func() *int64 { i := int64(150); return &i }(),
		Metadata:    []byte(`{"cooldown":10,"rarity":"epic"}`),
	}

	if err := repo.Create(context.Background(), original); err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := repo.GetByID(context.Background(), original.ID)
	if err != nil {
		t.Fatalf("get by id: %v", err)
	}

	if got.Name != original.Name {
		t.Errorf("name: want %q, got %q", original.Name, got.Name)
	}
	if got.Description != original.Description {
		t.Errorf("description: want %q, got %q", original.Description, got.Description)
	}
	if got.RconPayload != original.RconPayload {
		t.Errorf("rcon_payload: want %q, got %q", original.RconPayload, got.RconPayload)
	}
	if got.TargetType != original.TargetType {
		t.Errorf("target_type: want %q, got %q", original.TargetType, got.TargetType)
	}
	if got.Cost == nil || *got.Cost != *original.Cost {
		t.Errorf("cost: want %d, got %v", *original.Cost, got.Cost)
	}
	if string(got.Metadata) != string(original.Metadata) {
		t.Errorf("metadata: want %q, got %q", original.Metadata, got.Metadata)
	}
}

func TestUpdateMultipleFields(t *testing.T) {
	db := setupDB(t)
	repo := repository.NewShananiganRepo(db)

	s := &models.Shananigan{Name: "Initial", Description: "Init desc", RconPayload: "init", TargetType: "team"}
	if err := repo.Create(context.Background(), s); err != nil {
		t.Fatalf("create: %v", err)
	}

	updates := map[string]interface{}{
		"name":                 "Updated",
		"description":          "Updated desc",
		"rcon_payload":         "updated CMD",
		"target_type":          "all",
	}
	if err := repo.Update(context.Background(), s.ID, updates); err != nil {
		t.Fatalf("update: %v", err)
	}

	got, err := repo.GetByID(context.Background(), s.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	if got.Name != "Updated" {
		t.Errorf("name: want Updated, got %s", got.Name)
	}
	if got.Description != "Updated desc" {
		t.Errorf("description: want Updated desc, got %s", got.Description)
	}
	if got.RconPayload != "updated CMD" {
		t.Errorf("rcon_payload: want updated CMD, got %s", got.RconPayload)
	}
	if got.TargetType != "all" {
		t.Errorf("target_type: want all, got %s", got.TargetType)
	}
}

func TestGetByID_VerifyTimeStamps(t *testing.T) {
	db := setupDB(t)
	repo := repository.NewShananiganRepo(db)

	s := &models.Shananigan{Name: "TS Test", Description: "ts", RconPayload: "t", TargetType: "team"}
	if err := repo.Create(context.Background(), s); err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := repo.GetByID(context.Background(), s.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	if got.CreatedAt.IsZero() {
		t.Error("CreatedAt should not be zero")
	}
	if got.UpdatedAt.IsZero() {
		t.Error("UpdatedAt should not be zero")
	}
}

func TestGetAll_Ordering(t *testing.T) {
	db := setupDB(t)
	repo := repository.NewShananiganRepo(db)

	names := []string{"C", "A", "B"}
	for _, name := range names {
		s := &models.Shananigan{
			Name:        name,
			Description: fmt.Sprintf("desc-%s", name),
			RconPayload: name,
			TargetType:  "team",
		}
		if err := repo.Create(context.Background(), s); err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
	}

	all, err := repo.GetAll(context.Background())
	if err != nil {
		t.Fatalf("get all: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("expected 3, got %d", len(all))
	}
	// Should be ordered by ID (creation order), not by name
	if all[0].Name != "C" {
		t.Errorf("expected first to be C (creation order), got %s", all[0].Name)
	}
}

func TestShananiganRepo_Activate_Returns_UUID(t *testing.T) {
	db := setupDB(t)
	repo := repository.NewShananiganRepo(db)

	s := &models.Shananigan{Name: "UUID Test", Description: "uuid", RconPayload: "u", TargetType: "team"}
	if err := repo.Create(context.Background(), s); err != nil {
		t.Fatalf("create: %v", err)
	}

	_, err := repo.Activate(context.Background(), s.ID)
	if err != nil {
		t.Fatalf("activate: %v", err)
	}
	// Verify no duplicate UUIDs are generated
	record1, _ := repo.Activate(context.Background(), s.ID)
	record2, _ := repo.Activate(context.Background(), s.ID)
	if record1.PurchaseID == record2.PurchaseID {
		t.Error("expected different UUIDs for successive activations")
	}
}
