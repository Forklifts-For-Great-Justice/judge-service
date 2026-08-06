package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/forklifts-for-great-justice/judge-service/internal/models"
)

var ErrTeamNotInMatch = fmt.Errorf("your team is not in this round, WTF do you think you're doing")

// PlayerChallengeItem represents a challenge item enriched with solved status for a team.
type PlayerChallengeItem struct {
	models.Challenge
	Solved bool `json:"solved"`
}

// PlayerShenaniganItem represents an enabled player shenanigan (non-disabled, judge_only=false).
type PlayerShenaniganItem struct {
	ID          int64           `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	RconPayload string          `json:"rcon_payload"`
	TargetType  string          `json:"target_type"`
	Cost        *int64          `json:"cost,omitempty"`
	Price       int             `json:"price,omitempty"`
	Disabled    bool            `json:"disabled"`
	JudgeOnly   bool            `json:"judge_only"`
	Metadata    json.RawMessage `json:"metadata,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

// PlayerRepository defines database access methods for player-facing endpoints.
type PlayerRepository interface {
	GetChallengesForTeam(ctx context.Context, teamID int64) ([]*PlayerChallengeItem, error)
	SubmitFlag(ctx context.Context, challengeID int64, playerID string, teamID int64, submittedFlag string) (bool, int, error)
	GetEnabledPlayerShenanigans(ctx context.Context) ([]*models.Shananigan, error)
	BuyShenanigan(ctx context.Context, shenaniganID int64, buyerID string, teamID int64) (*PurchaseRecord, int64, error)
}

type PlayerRepo struct {
	db *sql.DB
}

func NewPlayerRepo(db *sql.DB) *PlayerRepo {
	return &PlayerRepo{db: db}
}

// GetChallengesForTeam lists all non-disabled challenges and marks solved=true if an accepted challenge_submission exists for teamID.
func (r *PlayerRepo) GetChallengesForTeam(ctx context.Context, teamID int64) ([]*PlayerChallengeItem, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("database connection not available")
	}

	query := `
		SELECT c.id, c.name, c.description, c.challenge_type, c.location, c.points, c.disabled, c.flag, c.created_at, c.updated_at,
		       EXISTS(
		           SELECT 1 FROM challenge_submission cs
		           WHERE cs.challenge_id = c.id AND cs.team_id = $1 AND cs.accepted = true
		       ) AS solved
		FROM challenge c
		WHERE (c.enabled = true OR c.disabled = false)
		ORDER BY c.id ASC`

	rows, err := r.db.QueryContext(ctx, query, teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]*PlayerChallengeItem, 0)
	for rows.Next() {
		item := &PlayerChallengeItem{}
		var ctype sql.NullString
		var loc sql.NullString
		var disabled sql.NullBool
		err := rows.Scan(
			&item.ID, &item.Name, &item.Description, &ctype, &loc,
			&item.Points, &disabled, &item.Flag,
			&item.CreatedAt, &item.UpdatedAt,
			&item.Solved,
		)
		if err != nil {
			return nil, err
		}
		item.ChallengeType = nullStringToPointer(ctype)
		item.Location = nullStringToPointer(loc)
		if disabled.Valid {
			item.Disabled = disabled.Bool
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

// SubmitFlag handles flag submission:
// Checks if challenge exists and is enabled, verifies exact flag match, inserts challenge_submission.
// If flag matches and team hasn't solved it yet, marks accepted=true, updates team hackcoins/points in current_match / matches, returns (correct=true, pointsAwarded).
func (r *PlayerRepo) SubmitFlag(ctx context.Context, challengeID int64, playerID string, teamID int64, submittedFlag string) (bool, int, error) {
	if r == nil || r.db == nil {
		return false, 0, fmt.Errorf("database connection not available")
	}

	// Verify team is in current_match
	var teamA, teamB sql.NullInt64
	errCM := r.db.QueryRowContext(ctx, `SELECT team_a_id, team_b_id FROM current_match LIMIT 1`).Scan(&teamA, &teamB)
	if errCM != nil || (!teamA.Valid || teamA.Int64 != teamID) && (!teamB.Valid || teamB.Int64 != teamID) {
		return false, 0, ErrTeamNotInMatch
	}

	cRepo := NewChallengeRepo(r.db)
	ch, err := cRepo.GetByID(ctx, challengeID)
	if err != nil {
		return false, 0, err
	}
	if ch.Disabled {
		return false, 0, ErrNotFound
	}

	correct := (ch.Flag == submittedFlag)

	// Check if team already solved this challenge
	var alreadySolved bool
	err = r.db.QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM challenge_submission
			WHERE challenge_id = $1 AND team_id = $2 AND accepted = true
		)`, challengeID, teamID).Scan(&alreadySolved)
	if err != nil {
		return false, 0, err
	}

	accepted := correct && !alreadySolved
	var acceptedAt *time.Time
	if accepted {
		now := time.Now()
		acceptedAt = &now
	}

	// Insert submission audit record
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO challenge_submission (challenge_id, player_id, team_id, submitted_flag, accepted, accepted_at)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		challengeID, playerID, teamID, submittedFlag, accepted, acceptedAt,
	)
	if err != nil {
		return false, 0, fmt.Errorf("failed to record submission: %w", err)
	}

	pointsAwarded := 0
	if accepted {
		pointsAwarded = ch.Points

		// Award points & hackcoins to team in current_match
		if teamA.Valid && teamA.Int64 == teamID {
			_, _ = r.db.ExecContext(ctx, `
				UPDATE current_match
				SET team_a_hackcoins = team_a_hackcoins + $1,
				    team_a_hack_points = team_a_hack_points + $1`, pointsAwarded)
		} else if teamB.Valid && teamB.Int64 == teamID {
			_, _ = r.db.ExecContext(ctx, `
				UPDATE current_match
				SET team_b_hackcoins = team_b_hackcoins + $1,
				    team_b_hack_points = team_b_hack_points + $1`, pointsAwarded)
		}
	}

	return correct, pointsAwarded, nil
}

// GetEnabledPlayerShenanigans lists all non-deleted, non-disabled shenanigans where judge_only = false (or judge_only is false/null).
func (r *PlayerRepo) GetEnabledPlayerShenanigans(ctx context.Context) ([]*models.Shananigan, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("database connection not available")
	}

	// Supports both table structures: checking deleted_at IS NULL if column exists, and disabled=false, judge_only=false if exists
	// We'll query shenanigans table with safe fallbacks.
	var query string
	// Check if judge_only column exists in shenanigans table
	var hasJudgeOnly bool
	err := r.db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.columns 
			WHERE table_name = 'shenanigans' AND column_name = 'judge_only'
		)`).Scan(&hasJudgeOnly)
	if err != nil {
		hasJudgeOnly = false
	}

	if hasJudgeOnly {
		query = `SELECT id, name, description, rcon_payload, target_type, cost, metadata, created_at, updated_at
		         FROM shenanigans WHERE deleted_at IS NULL AND (enabled = true OR enabled IS NULL) AND judge_only = false ORDER BY id ASC`
	} else {
		query = `SELECT id, name, description, rcon_payload, target_type, cost, metadata, created_at, updated_at
		         FROM shenanigans WHERE deleted_at IS NULL ORDER BY id ASC`
	}

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]*models.Shananigan, 0)
	for rows.Next() {
		s := &models.Shananigan{}
		var meta *string
		err := rows.Scan(
			&s.ID, &s.Name, &s.Description, &s.RconPayload, &s.TargetType,
			&s.Cost, &meta, &s.CreatedAt, &s.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		if meta != nil {
			s.Metadata = json.RawMessage(*meta)
		}
		result = append(result, s)
	}
	return result, rows.Err()
}

