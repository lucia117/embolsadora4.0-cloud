package dashboard_layouts

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	domain "github.com/tu-org/embolsadora-api/internal/domain/dashboard_layouts"
)

// PostgresRepository implements domain.Repository using PostgreSQL.
type PostgresRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresRepository creates a new PostgreSQL repository for dashboard layouts.
func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

// List returns all active layouts for the (tenant, user), ordered by creation date ascending.
func (r *PostgresRepository) List(ctx context.Context, tenantID, userID uuid.UUID) ([]*domain.DashboardLayout, error) {
	query := `
		SELECT id, tenant_id, user_id, name, widgets, created_at, updated_at
		FROM dashboard_layouts
		WHERE tenant_id = $1 AND user_id = $2 AND deleted_at IS NULL
		ORDER BY created_at ASC
	`
	rows, err := r.pool.Query(ctx, query, tenantID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var layouts []*domain.DashboardLayout
	for rows.Next() {
		layout, err := scanLayout(rows)
		if err != nil {
			return nil, err
		}
		layouts = append(layouts, layout)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return layouts, nil
}

// GetByID returns a single active layout by ID within the (tenant, user) scope.
func (r *PostgresRepository) GetByID(ctx context.Context, tenantID, userID, layoutID uuid.UUID) (*domain.DashboardLayout, error) {
	query := `
		SELECT id, tenant_id, user_id, name, widgets, created_at, updated_at
		FROM dashboard_layouts
		WHERE id = $1 AND tenant_id = $2 AND user_id = $3 AND deleted_at IS NULL
	`
	row := r.pool.QueryRow(ctx, query, layoutID, tenantID, userID)
	layout, err := scanLayout(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrLayoutNotFound
		}
		return nil, err
	}
	return layout, nil
}

// CountByTenantUser returns the number of active layouts for the (tenant, user) pair.
func (r *PostgresRepository) CountByTenantUser(ctx context.Context, tenantID, userID uuid.UUID) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM dashboard_layouts WHERE tenant_id = $1 AND user_id = $2 AND deleted_at IS NULL`,
		tenantID, userID,
	).Scan(&count)
	return count, err
}

// Create persists a new layout and populates server-assigned fields.
// The limit check and insert are executed within a single transaction using SELECT FOR UPDATE
// so that concurrent requests cannot both pass the count check and exceed the per-(tenant,user) limit.
func (r *PostgresRepository) Create(ctx context.Context, layout *domain.DashboardLayout) error {
	widgetsJSON, err := json.Marshal(layout.Widgets)
	if err != nil {
		return err
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Lock all active layouts for this (tenant, user) to serialize concurrent creates.
	lockRows, err := tx.Query(ctx,
		`SELECT id FROM dashboard_layouts WHERE tenant_id = $1 AND user_id = $2 AND deleted_at IS NULL FOR UPDATE`,
		layout.TenantID, layout.UserID,
	)
	if err != nil {
		return err
	}
	var count int
	for lockRows.Next() {
		var id uuid.UUID
		if err := lockRows.Scan(&id); err != nil {
			lockRows.Close()
			return err
		}
		count++
	}
	lockRows.Close()
	if err := lockRows.Err(); err != nil {
		return err
	}
	if count >= domain.MaxLayoutsPerUser {
		return domain.ErrLimitReached
	}

	if err := tx.QueryRow(ctx, `
		INSERT INTO dashboard_layouts (id, tenant_id, user_id, name, widgets)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING created_at, updated_at
	`, layout.ID, layout.TenantID, layout.UserID, layout.Name, widgetsJSON,
	).Scan(&layout.CreatedAt, &layout.UpdatedAt); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domain.ErrDuplicateName
		}
		return err
	}

	return tx.Commit(ctx)
}

// Update replaces name and widgets of an existing layout.
func (r *PostgresRepository) Update(ctx context.Context, layout *domain.DashboardLayout) error {
	widgetsJSON, err := json.Marshal(layout.Widgets)
	if err != nil {
		return err
	}

	err = r.pool.QueryRow(ctx, `
		UPDATE dashboard_layouts
		SET name = $1, widgets = $2
		WHERE id = $3 AND tenant_id = $4 AND user_id = $5 AND deleted_at IS NULL
		RETURNING updated_at
	`, layout.Name, widgetsJSON, layout.ID, layout.TenantID, layout.UserID).Scan(&layout.UpdatedAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrLayoutNotFound
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domain.ErrDuplicateName
		}
		return err
	}
	return nil
}

// SoftDelete sets deleted_at on the layout.
// The existence check, count check, and delete are executed within a single transaction
// using SELECT FOR UPDATE to prevent concurrent deletes from leaving a user with zero layouts.
func (r *PostgresRepository) SoftDelete(ctx context.Context, tenantID, userID, layoutID uuid.UUID) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Lock all active layouts for this (tenant, user) to serialize concurrent deletes.
	rows, err := tx.Query(ctx,
		`SELECT id FROM dashboard_layouts WHERE tenant_id = $1 AND user_id = $2 AND deleted_at IS NULL FOR UPDATE`,
		tenantID, userID,
	)
	if err != nil {
		return err
	}

	var count int
	var found bool
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		count++
		if id == layoutID {
			found = true
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	if !found {
		return domain.ErrLayoutNotFound
	}
	if count <= 1 {
		return domain.ErrCannotDeleteLastLayout
	}

	if _, err := tx.Exec(ctx, `
		UPDATE dashboard_layouts
		SET deleted_at = CURRENT_TIMESTAMP
		WHERE id = $1 AND tenant_id = $2 AND user_id = $3 AND deleted_at IS NULL
	`, layoutID, tenantID, userID); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// scanLayout reads a layout row from either pgx.Rows or pgx.Row.
func scanLayout(row interface {
	Scan(dest ...any) error
}) (*domain.DashboardLayout, error) {
	var layout domain.DashboardLayout
	var widgetsJSON []byte

	err := row.Scan(
		&layout.ID,
		&layout.TenantID,
		&layout.UserID,
		&layout.Name,
		&widgetsJSON,
		&layout.CreatedAt,
		&layout.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(widgetsJSON, &layout.Widgets); err != nil {
		return nil, err
	}
	if layout.Widgets == nil {
		layout.Widgets = []domain.Widget{}
	}
	return &layout, nil
}
