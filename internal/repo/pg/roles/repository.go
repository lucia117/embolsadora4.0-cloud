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
	// List devuelve los roles visibles para el tenant. includeGlobal habilita
	// los roles is_global (super_admin, tenant_manager); solo debe ser true
	// cuando el caller es super_admin (security.CanSeePlatformInternals).
	List(ctx context.Context, tenantID uuid.UUID, includeGlobal bool) ([]*domain.Role, error)
	GetByIDForTenant(ctx context.Context, id string, tenantID uuid.UUID, includeGlobal bool) (*domain.Role, error)
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

// List devuelve los roles activos del tenant más los roles archetype del sistema
// (tenant_id IS NULL, is_global=false), ordenados: roles del sistema primero, luego
// alfabéticamente por nombre.
//
// Dos ejes de visibilidad se combinan acá:
//  1. tenant_can_use_role() (migración 000004): los roles is_global solo existen
//     dentro del tenant plataforma de MRG.
//  2. includeGlobal: aun dentro del tenant plataforma, los roles is_global solo
//     son visibles para super_admin. Un platform_admin administra la plataforma
//     sin saber que la capa super_admin existe.
func (r *PostgresRepository) List(ctx context.Context, tenantID uuid.UUID, includeGlobal bool) ([]*domain.Role, error) {
	query := `
		SELECT id, name, description, is_system_role, is_global, tenant_id, permissions, created_at, updated_at
		FROM roles
		WHERE (tenant_id = $1 OR tenant_id IS NULL)
		  AND deleted_at IS NULL
		  AND tenant_can_use_role($1, is_global)
		  AND (NOT is_global OR $2)
		ORDER BY is_system_role DESC, name ASC
	`
	rows, err := r.pool.Query(ctx, query, tenantID, includeGlobal)
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

// GetByIDForTenant devuelve un rol por su ID, aplicando la misma regla de visibilidad
// que List. Dos ejes se combinan en el WHERE, ninguno opcional:
//  1. Scoping por tenant: el rol tiene que pertenecer al tenant que pregunta
//     (tenant_id = $2) o ser un archetype reusable por cualquier tenant
//     (tenant_id IS NULL) — la misma condición que usa List. Sin esto, cualquier
//     admin que conociera el id de un rol custom de otro tenant podía leerlo/
//     editarlo/borrarlo con solo apuntar UpdateRole/DeleteRole a GetByID, que no
//     filtraba por tenant en absoluto.
//  2. tenant_can_use_role() + includeGlobal: los roles is_global=TRUE (super_admin,
//     tenant_manager) solo son visibles dentro del tenant plataforma de MRG, y
//     ahí solo cuando includeGlobal es true (caller super_admin).
//
// Devuelve ErrRoleNotFound tanto si el rol no existe como si existe pero no es
// visible para este caller (mismo error en ambos casos: un 403 en su lugar
// confirmaría que el rol existe, exactamente lo que esta función evita).
func (r *PostgresRepository) GetByIDForTenant(ctx context.Context, id string, tenantID uuid.UUID, includeGlobal bool) (*domain.Role, error) {
	query := `
		SELECT id, name, description, is_system_role, is_global, tenant_id, permissions, created_at, updated_at
		FROM roles
		WHERE id = $1
		  AND deleted_at IS NULL
		  AND (tenant_id = $2 OR tenant_id IS NULL)
		  AND tenant_can_use_role($2, is_global)
		  AND (NOT is_global OR $3)
	`
	row := r.pool.QueryRow(ctx, query, id, tenantID, includeGlobal)
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
