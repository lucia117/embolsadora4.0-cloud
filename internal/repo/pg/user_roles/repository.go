package user_roles

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/tu-org/embolsadora-api/internal/domain"
)

const (
	// PostgreSQL error codes
	errCodeUniqueViolation     = "23505"
	errCodeForeignKeyViolation = "23503"
	// errCodeCheckViolation is raised by the trg_enforce_platform_role_tenant trigger
	// (migration 000004) — the DB-level backstop for checkRoleAllowedForTenant below.
	errCodeCheckViolation = "23514"
)

// UserRoleRepository defines the persistence interface for user-tenant-role assignments.
//
// Todos los métodos reciben includeGlobal como parámetro booleano explícito: lo
// decide el handler con security.CanSeePlatformInternals y viaja handler → usecase
// → repo. El repo no lee el contexto, así que el cloaking se testea sin montar un
// contexto de Gin (mismo criterio que roles y users; ver spec §2).
type UserRoleRepository interface {
	FindByTenant(ctx context.Context, tenantID uuid.UUID, status *string, includeGlobal bool) ([]domain.UserTenantRoleDetail, error)
	FindByID(ctx context.Context, id uuid.UUID, includeGlobal bool) (*domain.UserTenantRole, error)
	Create(ctx context.Context, utr *domain.UserTenantRole, includeGlobal bool) (*domain.UserTenantRole, error)
	Update(ctx context.Context, utr *domain.UserTenantRole, includeGlobal bool) (*domain.UserTenantRole, error)
	Revoke(ctx context.Context, id uuid.UUID, tenantID uuid.UUID, includeGlobal bool) (*domain.UserTenantRole, error)
	BulkCreate(ctx context.Context, utrs []domain.UserTenantRole, includeGlobal bool) ([]domain.UserTenantRole, error)
	// FindByUser devuelve las membresías de un usuario. crossTenant habilita ver más
	// allá de tenantID (el tenant del request) y solo debe ser true para roles
	// cross-tenant (security.IsCrossTenantRole).
	FindByUser(ctx context.Context, userID, tenantID uuid.UUID, crossTenant, includeGlobal bool) ([]domain.UserRoleWithContext, error)
	// UpdateStatus changes the status of a user's UTR within a tenant (excludes pending assignments).
	UpdateStatus(ctx context.Context, userID, tenantID uuid.UUID, status domain.UserRoleStatus, includeGlobal bool) (*domain.UserTenantRole, error)
}

type userRoleRepository struct {
	db *pgxpool.Pool
}

// NewUserRoleRepository creates a new pgx-backed UserRoleRepository.
func NewUserRoleRepository(db *pgxpool.Pool) UserRoleRepository {
	return &userRoleRepository{db: db}
}

func (r *userRoleRepository) FindByTenant(ctx context.Context, tenantID uuid.UUID, status *string, includeGlobal bool) ([]domain.UserTenantRoleDetail, error) {
	var rows pgx.Rows
	var err error

	if status != nil {
		rows, err = r.db.Query(ctx, FindByTenantWithStatusQuery, tenantID, includeGlobal, *status)
	} else {
		rows, err = r.db.Query(ctx, FindByTenantQuery, tenantID, includeGlobal)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []domain.UserTenantRoleDetail
	for rows.Next() {
		d, err := scanUTRDetailFromRow(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, *d)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if result == nil {
		result = []domain.UserTenantRoleDetail{}
	}
	return result, nil
}

// FindByID devuelve nil (no error) cuando la asignación no existe O cuando existe
// pero su rol es is_global y el caller no puede verla: los usecases traducen ambos
// casos en el mismo ErrAssignmentNotFound → 404.
func (r *userRoleRepository) FindByID(ctx context.Context, id uuid.UUID, includeGlobal bool) (*domain.UserTenantRole, error) {
	utr, err := scanUTR(r.db.QueryRow(ctx, FindByIDQuery, id, includeGlobal))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return utr, nil
}

// checkRoleAllowedForTenant rejects assigning a role that either doesn't exist (or was
// soft-deleted), no pertenece al tenant, no es visible para el caller, o es platform-only
// (roles.is_global = TRUE) fuera del tenant plataforma de MRG.
//
// Los tres predicados del WHERE son los mismos que roles.GetByIDForTenant, que es la
// validación primaria que corre en el usecase antes de llegar acá. Esto es el backstop
// del repo — la última puerta antes del INSERT/UPDATE — con el mismo criterio con que
// Task 5 dejó el precheck de DeleteUser y el scoping por tenant dentro de RevokeQuery:
//   - (r.tenant_id = $2 OR r.tenant_id IS NULL): sin esto un admin podía asignar un rol
//     custom de OTRO tenant conociendo su id (tenant_can_use_role devuelve TRUE
//     incondicionalmente cuando is_global=false, así que no filtraba nada para custom).
//   - (NOT r.is_global OR $3): un rol global invisible para el caller sale de la consulta
//     como cero filas, o sea ErrInvalidRoleID — exactamente lo mismo que un rol
//     inexistente. Esa convergencia es la que impide usar la asignación como oráculo.
//
// tenant_can_use_role() (migración 000004) sigue siendo la única definición de la regla
// de plataforma, compartida con roles.List/GetByIDForTenant y con el trigger de la DB.
// Se evalúa en el SELECT y no en el WHERE a propósito: para un super_admin
// (includeGlobal=true) que ve el rol global pero intenta usarlo en un tenant cliente, la
// respuesta correcta es 403 ErrRoleNotAllowedForTenant ("lo ves pero no podés operarlo"),
// no 404.
func (r *userRoleRepository) checkRoleAllowedForTenant(ctx context.Context, roleID string, tenantID uuid.UUID, includeGlobal bool) error {
	var allowed bool
	err := r.db.QueryRow(ctx, `
		SELECT tenant_can_use_role($2, r.id)
		FROM roles r
		WHERE r.id = $1
		  AND r.deleted_at IS NULL
		  AND (r.tenant_id = $2 OR r.tenant_id IS NULL)
		  AND (NOT r.is_global OR $3)
	`, roleID, tenantID, includeGlobal).Scan(&allowed)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrInvalidRoleID
		}
		return err
	}
	if !allowed {
		return domain.ErrRoleNotAllowedForTenant
	}
	return nil
}