// BuyShenanigan handles player purchasing a shenanigan using team hackcoins.
// Checks cost, verifies team balance, deducts coins, creates PurchaseRecord.
func (r *PlayerRepo) BuyShenanigan(ctx context.Context, shenaniganID int64, buyerID string, teamID int64) (*PurchaseRecord, int64, error) {
	if r == nil || r.db == nil {
		return nil, 0, fmt.Errorf("database connection not available")
	}

	sRepo := NewShananiganRepo(r.db)
	s, err := sRepo.GetByID(ctx, shenaniganID)
	if err != nil {
		return nil, 0, err
	}

	cost := int64(0)
	if s.Cost != nil {
		cost = *s.Cost
	}

	// Check team balance in current_match
	var currentBalance int64
	var isTeamA bool
	var foundMatch bool

	queryCM := `SELECT team_a_id, team_b_id, team_a_hackcoins, team_b_hackcoins FROM current_match LIMIT 1`
	var teamA, teamB sql.NullInt64
	var coinsA, coinsB sql.NullInt64
	err = r.db.QueryRowContext(ctx, queryCM).Scan(&teamA, &teamB, &coinsA, &coinsB)
	if err == nil {
		if teamA.Valid && teamA.Int64 == teamID {
			currentBalance = coinsA.Int64
			isTeamA = true
			foundMatch = true
		} else if teamB.Valid && teamB.Int64 == teamID {
			currentBalance = coinsB.Int64
			isTeamA = false
			foundMatch = true
		}
	}

	if !foundMatch {
		return nil, 0, ErrTeamNotInMatch
	}

	if cost > 0 && currentBalance < cost {
		return nil, currentBalance, fmt.Errorf("insufficient hackcoins: team has %d, required %d", currentBalance, cost)
	}

	// Deduct balance if cost > 0
	if cost > 0 && foundMatch {
		if isTeamA {
			_, _ = r.db.ExecContext(ctx, `UPDATE current_match SET team_a_hackcoins = team_a_hackcoins - $1`, cost)
		} else {
			_, _ = r.db.ExecContext(ctx, `UPDATE current_match SET team_b_hackcoins = team_b_hackcoins - $1`, cost)
		}
	}

	purchaseID := uuid.New()
	record := &PurchaseRecord{
		PurchaseID:  purchaseID,
		RconPayload: s.RconPayload,
	}

	remainingCoins := currentBalance - cost
	if remainingCoins < 0 {
		remainingCoins = 0
	}

	return record, remainingCoins, nil
}
