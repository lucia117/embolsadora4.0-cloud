package permissions

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/tu-org/embolsadora-api/internal/domain"
)

// Repository define las operaciones de persistencia para permisos.
type Repository interface {
	List(ctx context.Context, tenantID uuid.UUID) ([]*domain.Permission, error)
	GetByID(ctx context.Context, id string, tenantID uuid.UUID) (*domain.Permission, error)
	Create(ctx context.Context, p *domain.Permission) error
	Update(ctx context.Context, p *domain.Permission) error
	Delete(ctx context.Context, id string, tenantID uuid.UUID) error
}

// PostgresRepository implementa Repository usando PostgreSQL.
type PostgresRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresRepository crea un nuevo repositorio PostgreSQL para permisos.
func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

// List devuelve todos los permisos de sistema (globales) más los permisos custom del tenant,
// ordenados: permisos de sistema primero, luego alfabéticamente por nombre.
func (r *PostgresRepository) List(ctx context.Context, tenantID uuid.UUID) ([]*domain.Permission, error) {
	query := `
		SELECT id, name, section, description, is_system_permission, tenant_id, created_at, updated_at
		FROM permissions
		WHERE (tenant_id = $1 OR is_system_permission = TRUE)
		ORDER BY is_system_permission DESC, name ASC
	`
	rows, err := r.pool.Query(ctx, query, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var perms []*domain.Permission
	for rows.Next() {
		p, err := scanPermission(rows)
		if err != nil {
			return nil, err
		}
		perms = append(perms, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if perms == nil {
		perms = []*domain.Permission{}
	}
	return perms, nil
}

// GetByID devuelve un permiso por su ID.
// Para permisos de sistema (is_system_permission=TRUE) no aplica filtro de tenant.
// Para permisos custom, solo devuelve el permiso si pertenece al tenant indicado.
func (r *PostgresRepository) GetByID(ctx context.Context, id string, tenantID uuid.UUID) (*domain.Permission, error) {
	query := `
		SELECT id, name, section, description, is_system_permission, tenant_id, created_at, updated_at
		FROM permissions
		WHERE id = $1 AND (is_system_permission = TRUE OR tenant_id = $2)
	`
	row := r.pool.QueryRow(ctx, query, id, tenantID)
	p, err := scanPermission(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrPermissionNotFound
		}
		return nil, err
	}
	return p, nil
}

// Create inserta un nuevo permiso custom. El ID debe ser provisto por el caller.
func (r *PostgresRepository) Create(ctx context.Context, p *domain.Permission) error {
	query := `
		INSERT INTO permissions (id, name, section, description, is_system_permission, tenant_id)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	_, err := r.pool.Exec(ctx, query,
		p.ID, p.Name, p.Section, p.Description, p.IsSystemPermission, p.TenantID,
	)
	return err
}

// Update actualiza nombre, sección y descripción de un permiso custom existente.
// La guarda AND is_system_permission=FALSE evita modificar permisos de sistema en BD directamente.
func (r *PostgresRepository) Update(ctx context.Context, p *domain.Permission) error {
	query := `
		UPDATE permissions
		SET name = $1, section = $2, description = $3
		WHERE id = $4 AND is_system_permission = FALSE
	`
	tag, err := r.pool.Exec(ctx, query, p.Name, p.Section, p.Description, p.ID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrPermissionNotFound
	}
	return nil
}

// Delete elimina permanentemente un permiso custom del tenant.
// Retorna ErrPermissionIsSystem si es de sistema, ErrPermissionNotFound si no existe o no pertenece al tenant.
func (r *PostgresRepository) Delete(ctx context.Context, id string, tenantID uuid.UUID) error {
	// Verificar existencia y tipo antes de eliminar (con filtro de tenant)
	p, err := r.GetByID(ctx, id, tenantID)
	if err != nil {
		return err
	}
	if p.IsSystemPermission {
		return domain.ErrPermissionIsSystem
	}

	query := `DELETE FROM permissions WHERE id = $1 AND is_system_permission = FALSE AND tenant_id = $2`
	tag, err := r.pool.Exec(ctx, query, id, tenantID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrPermissionNotFound
	}
	return nil
}

// scanner es una interfaz que abarca pgx.Row y pgx.Rows para reutilizar scanPermission.
type scanner interface {
	Scan(dest ...any) error
}

func scanPermission(s scanner) (*domain.Permission, error) {
	var p domain.Permission
	err := s.Scan(
		&p.ID,
		&p.Name,
		&p.Section,
		&p.Description,
		&p.IsSystemPermission,
		&p.TenantID,
		&p.CreatedAt,
		&p.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &p, nil
}
