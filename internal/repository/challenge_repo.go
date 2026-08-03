// Package repository provides database access for challenges.
package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/forklifts-for-great-justice/judge-service/internal/models"
)

// ChallengeRepository defines the challenge persistence interface.
type ChallengeRepository interface {
	Create(ctx context.Context, c *models.Challenge) error
	GetByID(ctx context.Context, id int64) (*models.Challenge, error)
	Update(ctx context.Context, id int64, updates map[string]any) error
	SoftDelete(ctx context.Context, id int64) error
	GetAll(ctx context.Context) ([]*models.Challenge, error)
}

// ErrChallengeNotFound is the error returned when a challenge is not found.
// Uses same type as ErrNotFound so handler equality checks work.
var ErrChallengeNotFound = ErrNotFound

type ChallengeRepo struct {
	db *sql.DB
}

func NewChallengeRepo(db *sql.DB) *ChallengeRepo {
	return &ChallengeRepo{db: db}
}

// nullStringToPointer converts sql.NullString to a *string, returning nil if invalid.
func nullStringToPointer(ns sql.NullString) *string {
	if !ns.Valid {
		return nil
	}
	v := ns.String
	return &v
}

// nullBoolToPointer converts sql.NullBool to a *bool, returning nil if invalid.
func nullBoolToPointer(nb sql.NullBool) *bool {
	if !nb.Valid {
		return nil
	}
	v := nb.Bool
	return &v
}

// Create inserts a new challenge. Checks for duplicate name.
func (r *ChallengeRepo) Create(ctx context.Context, c *models.Challenge) error {
	query := `
		INSERT INTO challenge (name, description, challenge_type, location, points, disabled, flag)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at, updated_at`

	err := r.db.QueryRowContext(ctx, query,
		c.Name,
		c.Description,
		c.ChallengeType,
		c.Location,
		c.Points,
		c.Disabled,
		c.Flag,
	).Scan(&c.ID, &c.CreatedAt, &c.UpdatedAt)
	if err != nil && IsDuplicatePostgres(err) {
		return ErrDuplicate
	}
	return err
}

// GetByID retrieves a single challenge by ID. Returns ErrNotFound if absent.
// Nullable columns (challenge_type, location) are scanned via sql.NullString helpers.
func (r *ChallengeRepo) GetByID(ctx context.Context, id int64) (*models.Challenge, error) {
	c := &models.Challenge{}
	query := `
		SELECT id, name, description, challenge_type, location, points, disabled, flag, created_at, updated_at
		FROM challenge WHERE id = $1`

	var ctype sql.NullString
	var loc sql.NullString
	var disabled sql.NullBool

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&c.ID, &c.Name, &c.Description, &ctype, &loc,
		&c.Points, &disabled, &c.Flag,
		&c.CreatedAt, &c.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	c.ChallengeType = nullStringToPointer(ctype)
	c.Location = nullStringToPointer(loc)
	if disabled.Valid {
		c.Disabled = disabled.Bool
	}
	return c, err
}

// GetAll returns all challenges ordered by ID ascending. Never returns nil.
func (r *ChallengeRepo) GetAll(ctx context.Context) ([]*models.Challenge, error) {
	query := `
		SELECT id, name, description, challenge_type, location, points, disabled, flag, created_at, updated_at
		FROM challenge WHERE disabled = false ORDER BY id ASC`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]*models.Challenge, 0)
	for rows.Next() {
		c := &models.Challenge{}
		var ctype sql.NullString
		var loc sql.NullString
		var disabled sql.NullBool
		err := rows.Scan(
			&c.ID, &c.Name, &c.Description, &ctype, &loc,
			&c.Points, &disabled, &c.Flag,
			&c.CreatedAt, &c.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		c.ChallengeType = nullStringToPointer(ctype)
		c.Location = nullStringToPointer(loc)
		if disabled.Valid {
			c.Disabled = disabled.Bool
		}
		result = append(result, c)
	}
	return result, rows.Err()
}

// Update modifies a challenge by ID. The "updated_at" key is skipped — trigger handles it.
func (r *ChallengeRepo) Update(ctx context.Context, id int64, updates map[string]any) error {
	if len(updates) == 0 {
		return nil
	}

	clauses := make([]string, 0, len(updates))
	args := make([]any, 0, len(updates)+1)
	idx := 1

	for k, v := range updates {
		if k == "updated_at" {
			continue
		}
		clauses = append(clauses, fmt.Sprintf("%s=$%d", k, idx))
		args = append(args, v)
		idx++
	}

	if len(clauses) == 0 {
		return nil
	}

	args = append(args, id)

	query := fmt.Sprintf("UPDATE challenge SET %s WHERE id=$%d",
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

// SoftDelete sets disabled = true. Preserves unique name constraint for re-creation.
func (r *ChallengeRepo) SoftDelete(ctx context.Context, id int64) error {
	result, err := r.db.ExecContext(ctx,
		"UPDATE challenge SET disabled = true WHERE id = $1 AND disabled = false", id)
	if err != nil {
		return err
	}
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return ErrChallengeNotFound
	}
	return nil
}
