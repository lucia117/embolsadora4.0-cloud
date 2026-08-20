package users_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	usersRepo "github.com/tu-org/embolsadora-api/internal/repo/pg/users"
)

// TestGetByIDWithRolesCrossTenant cubre el item #3 del handoff 2026-08-19: el
// fix de Hallazgo A solo llegó a GetByID (la variante sin ?include=roles),
// GetByIDWithRoles quedó con el mismo bug de resolución cross-tenant.
func TestGetByIDWithRolesCrossTenantFalseDaNotFoundParaUsuarioDeOtroTenant(t *testing.T) {
	pool := poolOrSkip(t)
	repo := usersRepo.NewPostgresRepository(pool)
	ctx := context.Background()

	tenantA := seedTenant(t, pool)
	tenantB := seedTenant(t, pool)
	userInB := seedUserInTenant(t, pool, tenantB)

	_, err := repo.GetByIDWithRoles(ctx, tenantA, userInB, false, false)
	require.Error(t, err, "sin crossTenant, un usuario de otro tenant debe seguir devolviendo 404")
}

func TestGetByIDWithRolesCrossTenantTrueEncuentraUsuarioDeOtroTenant(t *testing.T) {
	pool := poolOrSkip(t)
	repo := usersRepo.NewPostgresRepository(pool)
	ctx := context.Background()

	tenantA := seedTenant(t, pool)
	tenantB := seedTenant(t, pool)
	userInB := seedUserInTenant(t, pool, tenantB)

	uwr, err := repo.GetByIDWithRoles(ctx, tenantA, userInB, true, false)
	require.NoError(t, err)
	require.Equal(t, userInB, uwr.User.ID)
	require.Equal(t, tenantB, uwr.User.TenantID, "el tenant_id debe ser el real del target")
	require.Len(t, uwr.Roles, 1)
	require.Equal(t, "cliente_operario", uwr.Roles[0].ID)
}

// TestGetByIDWithRolesCrossTenantTrueNoBypasaCloakingDeRolGlobal es el
// espejo, para GetByIDWithRoles, de
// TestGetByIDCrossTenantTrueNoBypasaCloakingDeRolGlobal en
// cross_tenant_test.go. Task 5 reescribió GetByIDWithRoles del JOIN fijo a
// $2 (que con un target de otro tenant siempre dejaba utr/r en NULL, y por
// lo tanto COALESCE(r.is_global, FALSE) = FALSE incondicionalmente — el
// cloak de rol global nunca se disparaba para targets cross-tenant) al mismo
// patrón LEFT JOIN LATERAL que GetByID. Ese cambio es una mejora de
// seguridad genuina: ahora el cloak SÍ aplica a un super_admin/tenant_manager
// ajeno. Pero GetByIDWithRoles es el endpoint que además devuelve el nombre
// del rol y los permisos en el body — "una fuga peor que la de GetByID" si
// esto regresara — y no tenía ningún test que lo asegurara.
func TestGetByIDWithRolesCrossTenantTrueNoBypasaCloakingDeRolGlobal(t *testing.T) {
	pool := poolOrSkip(t)
	repo := usersRepo.NewPostgresRepository(pool)
	ctx := context.Background()

	// super_admin (is_global) solo puede asignarse en el tenant plataforma —
	// trg_enforce_platform_role_tenant (migración 000004) lo impone a nivel DB.
	tenantA := seedTenant(t, pool)
	superInPlatform := seedUserWithRole(t, pool, platformTenant, "super_admin")

	// crossTenant=true (p.ej. platform_admin/tenant_manager) pero
	// includeGlobal=false (no es super_admin, no puede ver internals de
	// plataforma): debe seguir dando 404 -- sin filtrar rol ni permisos --
	// para un usuario cuyo rol activo real es is_global, sin importar en qué
	// tenant viva.
	_, err := repo.GetByIDWithRoles(ctx, tenantA, superInPlatform, true, false)
	require.Error(t, err, "crossTenant=true no debe bypasear el cloaking de is_global en GetByIDWithRoles")
}
