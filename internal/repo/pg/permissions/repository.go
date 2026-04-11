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
	GetByID(ctx context.Context, id string) (*domain.Permission, error)
	Create(ctx context.Context, p *domain.Permission) error
	Update(ctx context.Context, p *domain.Permission) error
	Delete(ctx context.Context, id string) error
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
// No filtra por tenant: permite acceder a permisos de sistema (tenant_id NULL) sin requerir tenant_id.
func (r *PostgresRepository) GetByID(ctx context.Context, id string) (*domain.Permission, error) {
	query := `
		SELECT id, name, section, description, is_system_permission, tenant_id, created_at, updated_at
		FROM permissions
		WHERE id = $1
	`
	row := r.pool.QueryRow(ctx, query, id)
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
func (r *PostgresRepository) Update(ctx context.Context, p *domain.Permission) error {
	query := `
		UPDATE permissions
		SET name = $1, section = $2, description = $3
		WHERE id = $4
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

// Delete elimina permanentemente un permiso. Retorna ErrPermissionIsSystem si es de sistema,
// ErrPermissionNotFound si no existe.
func (r *PostgresRepository) Delete(ctx context.Context, id string) error {
	// Verificar existencia y tipo antes de eliminar
	p, err := r.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if p.IsSystemPermission {
		return domain.ErrPermissionIsSystem
	}

	query := `DELETE FROM permissions WHERE id = $1`
	_, err = r.pool.Exec(ctx, query, id)
	return err
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
