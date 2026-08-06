package repository_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/forklifts-for-great-justice/judge-service/internal/models"
	"github.com/forklifts-for-great-justice/judge-service/internal/repository"
	s3 "github.com/mattn/go-sqlite3"
)

// registerNowFunc registers a NOW() SQLite function so that
// the PostgreSQL-style NOW() in round_repo.go works.
func registerNowFunc(db *sql.DB) error {
	ctx := context.Background()
	conn, err := db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	return conn.Raw(func(driverConn any) error {
		c := driverConn.(*s3.SQLiteConn)
		return c.RegisterFunc("NOW", func() string {
			return time.Now().Format(time.RFC3339)
		}, true)
	})
}

func setupRoundDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	// Register NOW() function — the actual round_repo.go uses PostgreSQL NOW()
	// which SQLite doesn't have natively. Register it as a pure function.
	if err := registerNowFunc(db); err != nil {
		t.Fatalf("register NOW(): %v", err)
	}

	createTable := `
	CREATE TABLE IF NOT EXISTS team (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		slug TEXT NOT NULL,
		name TEXT NOT NULL,
		alt_name TEXT NOT NULL,
		clan_tag TEXT NOT NULL
	);

	CREATE TABLE IF NOT EXISTS matches (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		round_name TEXT NOT NULL,
		team_a_id INTEGER NOT NULL,
		team_b_id INTEGER NOT NULL,
		team_a_points INTEGER NOT NULL DEFAULT 0,
		team_b_points INTEGER NOT NULL DEFAULT 0,
		team_a_hack_points INTEGER NOT NULL DEFAULT 0,
		team_b_hack_points INTEGER NOT NULL DEFAULT 0,
		team_a_hackcoins INTEGER NOT NULL DEFAULT 0,
		team_b_hackcoins INTEGER NOT NULL DEFAULT 0,
		status TEXT NOT NULL DEFAULT 'scheduled',
		ready BOOLEAN NOT NULL DEFAULT FALSE,
		live BOOLEAN NOT NULL DEFAULT FALSE,
		disabled BOOLEAN NOT NULL DEFAULT FALSE,
		ready_at TIMESTAMP,
		live_at TIMESTAMP,
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS current_match (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		match_id INTEGER,
		team_a_id INTEGER NOT NULL,
		team_b_id INTEGER NOT NULL,
		round_name TEXT NOT NULL DEFAULT '',
		team_a_points INTEGER NOT NULL DEFAULT 0,
		team_b_points INTEGER NOT NULL DEFAULT 0,
		team_a_hack_points INTEGER NOT NULL DEFAULT 0,
		team_b_hack_points INTEGER NOT NULL DEFAULT 0,
		team_a_hackcoins INTEGER NOT NULL DEFAULT 0,
		team_b_hackcoins INTEGER NOT NULL DEFAULT 0
	);`

	if _, err := db.Exec(createTable); err != nil {
		t.Fatalf("create table: %v", err)
	}
	return db
}

func TestRoundCreate(t *testing.T) {
	db := setupRoundDB(t)
	repo := repository.NewRoundRepo(db)

	r := &models.Round{
		RoundName: "Quarter Finals",
		TeamAID:   1,
		TeamBID:   2,
	}

	ctx := context.Background()
	if err := repo.Create(ctx, r); err != nil {
		t.Fatalf("create: %v", err)
	}
	if r.ID == 0 {
		t.Error("expected non-zero ID after create")
	}
	if r.CreatedAt.IsZero() {
		t.Error("expected non-zero CreatedAt")
	}
	if r.UpdatedAt.IsZero() {
		t.Error("expected non-zero UpdatedAt")
	}
}

func TestRoundGetByID(t *testing.T) {
	db := setupRoundDB(t)
	repo := repository.NewRoundRepo(db)

	r := &models.Round{
		RoundName: "Semifinals",
		TeamAID:   1,
		TeamBID:   3,
	}
	if err := repo.Create(context.Background(), r); err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := repo.GetByID(context.Background(), r.ID)
	if err != nil {
		t.Fatalf("get by id: %v", err)
	}
	if got.ID != r.ID {
		t.Errorf("expected ID %d, got %d", r.ID, got.ID)
	}
	if got.RoundName != "Semifinals" {
		t.Errorf("expected round_name Semifinals, got %s", got.RoundName)
	}
	if got.TeamAID != 1 {
		t.Errorf("expected team_a_id 1, got %d", got.TeamAID)
	}
}

