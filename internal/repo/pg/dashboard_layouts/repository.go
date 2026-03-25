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

// List returns all active layouts for the tenant, ordered by creation date ascending.
func (r *PostgresRepository) List(ctx context.Context, tenantID uuid.UUID) ([]*domain.DashboardLayout, error) {
	query := `
		SELECT id, tenant_id, name, widgets, created_at, updated_at
		FROM dashboard_layouts
		WHERE tenant_id = $1 AND deleted_at IS NULL
		ORDER BY created_at ASC
	`
	rows, err := r.pool.Query(ctx, query, tenantID)
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

// GetByID returns a single active layout by ID within the tenant.
func (r *PostgresRepository) GetByID(ctx context.Context, tenantID, layoutID uuid.UUID) (*domain.DashboardLayout, error) {
	query := `
		SELECT id, tenant_id, name, widgets, created_at, updated_at
		FROM dashboard_layouts
		WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL
	`
	row := r.pool.QueryRow(ctx, query, layoutID, tenantID)
	layout, err := scanLayout(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrLayoutNotFound
		}
		return nil, err
	}
	return layout, nil
}

// CountByTenant returns the number of active layouts for the tenant.
func (r *PostgresRepository) CountByTenant(ctx context.Context, tenantID uuid.UUID) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM dashboard_layouts WHERE tenant_id = $1 AND deleted_at IS NULL`,
		tenantID,
	).Scan(&count)
	return count, err
}

// ExistsByName returns true if an active layout with the given name exists in the tenant.
// When excludeID is non-nil, that layout is excluded from the check.
func (r *PostgresRepository) ExistsByName(ctx context.Context, tenantID uuid.UUID, name string, excludeID *uuid.UUID) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM dashboard_layouts
			WHERE tenant_id = $1
			  AND name = $2
			  AND deleted_at IS NULL
			  AND ($3::uuid IS NULL OR id != $3)
		)
	`, tenantID, name, excludeID).Scan(&exists)
	return exists, err
}

// Create persists a new layout and populates server-assigned fields.
func (r *PostgresRepository) Create(ctx context.Context, layout *domain.DashboardLayout) error {
	widgetsJSON, err := json.Marshal(layout.Widgets)
	if err != nil {
		return err
	}

	err = r.pool.QueryRow(ctx, `
		INSERT INTO dashboard_layouts (id, tenant_id, name, widgets)
		VALUES ($1, $2, $3, $4)
		RETURNING created_at, updated_at
	`,
		layout.ID, layout.TenantID, layout.Name, widgetsJSON,
	).Scan(&layout.CreatedAt, &layout.UpdatedAt)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domain.ErrDuplicateName
		}
		return err
	}
	return nil
}

// Update replaces name and widgets of an existing layout.
func (r *PostgresRepository) Update(ctx context.Context, layout *domain.DashboardLayout) error {
	widgetsJSON, err := json.Marshal(layout.Widgets)
	if err != nil {
		return err
	}

	tag, err := r.pool.Exec(ctx, `
		UPDATE dashboard_layouts
		SET name = $1, widgets = $2
		WHERE id = $3 AND tenant_id = $4 AND deleted_at IS NULL
	`, layout.Name, widgetsJSON, layout.ID, layout.TenantID)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domain.ErrDuplicateName
		}
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrLayoutNotFound
	}

	// Refresh updated_at from DB
	return r.pool.QueryRow(ctx,
		`SELECT updated_at FROM dashboard_layouts WHERE id = $1`,
		layout.ID,
	).Scan(&layout.UpdatedAt)
}

// SoftDelete sets deleted_at on the layout.
func (r *PostgresRepository) SoftDelete(ctx context.Context, tenantID, layoutID uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE dashboard_layouts
		SET deleted_at = CURRENT_TIMESTAMP
		WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL
	`, layoutID, tenantID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrLayoutNotFound
	}
	return nil
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
