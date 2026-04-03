package roles

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/tu-org/embolsadora-api/internal/domain"
)

// Repository define las operaciones de persistencia para roles.
type Repository interface {
	List(ctx context.Context, tenantID uuid.UUID) ([]*domain.Role, error)
	GetByID(ctx context.Context, id string) (*domain.Role, error)
	CountCustomByTenant(ctx context.Context, tenantID uuid.UUID) (int, error)
	Create(ctx context.Context, role *domain.Role) error
	Update(ctx context.Context, role *domain.Role) error
	SoftDelete(ctx context.Context, id string) error
	CountActiveAssignments(ctx context.Context, roleID string) (int, error)
}

// PostgresRepository implementa Repository usando PostgreSQL.
type PostgresRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresRepository crea un nuevo repositorio PostgreSQL para roles.
func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

// List devuelve todos los roles activos del tenant más los roles globales del sistema,
// ordenados: roles del sistema primero, luego alfabéticamente por nombre.
func (r *PostgresRepository) List(ctx context.Context, tenantID uuid.UUID) ([]*domain.Role, error) {
	query := `
		SELECT id, name, description, is_system_role, is_global, tenant_id, permissions, created_at, updated_at
		FROM roles
		WHERE (tenant_id = $1 OR is_global = TRUE) AND deleted_at IS NULL
		ORDER BY is_system_role DESC, name ASC
	`
	rows, err := r.pool.Query(ctx, query, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var roles []*domain.Role
	for rows.Next() {
		role, err := scanRole(rows)
		if err != nil {
			return nil, err
		}
		roles = append(roles, role)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if roles == nil {
		roles = []*domain.Role{}
	}
	return roles, nil
}

// GetByID devuelve un rol por su ID (activo o del sistema).
func (r *PostgresRepository) GetByID(ctx context.Context, id string) (*domain.Role, error) {
	query := `
		SELECT id, name, description, is_system_role, is_global, tenant_id, permissions, created_at, updated_at
		FROM roles
		WHERE id = $1 AND deleted_at IS NULL
	`
	row := r.pool.QueryRow(ctx, query, id)
	role, err := scanRole(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrRoleNotFound
		}
		return nil, err
	}
	return role, nil
}

// CountCustomByTenant cuenta los roles personalizados activos de un tenant.
func (r *PostgresRepository) CountCustomByTenant(ctx context.Context, tenantID uuid.UUID) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM roles WHERE tenant_id = $1 AND is_system_role = FALSE AND deleted_at IS NULL`,
		tenantID,
	).Scan(&count)
	return count, err
}

// Create persiste un nuevo rol personalizado.
func (r *PostgresRepository) Create(ctx context.Context, role *domain.Role) error {
	permJSON, err := json.Marshal(role.Permissions)
	if err != nil {
		return err
	}

	err = r.pool.QueryRow(ctx, `
		INSERT INTO roles (id, name, description, is_system_role, is_global, tenant_id, permissions)
		VALUES ($1, $2, $3, FALSE, FALSE, $4, $5)
		RETURNING created_at, updated_at
	`, role.ID, role.Name, role.Description, role.TenantID, permJSON,
	).Scan(&role.CreatedAt, &role.UpdatedAt)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domain.ErrRoleDuplicateName
		}
		return err
	}
	return nil
}

// Update modifica nombre, descripción y permisos de un rol personalizado.
func (r *PostgresRepository) Update(ctx context.Context, role *domain.Role) error {
	permJSON, err := json.Marshal(role.Permissions)
	if err != nil {
		return err
	}

	err = r.pool.QueryRow(ctx, `
		UPDATE roles
		SET name = $1, description = $2, permissions = $3, updated_at = NOW()
		WHERE id = $4 AND deleted_at IS NULL
		RETURNING updated_at
	`, role.Name, role.Description, permJSON, role.ID,
	).Scan(&role.UpdatedAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrRoleNotFound
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domain.ErrRoleDuplicateName
		}
		return err
	}
	return nil
}

// SoftDelete marca un rol como eliminado.
func (r *PostgresRepository) SoftDelete(ctx context.Context, id string) error {
	result, err := r.pool.Exec(ctx,
		`UPDATE roles SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL`,
		id,
	)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return domain.ErrRoleNotFound
	}
	return nil
}

// CountActiveAssignments cuenta los user_tenant_roles activos que usan este rol.
func (r *PostgresRepository) CountActiveAssignments(ctx context.Context, roleID string) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM user_tenant_roles WHERE role_id = $1 AND status = 'active'`,
		roleID,
	).Scan(&count)
	return count, err
}

// scanRole lee una fila de rol desde pgx.Rows o pgx.Row.
func scanRole(row interface {
	Scan(dest ...any) error
}) (*domain.Role, error) {
	var role domain.Role
	var permJSON []byte
	var tenantID *uuid.UUID
	var description *string // nullable en la tabla (migration 000003 define TEXT sin NOT NULL)

	err := row.Scan(
		&role.ID,
		&role.Name,
		&description,
		&role.IsSystemRole,
		&role.IsGlobal,
		&tenantID,
		&permJSON,
		&role.CreatedAt,
		&role.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	if description != nil {
		role.Description = *description
	}
	role.TenantID = tenantID

	if err := json.Unmarshal(permJSON, &role.Permissions); err != nil {
		return nil, err
	}
	if role.Permissions == nil {
		role.Permissions = []string{}
	}
	return &role, nil
}
