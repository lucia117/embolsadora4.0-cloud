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
	User         UserProfileResponse  `json:"user"`
	Tenant       *TenantInfoResponse  `json:"tenant"`
	Role         *RoleInfoResponse    `json:"role"`
	Permissions  []string             `json:"permissions"`
	Capabilities CapabilitiesResponse `json:"capabilities"`
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
	ID         string `json:"id"`
	Name       string `json:"name"`
	Subdomain  string `json:"subdomain"`
	IsPlatform bool   `json:"isPlatform"`
}

// RoleInfoResponse contains the user's role in the current tenant.
type RoleInfoResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// CapabilitiesResponse expone capacidades derivadas del rol efectivo, no de
// permisos asignables. El frontend pregunta por acá en vez de comparar el rol
// contra una lista hardcodeada.
type CapabilitiesResponse struct {
	// CanCrossTenant: el usuario puede ver y operar datos de tenants distintos
	// al suyo. Deriva de isCrossTenantRoleName sobre el rol efectivo (GetMe no
	// pasa por TenantFromHeader, ver comentario ahí).
	CanCrossTenant bool `json:"canCrossTenant"`
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

	// Query the user's active tenant+role. `roles.permissions` es la fuente de
	// verdad del catálogo perm_* que consume tanto el frontend (acá) como el
	// enforcement interno (security.Can / RBACCheck, una vez que Task 4/5
	// conecten RoleContext a esta misma columna).
	var tenantID, tenantName, tenantSubdomain, roleID, roleName string
	var isPlatformTenant bool
	var permissions []string
	err := uc.db.QueryRow(ctx, `
		SELECT t.id, t.name, t.subdomain, t.is_platform_tenant,
		       r.id, r.name,
		       ARRAY(SELECT jsonb_array_elements_text(r.permissions))
		FROM user_tenant_roles utr
		JOIN tenants t ON t.id = utr.tenant_id
		JOIN roles r ON r.id = utr.role_id
		WHERE utr.user_id = $1 AND utr.status = 'active'
		LIMIT 1
	`, user.ID).Scan(&tenantID, &tenantName, &tenantSubdomain, &isPlatformTenant,
		&roleID, &roleName, &permissions)

	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}

	if err == nil {
		effectiveRole := security.EffectiveRole(roleID, isPlatformTenant)

		resp.Tenant = &TenantInfoResponse{
			ID:         tenantID,
			Name:       tenantName,
			Subdomain:  tenantSubdomain,
			IsPlatform: isPlatformTenant,
		}
		resp.Role = &RoleInfoResponse{
			ID:   effectiveRole,
			Name: roleName,
		}
		if permissions == nil {
			permissions = []string{}
		}
		resp.Permissions = permissions
		resp.Capabilities = CapabilitiesResponse{
			CanCrossTenant: isCrossTenantRoleName(effectiveRole),
		}
	}

	return resp, nil
}

// isCrossTenantRoleName reproduce localmente la lista de roles cross-tenant
// que antes vivía en el mapa security.crossTenantRoles (ver comentario en
// security.IsCrossTenantRole: GetMe no pasa por TenantFromHeader, así que no
// tiene un security.RoleContext resuelto en su ctx). Placeholder hasta que
// Task 5 conecte esto a roles.is_global directamente.
func isCrossTenantRoleName(roleName string) bool {
	switch roleName {
	case "super_admin", "tenant_manager", "platform_admin":
		return true
	default:
		return false
	}
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
