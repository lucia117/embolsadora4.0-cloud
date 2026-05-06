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
	GetPendingByEmailAndTenant(ctx context.Context, email, tenantID string) (*domain.UserInvitation, error)
	GetByID(ctx context.Context, id, tenantID string) (*domain.UserInvitation, error)
	ListByTenant(ctx context.Context, tenantID string, status *string) ([]domain.UserInvitation, error)
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

func (r *pgInvitationRepo) GetPendingByEmailAndTenant(ctx context.Context, email, tenantID string) (*domain.UserInvitation, error) {
	const q = `
		SELECT id, tenant_id, email, role_id, status, invited_by, created_at, updated_at, expires_at
		FROM user_invitations
		WHERE email = $1 AND tenant_id = $2 AND status = 'pending'
		LIMIT 1`

	row := r.db.QueryRow(ctx, q, email, tenantID)
	inv, err := scanInvitation(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return inv, nil
}

func (r *pgInvitationRepo) GetByID(ctx context.Context, id, tenantID string) (*domain.UserInvitation, error) {
	const q = `
		SELECT id, tenant_id, email, role_id, status, invited_by, created_at, updated_at, expires_at
		FROM user_invitations
		WHERE id = $1 AND tenant_id = $2
		LIMIT 1`

	row := r.db.QueryRow(ctx, q, id, tenantID)
	inv, err := scanInvitation(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return inv, nil
}

func (r *pgInvitationRepo) ListByTenant(ctx context.Context, tenantID string, status *string) ([]domain.UserInvitation, error) {
	var rows pgx.Rows
	var err error

	if status != nil {
		rows, err = r.db.Query(ctx,
			`SELECT id, tenant_id, email, role_id, status, invited_by, created_at, updated_at, expires_at
			 FROM user_invitations WHERE tenant_id = $1 AND status = $2 ORDER BY created_at DESC`,
			tenantID, *status)
	} else {
		rows, err = r.db.Query(ctx,
			`SELECT id, tenant_id, email, role_id, status, invited_by, created_at, updated_at, expires_at
			 FROM user_invitations WHERE tenant_id = $1 ORDER BY created_at DESC`,
			tenantID)
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
