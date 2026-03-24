package usecases

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/tu-org/embolsadora-api/internal/domain"
	"github.com/tu-org/embolsadora-api/internal/platform"
	"github.com/tu-org/embolsadora-api/internal/security"
)

// MeResponse is the response for GET /api/v1/me.
type MeResponse struct {
	User        UserProfileResponse `json:"user"`
	Tenant      *TenantInfoResponse `json:"tenant"`
	Role        *RoleInfoResponse   `json:"role"`
	Permissions []string            `json:"permissions"`
}

// UserProfileResponse contains public user identity fields.
type UserProfileResponse struct {
	ID                     string  `json:"id"`
	Email                  *string `json:"email"`
	Name                   *string `json:"name"`
	PasswordChangeRequired bool    `json:"password_change_required"`
}

// TenantInfoResponse contains tenant display fields.
type TenantInfoResponse struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Subdomain string `json:"subdomain"`
}

// RoleInfoResponse contains the user's role in the current tenant.
type RoleInfoResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// MeUsecase handles the GET /api/v1/me endpoint logic.
type MeUsecase struct {
	db *pgxpool.Pool
}

func NewMeUsecase(db *pgxpool.Pool) *MeUsecase {
	return &MeUsecase{db: db}
}

// GetMe builds the full user profile response including tenant, role and permissions.
func (uc *MeUsecase) GetMe(ctx context.Context) (*MeResponse, error) {
	user, ok := platform.DomainUser(ctx).(*domain.User)
	if !ok || user == nil {
		return nil, domain.ErrForbidden
	}

	resp := &MeResponse{
		User: UserProfileResponse{
			ID:                     user.ID,
			Email:                  strPtr(user.Email),
			Name:                   strPtr(user.Name),
			PasswordChangeRequired: user.PasswordChangeRequired,
		},
		Tenant:      nil,
		Role:        nil,
		Permissions: []string{},
	}

	// Query the user's active tenant+role (regular users belong to exactly one tenant)
	var tenantID, tenantName, tenantSubdomain, roleID, roleName string
	err := uc.db.QueryRow(ctx, `
		SELECT t.id, t.name, t.subdomain, r.id, r.name
		FROM user_tenant_roles utr
		JOIN tenants t ON t.id = utr.tenant_id
		JOIN roles r ON r.id = utr.role_id
		WHERE utr.user_id = $1 AND utr.status = 'active'
		LIMIT 1
	`, user.ID).Scan(&tenantID, &tenantName, &tenantSubdomain, &roleID, &roleName)

	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}

	if err == nil {
		resp.Tenant = &TenantInfoResponse{
			ID:        tenantID,
			Name:      tenantName,
			Subdomain: tenantSubdomain,
		}
		resp.Role = &RoleInfoResponse{
			ID:   roleID,
			Name: roleName,
		}
		resp.Permissions = security.PermissionsForRole(roleName)
	}

	return resp, nil
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
