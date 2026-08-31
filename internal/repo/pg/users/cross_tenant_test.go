package users_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	usersRepo "github.com/tu-org/embolsadora-api/internal/repo/pg/users"
)

// seedTenant crea un tenant nuevo con subdominio único y lo limpia al terminar.
func seedTenant(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	ctx := context.Background()
	id := uuid.New().String()
	_, err := pool.Exec(ctx, `
		INSERT INTO tenants (id, name, company_name, subdomain)
		VALUES ($1, 'Cross Tenant Test', 'Cross Tenant Test', $2)
	`, id, "xt-"+id[:8])
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM tenants WHERE id = $1`, id)
	})
	return id
}

// seedUserInTenant crea un usuario con membresía activa 'cliente_operario' en
// el tenant dado (rol tenant-scoped, no is_global, para no mezclar con el eje
// includeGlobal que ya cubre cloaking_test.go).
func seedUserInTenant(t *testing.T, pool *pgxpool.Pool, tenantID string) string {
	t.Helper()
	return seedUserWithRole(t, pool, tenantID, "cliente_operario")
}

// seedUserWithRole crea un usuario con membresía activa con el rol dado en el
// tenant dado. A diferencia de seedUserInTenant, permite parametrizar el rol
// para poder sembrar un is_global (super_admin) en un tenant distinto del que
// pide la request — ver TestGetByIDCrossTenantTrueNoBypasaCloakingDeRolGlobal.
func seedUserWithRole(t *testing.T, pool *pgxpool.Pool, tenantID, roleID string) string {
	t.Helper()
	ctx := context.Background()
	userID := uuid.New().String()
	utrID := uuid.New().String()

	// last_name se siembra explícito (a diferencia de first_name, que cae a
	// COALESCE(u.first_name, u.name, '') en las lecturas) porque
	// UpdateUser.Validate/Repository.Update validan el User completo, no solo
	// los campos que cambian: sin esto, TestUpdateUserCrossTenantTrue... (que
	// solo pisa FirstName) fallaba con "last_name is required" al reusar el
	// last_name vacío que trae este fixture.
	_, err := pool.Exec(ctx,
		`INSERT INTO users (id, email, name, last_name, status) VALUES ($1, $2, 'Cross Tenant User', 'Tenant', 'active')`,
		userID, userID+"@xtenant.local")
	require.NoError(t, err)
	_, err = pool.Exec(ctx,
		`INSERT INTO user_tenant_roles (id, user_id, tenant_id, role_id, status, assigned_at)
		 VALUES ($1, $2, $3, $4, 'active', NOW())`,
		utrID, userID, tenantID, roleID)
	require.NoError(t, err)

	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM user_tenant_roles WHERE id = $1`, utrID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, userID)
	})
	return userID
}

func TestGetByIDCrossTenantFalseDaNotFoundParaUsuarioDeOtroTenant(t *testing.T) {
	pool := poolOrSkip(t)
	repo := usersRepo.NewPostgresRepository(pool)
	ctx := context.Background()

	tenantA := seedTenant(t, pool)
	tenantB := seedTenant(t, pool)
	userInB := seedUserInTenant(t, pool, tenantB)

	// Regresión: un caller sin capability cross-tenant (crossTenant=false, el
	// comportamiento de siempre) NO debe poder ver un usuario de otro tenant.
	_, err := repo.GetByID(ctx, tenantA, userInB, false, false)
	require.Error(t, err, "sin crossTenant, un usuario de otro tenant debe seguir devolviendo 404")
}

func TestGetByIDCrossTenantTrueEncuentraUsuarioDeOtroTenant(t *testing.T) {
	pool := poolOrSkip(t)
	repo := usersRepo.NewPostgresRepository(pool)
	ctx := context.Background()

	tenantA := seedTenant(t, pool)
	tenantB := seedTenant(t, pool)
	userInB := seedUserInTenant(t, pool, tenantB)

	// Hallazgo A: un super_admin parado en el contexto de tenantA pidiendo un
	// usuario de tenantB debe encontrarlo, no 404.
	u, err := repo.GetByID(ctx, tenantA, userInB, true, false)
	require.NoError(t, err)
	require.Equal(t, userInB, u.ID)

	// Hallazgo 2 (revisión final): el rol y el tenant devueltos deben ser los
	// REALES del target (cliente_operario en tenantB), no el fallback legado
	// a users.role ('user') ni el tenant del caller (tenantA) — antes del fix,
	// el join fijo a $2 dejaba utr/r en NULL para un target de otro tenant y
	// las COALESCE caían silenciosamente a esos defaults.
	require.Equal(t, "cliente_operario", u.Role, "el rol debe ser el real del target, no el fallback legado")
	require.Equal(t, tenantB, u.TenantID, "el tenant_id debe ser el real del target, no el del caller")
}

