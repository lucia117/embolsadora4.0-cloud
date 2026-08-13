package roles_test

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/tu-org/embolsadora-api/internal/domain"
	rolesRepo "github.com/tu-org/embolsadora-api/internal/repo/pg/roles"
)

// platformTenantUUID es el tenant MRG sembrado por la migración 000002.
var platformTenantUUID = uuid.MustParse("11b36b85-033d-4bb3-9e31-4c92161887c0")

func openPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	return pool
}

func roleIDs(roles []*domain.Role) []string {
	ids := make([]string, 0, len(roles))
	for _, r := range roles {
		ids = append(ids, r.ID)
	}
	return ids
}

// createTestTenant siembra un tenant descartable para aislar los tests de
// scoping por tenant. El cleanup se registra antes de que exista ninguna fila
// que dependa de él, siguiendo el mismo orden que openPool: primero el borrado
// del tenant (los roles custom que cuelguen de él se van con el ON DELETE
// CASCADE de roles.tenant_id), después nada más — el pool lo cierra
// openPool.
func createTestTenant(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	id := uuid.New()
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM tenants WHERE id = $1`, id)
	})
	_, err := pool.Exec(context.Background(), `
		INSERT INTO tenants (id, name, company_name, subdomain)
		VALUES ($1, 'Tenant de test', 'Tenant de test', $2)
	`, id, "test-"+id.String())
	require.NoError(t, err)
	return id
}

func TestListOcultaRolesGlobalesAlNoSuperadmin(t *testing.T) {
	pool := openPool(t)
	repo := rolesRepo.NewPostgresRepository(pool)

	roles, err := repo.List(context.Background(), platformTenantUUID, false)
	require.NoError(t, err)

	ids := roleIDs(roles)
	require.NotContains(t, ids, "super_admin", "super_admin debe ser invisible")
	require.NotContains(t, ids, "tenant_manager", "tenant_manager también es rol de plataforma")
	require.Contains(t, ids, "admin", "los roles tenant-scoped siguen visibles")
	require.Contains(t, ids, "operario")
}

func TestListMuestraRolesGlobalesAlSuperadmin(t *testing.T) {
	pool := openPool(t)
	repo := rolesRepo.NewPostgresRepository(pool)

	roles, err := repo.List(context.Background(), platformTenantUUID, true)
	require.NoError(t, err)

	ids := roleIDs(roles)
	require.Contains(t, ids, "super_admin")
	require.Contains(t, ids, "tenant_manager")
	require.Contains(t, ids, "admin")
}

func TestGetByIDForTenantDevuelveNotFoundParaRolOculto(t *testing.T) {
	pool := openPool(t)
	repo := rolesRepo.NewPostgresRepository(pool)

	// 404, no 403: un 403 confirmaría que el rol existe.
	_, err := repo.GetByIDForTenant(context.Background(), "super_admin", platformTenantUUID, false)
	require.ErrorIs(t, err, domain.ErrRoleNotFound)

	role, err := repo.GetByIDForTenant(context.Background(), "super_admin", platformTenantUUID, true)
	require.NoError(t, err)
	require.Equal(t, "super_admin", role.ID)
}

// TestGetByIDForTenantOcultaRolCustomDeOtroTenant cubre la segunda mitad del
// hallazgo crítico de la revisión: GetByIDForTenant no filtraba por tenant_id
// para roles no globales (tenant_can_use_role devuelve TRUE incondicionalmente
// cuando is_global=false), así que un admin de cualquier tenant que conociera
// el id de un rol custom ajeno podía leerlo/editarlo/borrarlo. El WHERE ahora
// exige tenant_id = $2 OR tenant_id IS NULL, igual que List.
func TestGetByIDForTenantOcultaRolCustomDeOtroTenant(t *testing.T) {
	pool := openPool(t)
	repo := rolesRepo.NewPostgresRepository(pool)
	ctx := context.Background()

	tenantA := createTestTenant(t, pool)
	tenantB := createTestTenant(t, pool)

	roleA := &domain.Role{
		ID:          "custom_" + uuid.New().String()[:6],
		Name:        "Rol custom de A",
		Permissions: []string{},
		TenantID:    &tenantA,
	}
	require.NoError(t, repo.Create(ctx, roleA))
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM roles WHERE id = $1`, roleA.ID)
	})

	// tenant B no puede ver (ni por lo tanto editar/borrar) el rol custom de tenant A.
	_, err := repo.GetByIDForTenant(ctx, roleA.ID, tenantB, false)
	require.ErrorIs(t, err, domain.ErrRoleNotFound, "un rol custom de otro tenant debe ser invisible, no solo protegido por el chequeo de IsSystemRole")

	// tenant A sigue viendo su propio rol.
	got, err := repo.GetByIDForTenant(ctx, roleA.ID, tenantA, false)
	require.NoError(t, err)
	require.Equal(t, roleA.ID, got.ID)
}

