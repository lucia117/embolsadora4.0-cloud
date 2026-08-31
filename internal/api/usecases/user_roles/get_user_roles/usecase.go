package get_user_roles

import (
	"context"

	"github.com/google/uuid"
	"github.com/tu-org/embolsadora-api/internal/domain"
	userrolesrepo "github.com/tu-org/embolsadora-api/internal/repo/pg/user_roles"
)

// Query son los parámetros de autorización de la consulta, todos resueltos en el
// handler (RBAC ya validado por middleware.RBACCheck("perm_users_view")).
type Query struct {
	UserID uuid.UUID
	// TenantID es el tenant del request (X-Tenant-ID ya validado por TenantFromHeader).
	TenantID uuid.UUID
	// CrossTenant = security.IsCrossTenantRole(rol efectivo): habilita ver las
	// membresías del usuario en otros tenants.
	CrossTenant bool
	// IncludeGlobal = security.CanSeePlatformInternals: habilita ver las membresías
	// a roles globales.
	IncludeGlobal bool
}

// UseCase defines the interface for retrieving a user's roles.
type UseCase interface {
	Execute(ctx context.Context, q Query) ([]domain.UserRoleWithContext, error)
}

type useCase struct {
	repo userrolesrepo.UserRoleRepository
}

// NewUseCase creates a new get_user_roles use case.
func NewUseCase(repo userrolesrepo.UserRoleRepository) UseCase {
	return &useCase{repo: repo}
}

// Execute retrieves the role assignments for a user, dentro del alcance que le
// corresponda al caller.
//
// El TODO que había acá ("RBAC check — platform admin only") quedó sin resolver y la
// ruta se registró sin ningún RBACCheck: cualquier usuario autenticado —un operario—
// con el user_id de un super_admin obtenía su role_id, el nombre del rol y la lista
// completa de tenants a los que pertenece. Ahora la autorización tiene tres capas, y
// ninguna vive acá adentro:
//  1. middleware.RBACCheck("perm_users_view") en el router, igual que GET /user-roles.
//  2. CrossTenant acota al tenant del request salvo para roles cross-tenant.
//  3. IncludeGlobal oculta las asignaciones a roles globales.
//
// Un usuario inexistente y uno cuyas membresías están todas cloakeadas devuelven los
// dos la misma lista vacía con 200 — indistinguibles, que es la regla.
func (uc *useCase) Execute(ctx context.Context, q Query) ([]domain.UserRoleWithContext, error) {
	return uc.repo.FindByUser(ctx, q.UserID, q.TenantID, q.CrossTenant, q.IncludeGlobal)
}
