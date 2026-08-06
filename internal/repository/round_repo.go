// Package repository provides database access for rounds.
package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/forklifts-for-great-justice/judge-service/internal/models"
)

// RoundRepository defines the round persistence interface.
type RoundRepository interface {
	Create(ctx context.Context, r *models.Round) error
	GetByID(ctx context.Context, id int64) (*models.Round, error)
	GetAll(ctx context.Context) ([]*models.Round, error)
	Update(ctx context.Context, id int64, updates map[string]any) error
	Disable(ctx context.Context, id int64) error
	ToggleReady(ctx context.Context, id int64) error
	SetLive(ctx context.Context, id int64) (string, error)
	GetCurrentTeams(ctx context.Context) (*models.CurrentTeams, error)
	SetCurrentTeams(ctx context.Context, teamAID, teamBID int64) (*models.CurrentTeams, error)
}

// RoundRepo implements RoundRepository via the matches table.
type RoundRepo struct {
	db *sql.DB
}

// NewRoundRepo creates a new RoundRepo.
func NewRoundRepo(db *sql.DB) *RoundRepo {
	return &RoundRepo{db: db}
}

// Create inserts a new round into the matches table.
func (r *RoundRepo) Create(ctx context.Context, round *models.Round) error {
	query := `INSERT INTO matches (team_a_id, team_b_id, round_name)
		VALUES ($1, $2, $3) RETURNING id, created_at, updated_at`
	err := r.db.QueryRowContext(ctx, query,
		round.TeamAID, round.TeamBID, round.RoundName,
	).Scan(&round.ID, &round.CreatedAt, &round.UpdatedAt)
	if isDuplicatePostgres(err) {
		return ErrDuplicate
	}
	return err
}

// GetByID returns a round by its ID. Returns ErrNotFound if absent.
func (r *RoundRepo) GetByID(ctx context.Context, id int64) (*models.Round, error) {
	re := &models.Round{}
	query := `SELECT id, team_a_id, team_b_id, round_name, team_a_points, team_b_points,
		team_a_hack_points, team_b_hack_points, team_a_hackcoins, team_b_hackcoins,
		status, ready, live, disabled, live_at, ready_at, created_at, updated_at
		FROM matches WHERE id = $1 AND disabled = false`
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&re.ID, &re.TeamAID, &re.TeamBID, &re.RoundName,
		&re.TeamAPoints, &re.TeamBPoints,
		&re.TeamAHackPoints, &re.TeamBHackPoints,
		&re.TeamAHackCoins, &re.TeamBHackCoins,
		&re.Status, &re.Ready, &re.Live, &re.Disabled,
		&re.LiveAt, &re.ReadyAt, &re.CreatedAt, &re.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	return re, err
}

// GetAll returns all non-disabled rounds from the matches table.
func (r *RoundRepo) GetAll(ctx context.Context) ([]*models.Round, error) {
	query := `SELECT id, team_a_id, team_b_id, round_name, team_a_points, team_b_points,
		team_a_hack_points, team_b_hack_points, team_a_hackcoins, team_b_hackcoins,
		status, ready, live, disabled, live_at, ready_at, created_at, updated_at
		FROM matches WHERE disabled = false ORDER BY id ASC`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]*models.Round, 0)
	for rows.Next() {
		var re models.Round
		if err := rows.Scan(
			&re.ID, &re.TeamAID, &re.TeamBID, &re.RoundName,
			&re.TeamAPoints, &re.TeamBPoints,
			&re.TeamAHackPoints, &re.TeamBHackPoints,
			&re.TeamAHackCoins, &re.TeamBHackCoins,
			&re.Status, &re.Ready, &re.Live, &re.Disabled,
			&re.LiveAt, &re.ReadyAt, &re.CreatedAt, &re.UpdatedAt,
		); err != nil {
			return nil, err
		}
		result = append(result, &re)
	}
	return result, rows.Err()
}

