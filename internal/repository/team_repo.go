// Package repository provides database access for teams.
package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/forklifts-for-great-justice/judge-service/internal/models"
)

// TeamRepository defines the team persistence interface.
// Concrete implementation in this package; used by the handler.
type TeamRepository interface {
	Create(ctx context.Context, t *models.Team) error
	GetByID(ctx context.Context, id int64) (*models.Team, error)
	Update(ctx context.Context, id int64, updates map[string]any) error
	Delete(ctx context.Context, id int64) error
	GetAll(ctx context.Context) ([]*models.Team, error)
}

// ErrDuplicate is returned when a duplicate key violation occurs.
var ErrDuplicate = errors.New("duplicate key violation")

func isDuplicatePostgres(err error) bool {
	return err != nil && strings.Contains(err.Error(), "duplicate")
}

// TeamRepo handles all team persistence operations.
type TeamRepo struct {
	db *sql.DB
}

// NewTeamRepo creates a new TeamRepo with the given database pool.
func NewTeamRepo(db *sql.DB) *TeamRepo {
	return &TeamRepo{db: db}
}

// Create inserts a new team and returns it with the generated ID.
func (r *TeamRepo) Create(ctx context.Context, t *models.Team) error {
	query := `
		INSERT INTO team (slug, name, alt_name, clan_tag)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at, updated_at`

	err := r.db.QueryRowContext(ctx, query,
		t.Slug,
		t.Name,
		t.AltName,
		t.ClanTag,
	).Scan(&t.ID, &t.CreatedAt, &t.UpdatedAt)
	if err != nil && isDuplicatePostgres(err) {
		return ErrDuplicate
	}
	return err
}

// GetByID retrieves a single team by its ID. Returns ErrNotFound if absent.
func (r *TeamRepo) GetByID(ctx context.Context, id int64) (*models.Team, error) {
	t := &models.Team{}
	query := `
		SELECT id, slug, name, alt_name, clan_tag, created_at, updated_at
		FROM team WHERE id = $1`

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&t.ID, &t.Slug, &t.Name, &t.AltName, &t.ClanTag,
		&t.CreatedAt, &t.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	return t, err
}

// GetAll returns all teams ordered by ID ascending.
func (r *TeamRepo) GetAll(ctx context.Context) ([]*models.Team, error) {
	query := `
		SELECT id, slug, name, alt_name, clan_tag, created_at, updated_at
		FROM team ORDER BY id ASC`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]*models.Team, 0)
	for rows.Next() {
		t := &models.Team{}
		err := rows.Scan(
			&t.ID, &t.Slug, &t.Name, &t.AltName, &t.ClanTag,
			&t.CreatedAt, &t.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		result = append(result, t)
	}
	return result, rows.Err()
}

// Update modifies an existing team by ID. Pass nil for fields you don't want changed.
// Note: updated_at is NOT included in the SET clause — the PostgreSQL trigger handles it.
func (r *TeamRepo) Update(ctx context.Context, id int64, updates map[string]any) error {
	if len(updates) == 0 {
		return nil
	}

	clauses := make([]string, 0, len(updates))
	args := make([]any, 0, len(updates)+1)
	idx := 1

	for k, v := range updates {
		// Skip updated_at — the trigger handles it.
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

	idx++ // for the WHERE clause parameter
	args = append(args, id)

	query := fmt.Sprintf("UPDATE team SET %s WHERE id=$%d",
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

// Delete removes a team by its ID. Returns ErrNotFound if absent.
func (r *TeamRepo) Delete(ctx context.Context, id int64) error {
	result, err := r.db.ExecContext(ctx, "DELETE FROM team WHERE id=$1", id)
	if err != nil {
		return err
	}
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}
