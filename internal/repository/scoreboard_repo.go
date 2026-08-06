// Package repository provides database access for the scoreboard.
package repository

import (
	"context"
	"database/sql"
	"fmt"
)

// ScoreboardData holds team names and scores for the current match.
type ScoreboardData struct {
	TeamAName       string
	TeamBName       string
	TeamAPoints     int
	TeamBPoints     int
	TeamAHackPoints int
	TeamBHackPoints int
	TeamAHackCoins  int
	TeamBHackCoins  int
}

// ScoreboardRepository defines the interface for fetching scoreboard data.
type ScoreboardRepository interface {
	GetScoreboard(ctx context.Context) (*ScoreboardData, error)
}

// ScoreboardRepo implements ScoreboardRepository.
type ScoreboardRepo struct {
	db *sql.DB
}

// NewScoreboardRepo creates a new ScoreboardRepo.
func NewScoreboardRepo(db *sql.DB) *ScoreboardRepo {
	return &ScoreboardRepo{db: db}
}

// GetScoreboard fetches score and team information for the active match.
func (r *ScoreboardRepo) GetScoreboard(ctx context.Context) (*ScoreboardData, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("database connection is nil")
	}

	var sb ScoreboardData

	// 1. Try current_match first
	queryCurrent := `SELECT 
		cm.team_a_points, cm.team_b_points, 
		cm.team_a_hack_points, cm.team_b_hack_points, 
		cm.team_a_hackcoins, cm.team_b_hackcoins,
		COALESCE(ta.clan_tag, ta.name, ''),
		COALESCE(tb.clan_tag, tb.name, '')
		FROM current_match cm
		LEFT JOIN team ta ON cm.team_a_id = ta.id
		LEFT JOIN team tb ON cm.team_b_id = tb.id
		LIMIT 1`
	err := r.db.QueryRowContext(ctx, queryCurrent).Scan(
		&sb.TeamAPoints, &sb.TeamBPoints,
		&sb.TeamAHackPoints, &sb.TeamBHackPoints,
		&sb.TeamAHackCoins, &sb.TeamBHackCoins,
		&sb.TeamAName, &sb.TeamBName,
	)
	if err == nil {
		return &sb, nil
	}

	// 2. Fallback to live/in_progress match in matches table
	queryMatches := `SELECT 
		m.team_a_points, m.team_b_points, 
		m.team_a_hack_points, m.team_b_hack_points, 
		m.team_a_hackcoins, m.team_b_hackcoins,
		COALESCE(ta.clan_tag, ta.name, ''),
		COALESCE(tb.clan_tag, tb.name, '')
		FROM matches m
		LEFT JOIN team ta ON m.team_a_id = ta.id
		LEFT JOIN team tb ON m.team_b_id = tb.id
		WHERE m.disabled = false AND (m.live = true OR m.status = 'in_progress')
		ORDER BY m.id DESC LIMIT 1`
	err = r.db.QueryRowContext(ctx, queryMatches).Scan(
		&sb.TeamAPoints, &sb.TeamBPoints,
		&sb.TeamAHackPoints, &sb.TeamBHackPoints,
		&sb.TeamAHackCoins, &sb.TeamBHackCoins,
		&sb.TeamAName, &sb.TeamBName,
	)
	if err != nil {
		return nil, err
	}

	return &sb, nil
}
