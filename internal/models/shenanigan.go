// Package models defines the entity structures used by JudgeService.
package models

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// ActivationRecord represents an activation (purchase) of a shenanigan.
// Status flows through: "pending" → "confirmed" or "failed".
type ActivationRecord struct {
	PurchaseID   uuid.UUID       `json:"purchase_id"`
	ShenaniganID int64           `json:"shenanigan_id"`
	Status       string          `json:"status"`
	ErrorMessage string          `json:"error_message,omitempty"`
	RconPayload  string          `json:"rcon_payload"`
	Metadata     json.RawMessage `json:"metadata,omitempty"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
}

// ActivationStatus represents the lifecycle state of an activation.
type ActivationStatus string

const (
	ActivationPending   ActivationStatus = "pending"
	ActivationConfirmed ActivationStatus = "confirmed"
	ActivationFailed    ActivationStatus = "failed"
)

// Activate checks that required fields are present on an ActivationRecord.
// The only truly required field is PurchaseID; the rest default to zero/empty values.
func (a *ActivationRecord) Validate() error {
	if a.PurchaseID == uuid.Nil {
		return fmt.Errorf("purchase_id must not be nil")
	}
	return nil
}

// GetStatus returns the activation status as an ActivationStatus type.
func (a *ActivationRecord) GetStatus() ActivationStatus {
	if a.Status == "" {
		return ActivationPending
	}
	return ActivationStatus(a.Status)
}

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
	DeletedAt   *time.Time      `json:"deleted_at"`
}

// Validate checks that a Shananigan has all required valid fields.
func (s *Shananigan) Validate() error {
	if s.Name == "" {
		return fmt.Errorf("name must not be empty")
	}
	if s.Description == "" {
		return fmt.Errorf("description must not be empty")
	}
	if s.RconPayload == "" {
		return fmt.Errorf("rcon_payload must not be empty")
	}
	if s.TargetType == "" {
		return fmt.Errorf("target_type must not be empty")
	}
	if s.TargetType != "team" && s.TargetType != "all" {
		return fmt.Errorf("target_type must be \"team\" or \"all\"")
	}
	return nil
}
