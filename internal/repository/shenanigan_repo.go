// Package repository provides database access for shenanigans.
package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/forklifts-for-great-justice/judge-service/internal/models"
)

// ErrNotFound is returned when a requested resource does not exist.
var ErrNotFound = errors.New("record not found")

// FilterOptions defines the filter and pagination parameters for GetFiltered.
type FilterOptions struct {
	TargetType string
	MinCost    *int64
	MaxCost    *int64
	Page       int
	PageSize   int
}

// Repository defines the shenanigan persistence interface.
// Concrete implementation in this package; used by the handler.
type Repository interface {
	Create(ctx context.Context, s *models.Shananigan) error
	GetByID(ctx context.Context, id int64) (*models.Shananigan, error)
	GetByIDDeleted(ctx context.Context, id int64) (*models.Shananigan, error)
	GetAll(ctx context.Context) ([]*models.Shananigan, error)
	GetFiltered(ctx context.Context, opts *FilterOptions) ([]*models.Shananigan, int64, error)
	Update(ctx context.Context, id int64, updates map[string]any) error
	Delete(ctx context.Context, id int64) error
	SoftDelete(ctx context.Context, id int64) error
	Activate(ctx context.Context, shenaniganID int64) (*PurchaseRecord, error)
	GetShenaniganByID(ctx context.Context, id int64) (*models.Shananigan, error)
	SaveActivation(ctx context.Context, a *models.ActivationRecord) error
	GetActivationsForShenanigan(ctx context.Context, shenaniganID int64, statusFilter string) ([]*models.ActivationRecord, error)
	GetActivationByPurchaseID(ctx context.Context, purchaseID uuid.UUID) (*models.ActivationRecord, error)
	StatusUpdate(ctx context.Context, purchaseID uuid.UUID, status string) error
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

// GetByID retrieves a single non-deleted shenanigan by its ID. Returns ErrNotFound if absent.
func (r *ShananiganRepo) GetByID(ctx context.Context, id int64) (*models.Shananigan, error) {
	s := &models.Shananigan{}
	query := `
		SELECT id, name, description, rcon_payload, target_type, cost, metadata, created_at, updated_at, deleted_at
		FROM shenanigans WHERE id = $1 AND deleted_at IS NULL`

	var deletedAt *time.Time
	var meta *string
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&s.ID, &s.Name, &s.Description, &s.RconPayload, &s.TargetType,
		&s.Cost, &meta, &s.CreatedAt, &s.UpdatedAt, &deletedAt,
	)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if meta != nil {
		s.Metadata = json.RawMessage(*meta)
	}
	s.DeletedAt = deletedAt
	return s, err
}

// GetByIDDeleted retrieves a shenanigan by ID including soft-deleted records.
func (r *ShananiganRepo) GetByIDDeleted(ctx context.Context, id int64) (*models.Shananigan, error) {
	s := &models.Shananigan{}
	query := `
		SELECT id, name, description, rcon_payload, target_type, cost, metadata, created_at, updated_at, deleted_at
		FROM shenanigans WHERE id = $1`

	var deletedAt *time.Time
	var meta *string
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&s.ID, &s.Name, &s.Description, &s.RconPayload, &s.TargetType,
		&s.Cost, &meta, &s.CreatedAt, &s.UpdatedAt, &deletedAt,
	)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if meta != nil {
		s.Metadata = json.RawMessage(*meta)
	}
	s.DeletedAt = deletedAt
	return s, err
}

// GetAll returns all non-deleted shenanigans ordered by ID ascending.
func (r *ShananiganRepo) GetAll(ctx context.Context) ([]*models.Shananigan, error) {
	query := `
		SELECT id, name, description, rcon_payload, target_type, cost, metadata, created_at, updated_at, deleted_at
		FROM shenanigans WHERE deleted_at IS NULL ORDER BY id ASC`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]*models.Shananigan, 0)
	for rows.Next() {
		s := &models.Shananigan{}
		var deletedAt *time.Time
		var meta *string
		err := rows.Scan(
			&s.ID, &s.Name, &s.Description, &s.RconPayload, &s.TargetType,
			&s.Cost, &meta, &s.CreatedAt, &s.UpdatedAt, &deletedAt,
		)
		if err != nil {
			return nil, err
		}
		if meta != nil {
			s.Metadata = json.RawMessage(*meta)
		}
		s.DeletedAt = deletedAt
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
		cast := ""
		switch v.(type) {
		case string:
			cast = "::text"
		case int, int64:
			cast = "::bigint"
		case *int64:
			cast = "::bigint"
		case json.RawMessage:
			cast = "::jsonb"
		}
		clauses = append(clauses, fmt.Sprintf("%s=$%d%s", k, idx, cast))
		args = append(args, v)
		idx++
	}

	query := fmt.Sprintf("UPDATE shenanigans SET %s, updated_at=CURRENT_TIMESTAMP WHERE id=$%d",
		joinStrings(clauses, ", "), len(updates)+1)
	args = append(args, id)

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

