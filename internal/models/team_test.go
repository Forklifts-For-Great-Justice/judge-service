package models_test

import (
	"testing"
	"time"

	"github.com/forklifts-for-great-justice/judge-service/internal/models"
)

func TestTeam_Validate_Valid(t *testing.T) {
	team := models.Team{
		Slug:      "my-team",
		Name:      "My Team",
		AltName:   "Alternate",
		ClanTag:   "MT",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	if err := team.Validate(); err != nil {
		t.Fatalf("expected nil, got error: %v", err)
	}
}

func TestTeam_Validate_EmptySlug(t *testing.T) {
	team := models.Team{
		Slug:      "",
		Name:      "My Team",
		ClanTag:   "MT",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	err := team.Validate()
	if err == nil {
		t.Fatal("expected error for empty slug, got nil")
	}
}

func TestTeam_Validate_BadSlugFormat(t *testing.T) {
	team := models.Team{
		Slug:      "My-Team",
		Name:      "My Team",
		ClanTag:   "MT",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	err := team.Validate()
	if err == nil {
		t.Fatal("expected error for uppercase slug, got nil")
	}
}

func TestTeam_Validate_EmptyName(t *testing.T) {
	team := models.Team{
		Slug:      "my-team",
		Name:      "",
		ClanTag:   "MT",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	err := team.Validate()
	if err == nil {
		t.Fatal("expected error for empty name, got nil")
	}
}

func TestTeam_Validate_EmptyClanTag(t *testing.T) {
	team := models.Team{
		Slug:      "my-team",
		Name:      "My Team",
		ClanTag:   "",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	err := team.Validate()
	if err == nil {
		t.Fatal("expected error for empty clan_tag, got nil")
	}
}

func TestTeam_Validate_SlugTooShort(t *testing.T) {
	team := models.Team{
		Slug:      "a",
		Name:      "My Team",
		ClanTag:   "MT",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	err := team.Validate()
	if err == nil {
		t.Fatal("expected error for 1-char slug, got nil")
	}
}

func TestTeam_Validate_SlugLength64(t *testing.T) {
	slug := ""
	for i := 0; i < 64; i++ {
		slug += "a"
	}
	team := models.Team{
		Slug:      slug,
		Name:      "My Team",
		ClanTag:   "MT",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	if err := team.Validate(); err != nil {
		t.Fatalf("expected nil for 64-char slug, got error: %v", err)
	}
}

func TestTeam_Validate_SlugTooLong(t *testing.T) {
	slug := ""
	for i := 0; i < 65; i++ {
		slug += "a"
	}
	team := models.Team{
		Slug:      slug,
		Name:      "My Team",
		ClanTag:   "MT",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	err := team.Validate()
	if err == nil {
		t.Fatal("expected error for 65-char slug, got nil")
	}
}

func TestTeam_Validate_SlugSpecialChars(t *testing.T) {
	cases := []struct {
		name string
		slug string
	}{
		{"underscore", "a_b"},
		{"special_char", "a!b"},
		{"uppercase", "A-b"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			team := models.Team{
				Slug:      tc.slug,
				Name:      "My Team",
				ClanTag:   "MT",
				CreatedAt: time.Now().UTC(),
				UpdatedAt: time.Now().UTC(),
			}
			err := team.Validate()
			if err == nil {
				t.Fatalf("expected error for slug %q, got nil", tc.slug)
			}
		})
	}
}