// Update applies partial updates to a round using a dynamic SET clause.
// The updated_at column is excluded (handled by PostgreSQL trigger).
func (r *RoundRepo) Update(ctx context.Context, id int64, updates map[string]any) error {
	if len(updates) == 0 {
		return nil
	}
	clauses := make([]string, 0, len(updates))
	args := make([]any, 0, len(updates)+1)
	idx := 1
	for k, v := range updates {
		if k == "updated_at" {
			continue
		}
		clauses = append(clauses, fmt.Sprintf("%s=$%d", k, idx))
		args = append(args, v)
		idx++
	}
	if len(clauses) == 0 {
		return nil
	}
	args = append(args, id)
	query := fmt.Sprintf("UPDATE matches SET %s WHERE id=$%d", strings.Join(clauses, ", "), idx)
	result, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// Disable soft-deletes a round by setting disabled=true.
func (r *RoundRepo) Disable(ctx context.Context, id int64) error {
	result, err := r.db.ExecContext(ctx,
		"UPDATE matches SET disabled=true WHERE id=$1 AND disabled=false", id)
	if err != nil {
		return err
	}
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// ToggleReady toggles the ready state. Sets status='scheduled'.
func (r *RoundRepo) ToggleReady(ctx context.Context, id int64) error {
	var ready bool
	err := r.db.QueryRowContext(ctx,
		"SELECT ready FROM matches WHERE id=$1 AND disabled=false", id).Scan(&ready)
	if err == sql.ErrNoRows {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if !ready {
		_, err = r.db.ExecContext(ctx,
			"UPDATE matches SET ready=true, ready_at=NOW(), status='scheduled' WHERE id=$1 AND disabled=false", id)
	} else {
		// Use NULL (not DEFAULT NULL) for SQLite compatibility — semantically identical.
		_, err = r.db.ExecContext(ctx,
			"UPDATE matches SET ready=false, ready_at=NULL, status='scheduled' WHERE id=$1 AND disabled=false", id)
	}
	if err != nil {
		return err
	}
	return nil
}

// SetLive toggles the live state. Returns the previous status.
func (r *RoundRepo) SetLive(ctx context.Context, id int64) (string, error) {
	var live bool
	var status string
	err := r.db.QueryRowContext(ctx,
		"SELECT live, status FROM matches WHERE id=$1 AND disabled=false", id).Scan(&live, &status)
	if err == sql.ErrNoRows {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	prevStatus := status
	var query string
	if !live {
		query = "UPDATE matches SET live=true, live_at=NOW(), status='in_progress' WHERE id=$1 AND disabled=false"
	} else {
		// Use NULL (not DEFAULT NULL) for SQLite compatibility — semantically identical.
		query = "UPDATE matches SET live=false, live_at=NULL, status='scheduled' WHERE id=$1 AND disabled=false"
	}
	res, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return prevStatus, err
	}
	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return prevStatus, ErrNotFound
	}
	return prevStatus, nil
}

// GetCurrentTeams returns team details for the currently active match.
func (r *RoundRepo) GetCurrentTeams(ctx context.Context) (*models.CurrentTeams, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("database connection is nil")
	}

	var matchID sql.NullInt64
	var teamA, teamB models.Team

	// 1. Query current_match joined with team
	queryCurrent := `SELECT 
		cm.match_id,
		ta.id, ta.slug, ta.name, ta.alt_name, ta.clan_tag,
		tb.id, tb.slug, tb.name, tb.alt_name, tb.clan_tag
		FROM current_match cm
		JOIN team ta ON cm.team_a_id = ta.id
		JOIN team tb ON cm.team_b_id = tb.id
		WHERE cm.is_current = TRUE LIMIT 1`

	err := r.db.QueryRowContext(ctx, queryCurrent).Scan(
		&matchID,
		&teamA.ID, &teamA.Slug, &teamA.Name, &teamA.AltName, &teamA.ClanTag,
		&teamB.ID, &teamB.Slug, &teamB.Name, &teamB.AltName, &teamB.ClanTag,
	)
	if err == nil {
		res := &models.CurrentTeams{
			TeamAID: teamA.ID,
			TeamBID: teamB.ID,
			TeamA:   &teamA,
			TeamB:   &teamB,
		}
		if matchID.Valid {
			res.MatchID = matchID.Int64
		}
		return res, nil
	}

	// 2. Fallback to live/in_progress match in matches table
	queryMatches := `SELECT 
		m.id,
		ta.id, ta.slug, ta.name, ta.alt_name, ta.clan_tag,
		tb.id, tb.slug, tb.name, tb.alt_name, tb.clan_tag
		FROM matches m
		JOIN team ta ON m.team_a_id = ta.id
		JOIN team tb ON m.team_b_id = tb.id
		WHERE m.disabled = false AND (m.live = true OR m.status = 'in_progress')
		ORDER BY m.id DESC LIMIT 1`

	err = r.db.QueryRowContext(ctx, queryMatches).Scan(
		&matchID,
		&teamA.ID, &teamA.Slug, &teamA.Name, &teamA.AltName, &teamA.ClanTag,
		&teamB.ID, &teamB.Slug, &teamB.Name, &teamB.AltName, &teamB.ClanTag,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}

	res := &models.CurrentTeams{
		TeamAID: teamA.ID,
		TeamBID: teamB.ID,
		TeamA:   &teamA,
		TeamB:   &teamB,
	}
	if matchID.Valid {
		res.MatchID = matchID.Int64
	}
	return res, nil
}

// SetCurrentTeams updates/sets the active teams in current_match and matches.
func (r *RoundRepo) SetCurrentTeams(ctx context.Context, teamAID, teamBID int64) (*models.CurrentTeams, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("database connection is nil")
	}
	if teamAID <= 0 || teamBID <= 0 {
		return nil, fmt.Errorf("team_a_id and team_b_id must be valid positive integers")
	}
	if teamAID == teamBID {
		return nil, fmt.Errorf("team_a_id and team_b_id must be different")
	}

	// 1. Verify both team_a_id and team_b_id exist in team table
	var teamA, teamB models.Team
	queryTeam := `SELECT id, slug, name, alt_name, clan_tag FROM team WHERE id = $1`
	if err := r.db.QueryRowContext(ctx, queryTeam, teamAID).Scan(&teamA.ID, &teamA.Slug, &teamA.Name, &teamA.AltName, &teamA.ClanTag); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("team_a_id %d does not exist in team table", teamAID)
		}
		return nil, fmt.Errorf("failed to fetch team_a: %w", err)
	}

	if err := r.db.QueryRowContext(ctx, queryTeam, teamBID).Scan(&teamB.ID, &teamB.Slug, &teamB.Name, &teamB.AltName, &teamB.ClanTag); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("team_b_id %d does not exist in team table", teamBID)
		}
		return nil, fmt.Errorf("failed to fetch team_b: %w", err)
	}

	// 2. Find or create match in matches table
	var matchID int64
	queryFindMatch := `SELECT id FROM matches WHERE team_a_id = $1 AND team_b_id = $2 AND disabled = false ORDER BY id DESC LIMIT 1`
	err := r.db.QueryRowContext(ctx, queryFindMatch, teamAID, teamBID).Scan(&matchID)
	if err != nil {
		// Create match
		queryInsertMatch := `INSERT INTO matches (team_a_id, team_b_id, round_name, status, live) VALUES ($1, $2, 'Current Match', 'in_progress', true) RETURNING id`
		if err := r.db.QueryRowContext(ctx, queryInsertMatch, teamAID, teamBID).Scan(&matchID); err != nil {
			res, errExec := r.db.ExecContext(ctx, `INSERT INTO matches (team_a_id, team_b_id, round_name, status, live) VALUES ($1, $2, 'Current Match', 'in_progress', true)`, teamAID, teamBID)
			if errExec != nil {
				return nil, fmt.Errorf("failed to create match: %w", errExec)
			}
			matchID, _ = res.LastInsertId()
		}
	} else {
		// Set live=true on this match and live=false on others
		_, _ = r.db.ExecContext(ctx, `UPDATE matches SET live = false WHERE id <> $1`, matchID)
		_, _ = r.db.ExecContext(ctx, `UPDATE matches SET live = true, status = 'in_progress' WHERE id = $1`, matchID)
	}

	// 3. Clear existing current_match and insert current active row
	_, _ = r.db.ExecContext(ctx, `DELETE FROM current_match`)
	queryInsertCurrent := `INSERT INTO current_match (match_id, team_a_id, team_b_id, round_name, is_current) VALUES ($1, $2, $3, 'Current Match', true)`
	if _, err := r.db.ExecContext(ctx, queryInsertCurrent, matchID, teamAID, teamBID); err != nil {
		return nil, fmt.Errorf("failed to update current_match: %w", err)
	}

	return &models.CurrentTeams{
		MatchID: matchID,
		TeamAID: teamAID,
		TeamBID: teamBID,
		TeamA:   &teamA,
		TeamB:   &teamB,
	}, nil
}