// TestGetByIDCrossTenantTrueNoBypasaCloakingDeRolGlobal es la regresión del
// Hallazgo 1 de la revisión final: con el join fijo a $2, un target de otro
// tenant siempre resolvía utr/r en NULL, y COALESCE(r.is_global, FALSE)
// evaluaba a TRUE incondicionalmente — bypaseando includeGlobal por completo.
// Eso dejaba a un caller cross-tenant SIN CanSeePlatformInternals (p.ej.
// tenant_manager, platform_admin — cross-tenant pero no super_admin) resolver
// un super_admin ajeno, exactamente la fuga de existencia que
// TestGetByIDDevuelveNotFoundParaUsuarioOculto y el comentario "un 403
// confirmaría que el usuario existe" en este archivo existen para evitar.
func TestGetByIDCrossTenantTrueNoBypasaCloakingDeRolGlobal(t *testing.T) {
	pool := poolOrSkip(t)
	repo := usersRepo.NewPostgresRepository(pool)
	ctx := context.Background()

	// super_admin (is_global) solo puede asignarse en el tenant plataforma —
	// trg_enforce_platform_role_tenant (migración 000004) lo impone a nivel DB.
	// El caller pide parado en un tenant distinto (tenantA), simulando un
	// platform_admin/tenant_manager (cross-tenant) resolviendo un usuario que
	// en realidad vive en el tenant plataforma.
	tenantA := seedTenant(t, pool)
	superInPlatform := seedUserWithRole(t, pool, platformTenant, "super_admin")

	// crossTenant=true (p.ej. platform_admin) pero includeGlobal=false (no es
	// super_admin, no puede ver internals de plataforma): debe seguir dando
	// 404 para un usuario cuyo rol activo real es is_global, sin importar en
	// qué tenant viva.
	_, err := repo.GetByID(ctx, tenantA, superInPlatform, true, false)
	require.Error(t, err, "crossTenant=true no debe bypasear el cloaking de is_global")
}

// TestGetByIDCrossTenantTrueConDobleMembresiaPrefiereLaDelTenantDeLaRequest
// cubre el desempate determinístico: un usuario puede tener membresías
// activas simultáneas en más de un tenant (índice único es (user_id,
// tenant_id), no (user_id) — ver idx_utr_active_unique en la migración). Con
// el join relajado a "cualquier tenant" bajo crossTenant=true, sin desempate
// el resultado sería no determinístico (QueryRow devolvería la fila que
// Postgres eligiera primero). El ORDER BY debe preferir siempre la membresía
// del tenant de la request cuando existe.
func TestGetByIDCrossTenantTrueConDobleMembresiaPrefiereLaDelTenantDeLaRequest(t *testing.T) {
	pool := poolOrSkip(t)
	repo := usersRepo.NewPostgresRepository(pool)
	ctx := context.Background()

	tenantA := seedTenant(t, pool)
	tenantB := seedTenant(t, pool)

	userID := seedUserWithRole(t, pool, tenantA, "cliente_operario")
	utrBID := uuid.New().String()
	_, err := pool.Exec(ctx,
		`INSERT INTO user_tenant_roles (id, user_id, tenant_id, role_id, status, assigned_at)
		 VALUES ($1, $2, $3, 'cliente_admin', 'active', NOW())`,
		utrBID, userID, tenantB)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM user_tenant_roles WHERE id = $1`, utrBID)
	})

	// La request pide el usuario parada en el contexto de tenantA: el
	// desempate debe preferir la membresía de tenantA sobre la de tenantB,
	// aunque ambas sean activas y válidas.
	u, err := repo.GetByID(ctx, tenantA, userID, true, false)
	require.NoError(t, err)
	require.Equal(t, tenantA, u.TenantID, "debe ganar la membresía del tenant de la request")
	require.Equal(t, "cliente_operario", u.Role, "el rol debe corresponder a la membresía de tenantA, no a la de tenantB")
}

// TestGetByIDCrossTenantTrueConMembresiaSinAssignedAtNoGanaElDesempate cubre
// item #7 del handoff 2026-08-19: sin NULLS LAST explícito, el default de
// Postgres para DESC es NULLS FIRST, así que una membresía activa con
// assigned_at NULL ganaba el desempate aunque no fuera "la más reciente".
func TestGetByIDCrossTenantTrueConMembresiaSinAssignedAtNoGanaElDesempate(t *testing.T) {
	pool := poolOrSkip(t)
	repo := usersRepo.NewPostgresRepository(pool)
	ctx := context.Background()

	tenantA := seedTenant(t, pool)
	tenantB := seedTenant(t, pool)

	// Membresía en tenantB con assigned_at seteado (la que debe ganar el desempate).
	userID := seedUserWithRole(t, pool, tenantB, "cliente_operario")

	// Segunda membresía activa, en tenantC, con assigned_at NULL — antes del
	// fix, NULLS FIRST la hacía ganar el ORDER BY ... assigned_at DESC pese a
	// no tener fecha real.
	tenantC := seedTenant(t, pool)
	utrCID := uuid.New().String()
	_, err := pool.Exec(ctx,
		`INSERT INTO user_tenant_roles (id, user_id, tenant_id, role_id, status, assigned_at)
		 VALUES ($1, $2, $3, 'cliente_admin', 'active', NULL)`,
		utrCID, userID, tenantC)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM user_tenant_roles WHERE id = $1`, utrCID)
	})

	// La request pide parada en tenantA (no matchea ninguna de las dos
	// membresías), así que el desempate cae en assigned_at DESC: debe ganar
	// la de tenantB (assigned_at real), no la de tenantC (NULL).
	u, err := repo.GetByID(ctx, tenantA, userID, true, false)
	require.NoError(t, err)
	require.Equal(t, tenantB, u.TenantID, "la membresía con assigned_at real debe ganar sobre la de assigned_at NULL")
}
