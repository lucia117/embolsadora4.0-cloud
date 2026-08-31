package roles

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/tu-org/embolsadora-api/internal/domain"
	rolesRepo "github.com/tu-org/embolsadora-api/internal/repo/pg/roles"
)

// EnsureAssignable valida que roleID pueda asignarse a un usuario dentro de tenantID
// por un caller cuya visibilidad de las internas de plataforma es includeGlobal
// (security.CanSeePlatformInternals, decidido en el handler).
//
// Existe porque el rol destino de una asignación se elige por id en el body del
// request, y ningún otro control lo validaba:
//   - users.CreateWithRole solo tenía la FK de user_tenant_roles.role_id, que acepta
//     'super_admin' porque el rol existe de verdad.
//   - user_roles.checkRoleAllowedForTenant preguntaba solo tenant_can_use_role(),
//     que dentro del tenant plataforma (MRG) devuelve TRUE para is_global.
//
// El resultado era escalada de privilegios: un admin de MRG (rol efectivo
// platform_admin, con users:write) creaba un usuario suyo con role='super_admin' y
// entraba como superadmin vía force-password-change. Ver el informe de la revisión
// final, Crítico 1.
//
// La validación es el mismo lookup cloakeado que ya usan ListRoles/GetRole y
// CreateInvitation: roles.GetByIDForTenant con includeGlobal. Cubre los tres ejes de
// una sola vez — existencia, pertenencia al tenant y visibilidad — y devuelve
// ErrRoleNotFound indistintamente en los tres casos.
//
// Se traduce a domain.ErrInvalidRoleID, que es lo que estos flujos ya devolvían para
// un rol inexistente (400): la convergencia por status Y por cuerpo es justamente lo
// que impide usar la asignación como oráculo de existencia de un rol de plataforma.
func EnsureAssignable(ctx context.Context, repo rolesRepo.Repository, roleID string, tenantID uuid.UUID, includeGlobal bool) error {
	if _, err := repo.GetByIDForTenant(ctx, roleID, tenantID, includeGlobal); err != nil {
		if errors.Is(err, domain.ErrRoleNotFound) {
			return domain.ErrInvalidRoleID
		}
		return fmt.Errorf("validar rol asignable: %w", err)
	}
	return nil
}
