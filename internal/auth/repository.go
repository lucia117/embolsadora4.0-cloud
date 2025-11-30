package auth

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// UserRepository maneja el acceso a datos de usuarios
type UserRepository struct {
	db *pgxpool.Pool
}

// NewUserRepository crea una nueva instancia de UserRepository
func NewUserRepository(db *pgxpool.Pool) *UserRepository {
	return &UserRepository{db: db}
}

// FindByEmail busca un usuario por email
func (r *UserRepository) FindByEmail(ctx context.Context, email string) (*User, error) {
	query := `
		SELECT id, email, name, password_hash, image, tenant_id, status, created_at, updated_at
		FROM users
		WHERE email = $1 AND status = 'active'
	`

	var user User
	err := r.db.QueryRow(ctx, query, email).Scan(
		&user.ID,
		&user.Email,
		&user.Name,
		&user.PasswordHash,
		&user.Image,
		&user.TenantID,
		&user.Status,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	return &user, nil
}

// FindByID busca un usuario por ID
func (r *UserRepository) FindByID(ctx context.Context, id string) (*User, error) {
	query := `
		SELECT id, email, name, password_hash, image, tenant_id, status, created_at, updated_at
		FROM users
		WHERE id = $1
	`

	var user User
	err := r.db.QueryRow(ctx, query, id).Scan(
		&user.ID,
		&user.Email,
		&user.Name,
		&user.PasswordHash,
		&user.Image,
		&user.TenantID,
		&user.Status,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	return &user, nil
}

// UpdatePassword actualiza la contraseña de un usuario
func (r *UserRepository) UpdatePassword(ctx context.Context, userID, passwordHash string) error {
	query := `
		UPDATE users
		SET password_hash = $1, updated_at = NOW()
		WHERE id = $2
	`

	_, err := r.db.Exec(ctx, query, passwordHash, userID)
	return err
}

// TenantRepository maneja el acceso a datos de tenants
type TenantRepository struct {
	db *pgxpool.Pool
}

// NewTenantRepository crea una nueva instancia de TenantRepository
func NewTenantRepository(db *pgxpool.Pool) *TenantRepository {
	return &TenantRepository{db: db}
}

// FindByID busca un tenant por ID
func (r *TenantRepository) FindByID(ctx context.Context, id string) (*Tenant, error) {
	query := `
		SELECT id, name, company_name, subdomain, created_at, updated_at
		FROM tenants
		WHERE id = $1
	`

	var tenant Tenant
	err := r.db.QueryRow(ctx, query, id).Scan(
		&tenant.ID,
		&tenant.Name,
		&tenant.CompanyName,
		&tenant.Subdomain,
		&tenant.CreatedAt,
		&tenant.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	return &tenant, nil
}

// SessionRepository maneja el acceso a datos de sesiones
type SessionRepository struct {
	db *pgxpool.Pool
}

// NewSessionRepository crea una nueva instancia de SessionRepository
func NewSessionRepository(db *pgxpool.Pool) *SessionRepository {
	return &SessionRepository{db: db}
}

// Create crea una nueva sesión
func (r *SessionRepository) Create(ctx context.Context, session *Session) error {
	query := `
		INSERT INTO sessions (token, user_id, expires_at, created_at)
		VALUES ($1, $2, $3, $4)
	`

	_, err := r.db.Exec(ctx, query, session.Token, session.UserID, session.ExpiresAt, session.CreatedAt)
	return err
}

// FindByToken busca una sesión por token
func (r *SessionRepository) FindByToken(ctx context.Context, token string) (*Session, error) {
	query := `
		SELECT token, user_id, expires_at, created_at
		FROM sessions
		WHERE token = $1 AND expires_at > NOW()
	`

	var session Session
	err := r.db.QueryRow(ctx, query, token).Scan(
		&session.Token,
		&session.UserID,
		&session.ExpiresAt,
		&session.CreatedAt,
	)

	if err != nil {
		return nil, err
	}

	return &session, nil
}

// Delete elimina una sesión por token
func (r *SessionRepository) Delete(ctx context.Context, token string) error {
	query := `DELETE FROM sessions WHERE token = $1`
	_, err := r.db.Exec(ctx, query, token)
	return err
}

// DeleteExpired elimina sesiones expiradas
func (r *SessionRepository) DeleteExpired(ctx context.Context) error {
	query := `DELETE FROM sessions WHERE expires_at <= NOW()`
	_, err := r.db.Exec(ctx, query)
	return err
}

// PasswordResetTokenRepository maneja el acceso a datos de tokens de reseteo
type PasswordResetTokenRepository struct {
	db *pgxpool.Pool
}

// NewPasswordResetTokenRepository crea una nueva instancia
func NewPasswordResetTokenRepository(db *pgxpool.Pool) *PasswordResetTokenRepository {
	return &PasswordResetTokenRepository{db: db}
}

// Create crea un nuevo token de reseteo
func (r *PasswordResetTokenRepository) Create(ctx context.Context, token *PasswordResetToken) error {
	query := `
		INSERT INTO password_reset_tokens (id, user_id, token, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`

	_, err := r.db.Exec(ctx, query, token.ID, token.UserID, token.Token, token.ExpiresAt, token.CreatedAt)
	return err
}

// FindValidByToken busca un token válido (no usado y no expirado)
func (r *PasswordResetTokenRepository) FindValidByToken(ctx context.Context, token string) (*PasswordResetToken, error) {
	query := `
		SELECT id, user_id, token, expires_at, used_at, created_at
		FROM password_reset_tokens
		WHERE token = $1 AND used_at IS NULL AND expires_at > NOW()
	`

	var resetToken PasswordResetToken
	err := r.db.QueryRow(ctx, query, token).Scan(
		&resetToken.ID,
		&resetToken.UserID,
		&resetToken.Token,
		&resetToken.ExpiresAt,
		&resetToken.UsedAt,
		&resetToken.CreatedAt,
	)

	if err != nil {
		return nil, err
	}

	return &resetToken, nil
}

// MarkAsUsed marca un token como usado
func (r *PasswordResetTokenRepository) MarkAsUsed(ctx context.Context, tokenID string) error {
	query := `
		UPDATE password_reset_tokens
		SET used_at = NOW()
		WHERE id = $1
	`

	_, err := r.db.Exec(ctx, query, tokenID)
	return err
}

// InvalidateUserTokens invalida todos los tokens de un usuario
func (r *PasswordResetTokenRepository) InvalidateUserTokens(ctx context.Context, userID string) error {
	query := `
		UPDATE password_reset_tokens
		SET used_at = NOW()
		WHERE user_id = $1 AND used_at IS NULL
	`

	_, err := r.db.Exec(ctx, query, userID)
	return err
}