func TestRoundGetByIDNotFound(t *testing.T) {
	db := setupRoundDB(t)
	repo := repository.NewRoundRepo(db)

	_, err := repo.GetByID(context.Background(), 9999)
	if err != repository.ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestRoundGetAll(t *testing.T) {
	db := setupRoundDB(t)
	repo := repository.NewRoundRepo(db)

	r1 := &models.Round{RoundName: "Round 1", TeamAID: 1, TeamBID: 2}
	r2 := &models.Round{RoundName: "Round 2", TeamAID: 3, TeamBID: 4}
	if err := repo.Create(context.Background(), r1); err != nil {
		t.Fatalf("create r1: %v", err)
	}
	if err := repo.Create(context.Background(), r2); err != nil {
		t.Fatalf("create r2: %v", err)
	}

	all, err := repo.GetAll(context.Background())
	if err != nil {
		t.Fatalf("get all: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2 rounds, got %d", len(all))
	}
	if all[0].RoundName != "Round 1" {
		t.Errorf("expected first name Round 1, got %s", all[0].RoundName)
	}
	if all[1].RoundName != "Round 2" {
		t.Errorf("expected second name Round 2, got %s", all[1].RoundName)
	}
}

func TestRoundGetAllEmpty(t *testing.T) {
	db := setupRoundDB(t)
	repo := repository.NewRoundRepo(db)

	all, err := repo.GetAll(context.Background())
	if err != nil {
		t.Fatalf("get all: %v", err)
	}
	if len(all) != 0 {
		t.Errorf("expected 0 rounds, got %d", len(all))
	}
}

func TestRoundUpdate(t *testing.T) {
	db := setupRoundDB(t)
	repo := repository.NewRoundRepo(db)

	r := &models.Round{RoundName: "Orig", TeamAID: 1, TeamBID: 2}
	if err := repo.Create(context.Background(), r); err != nil {
		t.Fatalf("create: %v", err)
	}

	updates := map[string]any{
		"round_name": "Updated",
	}
	if err := repo.Update(context.Background(), r.ID, updates); err != nil {
		t.Fatalf("update: %v", err)
	}

	got, err := repo.GetByID(context.Background(), r.ID)
	if err != nil {
		t.Fatalf("get after update: %v", err)
	}
	if got.RoundName != "Updated" {
		t.Errorf("expected round_name Updated, got %s", got.RoundName)
	}
	// Other fields should remain unchanged
	if got.TeamAID != 1 {
		t.Errorf("expected team_a_id preserved at 1, got %d", got.TeamAID)
	}
}

func TestRoundDisable(t *testing.T) {
	db := setupRoundDB(t)
	repo := repository.NewRoundRepo(db)

	r := &models.Round{RoundName: "To Disable", TeamAID: 1, TeamBID: 2}
	if err := repo.Create(context.Background(), r); err != nil {
		t.Fatalf("create: %v", err)
	}

	// Disable should succeed
	if err := repo.Disable(context.Background(), r.ID); err != nil {
		t.Fatalf("disable: %v", err)
	}

	// GetByID should now return ErrNotFound (disabled=false filter)
	_, err := repo.GetByID(context.Background(), r.ID)
	if err != repository.ErrNotFound {
		t.Errorf("expected ErrNotFound after disable, got %v", err)
	}

	// GetAll should also exclude it
	all, err := repo.GetAll(context.Background())
	if err != nil {
		t.Fatalf("get all after disable: %v", err)
	}
	if len(all) != 0 {
		t.Errorf("expected 0 rounds in GetAll after disable, got %d", len(all))
	}
}

func TestRoundDisableNotFound(t *testing.T) {
	db := setupRoundDB(t)
	repo := repository.NewRoundRepo(db)

	r := &models.Round{RoundName: "Once", TeamAID: 1, TeamBID: 2}
	if err := repo.Create(context.Background(), r); err != nil {
		t.Fatalf("create: %v", err)
	}

	// Disable once — should succeed
	if err := repo.Disable(context.Background(), r.ID); err != nil {
		t.Fatalf("disable first time: %v", err)
	}

	// Disable again — should return ErrNotFound (already disabled)
	err := repo.Disable(context.Background(), r.ID)
	if err != repository.ErrNotFound {
		t.Fatalf("expected ErrNotFound on second disable, got %v", err)
	}
}

func TestRoundToggleReady(t *testing.T) {
	db := setupRoundDB(t)
	repo := repository.NewRoundRepo(db)

	r := &models.Round{RoundName: "Ready Round", TeamAID: 1, TeamBID: 2}
	if err := repo.Create(context.Background(), r); err != nil {
		t.Fatalf("create: %v", err)
	}

	// Toggle ready ON
	if err := repo.ToggleReady(context.Background(), r.ID); err != nil {
		t.Fatalf("toggle ready on: %v", err)
	}

	got, err := repo.GetByID(context.Background(), r.ID)
	if err != nil {
		t.Fatalf("get after toggle on: %v", err)
	}
	if !got.Ready {
		t.Error("expected Ready=true after toggle on")
	}
	if got.Status != "scheduled" {
		t.Errorf("expected status=scheduled after toggle on, got %s", got.Status)
	}

	// Toggle ready OFF
	if err := repo.ToggleReady(context.Background(), r.ID); err != nil {
		t.Fatalf("toggle ready off: %v", err)
	}

	got, err = repo.GetByID(context.Background(), r.ID)
	if err != nil {
		t.Fatalf("get after toggle off: %v", err)
	}
	if got.Ready {
		t.Error("expected Ready=false after toggle off")
	}
	if got.Status != "scheduled" {
		t.Errorf("expected status=scheduled after toggle off, got %s", got.Status)
	}
}

func TestRoundSetLive(t *testing.T) {
	db := setupRoundDB(t)
	repo := repository.NewRoundRepo(db)

	r := &models.Round{RoundName: "Live Round", TeamAID: 1, TeamBID: 2}
	if err := repo.Create(context.Background(), r); err != nil {
		t.Fatalf("create: %v", err)
	}

	// Set live ON — should return "scheduled" as previous status
	prev, err := repo.SetLive(context.Background(), r.ID)
	if err != nil {
		t.Fatalf("set live on: %v", err)
	}
	if prev != "scheduled" {
		t.Errorf("expected previous status 'scheduled', got %s", prev)
	}

	got, err := repo.GetByID(context.Background(), r.ID)
	if err != nil {
		t.Fatalf("get after set live: %v", err)
	}
	if !got.Live {
		t.Error("expected Live=true after set live on")
	}
	if got.Status != "in_progress" {
		t.Errorf("expected status=in_progress after set live on, got %s", got.Status)
	}

	// Set live OFF — should return "in_progress" as previous status
	prev, err = repo.SetLive(context.Background(), r.ID)
	if err != nil {
		t.Fatalf("set live off: %v", err)
	}
	if prev != "in_progress" {
		t.Errorf("expected previous status 'in_progress', got %s", prev)
	}

	got, err = repo.GetByID(context.Background(), r.ID)
	if err != nil {
		t.Fatalf("get after set live off: %v", err)
	}
	if got.Live {
		t.Error("expected Live=false after set live off")
	}
}

func TestSetAndGetCurrentTeams(t *testing.T) {
	db := setupRoundDB(t)
	repo := repository.NewRoundRepo(db)

	_, err := db.Exec(`INSERT INTO team (id, slug, name, alt_name, clan_tag) VALUES 
		(1, 'red', 'Red Team', 'Red Alt', 'RED'),
		(2, 'blue', 'Blue Team', 'Blue Alt', 'BLUE')`)
	if err != nil {
		t.Fatalf("insert teams: %v", err)
	}

	// 1. Set current teams
	ct, err := repo.SetCurrentTeams(context.Background(), 1, 2)
	if err != nil {
		t.Fatalf("SetCurrentTeams error: %v", err)
	}
	if ct.TeamAID != 1 || ct.TeamBID != 2 {
		t.Errorf("got team IDs %d, %d; want 1, 2", ct.TeamAID, ct.TeamBID)
	}
	if ct.TeamA.Name != "Red Team" || ct.TeamB.Name != "Blue Team" {
		t.Errorf("got team names %s, %s; want Red Team, Blue Team", ct.TeamA.Name, ct.TeamB.Name)
	}

	// 2. Get current teams
	got, err := repo.GetCurrentTeams(context.Background())
	if err != nil {
		t.Fatalf("GetCurrentTeams error: %v", err)
	}
	if got.TeamAID != 1 || got.TeamBID != 2 {
		t.Errorf("got team IDs %d, %d; want 1, 2", got.TeamAID, got.TeamBID)
	}

	// 3. Set with invalid team ID
	_, err = repo.SetCurrentTeams(context.Background(), 1, 999)
	if err == nil {
		t.Error("expected error for non-existent team_b_id 999")
	}
}