// hasVisibleActiveAssignment responde si la asignación activa que bloquea un INSERT
// (idx_utr_active_unique) es visible para este caller. Ver Create para el porqué.
func (r *userRoleRepository) hasVisibleActiveAssignment(ctx context.Context, userID, tenantID uuid.UUID, includeGlobal bool) (bool, error) {
	var visible bool
	if err := r.db.QueryRow(ctx, HasVisibleActiveAssignmentQuery, userID, tenantID, includeGlobal).Scan(&visible); err != nil {
		return false, err
	}
	return visible, nil
}

// resolveActiveUniqueViolation elige el error para un choque contra
// idx_utr_active_unique (único sobre (user_id, tenant_id) WHERE status='active').
//
// El índice no sabe de cloaking: ve la membresía activa del superadmin igual que
// cualquier otra. Sin esto, POST /user-roles distinguía tres casos por status —
// 409 (usuario oculto con membresía activa), 400 (user_id inexistente, violación de
// FK) y 201 (usuario sin membresía) — o sea un oráculo que sobrevivía al cloaking de
// las listas: 409 vs 400 contestaba "existe pero no te lo muestro" vs "no existe".
//
// Criterio, el mismo que invitations.Create (commit 6ffb50a): converger a la
// respuesta del caso que el caller SÍ puede observar. Acá son dos casos distintos,
// así que la convergencia es hacia cada lado según qué haya:
//   - conflicto visible → ErrUserAlreadyHasActiveRole (409), sin cambios: es un
//     duplicado legítimo y el mensaje tiene que ser accionable.
//   - conflicto oculto → ErrInvalidUserID (400), byte-idéntico a lo que devuelve la
//     violación de FK de un user_id inexistente. Para este caller ese usuario no
//     existe, y esa es exactamente la respuesta que corresponde.
func (r *userRoleRepository) resolveActiveUniqueViolation(ctx context.Context, userID, tenantID uuid.UUID, includeGlobal bool) error {
	visible, err := r.hasVisibleActiveAssignment(ctx, userID, tenantID, includeGlobal)
	if err != nil {
		return err
	}
	if visible {
		return domain.ErrUserAlreadyHasActiveRole
	}
	return domain.ErrInvalidUserID
}

// mapForeignKeyViolation translates a user_tenant_roles FK violation into the domain
// error matching whichever constraint failed. Returns nil for FKs that shouldn't
// realistically fail from client input (tenant_id, assigned_by — both already validated
// upstream), so callers fall back to returning the raw pgconn error for those.
func mapForeignKeyViolation(pgErr *pgconn.PgError) error {
	switch pgErr.ConstraintName {
	case "user_tenant_roles_role_id_fkey":
		return domain.ErrInvalidRoleID
	case "user_tenant_roles_user_id_fkey":
		return domain.ErrInvalidUserID
	default:
		return nil
	}
}

