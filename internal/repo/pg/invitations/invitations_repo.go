package invitations

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/tu-org/embolsadora-api/internal/domain"
)

// InvitationRepository defines persistence operations for user invitations.
type InvitationRepository interface {
	Create(ctx context.Context, inv *domain.UserInvitation) (*domain.UserInvitation, error)
	// GetPendingByEmailAndTenant busca una invitación pendiente para email+tenant.
	// includeGlobal=false oculta las que apuntan a un rol is_global (super_admin,
	// tenant_manager): si no se ocultara, un caller no-super_admin podría invitar
	// de nuevo a ese email con un rol cualquiera, recibir "ya pendiente" y así
	// confirmar que existe una invitación oculta — la misma lista (ListByTenant)
	// ya la esconde, así que este chequeo de duplicados tiene que estar de acuerdo.
	GetPendingByEmailAndTenant(ctx context.Context, email, tenantID string, includeGlobal bool) (*domain.UserInvitation, error)
	// ListPendingByEmail devuelve TODAS las invitaciones pendientes de un email,
	// sin cloaking: la usa ActivatePendingInvitations en nombre del propio
	// usuario que se está logueando (self-action), no de un tercero.
	ListPendingByEmail(ctx context.Context, email string) ([]domain.UserInvitation, error)
	// GetByID busca una invitación por id dentro de un tenant. includeGlobal=false
	// la oculta (ErrNotFound) si su rol es is_global — la usan Resend y Revoke
	// para no reenviar mail ni revelar el role_id de una invitación oculta.
	GetByID(ctx context.Context, id, tenantID string, includeGlobal bool) (*domain.UserInvitation, error)
	// ListByTenant devuelve las invitaciones del tenant, opcionalmente filtradas
	// por status. includeGlobal=false oculta las invitaciones a roles is_global
	// (super_admin, tenant_manager): una invitación pendiente delata el rol igual
	// que un miembro activo.
	ListByTenant(ctx context.Context, tenantID string, status *string, includeGlobal bool) ([]domain.UserInvitation, error)
	UpdateStatus(ctx context.Context, id string, status domain.InvitationStatus) error
}

type pgInvitationRepo struct {
	db *pgxpool.Pool
}

func NewInvitationRepository(db *pgxpool.Pool) InvitationRepository {
	return &pgInvitationRepo{db: db}
}

func (r *pgInvitationRepo) Create(ctx context.Context, inv *domain.UserInvitation) (*domain.UserInvitation, error) {
	const q = `
		INSERT INTO user_invitations (id, tenant_id, email, role_id, status, invited_by, created_at, updated_at, expires_at)
		VALUES ($1, $2, $3, $4, 'pending', $5, NOW(), NOW(), NOW() + INTERVAL '7 days')
		RETURNING id, tenant_id, email, role_id, status, invited_by, created_at, updated_at, expires_at`

	id := uuid.New().String()
	row := r.db.QueryRow(ctx, q, id, inv.TenantID, inv.Email, inv.RoleID, inv.InvitedBy)
	return scanInvitation(row)
}

func (r *pgInvitationRepo) GetPendingByEmailAndTenant(ctx context.Context, email, tenantID string, includeGlobal bool) (*domain.UserInvitation, error) {
	const q = `
		SELECT i.id, i.tenant_id, i.email, i.role_id, i.status, i.invited_by,
		       i.created_at, i.updated_at, i.expires_at
		FROM user_invitations i
		LEFT JOIN roles r ON r.id = i.role_id
		WHERE i.email = $1 AND i.tenant_id = $2 AND i.status = 'pending'
		  AND (COALESCE(r.is_global, FALSE) = FALSE OR $3)
		LIMIT 1`

	row := r.db.QueryRow(ctx, q, email, tenantID, includeGlobal)
	inv, err := scanInvitation(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return inv, nil
}

// ListPendingByEmail returns every pending invitation for an email across all
// tenants. Used on first login to activate the user's memberships.
func (r *pgInvitationRepo) ListPendingByEmail(ctx context.Context, email string) ([]domain.UserInvitation, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, tenant_id, email, role_id, status, invited_by, created_at, updated_at, expires_at
		 FROM user_invitations
		 WHERE email = $1 AND status = 'pending'
		 ORDER BY created_at ASC`,
		email)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []domain.UserInvitation
	for rows.Next() {
		inv, err := scanInvitation(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, *inv)
	}
	return result, rows.Err()
}

func (r *pgInvitationRepo) GetByID(ctx context.Context, id, tenantID string, includeGlobal bool) (*domain.UserInvitation, error) {
	const q = `
		SELECT i.id, i.tenant_id, i.email, i.role_id, i.status, i.invited_by,
		       i.created_at, i.updated_at, i.expires_at
		FROM user_invitations i
		LEFT JOIN roles r ON r.id = i.role_id
		WHERE i.id = $1 AND i.tenant_id = $2
		  AND (COALESCE(r.is_global, FALSE) = FALSE OR $3)
		LIMIT 1`

	row := r.db.QueryRow(ctx, q, id, tenantID, includeGlobal)
	inv, err := scanInvitation(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return inv, nil
}

// ListByTenant devuelve las invitaciones del tenant, opcionalmente filtradas por status.
// includeGlobal=false oculta las invitaciones a roles is_global (super_admin,
// tenant_manager): una invitación pendiente delata el rol igual que un miembro activo.
func (r *pgInvitationRepo) ListByTenant(ctx context.Context, tenantID string, status *string, includeGlobal bool) ([]domain.UserInvitation, error) {
	var rows pgx.Rows
	var err error

	const base = `SELECT i.id, i.tenant_id, i.email, i.role_id, i.status, i.invited_by,
	                     i.created_at, i.updated_at, i.expires_at
	              FROM user_invitations i
	              LEFT JOIN roles r ON r.id = i.role_id
	              WHERE i.tenant_id = $1
	                AND (COALESCE(r.is_global, FALSE) = FALSE OR $2)`

	if status != nil {
		rows, err = r.db.Query(ctx, base+` AND i.status = $3 ORDER BY i.created_at DESC`,
			tenantID, includeGlobal, *status)
	} else {
		rows, err = r.db.Query(ctx, base+` ORDER BY i.created_at DESC`,
			tenantID, includeGlobal)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []domain.UserInvitation
	for rows.Next() {
		inv, err := scanInvitation(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, *inv)
	}
	return result, rows.Err()
}

func (r *pgInvitationRepo) UpdateStatus(ctx context.Context, id string, status domain.InvitationStatus) error {
	const q = `UPDATE user_invitations SET status = $1, updated_at = NOW() WHERE id = $2`
	_, err := r.db.Exec(ctx, q, string(status), id)
	return err
}

// scanner is satisfied by both pgx.Row and pgx.Rows.
type scanner interface {
	Scan(dest ...interface{}) error
}

func scanInvitation(row scanner) (*domain.UserInvitation, error) {
	var inv domain.UserInvitation
	var updatedAt time.Time
	err := row.Scan(
		&inv.ID,
		&inv.TenantID,
		&inv.Email,
		&inv.RoleID,
		&inv.Status,
		&inv.InvitedBy,
		&inv.CreatedAt,
		&updatedAt,
		&inv.ExpiresAt,
	)
	if err != nil {
		return nil, err
	}
	inv.UpdatedAt = updatedAt
	return &inv, nil
}
