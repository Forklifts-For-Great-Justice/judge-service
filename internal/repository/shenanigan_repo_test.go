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
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
		deleted_at TIMESTAMP
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

func TestGetFiltered_ByTargetType(t *testing.T) {
	db := setupDB(t)
	repo := repository.NewShananiganRepo(db)
	ctx := context.Background()

	_ = repo.Create(ctx, &models.Shananigan{Name: "A", Description: "x", RconPayload: "a", TargetType: "team"})
	_ = repo.Create(ctx, &models.Shananigan{Name: "B", Description: "y", RconPayload: "b", TargetType: "all"})
	_ = repo.Create(ctx, &models.Shananigan{Name: "C", Description: "z", RconPayload: "c", TargetType: "team"})

	allT, totalT, err := repo.GetFiltered(ctx, &repository.FilterOptions{TargetType: "team"})
	if err != nil {
		t.Fatalf("getFiltered team: %v", err)
	}
	if totalT != 2 {
		t.Errorf("total: want 2, got %d", totalT)
	}
	if len(allT) != 2 {
		t.Fatalf("team results: want 2, got %d", len(allT))
	}
	for _, s := range allT {
		if s.TargetType != "team" {
			t.Errorf("expected team, got %s", s.TargetType)
		}
	}

	allAll, _, _ := repo.GetFiltered(ctx, &repository.FilterOptions{TargetType: "all"})
	if len(allAll) != 1 {
		t.Errorf("all results: want 1, got %d", len(allAll))
	}
}

func TestGetFiltered_ByCostRange(t *testing.T) {
	db := setupDB(t)
	repo := repository.NewShananiganRepo(db)
	ctx := context.Background()

	c1 := int64(10)
	_ = repo.Create(ctx, &models.Shananigan{Name: "A", Description: "x", RconPayload: "a", TargetType: "team", Cost: &c1})
	c2 := int64(50)
	_ = repo.Create(ctx, &models.Shananigan{Name: "B", Description: "y", RconPayload: "b", TargetType: "all", Cost: &c2})
	c3 := int64(100)
	_ = repo.Create(ctx, &models.Shananigan{Name: "C", Description: "z", RconPayload: "c", TargetType: "team", Cost: &c3})

	filtered, total, err := repo.GetFiltered(ctx, &repository.FilterOptions{MinCost: &c2, MaxCost: &c3})
	if err != nil {
		t.Fatalf("getFiltered cost range: %v", err)
	}
	if total != 2 {
		t.Errorf("total: want 2, got %d", total)
	}
	if len(filtered) != 2 {
		t.Fatalf("cost range results: want 2, got %d", len(filtered))
	}
	for _, s := range filtered {
		if *s.Cost < 50 || *s.Cost > 100 {
			t.Errorf("cost %d not in range [50,100]", *s.Cost)
		}
	}
}

func filterPaginated(db *sql.DB, repo *repository.ShananiganRepo, ctx context.Context, targetType string, page, pageSize int) ([]*models.Shananigan, int64, error) {
	return repo.GetFiltered(ctx, &repository.FilterOptions{
		TargetType: targetType,
		Page:       page,
		PageSize:   pageSize,
	})
}

func TestGetFiltered_PaginationFirstPage(t *testing.T) {
	db := setupDB(t)
	repo := repository.NewShananiganRepo(db)
	ctx := context.Background()

	cost := int64(100)
	for i := 0; i < 7; i++ {
		_ = repo.Create(ctx, &models.Shananigan{
			Name:        fmt.Sprintf("T%d", i),
			Description: "x",
			RconPayload: fmt.Sprintf("r%d", i),
			TargetType:  "team",
			Cost:        &cost,
		})
	}

	p1, total, _ := filterPaginated(db, repo, ctx, "team", 1, 3)
	if total != 7 {
		t.Errorf("total: want 7, got %d", total)
	}
	if len(p1) != 3 {
		t.Errorf("page 1 results: want 3, got %d", len(p1))
	}
}

func TestGetFiltered_PaginationLastPage(t *testing.T) {
	db := setupDB(t)
	repo := repository.NewShananiganRepo(db)
	ctx := context.Background()

	cost := int64(100)
	for i := 0; i < 7; i++ {
		_ = repo.Create(ctx, &models.Shananigan{
			Name:        fmt.Sprintf("T%d", i),
			Description: "x",
			RconPayload: fmt.Sprintf("r%d", i),
			TargetType:  "team",
			Cost:        &cost,
		})
	}

	p3, total, _ := filterPaginated(db, repo, ctx, "team", 3, 3)
	if len(p3) != 1 {
		t.Errorf("page 3 (last) results: want 1, got %d", len(p3))
	}
	if total != 7 {
		t.Errorf("total: want 7, got %d", total)
	}
}