// Create escribe la membresía. Dos ejes de cloaking, independientes entre sí:
//   - el ROL pedido (checkRoleAllowedForTenant): un rol global invisible responde
//     como rol inexistente → ErrInvalidRoleID.
//   - la IDENTIDAD destino (guard dentro de CreateQuery): un usuario que es cuenta
//     de plataforma responde como usuario inexistente → ErrInvalidUserID.
//
// El segundo hacía falta porque el primero solo mira el rol que eligió el caller:
// pedir 'operario' para el super_admin pasaba las dos validaciones de rol y dejaba
// una UTR real y visible con el email y el nombre de la cuenta interna.
func (r *userRoleRepository) Create(ctx context.Context, utr *domain.UserTenantRole, includeGlobal bool) (*domain.UserTenantRole, error) {
	if utr.RoleID != nil {
		if err := r.checkRoleAllowedForTenant(ctx, *utr.RoleID, utr.TenantID, includeGlobal); err != nil {
			return nil, err
		}
	}

	created, err := scanUTR(r.db.QueryRow(ctx, CreateQuery,
		utr.ID, utr.UserID, utr.TenantID, utr.RoleID, utr.Status, utr.AssignedBy, utr.AssignedAt, includeGlobal,
	))
	if err != nil {
		// Cero filas = el guard de identidad de CreateQuery suprimió el INSERT: el
		// destino es una cuenta de plataforma y este caller no puede verla. Misma
		// respuesta que el user_id inexistente (que llega por la violación de FK
		// unas líneas más abajo) — es la convergencia que impide usar POST
		// /user-roles como oráculo de identidad.
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrInvalidUserID
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			if pgErr.Code == errCodeUniqueViolation {
				return nil, r.resolveActiveUniqueViolation(ctx, utr.UserID, utr.TenantID, includeGlobal)
			}
			if pgErr.Code == errCodeForeignKeyViolation {
				if mapped := mapForeignKeyViolation(pgErr); mapped != nil {
					return nil, mapped
				}
			}
			if pgErr.Code == errCodeCheckViolation {
				return nil, domain.ErrRoleNotAllowedForTenant
			}
		}
		return nil, err
	}
	return created, nil
}

func (r *userRoleRepository) Update(ctx context.Context, utr *domain.UserTenantRole, includeGlobal bool) (*domain.UserTenantRole, error) {
	if utr.RoleID != nil {
		if err := r.checkRoleAllowedForTenant(ctx, *utr.RoleID, utr.TenantID, includeGlobal); err != nil {
			return nil, err
		}
	}

	updated, err := scanUTR(r.db.QueryRow(ctx, UpdateQuery, utr.RoleID, utr.ID, includeGlobal))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			if pgErr.Code == errCodeForeignKeyViolation {
				if mapped := mapForeignKeyViolation(pgErr); mapped != nil {
					return nil, mapped
				}
			}
			if pgErr.Code == errCodeCheckViolation {
				return nil, domain.ErrRoleNotAllowedForTenant
			}
		}
		return nil, err
	}
	return updated, nil
}

func (r *userRoleRepository) Revoke(ctx context.Context, id uuid.UUID, tenantID uuid.UUID, includeGlobal bool) (*domain.UserTenantRole, error) {
	revoked, err := scanUTR(r.db.QueryRow(ctx, RevokeQuery, id, tenantID, includeGlobal))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return revoked, nil
}

func (r *userRoleRepository) BulkCreate(ctx context.Context, utrs []domain.UserTenantRole, includeGlobal bool) ([]domain.UserTenantRole, error) {
	// Callers always send the same tenant+role for the whole batch (see
	// bulk_assign_user_roles usecase), so one check up front covers all rows.
	if len(utrs) > 0 && utrs[0].RoleID != nil {
		if err := r.checkRoleAllowedForTenant(ctx, *utrs[0].RoleID, utrs[0].TenantID, includeGlobal); err != nil {
			return nil, err
		}
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	results := make([]domain.UserTenantRole, 0, len(utrs))
	for _, utr := range utrs {
		created, err := scanUTR(tx.QueryRow(ctx, CreateQuery,
			utr.ID, utr.UserID, utr.TenantID, utr.RoleID, utr.Status, utr.AssignedBy, utr.AssignedAt, includeGlobal,
		))
		if err != nil {
			// Mismo guard de identidad que Create, y el mismo error envuelto con el
			// user id que el resto de los casos del lote. La transacción se
			// rollbackea entera: el batch es all-or-nothing, así que el usuario
			// legítimo que venía en el mismo lote tampoco entra.
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, fmt.Errorf("%w: user %s", domain.ErrInvalidUserID, utr.UserID)
			}
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) {
				if pgErr.Code == errCodeUniqueViolation {
					// Mismo criterio que Create: el conflicto oculto se responde como
					// user_id inexistente. resolveActiveUniqueViolation consulta por el
					// pool (no por tx, que ya quedó abortada por el error del INSERT).
					return nil, fmt.Errorf("%w: user %s", r.resolveActiveUniqueViolation(ctx, utr.UserID, utr.TenantID, includeGlobal), utr.UserID)
				}
				if pgErr.Code == errCodeForeignKeyViolation {
					if mapped := mapForeignKeyViolation(pgErr); mapped != nil {
						return nil, fmt.Errorf("%w: user %s", mapped, utr.UserID)
					}
				}
				if pgErr.Code == errCodeCheckViolation {
					return nil, fmt.Errorf("%w: user %s", domain.ErrRoleNotAllowedForTenant, utr.UserID)
				}
			}
			return nil, err
		}
		results = append(results, *created)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return results, nil
}

