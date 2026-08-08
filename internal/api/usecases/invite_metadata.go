package usecases

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/tu-org/embolsadora-api/internal/domain"
	"github.com/tu-org/embolsadora-api/internal/platform"
	"go.uber.org/zap"
)

// TenantNameLookup es la porcion del repositorio de tenants que el mail de
// invitacion necesita. Se declara acá para poder testear la resolucion de
// nombres sin levantar el repositorio completo.
type TenantNameLookup interface {
	FindByID(ctx context.Context, id uuid.UUID) (*domain.Tenant, error)
}

// RoleNameLookup es la porcion del repositorio de roles que el mail necesita.
type RoleNameLookup interface {
	GetByIDForTenant(ctx context.Context, id string, tenantID uuid.UUID, includeGlobal bool) (*domain.Role, error)
}

// InviteDisplayNames son los valores legibles que se muestran en el mail.
// Todos los campos son best-effort: si la consulta falla el campo queda vacio
// y la plantilla cae a su copy generica. Resolver un nombre decorativo nunca
// puede bloquear una invitacion.
type InviteDisplayNames struct {
	TenantName string
	RoleName   string
}

// includeGlobal decide si la resolución de nombre de rol puede ver roles
// is_global (super_admin, tenant_manager). Lo calcula el caller a partir de
// security.CanSeePlatformInternals(ctx) — mismo criterio que ListRoles/GetRole,
// para no filtrar el nombre de un rol de plataforma en un mail de invitación
// creado por alguien que no debería saber que ese rol existe.
func resolveInviteDisplayNames(
	ctx context.Context,
	tenants TenantNameLookup,
	roles RoleNameLookup,
	tenantID string,
	roleID string,
	includeGlobal bool,
) InviteDisplayNames {
	var out InviteDisplayNames

	tenantUUID, err := uuid.Parse(tenantID)
	if err != nil {
		Log.Warn("metadata de invitacion: tenant id no parseable",
			zap.String("tenant_id", tenantID), zap.Error(err))
		return out
	}

	if tenants != nil {
		t, err := tenants.FindByID(ctx, tenantUUID)
		switch {
		case err != nil:
			Log.Warn("metadata de invitacion: fallo la consulta de tenant",
				zap.String("tenant_id", tenantID), zap.Error(err))
		case t != nil:
			out.TenantName = t.Name
		}
	}

	if roles != nil && roleID != "" {
		r, err := roles.GetByIDForTenant(ctx, roleID, tenantUUID, includeGlobal)
		switch {
		case err != nil:
			Log.Warn("metadata de invitacion: fallo la consulta de rol",
				zap.String("role_id", roleID), zap.Error(err))
		case r != nil:
			out.RoleName = r.Name
		}
	}

	return out
}

// callbackURL arma la URL de callback del frontend para este tenant, usando el
// origin que el BFF reporto para este request. Cuando no hay ninguno en el
// contexto —requests que no pasan por el middleware, o un origin rechazado—
// cae al default configurado. Invite y recovery comparten esta misma URL: la
// pagina de callback discrimina por el query param `type` que agrega GoTrue.
func callbackURL(ctx context.Context, fallbackBase, tenantID string) string {
	base := platform.AppBaseURL(ctx)
	if base == "" {
		base = fallbackBase
	}
	return fmt.Sprintf("%s/s/%s/auth/callback", base, tenantID)
}
