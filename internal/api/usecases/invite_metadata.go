package usecases

import (
	"context"

	"github.com/google/uuid"
	"github.com/tu-org/embolsadora-api/internal/domain"
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
	GetByIDForTenant(ctx context.Context, id string, tenantID uuid.UUID) (*domain.Role, error)
}

// InviteDisplayNames son los valores legibles que se muestran en el mail.
// Todos los campos son best-effort: si la consulta falla el campo queda vacio
// y la plantilla cae a su copy generica. Resolver un nombre decorativo nunca
// puede bloquear una invitacion.
type InviteDisplayNames struct {
	TenantName string
	RoleName   string
}

func resolveInviteDisplayNames(
	ctx context.Context,
	tenants TenantNameLookup,
	roles RoleNameLookup,
	tenantID string,
	roleID string,
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
		r, err := roles.GetByIDForTenant(ctx, roleID, tenantUUID)
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
