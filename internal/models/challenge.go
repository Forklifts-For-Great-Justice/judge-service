package models

import (
	"fmt"
	"strings"
	"time"
)

// Challenge represents a competition challenge entry.
type Challenge struct {
	ID             int64     `json:"id"`
	Name           string    `json:"name"`
	Description    string    `json:"description"`
	ChallengeType  *string   `json:"challenge_type,omitempty"`
	Location       *string   `json:"location,omitempty"`
	Points         int       `json:"points"`
	Disabled       bool      `json:"disabled"`
	Flag           string    `json:"flag"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// Validate checks that a Challenge has all required valid fields.
func (c *Challenge) Validate() error {
	name := strings.TrimSpace(c.Name)
	if name == "" {
		return fmt.Errorf("name must not be empty")
	}
	if len(name) < 1 || len(name) > 256 {
		return fmt.Errorf("name must be between 1 and 256 characters")
	}

	description := strings.TrimSpace(c.Description)
	if description == "" {
		return fmt.Errorf("description must not be empty")
	}

	if c.Points <= 0 {
		return fmt.Errorf("points must be greater than 0")
	}

	flag := strings.TrimSpace(c.Flag)
	if flag == "" {
		return fmt.Errorf("flag must not be empty")
	}

	// ChallengeType and Location are optional — nil or blank are both acceptable.
	// No validation needed beyond what the type system guarantees.

	return nil
}
