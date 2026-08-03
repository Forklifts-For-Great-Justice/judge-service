package repository_test

import (
	"context"
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"github.com/forklifts-for-great-justice/judge-service/internal/models"
	"github.com/forklifts-for-great-justice/judge-service/internal/repository"
)

func setupChallengeDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	createTable := `
	CREATE TABLE challenge (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL UNIQUE,
		description TEXT NOT NULL,
		challenge_type TEXT,
		location TEXT,
		points INTEGER NOT NULL DEFAULT 50,
		disabled BOOLEAN NOT NULL DEFAULT FALSE,
		flag TEXT NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL
	);`

	if _, err := db.Exec(createTable); err != nil {
		t.Fatalf("create table: %v", err)
	}
	return db
}

func TestChallengeCreate(t *testing.T) {
	db := setupChallengeDB(t)
	repo := repository.NewChallengeRepo(db)

	c := &models.Challenge{
		Name:        "Test Challenge",
		Description: "A test",
		Points:      100,
		Flag:        "secret",
	}

	ctx := context.Background()
	if err := repo.Create(ctx, c); err != nil {
		t.Fatalf("create: %v", err)
	}
	if c.ID == 0 {
		t.Error("expected non-zero ID")
	}
	if c.CreatedAt.IsZero() {
		t.Error("expected non-zero CreatedAt")
	}
	if c.UpdatedAt.IsZero() {
		t.Error("expected non-zero UpdatedAt")
	}
	if c.Disabled {
		t.Error("expected Disabled to be false (default)")
	}
}

func TestChallengeGetByID(t *testing.T) {
	db := setupChallengeDB(t)
	repo := repository.NewChallengeRepo(db)

	c := &models.Challenge{
		Name:        "Test Challenge",
		Description: "A test",
		Points:      100,
		Flag:        "secret",
	}
	if err := repo.Create(context.Background(), c); err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := repo.GetByID(context.Background(), c.ID)
	if err != nil {
		t.Fatalf("get by id: %v", err)
	}
	if got.Name != "Test Challenge" {
		t.Errorf("expected name 'Test Challenge', got %s", got.Name)
	}
	if got.Description != "A test" {
		t.Errorf("expected description 'A test', got %s", got.Description)
	}
	if got.Points != 100 {
		t.Errorf("expected points 100, got %d", got.Points)
	}
	if got.Flag != "secret" {
		t.Errorf("expected flag 'secret', got %s", got.Flag)
	}
}

func TestChallengeGetByIDNotFound(t *testing.T) {
	db := setupChallengeDB(t)
	repo := repository.NewChallengeRepo(db)

	_, err := repo.GetByID(context.Background(), 9999)
	if err != repository.ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestChallengeGetAll(t *testing.T) {
	db := setupChallengeDB(t)
	repo := repository.NewChallengeRepo(db)

	c1 := &models.Challenge{Name: "C1", Description: "D1", Points: 50, Flag: "f1"}
	c2 := &models.Challenge{Name: "C2", Description: "D2", Points: 100, Flag: "f2"}
	if err := repo.Create(context.Background(), c1); err != nil {
		t.Fatalf("create c1: %v", err)
	}
	if err := repo.Create(context.Background(), c2); err != nil {
		t.Fatalf("create c2: %v", err)
	}

	all, err := repo.GetAll(context.Background())
	if err != nil {
		t.Fatalf("get all: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2 challenges, got %d", len(all))
	}
	if all[0].Name != "C1" {
		t.Errorf("expected first name C1, got %s", all[0].Name)
	}
	if all[1].Name != "C2" {
		t.Errorf("expected second name C2, got %s", all[1].Name)
	}
}

func TestChallengeGetAllEmpty(t *testing.T) {
	db := setupChallengeDB(t)
	repo := repository.NewChallengeRepo(db)

	all, err := repo.GetAll(context.Background())
	if err != nil {
		t.Fatalf("get all: %v", err)
	}
	if len(all) != 0 {
		t.Errorf("expected 0 challenges, got %d", len(all))
	}
}

func TestChallengeUpdate(t *testing.T) {
	db := setupChallengeDB(t)
	repo := repository.NewChallengeRepo(db)

	c := &models.Challenge{Name: "Orig", Description: "Desc", Points: 50, Flag: "f"}
	if err := repo.Create(context.Background(), c); err != nil {
		t.Fatalf("create: %v", err)
	}

	updates := map[string]any{
		"name": "Updated",
	}
	if err := repo.Update(context.Background(), c.ID, updates); err != nil {
		t.Fatalf("update: %v", err)
	}

	got, err := repo.GetByID(context.Background(), c.ID)
	if err != nil {
		t.Fatalf("get after update: %v", err)
	}
	if got.Name != "Updated" {
		t.Errorf("expected name 'Updated', got %s", got.Name)
	}
	if got.Description != "Desc" {
		t.Errorf("expected description preserved, got %s", got.Description)
	}
}

func TestChallengeUpdateDisable(t *testing.T) {
	db := setupChallengeDB(t)
	repo := repository.NewChallengeRepo(db)

	c := &models.Challenge{Name: "Test", Description: "Desc", Points: 50, Flag: "f"}
	if err := repo.Create(context.Background(), c); err != nil {
		t.Fatalf("create: %v", err)
	}

	updates := map[string]any{
		"disabled": true,
	}
	if err := repo.Update(context.Background(), c.ID, updates); err != nil {
		t.Fatalf("update: %v", err)
	}

	got, err := repo.GetByID(context.Background(), c.ID)
	if err != nil {
		t.Fatalf("get after update: %v", err)
	}
	if !got.Disabled {
		t.Errorf("expected disabled true, got %v", got.Disabled)
	}
}

func TestChallengeSoftDelete(t *testing.T) {
	db := setupChallengeDB(t)
	repo := repository.NewChallengeRepo(db)

	c := &models.Challenge{Name: "Test", Description: "Desc", Points: 50, Flag: "f"}
	if err := repo.Create(context.Background(), c); err != nil {
		t.Fatalf("create: %v", err)
	}

	if err := repo.SoftDelete(context.Background(), c.ID); err != nil {
		t.Fatalf("soft delete: %v", err)
	}

	// Verify disabled flag was set (not verified via GetByID since it's a CRUD read)
	var disabled bool
	err := db.QueryRowContext(context.Background(),
		"SELECT disabled FROM challenge WHERE id = $1", c.ID).Scan(&disabled)
	if err != nil {
		t.Fatalf("query disabled: %v", err)
	}
	if !disabled {
		t.Errorf("expected disabled to be true after soft delete, got %v", disabled)
	}
}

func TestChallengeSoftDeleteNotFound(t *testing.T) {
	db := setupChallengeDB(t)
	repo := repository.NewChallengeRepo(db)

	err := repo.SoftDelete(context.Background(), 9999)
	if err != repository.ErrChallengeNotFound {
		t.Fatalf("expected ErrChallengeNotFound, got %v", err)
	}
}

func TestChallengeCreateDefaultDisabled(t *testing.T) {
	db := setupChallengeDB(t)
	repo := repository.NewChallengeRepo(db)

	c := &models.Challenge{
		Name:        "Test",
		Description: "Desc",
		Points:      75,
		Flag:        "flag123",
	}
	if err := repo.Create(context.Background(), c); err != nil {
		t.Fatalf("create: %v", err)
	}
	if c.Disabled != false {
		t.Errorf("expected Disabled false, got %v", c.Disabled)
	}
}
