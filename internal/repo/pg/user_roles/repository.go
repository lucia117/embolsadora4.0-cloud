package user_roles

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/tu-org/embolsadora-api/internal/domain"
)

// UserRoleRepository defines the persistence interface for user-tenant-role assignments.
type UserRoleRepository interface {
	FindByTenant(ctx context.Context, tenantID uuid.UUID, status *string) ([]domain.UserTenantRole, error)
	FindByID(ctx context.Context, id uuid.UUID) (*domain.UserTenantRole, error)
	Create(ctx context.Context, utr *domain.UserTenantRole) (*domain.UserTenantRole, error)
	Update(ctx context.Context, utr *domain.UserTenantRole) (*domain.UserTenantRole, error)
	Revoke(ctx context.Context, id uuid.UUID) (*domain.UserTenantRole, error)
	BulkCreate(ctx context.Context, utrs []domain.UserTenantRole) ([]domain.UserTenantRole, error)
	FindByUser(ctx context.Context, userID uuid.UUID) ([]domain.UserRoleWithContext, error)
}

type userRoleRepository struct {
	db *pgxpool.Pool
}

// NewUserRoleRepository creates a new pgx-backed UserRoleRepository.
func NewUserRoleRepository(db *pgxpool.Pool) UserRoleRepository {
	return &userRoleRepository{db: db}
}

func (r *userRoleRepository) FindByTenant(ctx context.Context, tenantID uuid.UUID, status *string) ([]domain.UserTenantRole, error) {
	var rows pgx.Rows
	var err error

	if status != nil {
		rows, err = r.db.Query(ctx, FindByTenantWithStatusQuery, tenantID, *status)
	} else {
		rows, err = r.db.Query(ctx, FindByTenantQuery, tenantID)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []domain.UserTenantRole
	for rows.Next() {
		utr, err := scanUTRFromRow(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, *utr)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if result == nil {
		result = []domain.UserTenantRole{}
	}
	return result, nil
}

func (r *userRoleRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.UserTenantRole, error) {
	utr, err := scanUTR(r.db.QueryRow(ctx, FindByIDQuery, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return utr, nil
}

func (r *userRoleRepository) Create(ctx context.Context, utr *domain.UserTenantRole) (*domain.UserTenantRole, error) {
	created, err := scanUTR(r.db.QueryRow(ctx, CreateQuery,
		utr.UserID, utr.TenantID, utr.RoleID, utr.Status, utr.AssignedBy, utr.AssignedAt,
	))
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, domain.ErrUserAlreadyHasActiveRole
		}
		return nil, err
	}
	return created, nil
}

func (r *userRoleRepository) Update(ctx context.Context, utr *domain.UserTenantRole) (*domain.UserTenantRole, error) {
	updated, err := scanUTR(r.db.QueryRow(ctx, UpdateQuery, utr.RoleID, utr.ID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return updated, nil
}

func (r *userRoleRepository) Revoke(ctx context.Context, id uuid.UUID) (*domain.UserTenantRole, error) {
	revoked, err := scanUTR(r.db.QueryRow(ctx, RevokeQuery, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return revoked, nil
}

func (r *userRoleRepository) BulkCreate(ctx context.Context, utrs []domain.UserTenantRole) ([]domain.UserTenantRole, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	results := make([]domain.UserTenantRole, 0, len(utrs))
	for _, utr := range utrs {
		created, err := scanUTR(tx.QueryRow(ctx, CreateQuery,
			utr.UserID, utr.TenantID, utr.RoleID, utr.Status, utr.AssignedBy, utr.AssignedAt,
		))
		if err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" {
				return nil, domain.ErrUserAlreadyHasActiveRole
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

func (r *userRoleRepository) FindByUser(ctx context.Context, userID uuid.UUID) ([]domain.UserRoleWithContext, error) {
	rows, err := r.db.Query(ctx, FindByUserQuery, userID)
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

// derefString converts a nullable *string to string, returning "" if nil.
func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
