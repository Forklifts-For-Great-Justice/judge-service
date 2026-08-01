// Package repository provides database access for shenanigans.
package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/forklifts-for-great-justice/judge-service/internal/models"
)

// ErrNotFound is returned when a requested resource does not exist.
var ErrNotFound = errors.New("record not found")

// Repository defines the shenanigan persistence interface.
// Concrete implementation in this package; used by the handler.
type Repository interface {
	Create(ctx context.Context, s *models.Shananigan) error
	GetByID(ctx context.Context, id int64) (*models.Shananigan, error)
	GetAll(ctx context.Context) ([]*models.Shananigan, error)
	Update(ctx context.Context, id int64, updates map[string]any) error
	Delete(ctx context.Context, id int64) error
	Activate(ctx context.Context, shenaniganID int64) (*PurchaseRecord, error)
}

// ShananiganRepo handles all shenanigan persistence operations.
type ShananiganRepo struct {
	db *sql.DB
}

// NewShananiganRepo creates a new ShananiganRepo with the given database pool.
func NewShananiganRepo(db *sql.DB) *ShananiganRepo {
	return &ShananiganRepo{db: db}
}

// Create inserts a new shenanigan and returns it with the generated ID.
func (r *ShananiganRepo) Create(ctx context.Context, s *models.Shananigan) error {
	query := `
		INSERT INTO shenanigans (name, description, rcon_payload, target_type, cost, metadata)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at, updated_at`

	return r.db.QueryRowContext(ctx, query,
		s.Name,
		s.Description,
		s.RconPayload,
		s.TargetType,
		s.Cost,
		s.Metadata,
	).Scan(&s.ID, &s.CreatedAt, &s.UpdatedAt)
}

// GetByID retrieves a single shenanigan by its ID. Returns ErrNotFound if absent.
func (r *ShananiganRepo) GetByID(ctx context.Context, id int64) (*models.Shananigan, error) {
	s := &models.Shananigan{}
	query := `
		SELECT id, name, description, rcon_payload, target_type, cost, metadata, created_at, updated_at
		FROM shenanigans WHERE id = $1`

	var meta *string
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&s.ID, &s.Name, &s.Description, &s.RconPayload, &s.TargetType,
		&s.Cost, &meta, &s.CreatedAt, &s.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if meta != nil {
		s.Metadata = json.RawMessage(*meta)
	}
	return s, err
}

// GetAll returns all shenanigans ordered by ID ascending.
func (r *ShananiganRepo) GetAll(ctx context.Context) ([]*models.Shananigan, error) {
	query := `
		SELECT id, name, description, rcon_payload, target_type, cost, metadata, created_at, updated_at
		FROM shenanigans ORDER BY id ASC`

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

// Update modifies an existing shenanigan by ID. Pass nil for fields you don't want changed.
func (r *ShananiganRepo) Update(ctx context.Context, id int64, updates map[string]any) error {
	if len(updates) == 0 {
		return nil
	}

	clauses := make([]string, 0, len(updates))
	args := make([]any, 0, len(updates)+1)
	idx := 1

	for k, v := range updates {
		clauses = append(clauses, fmt.Sprintf("%s=$%d", k, idx))
		args = append(args, v)
		idx++
	}

	idx++ // for the WHERE clause parameter
	args = append(args, id)

	query := fmt.Sprintf("UPDATE shenanigans SET %s, updated_at=CURRENT_TIMESTAMP WHERE id=$%d",
		joinStrings(clauses, ", "), idx)

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

// Delete removes a shenanigan by its ID. Returns ErrNotFound if absent.
func (r *ShananiganRepo) Delete(ctx context.Context, id int64) error {
	result, err := r.db.ExecContext(ctx, "DELETE FROM shenanigans WHERE id=$1", id)
	if err != nil {
		return err
	}
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// PurchaseRecord holds the result of activating a shenanigan.
type PurchaseRecord struct {
	PurchaseID  uuid.UUID
	RconPayload string
}

// Activate prepares a purchase record for a shenanigan. It retrieves the
// shenanigan from the database to extract the rcon_payload.
func (r *ShananiganRepo) Activate(ctx context.Context, shenaniganID int64) (*PurchaseRecord, error) {
	s := &models.Shananigan{}
	var meta *string
	err := r.db.QueryRowContext(ctx, `
		SELECT id, name, description, rcon_payload, target_type, cost, metadata, created_at, updated_at
		FROM shenanigans WHERE id = $1`, shenaniganID).Scan(
		&s.ID, &s.Name, &s.Description, &s.RconPayload, &s.TargetType,
		&s.Cost, &meta, &s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if meta != nil {
		s.Metadata = json.RawMessage(*meta)
	}

	return &PurchaseRecord{
		PurchaseID:  uuid.New(),
		RconPayload: s.RconPayload,
	}, nil
}

func joinStrings(ss []string, sep string) string {
	result := ""
	for i, s := range ss {
		if i > 0 {
			result += sep
		}
		result += s
	}
	return result
}
