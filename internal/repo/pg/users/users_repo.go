package users

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/tu-org/embolsadora-api/internal/domain"
)

// UserRepository defines the persistence interface for users.
type UserRepository interface {
	UpsertBySupabaseID(ctx context.Context, supabaseUserID, email string) (*domain.User, error)
	GetBySupabaseID(ctx context.Context, supabaseUserID string) (*domain.User, error)
	GetByID(ctx context.Context, id string) (*domain.User, error)
	SetStatus(ctx context.Context, userID string, status domain.UserStatus) error
	SetPasswordChangeRequired(ctx context.Context, userID string, value bool) error
}

type pgUserRepo struct {
	db *pgxpool.Pool
}

func NewUserRepository(db *pgxpool.Pool) UserRepository {
	return &pgUserRepo{db: db}
}

// UpsertBySupabaseID inserts or updates a user record based on their Supabase user ID.
// Idempotent: safe to call on every authenticated request.
func (r *pgUserRepo) UpsertBySupabaseID(ctx context.Context, supabaseUserID, email string) (*domain.User, error) {
	const q = `
		INSERT INTO users (id, supabase_user_id, email, status, created_at, updated_at, last_login_at)
		VALUES ($1, $2, $3, 'invited', NOW(), NOW(), NOW())
		ON CONFLICT (supabase_user_id)
		DO UPDATE SET
			email          = EXCLUDED.email,
			last_login_at  = NOW(),
			updated_at     = NOW()
		RETURNING id, supabase_user_id, email, name, status,
		          auth_provider, email_verified_at, last_login_at,
		          password_change_required, created_at, updated_at`

	row := r.db.QueryRow(ctx, q, uuid.New().String(), supabaseUserID, email)
	return scanUser(row)
}

// GetBySupabaseID retrieves a user by their Supabase user ID.
func (r *pgUserRepo) GetBySupabaseID(ctx context.Context, supabaseUserID string) (*domain.User, error) {
	const q = `
		SELECT id, supabase_user_id, email, name, status,
		       auth_provider, email_verified_at, last_login_at,
		       password_change_required, created_at, updated_at
		FROM users
		WHERE supabase_user_id = $1`

	row := r.db.QueryRow(ctx, q, supabaseUserID)
	u, err := scanUser(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return u, nil
}

// GetByID retrieves a user by their internal ID.
func (r *pgUserRepo) GetByID(ctx context.Context, id string) (*domain.User, error) {
	const q = `
		SELECT id, supabase_user_id, email, name, status,
		       auth_provider, email_verified_at, last_login_at,
		       password_change_required, created_at, updated_at
		FROM users
		WHERE id = $1`

	row := r.db.QueryRow(ctx, q, id)
	u, err := scanUser(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return u, nil
}

// SetStatus updates the status of a user.
func (r *pgUserRepo) SetStatus(ctx context.Context, userID string, status domain.UserStatus) error {
	const q = `UPDATE users SET status = $1, updated_at = NOW() WHERE id = $2`
	_, err := r.db.Exec(ctx, q, string(status), userID)
	return err
}

// SetPasswordChangeRequired sets or clears the password_change_required flag.
func (r *pgUserRepo) SetPasswordChangeRequired(ctx context.Context, userID string, value bool) error {
	const q = `UPDATE users SET password_change_required = $1, updated_at = NOW() WHERE id = $2`
	_, err := r.db.Exec(ctx, q, value, userID)
	return err
}

func scanUser(row pgx.Row) (*domain.User, error) {
	var u domain.User
	var name *string
	var authProvider *string
	var emailVerifiedAt *time.Time
	var lastLoginAt *time.Time

	err := row.Scan(
		&u.ID,
		&u.SupabaseUserID,
		&u.Email,
		&name,
		&u.Status,
		&authProvider,
		&emailVerifiedAt,
		&lastLoginAt,
		&u.PasswordChangeRequired,
		&u.CreatedAt,
		&u.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if name != nil {
		u.Name = *name
	}
	if authProvider != nil {
		u.AuthProvider = *authProvider
	}
	u.EmailVerifiedAt = emailVerifiedAt
	u.LastLoginAt = lastLoginAt
	return &u, nil
}