// FindByUser devuelve las membresías de un usuario, acotadas al tenant del request
// salvo que el caller tenga rol cross-tenant, y sin las asignaciones a roles globales
// salvo que sea super_admin. Un usuario inexistente y uno cuya única membresía está
// cloakeada devuelven los dos la misma lista vacía.
func (r *userRoleRepository) FindByUser(ctx context.Context, userID, tenantID uuid.UUID, crossTenant, includeGlobal bool) ([]domain.UserRoleWithContext, error) {
	rows, err := r.db.Query(ctx, FindByUserQuery, userID, tenantID, crossTenant, includeGlobal)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []domain.UserRoleWithContext
	for rows.Next() {
		var item domain.UserRoleWithContext
		var roleID *string
		err := rows.Scan(&item.TenantID, &item.TenantName, &roleID, &item.RoleName, &item.Status)
		if err != nil {
			return nil, err
		}
		if roleID != nil {
			item.RoleID = *roleID
		}
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if result == nil {
		result = []domain.UserRoleWithContext{}
	}
	return result, nil
}

// scanUTR scans a single UTR row from a QueryRow result.
func scanUTR(row pgx.Row) (*domain.UserTenantRole, error) {
	var utr domain.UserTenantRole
	var roleID *string
	var assignedBy *uuid.UUID
	err := row.Scan(
		&utr.ID, &utr.UserID, &utr.TenantID, &roleID, &utr.Status,
		&assignedBy, &utr.AssignedAt, &utr.CreatedAt, &utr.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	utr.RoleID = roleID
	utr.AssignedBy = assignedBy
	return &utr, nil
}

// scanUTRFromRow scans a single UTR row from a Rows iterator.
func scanUTRFromRow(rows pgx.Rows) (*domain.UserTenantRole, error) {
	var utr domain.UserTenantRole
	var roleID *string
	var assignedBy *uuid.UUID
	err := rows.Scan(
		&utr.ID, &utr.UserID, &utr.TenantID, &roleID, &utr.Status,
		&assignedBy, &utr.AssignedAt, &utr.CreatedAt, &utr.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	utr.RoleID = roleID
	utr.AssignedBy = assignedBy
	return &utr, nil
}

// scanUTRDetailFromRow scans a single UTR row plus its joined role name and
// user display fields, from the FindByTenant / FindByTenantWithStatus queries.
func scanUTRDetailFromRow(rows pgx.Rows) (*domain.UserTenantRoleDetail, error) {
	var d domain.UserTenantRoleDetail
	var roleID *string
	var assignedBy *uuid.UUID
	err := rows.Scan(
		&d.ID, &d.UserID, &d.TenantID, &roleID, &d.Status,
		&assignedBy, &d.AssignedAt, &d.CreatedAt, &d.UpdatedAt,
		&d.RoleName, &d.UserEmail, &d.UserName, &d.UserFirstName, &d.UserLastName,
	)
	if err != nil {
		return nil, err
	}
	d.RoleID = roleID
	d.AssignedBy = assignedBy
	return &d, nil
}

// UpdateStatus changes the status of a user's UTR within a tenant.
// Only affects non-pending assignments (pending → active is handled by invitation flow).
// Returns domain.ErrNoActiveAssignment directly if no non-pending row was updated.
func (r *userRoleRepository) UpdateStatus(ctx context.Context, userID, tenantID uuid.UUID, status domain.UserRoleStatus, includeGlobal bool) (*domain.UserTenantRole, error) {
	row := r.db.QueryRow(ctx, UpdateStatusQuery, string(status), userID, tenantID, includeGlobal)

	var utr domain.UserTenantRole
	var roleID *string
	var assignedBy *uuid.UUID
	err := row.Scan(
		&utr.ID, &utr.UserID, &utr.TenantID, &roleID, &utr.Status,
		&assignedBy, &utr.AssignedAt, &utr.CreatedAt, &utr.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNoActiveAssignment
		}
		return nil, err
	}
	utr.RoleID = roleID
	utr.AssignedBy = assignedBy
	return &utr, nil
}

// derefString converts a nullable *string to string, returning "" if nil.
func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