// SoftDelete marks a shenanigan as deleted by setting deleted_at to the current time.
// Returns ErrNotFound if the record does not exist or is already soft-deleted.
func (r *ShananiganRepo) SoftDelete(ctx context.Context, id int64) error {
	result, err := r.db.ExecContext(ctx,
		"UPDATE shenanigans SET deleted_at=CURRENT_TIMESTAMP WHERE id=$1 AND deleted_at IS NULL", id)
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
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
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

// GetFiltered returns shenanigans filtered by query options with pagination.
// Default page_size is 50, max is 200.
// Returns (results, total_count, error).
func (r *ShananiganRepo) GetFiltered(ctx context.Context, opts *FilterOptions) ([]*models.Shananigan, int64, error) {
	if r == nil || r.db == nil {
		return nil, 0, fmt.Errorf("database connection not available")
	}
	if opts == nil {
		opts = &FilterOptions{}
	}

	page := opts.Page
	pageSize := opts.PageSize
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 50
	}

	const maxPageSize = 200
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}

	whereClauses := make([]string, 0)
	args := make([]any, 0, 4)
	idx := 1

	if opts.TargetType != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("target_type=$%d", idx))
		args = append(args, opts.TargetType)
		idx++
	}

	if opts.MinCost != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("cost>=$%d", idx))
		args = append(args, *opts.MinCost)
		idx++
	}

	if opts.MaxCost != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("cost<=$%d", idx))
		args = append(args, *opts.MaxCost)
		idx++
	}

	whereClause := ""
	if len(whereClauses) > 0 {
		whereClause = "WHERE " + joinStrings(whereClauses, " AND ")
	}
	// Always ensure non-deleted records are returned (Phase 1C soft-delete).
	if whereClause != "" {
		whereClause += " AND deleted_at IS NULL"
	} else {
		whereClause = "WHERE deleted_at IS NULL"
	}

	countQuery := fmt.Sprintf(
		"SELECT COUNT(*) FROM shenanigans %s",
		whereClause,
	)
	var total int64
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	dataQuery := fmt.Sprintf(
		"SELECT id, name, description, rcon_payload, target_type, cost, metadata, created_at, updated_at FROM shenanigans %s ORDER BY id ASC LIMIT $%d OFFSET $%d",
		whereClause,
		idx, idx+1,
	)
	newArgs := make([]any, 0, len(args)+2)
	newArgs = append(newArgs, args...)
	newArgs = append(newArgs, pageSize, offset)

	rows, err := r.db.QueryContext(ctx, dataQuery, newArgs...)
	if err != nil {
		return nil, 0, err
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
			return nil, 0, err
		}
		if meta != nil {
			s.Metadata = json.RawMessage(*meta)
		}
		result = append(result, s)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return result, total, nil
}

// GetShenaniganByID retrieves a single non-deleted shenanigan by its ID.
// Behaves identically to GetByID but is named separately so the handler
// can distinguish "shenanigan not found" (404 on the activations route)
// from "activation not found" in other code paths.
func (r *ShananiganRepo) GetShenaniganByID(ctx context.Context, id int64) (*models.Shananigan, error) {
	return r.GetByID(ctx, id)
}

// SaveActivation stores a new activation record in the database.
func (r *ShananiganRepo) SaveActivation(ctx context.Context, a *models.ActivationRecord) error {
	query := `
		INSERT INTO shenanigan_activations
			(purchase_id, shenanigan_id, status, error_message, rcon_payload, metadata)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING created_at, updated_at`

	return r.db.QueryRowContext(ctx, query,
		a.PurchaseID,
		a.ShenaniganID,
		a.Status,
		a.ErrorMessage,
		a.RconPayload,
		a.Metadata,
	).Scan(&a.CreatedAt, &a.UpdatedAt)
}

// GetActivationsForShenanigan returns all activation records for a shenanigan,
// optionally filtered by status.
func (r *ShananiganRepo) GetActivationsForShenanigan(ctx context.Context, shenaniganID int64, statusFilter string) ([]*models.ActivationRecord, error) {
	query := `SELECT purchase_id, shenanigan_id, status, error_message, rcon_payload, metadata, created_at, updated_at FROM shenanigan_activations WHERE shenanigan_id=$1`
	args := []any{shenaniganID}
	idx := 2

	if statusFilter != "" {
		query += fmt.Sprintf(" AND status=$%d", idx)
		args = append(args, statusFilter)
		idx++
	}

	query += " ORDER BY created_at ASC"

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]*models.ActivationRecord, 0)
	for rows.Next() {
		a := &models.ActivationRecord{}
		var errMsg *string
		var meta *string
		err := rows.Scan(
			&a.PurchaseID, &a.ShenaniganID, &a.Status, &errMsg,
			&a.RconPayload, &meta, &a.CreatedAt, &a.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		if errMsg != nil {
			a.ErrorMessage = *errMsg
		}
		if meta != nil {
			a.Metadata = json.RawMessage(*meta)
		}
		result = append(result, a)
	}
	return result, rows.Err()
}

// GetActivationByPurchaseID returns a single activation record by purchase ID.
func (r *ShananiganRepo) GetActivationByPurchaseID(ctx context.Context, purchaseID uuid.UUID) (*models.ActivationRecord, error) {
	a := &models.ActivationRecord{}
	var meta *string
	query := `SELECT purchase_id, shenanigan_id, status, error_message, rcon_payload, metadata, created_at, updated_at FROM shenanigan_activations WHERE purchase_id=$1`

	err := r.db.QueryRowContext(ctx, query, purchaseID).Scan(
		&a.PurchaseID, &a.ShenaniganID, &a.Status, &a.ErrorMessage,
		&a.RconPayload, &meta, &a.CreatedAt, &a.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if meta != nil {
		a.Metadata = json.RawMessage(*meta)
	}
	return a, err
}

// StatusUpdate changes the status of an activation record identified by purchase_id.
func (r *ShananiganRepo) StatusUpdate(ctx context.Context, purchaseID uuid.UUID, status string) error {
	result, err := r.db.ExecContext(ctx,
		"UPDATE shenanigan_activations SET status=$1, updated_at=CURRENT_TIMESTAMP WHERE purchase_id=$2",
		status, purchaseID,
	)
	if err != nil {
		return err
	}
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}
