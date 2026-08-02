package repository_test

import (
	"context"
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"github.com/forklifts-for-great-justice/judge-service/internal/models"
	"github.com/forklifts-for-great-justice/judge-service/internal/repository"
)

func setupTeamDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	createTable := `
	CREATE TABLE team (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		slug TEXT NOT NULL UNIQUE,
		name TEXT NOT NULL,
		alt_name TEXT NOT NULL,
		clan_tag TEXT NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL
	);`

	if _, err := db.Exec(createTable); err != nil {
		t.Fatalf("create table: %v", err)
	}
	return db
}

func TestTeamCreate(t *testing.T) {
	db := setupTeamDB(t)
	repo := repository.NewTeamRepo(db)

	tm := &models.Team{
		Slug:    "dragon-squad",
		Name:    "Dragon Squad",
		AltName: "DragonSquad",
		ClanTag: "[DMN]",
	}

	ctx := context.Background()
	if err := repo.Create(ctx, tm); err != nil {
		t.Fatalf("create: %v", err)
	}
	if tm.ID == 0 {
		t.Error("expected non-zero ID after create")
	}
	if tm.CreatedAt.IsZero() {
		t.Error("expected non-zero CreatedAt")
	}
	if tm.UpdatedAt.IsZero() {
		t.Error("expected non-zero UpdatedAt")
	}
}

func TestTeamGetByID(t *testing.T) {
	db := setupTeamDB(t)
	repo := repository.NewTeamRepo(db)

	tm := &models.Team{
		Slug:    "thunder-bolt",
		Name:    "Thunder Bolt",
		AltName: "ThunderBolt",
		ClanTag: "[THD]",
	}
	if err := repo.Create(context.Background(), tm); err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := repo.GetByID(context.Background(), tm.ID)
	if err != nil {
		t.Fatalf("get by id: %v", err)
	}
	if got.ID != tm.ID {
		t.Errorf("expected ID %d, got %d", tm.ID, got.ID)
	}
	if got.Name != "Thunder Bolt" {
		t.Errorf("expected name Thunder Bolt, got %s", got.Name)
	}
	if got.Slug != "thunder-bolt" {
		t.Errorf("expected slug thunder-bolt, got %s", got.Slug)
	}
}

func TestTeamGetByIDNotFound(t *testing.T) {
	db := setupTeamDB(t)
	repo := repository.NewTeamRepo(db)

	_, err := repo.GetByID(context.Background(), 9999)
	if err != repository.ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestTeamGetAll(t *testing.T) {
	db := setupTeamDB(t)
	repo := repository.NewTeamRepo(db)

	t1 := &models.Team{Slug: "alpha", Name: "Alpha", AltName: "AlphaTeam", ClanTag: "[A]"}
	t2 := &models.Team{Slug: "beta", Name: "Beta", AltName: "BetaTeam", ClanTag: "[B]"}
	if err := repo.Create(context.Background(), t1); err != nil {
		t.Fatalf("create t1: %v", err)
	}
	if err := repo.Create(context.Background(), t2); err != nil {
		t.Fatalf("create t2: %v", err)
	}

	all, err := repo.GetAll(context.Background())
	if err != nil {
		t.Fatalf("get all: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2 teams, got %d", len(all))
	}
	if all[0].Slug != "alpha" {
		t.Errorf("expected first slug alpha, got %s", all[0].Slug)
	}
	if all[1].Slug != "beta" {
		t.Errorf("expected second slug beta, got %s", all[1].Slug)
	}
}

func TestTeamGetAllEmpty(t *testing.T) {
	db := setupTeamDB(t)
	repo := repository.NewTeamRepo(db)

	all, err := repo.GetAll(context.Background())
	if err != nil {
		t.Fatalf("get all: %v", err)
	}
	if len(all) != 0 {
		t.Errorf("expected 0 teams, got %d", len(all))
	}
}

func TestTeamUpdate(t *testing.T) {
	db := setupTeamDB(t)
	repo := repository.NewTeamRepo(db)

	tm := &models.Team{Slug: "update-me", Name: "Before", AltName: "BeforeTeam", ClanTag: "[U]"}
	if err := repo.Create(context.Background(), tm); err != nil {
		t.Fatalf("create: %v", err)
	}

	updates := map[string]interface{}{
		"name": "After",
	}
	if err := repo.Update(context.Background(), tm.ID, updates); err != nil {
		t.Fatalf("update: %v", err)
	}

	got, err := repo.GetByID(context.Background(), tm.ID)
	if err != nil {
		t.Fatalf("get after update: %v", err)
	}
	if got.Name != "After" {
		t.Errorf("expected updated name After, got %s", got.Name)
	}
	// Unchanged fields should remain the same
	if got.Slug != "update-me" {
		t.Errorf("expected unchanged slug update-me, got %s", got.Slug)
	}
	if got.ClanTag != "[U]" {
		t.Errorf("expected unchanged clan_tag [U], got %s", got.ClanTag)
	}
}

func TestTeamDelete(t *testing.T) {
	db := setupTeamDB(t)
	repo := repository.NewTeamRepo(db)

	tm := &models.Team{Slug: "delete-me", Name: "ToDelete", AltName: "DelTeam", ClanTag: "[D]"}
	if err := repo.Create(context.Background(), tm); err != nil {
		t.Fatalf("create: %v", err)
	}

	if err := repo.Delete(context.Background(), tm.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	_, err := repo.GetByID(context.Background(), tm.ID)
	if err != repository.ErrNotFound {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestTeamDeleteNotFound(t *testing.T) {
	db := setupTeamDB(t)
	repo := repository.NewTeamRepo(db)

	err := repo.Delete(context.Background(), 9999)
	if err != repository.ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
