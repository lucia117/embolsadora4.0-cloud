package users

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tu-org/embolsadora-api/internal/domain/users"
)

// PostgresRepository implements Repository using PostgreSQL
type PostgresRepository struct {
	db *pgxpool.Pool
}

// NewPostgresRepository creates a new PostgreSQL repository
func NewPostgresRepository(db *pgxpool.Pool) Repository {
	return &PostgresRepository{db: db}
}

// ListByTenant retrieves paginated users for a tenant, excluding soft-deleted
func (r *PostgresRepository) ListByTenant(ctx context.Context, tenantID string, limit, offset int) ([]*users.User, int64, error) {
	// Get total count
	var totalCount int64
	countQuery := `SELECT COUNT(*) FROM users WHERE tenant_id = $1 AND deleted_at IS NULL`
	if err := r.db.QueryRow(ctx, countQuery, tenantID).Scan(&totalCount); err != nil {
		return nil, 0, fmt.Errorf("failed to count users: %w", err)
	}

	// Get paginated results
	query := `
		SELECT id, tenant_id, first_name, last_name, email, role, image, created_at, updated_at, deleted_at
		FROM users
		WHERE tenant_id = $1 AND deleted_at IS NULL
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.db.Query(ctx, query, tenantID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query users: %w", err)
	}
	defer rows.Close()

	var result []*users.User
	for rows.Next() {
		user, err := scanUser(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan user: %w", err)
		}
		result = append(result, user)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("error iterating users: %w", err)
	}

	return result, totalCount, nil
}

// GetByID retrieves a single user by ID (returns ErrNotFound if soft-deleted)
func (r *PostgresRepository) GetByID(ctx context.Context, tenantID, userID string) (*users.User, error) {
	query := `
		SELECT id, tenant_id, first_name, last_name, email, role, image, created_at, updated_at, deleted_at
		FROM users
		WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL
	`

	row := r.db.QueryRow(ctx, query, userID, tenantID)
	user, err := scanUser(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, users.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return user, nil
}

// Create inserts a new user with server-generated fields
func (r *PostgresRepository) Create(ctx context.Context, user *users.User) (*users.User, error) {
	if err := user.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %v", users.ErrValidation, err)
	}

	now := time.Now().UTC()
	user.CreatedAt = now
	user.UpdatedAt = now
	user.DeletedAt = nil

	query := `
		INSERT INTO users (tenant_id, first_name, last_name, email, role, image, created_at, updated_at, deleted_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, tenant_id, first_name, last_name, email, role, image, created_at, updated_at, deleted_at
	`

	row := r.db.QueryRow(ctx, query, user.TenantID, user.FirstName, user.LastName, user.Email, user.Role, user.Image, user.CreatedAt, user.UpdatedAt, user.DeletedAt)
	created, err := scanUser(row)
	if err != nil {
		// Check for unique constraint violation (duplicate email in tenant)
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" { // UNIQUE_VIOLATION
			return nil, users.ErrEmailTaken
		}
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return created, nil
}

// Update modifies user fields (name, role, image only - immutable fields protected)
func (r *PostgresRepository) Update(ctx context.Context, user *users.User) (*users.User, error) {
	if err := user.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %v", users.ErrValidation, err)
	}

	// updated_at is auto-updated by trigger, but we ensure it's set
	user.UpdatedAt = time.Now().UTC()

	query := `
		UPDATE users
		SET first_name = $1, last_name = $2, role = $3, image = $4, updated_at = $5
		WHERE id = $6 AND tenant_id = $7 AND deleted_at IS NULL
		RETURNING id, tenant_id, first_name, last_name, email, role, image, created_at, updated_at, deleted_at
	`

	row := r.db.QueryRow(ctx, query, user.FirstName, user.LastName, user.Role, user.Image, user.UpdatedAt, user.ID, user.TenantID)
	updated, err := scanUser(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, users.ErrNotFound
		}
		return nil, fmt.Errorf("failed to update user: %w", err)
	}

	return updated, nil
}

// Delete soft-deletes a user by setting deleted_at
func (r *PostgresRepository) Delete(ctx context.Context, tenantID, userID string) error {
	query := `UPDATE users SET deleted_at = CURRENT_TIMESTAMP WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL`

	result, err := r.db.Exec(ctx, query, userID, tenantID)
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	if result.RowsAffected() == 0 {
		return users.ErrNotFound
	}

	return nil
}

// scanUser maps a row to a User struct
func scanUser(row interface {
	Scan(dest ...interface{}) error
}) (*users.User, error) {
	var user users.User
	err := row.Scan(
		&user.ID,
		&user.TenantID,
		&user.FirstName,
		&user.LastName,
		&user.Email,
		&user.Role,
		&user.Image,
		&user.CreatedAt,
		&user.UpdatedAt,
		&user.DeletedAt,
	)
	if err != nil {
		return nil, err
	}
	return &user, nil
}
