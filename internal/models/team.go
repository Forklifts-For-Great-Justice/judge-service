package models

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

var slugRegex = regexp.MustCompile(`^[a-z0-9-]+$`)

// Team represents a competition team.
type Team struct {
	ID        int64     `json:"id"`
	Slug      string    `json:"slug"`
	Name      string    `json:"name"`
	AltName   string    `json:"alt_name"`
	ClanTag   string    `json:"clan_tag"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Validate checks that a Team has all required valid fields.
func (t *Team) Validate() error {
	slug := strings.TrimSpace(t.Slug)
	if slug == "" {
		return fmt.Errorf("slug must not be empty")
	}
	if len(slug) < 2 || len(slug) > 64 {
		return fmt.Errorf("slug must be between 2 and 64 characters")
	}
	if !slugRegex.MatchString(slug) {
		return fmt.Errorf("slug must match ^[a-z0-9-]+$")
	}

	name := strings.TrimSpace(t.Name)
	if name == "" {
		return fmt.Errorf("name must not be empty")
	}

	clanTag := strings.TrimSpace(t.ClanTag)
	if clanTag == "" {
		return fmt.Errorf("clan_tag must not be empty")
	}

	return nil
}
