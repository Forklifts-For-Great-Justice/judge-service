// Package models defines the entity structures used by JudgeService.
package models

import (
	"encoding/json"
	"time"
)

// Shananigan is a catalogue entry representing an activatable event
// that a judge can trigger during a competition round.
type Shananigan struct {
	ID          int64           `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	RconPayload string          `json:"rcon_payload"`
	TargetType  string          `json:"target_type"` // "team" or "all"
	Cost        *int64          `json:"cost,omitempty"`
	Metadata    json.RawMessage `json:"metadata,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}
