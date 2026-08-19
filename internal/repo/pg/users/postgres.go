package users

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tu-org/embolsadora-api/internal/domain"
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

// ListByTenant retrieves paginated users for a tenant, excluding soft-deleted.
// A user belongs to the tenant if users.tenant_id matches (management CRUD) or
// if they hold an active membership in user_tenant_roles (auth-provisioned
// users have tenant_id NULL — their membership is the source of truth).
//
// includeGlobal=false oculta a los miembros cuyo rol es is_global (super_admin,
// tenant_manager). El filtro se aplica en el COUNT y en el SELECT: filtrar solo
// el SELECT dejaría un `total` mayor que las filas devueltas y la paginación
// mostraría una página final vacía.
func (r *PostgresRepository) ListByTenant(ctx context.Context, tenantID string, limit, offset int, includeGlobal bool) ([]*users.User, int64, error) {
	var totalCount int64
	countQuery := `
		SELECT COUNT(*)
		FROM users u
		LEFT JOIN user_tenant_roles utr
			ON utr.user_id = u.id AND utr.tenant_id = $1 AND utr.status = 'active'
		LEFT JOIN roles r ON r.id = utr.role_id
		WHERE u.deleted_at IS NULL
		  AND (u.tenant_id = $1 OR utr.id IS NOT NULL)
		  AND (COALESCE(r.is_global, FALSE) = FALSE OR $2)`
	if err := r.db.QueryRow(ctx, countQuery, tenantID, includeGlobal).Scan(&totalCount); err != nil {
		return nil, 0, fmt.Errorf("failed to count users: %w", err)
	}

	// Get paginated results. COALESCE covers auth-provisioned rows where
	// tenant_id/first_name/last_name are NULL (they only have `name`).
	query := `
		SELECT u.id,
		       COALESCE(u.tenant_id, $1) AS tenant_id,
		       COALESCE(u.first_name, u.name, '') AS first_name,
		       COALESCE(u.last_name, '') AS last_name,
		       u.email,
		       COALESCE(utr.role_id, u.role) AS role,
		       u.image, u.created_at, u.updated_at, u.deleted_at
		FROM users u
		LEFT JOIN user_tenant_roles utr
			ON utr.user_id = u.id AND utr.tenant_id = $1 AND utr.status = 'active'
		LEFT JOIN roles r ON r.id = utr.role_id
		WHERE u.deleted_at IS NULL
		  AND (u.tenant_id = $1 OR utr.id IS NOT NULL)
		  AND (COALESCE(r.is_global, FALSE) = FALSE OR $2)
		ORDER BY u.created_at DESC
		LIMIT $3 OFFSET $4
	`

	rows, err := r.db.Query(ctx, query, tenantID, includeGlobal, limit, offset)
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

// GetByID retrieves a single user by ID (returns ErrNotFound if soft-deleted).
// Membership via user_tenant_roles also qualifies (see ListByTenant).
//
// includeGlobal=false hace que un miembro con rol is_global sea indistinguible
// de uno inexistente: ambos devuelven ErrNotFound → 404. Un 403 confirmaría que
// el usuario existe, que es justamente lo que hay que no filtrar.
func (r *PostgresRepository) GetByID(ctx context.Context, tenantID, userID string, crossTenant, includeGlobal bool) (*users.User, error) {
	query := `
		SELECT u.id,
		       COALESCE(u.tenant_id, $2) AS tenant_id,
		       COALESCE(u.first_name, u.name, '') AS first_name,
		       COALESCE(u.last_name, '') AS last_name,
		       u.email,
		       COALESCE(utr.role_id, u.role) AS role,
		       u.image, u.created_at, u.updated_at, u.deleted_at
		FROM users u
		LEFT JOIN user_tenant_roles utr
			ON utr.user_id = u.id AND utr.tenant_id = $2 AND utr.status = 'active'
		LEFT JOIN roles r ON r.id = utr.role_id
		WHERE u.id = $1
		  AND u.deleted_at IS NULL
		  AND (u.tenant_id = $2 OR utr.id IS NOT NULL OR $4)
		  AND (COALESCE(r.is_global, FALSE) = FALSE OR $3)
	`

	row := r.db.QueryRow(ctx, query, userID, tenantID, includeGlobal, crossTenant)
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
	user.ID = uuid.New().String()
	user.CreatedAt = now
	user.UpdatedAt = now
	user.DeletedAt = nil

	query := `
		INSERT INTO users (id, tenant_id, first_name, last_name, email, role, image, created_at, updated_at, deleted_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, tenant_id, first_name, last_name, email, role, image, created_at, updated_at, deleted_at
	`

	row := r.db.QueryRow(ctx, query, user.ID, user.TenantID, user.FirstName, user.LastName, user.Email, user.Role, user.Image, user.CreatedAt, user.UpdatedAt, user.DeletedAt)
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

// CreateWithRole inserts a user and an active UTR atomically in a single transaction.
func (r *PostgresRepository) CreateWithRole(ctx context.Context, user *users.User, utr *domain.UserTenantRole) (*users.User, error) {
	if err := user.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %v", users.ErrValidation, err)
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	now := time.Now().UTC()
	user.ID = uuid.New().String()
	user.CreatedAt = now
	user.UpdatedAt = now
	user.DeletedAt = nil

	userQuery := `
		INSERT INTO users (id, tenant_id, first_name, last_name, email, role, image, created_at, updated_at, deleted_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, tenant_id, first_name, last_name, email, role, image, created_at, updated_at, deleted_at
	`
	row := tx.QueryRow(ctx, userQuery,
		user.ID, user.TenantID, user.FirstName, user.LastName, user.Email, user.Role, user.Image,
		user.CreatedAt, user.UpdatedAt, user.DeletedAt,
	)
	created, err := scanUser(row)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, users.ErrEmailTaken
		}
		return nil, fmt.Errorf("failed to insert user: %w", err)
	}

	utrQuery := `
		INSERT INTO user_tenant_roles (id, user_id, tenant_id, role_id, status, assigned_by, assigned_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NOW(), NOW())
	`
	_, err = tx.Exec(ctx, utrQuery,
		utr.ID, created.ID, utr.TenantID, utr.RoleID, string(utr.Status), utr.AssignedBy, utr.AssignedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Code {
			case "23503": // foreign key violation — only map role_id FK
				if pgErr.ConstraintName == "user_tenant_roles_role_id_fkey" {
					return nil, domain.ErrInvalidRoleID
				}
			case "23505": // unique violation — only map the active-UTR partial index
				if pgErr.ConstraintName == "idx_utr_active_unique" {
					return nil, domain.ErrUserAlreadyHasActiveRole
				}
			}
		}
		return nil, fmt.Errorf("failed to insert user_tenant_role: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return created, nil
}

// Update modifies user fields (name, role, image only - immutable fields protected).
// Membership via user_tenant_roles also qualifies (see GetByID) - auth-provisioned
// users have tenant_id NULL, so a strict column match alone would never find them.
func (r *PostgresRepository) Update(ctx context.Context, user *users.User) (*users.User, error) {
	if err := user.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %v", users.ErrValidation, err)
	}

	// updated_at is auto-updated by trigger, but we ensure it's set
	user.UpdatedAt = time.Now().UTC()

	query := `
		UPDATE users
		SET first_name = $1, last_name = $2, role = $3, image = $4, updated_at = $5
		WHERE id = $6 AND deleted_at IS NULL
		  AND (tenant_id = $7 OR EXISTS (
		      SELECT 1 FROM user_tenant_roles utr
		      WHERE utr.user_id = $6 AND utr.tenant_id = $7 AND utr.status = 'active'
		  ))
		RETURNING id, COALESCE(tenant_id, $7) AS tenant_id, first_name, last_name, email, role, image, created_at, updated_at, deleted_at
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

// Delete soft-deletes a user by setting deleted_at.
// Membership via user_tenant_roles also qualifies (see GetByID/Update).
func (r *PostgresRepository) Delete(ctx context.Context, tenantID, userID string) error {
	query := `
		UPDATE users SET deleted_at = CURRENT_TIMESTAMP
		WHERE id = $1 AND deleted_at IS NULL
		  AND (tenant_id = $2 OR EXISTS (
		      SELECT 1 FROM user_tenant_roles utr
		      WHERE utr.user_id = $1 AND utr.tenant_id = $2 AND utr.status = 'active'
		  ))
	`

	result, err := r.db.Exec(ctx, query, userID, tenantID)
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	if result.RowsAffected() == 0 {
		return users.ErrNotFound
	}

	return nil
}

// GetByIDWithRoles retrieves a user with their active role assignment in the tenant.
// Uses LEFT JOINs so users without an active UTR still return with Roles: [].
// Membership via user_tenant_roles also qualifies (see ListByTenant).
//
// includeGlobal=false aplica la misma regla de cloaking que GetByID: un miembro
// con rol is_global es indistinguible de uno inexistente (ErrNotFound → 404).
// Esta consulta expone además el nombre y permisos del rol, así que sin este
// filtro el ?include=roles delataría no solo la existencia del super_admin sino
// su rol completo — una fuga peor que la de GetByID.
func (r *PostgresRepository) GetByIDWithRoles(ctx context.Context, tenantID, userID string, includeGlobal bool) (*users.UserWithRoles, error) {
	query := `
		SELECT u.id,
		       COALESCE(u.tenant_id, $2) AS tenant_id,
		       COALESCE(u.first_name, u.name, '') AS first_name,
		       COALESCE(u.last_name, '') AS last_name,
		       u.email,
		       COALESCE(utr.role_id, u.role) AS role,
		       u.image,
		       u.created_at, u.updated_at, u.deleted_at,
		       r.id        AS role_id,
		       r.name      AS role_name,
		       r.permissions AS role_permissions
		FROM users u
		LEFT JOIN user_tenant_roles utr
		    ON utr.user_id = u.id
		    AND utr.tenant_id = $2
		    AND utr.status = 'active'
		LEFT JOIN roles r
		    ON r.id = utr.role_id
		    AND r.deleted_at IS NULL
		WHERE u.id = $1
		  AND u.deleted_at IS NULL
		  AND (u.tenant_id = $2 OR utr.id IS NOT NULL)
		  AND (COALESCE(r.is_global, FALSE) = FALSE OR $3)
	`

	row := r.db.QueryRow(ctx, query, userID, tenantID, includeGlobal)

	var u users.User
	var roleID *string
	var roleName *string
	var rolePermsJSON []byte // JSONB scanned as raw bytes, then unmarshalled

	err := row.Scan(
		&u.ID, &u.TenantID, &u.FirstName, &u.LastName, &u.Email, &u.Role, &u.Image,
		&u.CreatedAt, &u.UpdatedAt, &u.DeletedAt,
		&roleID, &roleName, &rolePermsJSON,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, users.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get user with roles: %w", err)
	}

	uwr := &users.UserWithRoles{
		User:  u,
		Roles: []users.AssignedRole{},
	}

	if roleID != nil && roleName != nil {
		var perms []string
		if len(rolePermsJSON) > 0 {
			if jsonErr := json.Unmarshal(rolePermsJSON, &perms); jsonErr != nil {
				return nil, fmt.Errorf("failed to parse role permissions: %w", jsonErr)
			}
		}
		if perms == nil {
			perms = []string{}
		}
		uwr.Roles = append(uwr.Roles, users.AssignedRole{
			ID:          *roleID,
			Name:        *roleName,
			Permissions: perms,
		})
	}

	return uwr, nil
}

// ListPendingByTenant retrieves users with a pending role assignment in the tenant.
// includeGlobal=false oculta las membresías pendientes a roles globales: una
// invitación pendiente a super_admin delata su existencia igual que un miembro activo.
//
// COALESCE en tenant_id/first_name/last_name: igual que en ListByTenant/GetByID,
// un usuario pending puede venir de auth (tenant_id NULL, sin first_name/last_name
// hasta completar el onboarding) — sin el COALESCE, escanear esa fila entra en
// panic de tipo al intentar volcar NULL en un string no-puntero.
func (r *PostgresRepository) ListPendingByTenant(ctx context.Context, tenantID string, includeGlobal bool) ([]*users.User, error) {
	query := `
		SELECT u.id,
		       COALESCE(u.tenant_id, $1) AS tenant_id,
		       COALESCE(u.first_name, u.name, '') AS first_name,
		       COALESCE(u.last_name, '') AS last_name,
		       u.email,
		       COALESCE(utr.role_id, u.role) AS role,
		       u.image, u.created_at, u.updated_at, u.deleted_at
		FROM users u
		JOIN user_tenant_roles utr
		    ON utr.user_id = u.id
		    AND utr.tenant_id = $1
		    AND utr.status = 'pending'
		LEFT JOIN roles r ON r.id = utr.role_id
		WHERE u.deleted_at IS NULL
		  AND (COALESCE(r.is_global, FALSE) = FALSE OR $2)
		ORDER BY utr.created_at DESC
	`

	rows, err := r.db.Query(ctx, query, tenantID, includeGlobal)
	if err != nil {
		return nil, fmt.Errorf("failed to query pending users: %w", err)
	}
	defer rows.Close()

	result := make([]*users.User, 0)
	for rows.Next() {
		user, err := scanUser(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan pending user: %w", err)
		}
		result = append(result, user)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating pending users: %w", err)
	}

	return result, nil
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