func TestGetFiltered_NoFilterReturnsAll(t *testing.T) {
	db := setupDB(t)
	repo := repository.NewShananiganRepo(db)
	ctx := context.Background()

	for i := 0; i < 10; i++ {
		_ = repo.Create(ctx, &models.Shananigan{
			Name:        fmt.Sprintf("S%d", i),
			Description: "x",
			RconPayload: fmt.Sprintf("r%d", i),
			TargetType:  "team",
		})
	}

	results, total, _ := repo.GetFiltered(ctx, &repository.FilterOptions{})
	if total != 10 {
		t.Errorf("total: want 10, got %d", total)
	}
	if len(results) != 10 {
		t.Errorf("all results: want 10, got %d", len(results))
	}
}

func TestGetFiltered_DefaultPageSize(t *testing.T) {
	db := setupDB(t)
	repo := repository.NewShananiganRepo(db)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		_ = repo.Create(ctx, &models.Shananigan{
			Name:        fmt.Sprintf("D%d", i),
			Description: "x",
			RconPayload: fmt.Sprintf("r%d", i),
			TargetType:  "team",
		})
	}

	withPageSize, _, _ := repo.GetFiltered(ctx, &repository.FilterOptions{Page: 1, PageSize: 5})
	if len(withPageSize) != 5 {
		t.Errorf("with size: want 5, got %d", len(withPageSize))
	}

	withoutPageSize, _, _ := repo.GetFiltered(ctx, &repository.FilterOptions{Page: 1})
	if len(withoutPageSize) != 5 {
		t.Errorf("default size should return all 5: got %d", len(withoutPageSize))
	}

	invalidPageSize, _, _ := repo.GetFiltered(ctx, &repository.FilterOptions{Page: 1, PageSize: 0})
	if len(invalidPageSize) != 5 {
		t.Errorf("invalid page size should default to 50: got %d", len(invalidPageSize))
	}
}

func TestGetFiltered_PaginationPageSizeExceedsTotal(t *testing.T) {
	db := setupDB(t)
	repo := repository.NewShananiganRepo(db)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		_ = repo.Create(ctx, &models.Shananigan{
			Name:        fmt.Sprintf("E%d", i),
			Description: "x",
			RconPayload: fmt.Sprintf("r%d", i),
			TargetType:  "team",
		})
	}

	result, total, _ := filterPaginated(db, repo, ctx, "team", 1, 100)
	if len(result) != 3 {
		t.Errorf("page size 100 with 3 items: want 3, got %d", len(result))
	}
	if total != 3 {
		t.Errorf("total: want 3, got %d", total)
	}
}

func TestGetFiltered_MaxPageSizeClamped(t *testing.T) {
	db := setupDB(t)
	repo := repository.NewShananiganRepo(db)
	ctx := context.Background()

	for i := 0; i < 250; i++ {
		_ = repo.Create(ctx, &models.Shananigan{
			Name:        fmt.Sprintf("M%d", i),
			Description: "x",
			RconPayload: fmt.Sprintf("r%d", i),
			TargetType:  "team",
		})
	}

	_, total, _ := repo.GetFiltered(ctx, &repository.FilterOptions{Page: 1, PageSize: 500})
	if total != 250 {
		t.Errorf("total with 250 items: want 250, got %d", total)
	}

	p1, _, _ := repo.GetFiltered(ctx, &repository.FilterOptions{Page: 1, PageSize: 500})
	if len(p1) > 200 {
		t.Errorf("page_size 500 should be clamped to 200: got %d items", len(p1))
	}
	if len(p1) != 200 {
		t.Errorf("page_size 500 clamped to 200, should return 200 items: got %d", len(p1))
	}

	p2, _, _ := repo.GetFiltered(ctx, &repository.FilterOptions{Page: 2, PageSize: 500})
	if totalP2 := len(p2); totalP2 != 50 {
		// 250 total, page_size clamped to 200, page 2 starts at offset 200
		t.Errorf("page 2 should return remaining 50: got %d", totalP2)
	}
}

