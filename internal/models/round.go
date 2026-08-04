package models

import (
	"fmt"
	"strings"
	"time"
)

// Round represents a competition round (match between two teams).
// Status values: scheduled, in_progress, completed, cancelled.
type Round struct {
	ID              int64      `json:"id"`
	TeamAID         int64      `json:"team_a_id"`
	TeamBID         int64      `json:"team_b_id"`
	RoundName       string     `json:"round_name"`
	TeamAPoints     int        `json:"team_a_points"`
	TeamBPoints     int        `json:"team_b_points"`
	TeamAHackPoints int        `json:"team_a_hack_points"`
	TeamBHackPoints int        `json:"team_b_hack_points"`
	TeamAHackCoins  int        `json:"team_a_hackcoins"`
	TeamBHackCoins  int        `json:"team_b_hackcoins"`
	Status          string     `json:"status"`
	Ready           bool       `json:"ready"`
	Live            bool       `json:"live"`
	Disabled        bool       `json:"disabled"`
	LiveAt          *time.Time `json:"live_at,omitempty"`
	ReadyAt         *time.Time `json:"ready_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// RoundCreateRequest validates and creates a new Round.
type RoundCreateRequest struct {
	RoundName string `json:"round_name"`
	TeamAID   int64  `json:"team_a_id"`
	TeamBID   int64  `json:"team_b_id"`
}

// RoundUpdateRequest partially updates a Round.
type RoundUpdateRequest struct {
	RoundName *string `json:"round_name,omitempty"`
	TeamAID   *int64  `json:"team_a_id,omitempty"`
	TeamBID   *int64  `json:"team_b_id,omitempty"`
}

// Validate checks that a Round has all required valid fields.
func (r *Round) Validate() error {
	name := strings.TrimSpace(r.RoundName)
	if name == "" {
		return fmt.Errorf("round_name must not be empty")
	}
	if len(name) < 1 || len(name) > 128 {
		return fmt.Errorf("round_name must be between 1 and 128 characters")
	}
	if r.TeamAID == 0 {
		return fmt.Errorf("team_a_id must be set")
	}
	if r.TeamBID == 0 {
		return fmt.Errorf("team_b_id must be set")
	}
	if r.TeamAID == r.TeamBID {
		return fmt.Errorf("self-match: team_a_id and team_b_id must be different")
	}
	return nil
}