// TestGetByIDForTenantRolGlobalDevuelveNotFoundNoSystemRole es el test que la
// revisión pidió explícitamente: confirma, a nivel de la consulta que
// UpdateRole/DeleteRole ahora usan, que un caller no-superadmin recibe
// ErrRoleNotFound para un rol global — nunca llega a existir la oportunidad de
// devolver ErrRoleIsSystemRole, porque el rol ni siquiera sale de la consulta.
// El test de service-level equivalente (que ejercita el orden de checks
// GetByIDForTenant → IsSystemRole con un fake) vive en
// internal/app/roles/service_test.go.
func TestGetByIDForTenantRolGlobalDevuelveNotFoundNoSystemRole(t *testing.T) {
	pool := openPool(t)
	repo := rolesRepo.NewPostgresRepository(pool)
	ctx := context.Background()

	role, err := repo.GetByIDForTenant(ctx, "super_admin", platformTenantUUID, false)
	require.Nil(t, role)
	require.ErrorIs(t, err, domain.ErrRoleNotFound)
	require.NotErrorIs(t, err, domain.ErrRoleIsSystemRole, "un rol oculto nunca debe llegar a la etapa donde se evalúa IsSystemRole")
}

// TestListOcultaRolesPlatformOnlyEnTenantCliente es el regression test de
// B-006: admin/operario son is_global=false (a diferencia de super_admin/
// tenant_manager), así que antes del fix tenant_can_use_role los dejaba pasar
// en cualquier tenant. Ahora deben comportarse igual que los roles is_global:
// invisibles fuera del tenant plataforma.
func TestListOcultaRolesPlatformOnlyEnTenantCliente(t *testing.T) {
	pool := openPool(t)
	repo := rolesRepo.NewPostgresRepository(pool)
	clientTenant := createTestTenant(t, pool)

	roles, err := repo.List(context.Background(), clientTenant, false)
	require.NoError(t, err)

	ids := roleIDs(roles)
	require.NotContains(t, ids, "admin", "admin es platform-only, no debe verse fuera de MRG")
	require.NotContains(t, ids, "operario", "operario es platform-only, no debe verse fuera de MRG")
	require.Contains(t, ids, "cliente_admin", "los roles de tenant cliente siguen visibles")
	require.Contains(t, ids, "cliente_operario")
}

// TestListMuestraRolesPlatformOnlyEnTenantPlataforma confirma que el fix no
// rompe el caso positivo: dentro de MRG, admin/operario siguen visibles.
func TestListMuestraRolesPlatformOnlyEnTenantPlataforma(t *testing.T) {
	pool := openPool(t)
	repo := rolesRepo.NewPostgresRepository(pool)

	roles, err := repo.List(context.Background(), platformTenantUUID, false)
	require.NoError(t, err)

	ids := roleIDs(roles)
	require.Contains(t, ids, "admin")
	require.Contains(t, ids, "operario")
}

// TestGetByIDForTenantOcultaAdminEnTenantCliente es el equivalente para
// GetByIDForTenant (la validación que usa EnsureAssignable en el camino de
// asignación de roles, no solo el listado).
func TestGetByIDForTenantOcultaAdminEnTenantCliente(t *testing.T) {
	pool := openPool(t)
	repo := rolesRepo.NewPostgresRepository(pool)
	clientTenant := createTestTenant(t, pool)

	_, err := repo.GetByIDForTenant(context.Background(), "admin", clientTenant, false)
	require.ErrorIs(t, err, domain.ErrRoleNotFound)

	role, err := repo.GetByIDForTenant(context.Background(), "cliente_admin", clientTenant, false)
	require.NoError(t, err)
	require.Equal(t, "cliente_admin", role.ID)
}