func TestGetFiltered_OrderByID(t *testing.T) {
	db := setupDB(t)
	repo := repository.NewShananiganRepo(db)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		_ = repo.Create(ctx, &models.Shananigan{
			Name:        fmt.Sprintf("X%d", 4-i),
			Description: "x",
			RconPayload: fmt.Sprintf("r%d", 4-i),
			TargetType:  "team",
		})
	}

	results, _, _ := repo.GetFiltered(ctx, &repository.FilterOptions{})
	if len(results) != 5 {
		t.Fatalf("want 5 results, got %d", len(results))
	}
	if results[0].Name != "X4" {
		t.Errorf("first item should be X4 (creation order), got %s", results[0].Name)
	}
	if results[4].Name != "X0" {
		t.Errorf("last item should be X0 (creation order), got %s", results[4].Name)
	}
}

// ===== Phase 1C soft-delete tests =====

func TestSoftDelete_HidesFromQueries(t *testing.T) {
	db := setupDB(t)
	repo := repository.NewShananiganRepo(db)
	ctx := context.Background()

	// Create two records
	_ = repo.Create(ctx, &models.Shananigan{Name: "Hidden", Description: "gone", RconPayload: "h", TargetType: "team"})
	_ = repo.Create(ctx, &models.Shananigan{Name: "Visible", Description: "stays", RconPayload: "v", TargetType: "team"})

	// Soft-delete record 1
	if err := repo.SoftDelete(ctx, 1); err != nil {
		t.Fatalf("soft delete: %v", err)
	}

	// Verify GetByID returns ErrNotFound for deleted record
	_, err := repo.GetByID(ctx, 1)
	if err != repository.ErrNotFound {
		t.Fatalf("expected ErrNotFound after soft delete, got %v", err)
	}

	// Verify GetByID still works for non-deleted record
	v, err := repo.GetByID(ctx, 2)
	if err != nil {
		t.Fatalf("GetByID for non-deleted record: %v", err)
	}
	if v.Name != "Visible" {
		t.Errorf("expected 'Visible', got %s", v.Name)
	}

	// Verify GetAll excludes soft-deleted
	all, err := repo.GetAll(ctx)
	if err != nil {
		t.Fatalf("get all: %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("expected 1 non-deleted, got %d", len(all))
	}
	if all[0].Name != "Visible" {
		t.Errorf("expected 'Visible', got %s", all[0].Name)
	}

	// Verify GetFiltered excludes soft-deleted
	results, total, err := repo.GetFiltered(ctx, &repository.FilterOptions{})
	if err != nil {
		t.Fatalf("get filtered: %v", err)
	}
	if total != 1 {
		t.Errorf("total: want 1, got %d", total)
	}
	if len(results) != 1 {
		t.Fatalf("filtered: want 1, got %d", len(results))
	}
}

func TestSoftDelete_SqliteCompatibility(t *testing.T) {
	db := setupDB(t)
	repo := repository.NewShananiganRepo(db)
	ctx := context.Background()

	// Insert directly (bypass repo Create) to test DELETE with CURRENT_TIMESTAMP
	// This verifies the SoftDelete SQL works in SQLite
	_, err := db.ExecContext(ctx,
		`INSERT INTO shenanigans (name, description, rcon_payload, target_type, cost, metadata) VALUES (?, ?, ?, ?, ?, ?)`,
		"DbDirect", "desc", "db", "all", 10, "{}")

	if err != nil {
		t.Fatalf("direct insert: %v", err)
	}

	// Verify SoftDelete works with SQLite
	if err := repo.SoftDelete(ctx, 1); err != nil {
		t.Fatalf("soft delete on sqlite: %v", err)
	}

	// Verify SoftDelete on already-soft-deleted returns ErrNotFound
	if err := repo.SoftDelete(ctx, 1); err != repository.ErrNotFound {
		t.Fatalf("re-soft-delete expected ErrNotFound, got %v", err)
	}
}

func TestSoftDelete_DuplicateSoftDelete(t *testing.T) {
	db := setupDB(t)
	repo := repository.NewShananiganRepo(db)
	ctx := context.Background()

	s := &models.Shananigan{Name: "Duplicate", Description: "test", RconPayload: "t", TargetType: "team"}
	if err := repo.Create(ctx, s); err != nil {
		t.Fatalf("create: %v", err)
	}

	// First soft-delete succeeds
	if err := repo.SoftDelete(ctx, s.ID); err != nil {
		t.Fatalf("first soft delete: %v", err)
	}

	// Second soft-delete returns ErrNotFound (idempotent constraint)
	if err := repo.SoftDelete(ctx, s.ID); err != repository.ErrNotFound {
		t.Fatalf("expected ErrNotFound on re-delete, got %v", err)
	}

	// Verify GetByID still returns ErrNotFound (record is still filtered)
	_, err := repo.GetByID(ctx, s.ID)
	if err != repository.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}
