package models_test

import (
	"strings"
	"testing"
	"time"

	"github.com/forklifts-for-great-justice/judge-service/internal/models"
)

func TestRound_Validate_Valid(t *testing.T) {
	round := models.Round{
		ID:              1,
		TeamAID:         10,
		TeamBID:         20,
		RoundName:       "Quarterfinal 1",
		Status:          "scheduled",
		CreatedAt:       time.Now().UTC(),
		UpdatedAt:       time.Now().UTC(),
	}
	if err := round.Validate(); err != nil {
		t.Fatalf("expected nil, got error: %v", err)
	}
}

func TestRound_Validate_EmptyName(t *testing.T) {
	round := models.Round{
		TeamAID:   10,
		TeamBID:   20,
		RoundName: "",
	}
	err := round.Validate()
	if err == nil {
		t.Fatal("expected error for empty name, got nil")
	}
}

func TestRound_Validate_NameTooLong(t *testing.T) {
	round := models.Round{
		TeamAID:   10,
		TeamBID:   20,
		RoundName: strings.Repeat("a", 129),
	}
	err := round.Validate()
	if err == nil {
		t.Fatal("expected error for name > 128 chars, got nil")
	}
}

func TestRound_Validate_SelfMatch(t *testing.T) {
	round := models.Round{
		TeamAID:   10,
		TeamBID:   10,
		RoundName: "Self Match",
	}
	err := round.Validate()
	if err == nil {
		t.Fatal("expected self-match error, got nil")
	}
}

func TestRound_Validate_MissingTeamA(t *testing.T) {
	round := models.Round{
		TeamAID:   0,
		TeamBID:   20,
		RoundName: "Round A",
	}
	err := round.Validate()
	if err == nil {
		t.Fatal("expected error for missing team_a_id, got nil")
	}
}

func TestRound_Validate_MissingTeamB(t *testing.T) {
	round := models.Round{
		TeamAID:   10,
		TeamBID:   0,
		RoundName: "Round B",
	}
	err := round.Validate()
	if err == nil {
		t.Fatal("expected error for missing team_b_id, got nil")
	}
}
