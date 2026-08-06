package repository_test

import (
	"context"
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"github.com/forklifts-for-great-justice/judge-service/internal/repository"
)

func setupScoreboardDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	createTables := `
	CREATE TABLE IF NOT EXISTS team (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		slug TEXT NOT NULL,
		name TEXT NOT NULL,
		alt_name TEXT NOT NULL,
		clan_tag TEXT NOT NULL
	);

	CREATE TABLE IF NOT EXISTS matches (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		team_a_id INTEGER NOT NULL,
		team_b_id INTEGER NOT NULL,
		team_a_points INTEGER NOT NULL DEFAULT 0,
		team_b_points INTEGER NOT NULL DEFAULT 0,
		team_a_hack_points INTEGER NOT NULL DEFAULT 0,
		team_b_hack_points INTEGER NOT NULL DEFAULT 0,
		team_a_hackcoins INTEGER NOT NULL DEFAULT 0,
		team_b_hackcoins INTEGER NOT NULL DEFAULT 0,
		status TEXT NOT NULL DEFAULT 'scheduled',
		live BOOLEAN NOT NULL DEFAULT FALSE,
		disabled BOOLEAN NOT NULL DEFAULT FALSE
	);

	CREATE TABLE IF NOT EXISTS current_match (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		team_a_id INTEGER NOT NULL,
		team_b_id INTEGER NOT NULL,
		team_a_points INTEGER NOT NULL DEFAULT 0,
		team_b_points INTEGER NOT NULL DEFAULT 0,
		team_a_hack_points INTEGER NOT NULL DEFAULT 0,
		team_b_hack_points INTEGER NOT NULL DEFAULT 0,
		team_a_hackcoins INTEGER NOT NULL DEFAULT 0,
		team_b_hackcoins INTEGER NOT NULL DEFAULT 0
	);
	`
	if _, err := db.Exec(createTables); err != nil {
		t.Fatalf("create tables: %v", err)
	}
	return db
}

func TestScoreboardGetFromCurrentMatch(t *testing.T) {
	db := setupScoreboardDB(t)
	repo := repository.NewScoreboardRepo(db)

	_, err := db.Exec(`INSERT INTO team (id, slug, name, alt_name, clan_tag) VALUES 
		(1, 'foo', 'Foo Team', 'Foo Alt', 'FOO'),
		(2, 'bar', 'Bar Team', 'Bar Alt', 'BAR')`)
	if err != nil {
		t.Fatalf("insert teams: %v", err)
	}

	_, err = db.Exec(`INSERT INTO current_match (team_a_id, team_b_id, team_a_points, team_b_points, team_a_hack_points, team_b_hack_points, team_a_hackcoins, team_b_hackcoins)
		VALUES (1, 2, 10, 5, 100, 50, 20, 15)`)
	if err != nil {
		t.Fatalf("insert current_match: %v", err)
	}

	sb, err := repo.GetScoreboard(context.Background())
	if err != nil {
		t.Fatalf("GetScoreboard error: %v", err)
	}
	if sb.TeamAName != "FOO" || sb.TeamBName != "BAR" {
		t.Errorf("got team names %s, %s; want FOO, BAR", sb.TeamAName, sb.TeamBName)
	}
	if sb.TeamAPoints != 10 || sb.TeamAHackPoints != 100 || sb.TeamAHackCoins != 20 {
		t.Errorf("got team A stats %v, %v, %v", sb.TeamAPoints, sb.TeamAHackPoints, sb.TeamAHackCoins)
	}
}

func TestScoreboardGetEmpty(t *testing.T) {
	db := setupScoreboardDB(t)
	repo := repository.NewScoreboardRepo(db)

	_, err := repo.GetScoreboard(context.Background())
	if err == nil {
		t.Error("expected error when no active match exists, got nil")
	}
}
