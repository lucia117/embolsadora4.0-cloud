package assign_user_role_test

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	ucAssign "github.com/tu-org/embolsadora-api/internal/api/usecases/user_roles/assign_user_role"
	"github.com/tu-org/embolsadora-api/internal/domain"
	rolesRepo "github.com/tu-org/embolsadora-api/internal/repo/pg/roles"
	userRolesRepo "github.com/tu-org/embolsadora-api/internal/repo/pg/user_roles"
)

// Regresión del Crítico 1 de la revisión final, en la capa que decide la respuesta
// HTTP: un platform_admin (admin del tenant plataforma) hacía
// POST /api/v1/user-roles {"roleId":"super_admin"} y se llevaba una UTR activa de
// superadmin. Ni el usecase ni checkRoleAllowedForTenant validaban el rol contra la
// visibilidad del caller — tenant_can_use_role() devuelve TRUE para is_global dentro
// de MRG, que es exactamente el tenant desde el que opera el atacante.
//
// Contra Postgres real: la validación es una consulta (roles.GetByIDForTenant), y lo
// que se está fijando es su WHERE.

var platformTenantUUID = uuid.MustParse("11b36b85-033d-4bb3-9e31-4c92161887c0")

func poolOrSkip(t *testing.T) *pgxpool.Pool {
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

func seedUser(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	id := uuid.New()
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM user_tenant_roles WHERE user_id = $1`, id)
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, id)
	})
	_, err := pool.Exec(context.Background(),
		`INSERT INTO users (id, email, name, status) VALUES ($1, $2, 'Escalada Test', 'active')`,
		id, id.String()+"@escalada.local")
	require.NoError(t, err)
	return id
}

func newUseCase(pool *pgxpool.Pool) ucAssign.UseCase {
	return ucAssign.NewUseCase(
		userRolesRepo.NewUserRoleRepository(pool),
		rolesRepo.NewPostgresRepository(pool),
	)
}

// TestAssignSuperAdminComoPlatformAdminFalla es el escenario del informe, tal cual.
func TestAssignSuperAdminComoPlatformAdminFalla(t *testing.T) {
	pool := poolOrSkip(t)
	uc := newUseCase(pool)
	ctx := context.Background()

	victima := seedUser(t, pool)

	// IncludeGlobal=false es lo que security.CanSeePlatformInternals devuelve para
	// platform_admin (solo super_admin da true).
	res, err := uc.Execute(ctx, ucAssign.AssignRequest{
		UserID:        victima,
		TenantID:      platformTenantUUID,
		RoleID:        "super_admin",
		IncludeGlobal: false,
	})

	require.Nil(t, res)
	require.ErrorIs(t, err, domain.ErrInvalidRoleID,
		"asignar super_admin sin poder verlo tiene que fallar como si el rol no existiera (400), no 403 ni 201")

	var count int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM user_tenant_roles WHERE user_id = $1`, victima).Scan(&count))
	require.Zero(t, count, "no se creó ninguna membresía: la validación corre ANTES del INSERT")
}

// TestAssignRolInexistenteDaElMismoError: la convergencia que hace que el fallo no
// sea un oráculo. Mismo error, mismo mensaje, mismo status.
func TestAssignRolInexistenteDaElMismoError(t *testing.T) {
	pool := poolOrSkip(t)
	uc := newUseCase(pool)
	ctx := context.Background()

	victima := seedUser(t, pool)
	base := ucAssign.AssignRequest{UserID: victima, TenantID: platformTenantUUID, IncludeGlobal: false}

	oculto := base
	oculto.RoleID = "super_admin"
	_, errOculto := uc.Execute(ctx, oculto)

	inexistente := base
	inexistente.RoleID = "rol_que_no_existe_" + uuid.New().String()[:8]
	_, errInexistente := uc.Execute(ctx, inexistente)

	require.ErrorIs(t, errOculto, domain.ErrInvalidRoleID)
	require.ErrorIs(t, errInexistente, domain.ErrInvalidRoleID)
	require.Equal(t, errInexistente.Error(), errOculto.Error())
}

// TestAssignSuperAdminComoSuperAdminFunciona es el control positivo: el caso
// legítimo no se rompió. Sin este test, "todo devuelve 400" también pasaría.
func TestAssignSuperAdminComoSuperAdminFunciona(t *testing.T) {
	pool := poolOrSkip(t)
	uc := newUseCase(pool)
	ctx := context.Background()

	victima := seedUser(t, pool)

	res, err := uc.Execute(ctx, ucAssign.AssignRequest{
		UserID:        victima,
		TenantID:      platformTenantUUID,
		RoleID:        "super_admin",
		IncludeGlobal: true,
	})
	require.NoError(t, err)
	require.NotNil(t, res)
	require.Equal(t, "super_admin", *res.RoleID)
}

// TestAssignRolNormalSigueFuncionando: el flujo cotidiano (asignar 'operario' en el
// tenant plataforma) no se ve afectado por la validación nueva.
func TestAssignRolNormalSigueFuncionando(t *testing.T) {
	pool := poolOrSkip(t)
	uc := newUseCase(pool)
	ctx := context.Background()

	victima := seedUser(t, pool)

	res, err := uc.Execute(ctx, ucAssign.AssignRequest{
		UserID:        victima,
		TenantID:      platformTenantUUID,
		RoleID:        "operario",
		IncludeGlobal: false,
	})
	require.NoError(t, err)
	require.NotNil(t, res)
	require.Equal(t, "operario", *res.RoleID)
}

// ── Des-anonimización por escritura ─────────────────────────────────────────
//
// El cloaking de user_tenant_roles es por fila: mira el rol de la fila. Pero acá
// la fila la escribe el atacante y el rol lo elige él. Un admin de un tenant
// cliente que conozca el uuid del super_admin hacía POST /user-roles con
// {"userId": <super_admin>, "roleId": "operario"} contra SU tenant, y la UTR
// quedaba escrita y visible en GET /user-roles con email y nombre reales.
//
// La regla es por identidad: si el destino es una cuenta de plataforma (membresía
// no revocada a un rol is_global en algún tenant), el caller sin includeGlobal
// recibe exactamente lo mismo que si el userId no existiera.

func seedTenantAjeno(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	id := uuid.New()
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM tenants WHERE id = $1`, id)
	})
	_, err := pool.Exec(context.Background(), `
		INSERT INTO tenants (id, name, company_name, subdomain)
		VALUES ($1, 'Cordoba test', 'Cordoba test', $2)
	`, id, "cba-"+id.String())
	require.NoError(t, err)
	return id
}

// seedSuperAdmin siembra la cuenta interna: usuario + super_admin activo en MRG.
func seedSuperAdmin(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	id := seedUser(t, pool)
	_, err := pool.Exec(context.Background(),
		`INSERT INTO user_tenant_roles (id, user_id, tenant_id, role_id, status, assigned_at)
		 VALUES ($1, $2, $3, 'super_admin', 'active', NOW())`,
		uuid.New(), id, platformTenantUUID)
	require.NoError(t, err)
	return id
}

// TestAssignSobreSuperAdminDesdeTenantClienteFalla es el escenario del hallazgo.
func TestAssignSobreSuperAdminDesdeTenantClienteFalla(t *testing.T) {
	pool := poolOrSkip(t)
	uc := newUseCase(pool)
	ctx := context.Background()

	cordoba := seedTenantAjeno(t, pool)
	superadmin := seedSuperAdmin(t, pool)

	// cliente_operario, no operario: desde la migración 000010 operario es
	// platform-only y tenant_can_use_role lo rechazaría antes de llegar al
	// guard de identidad que este test ejercita.
	res, err := uc.Execute(ctx, ucAssign.AssignRequest{
		UserID:        superadmin,
		TenantID:      cordoba,
		RoleID:        "cliente_operario",
		IncludeGlobal: false,
	})
	require.Nil(t, res)
	require.ErrorIs(t, err, domain.ErrInvalidUserID,
		"asignarle un rol a un usuario que no podés ver tiene que responder como userId inexistente (400)")

	var count int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM user_tenant_roles WHERE user_id = $1 AND tenant_id = $2`,
		superadmin, cordoba).Scan(&count))
	require.Zero(t, count, "no puede quedar escrita ninguna membresía")
}

// TestAssignSobreSuperAdminConvergeConUsuarioInexistente: mismo error, mismo
// mensaje, mismo status. Sin esto sigue siendo un oráculo.
func TestAssignSobreSuperAdminConvergeConUsuarioInexistente(t *testing.T) {
	pool := poolOrSkip(t)
	uc := newUseCase(pool)
	ctx := context.Background()

	cordoba := seedTenantAjeno(t, pool)
	superadmin := seedSuperAdmin(t, pool)

	base := ucAssign.AssignRequest{TenantID: cordoba, RoleID: "cliente_operario", IncludeGlobal: false}

	oculto := base
	oculto.UserID = superadmin
	_, errOculto := uc.Execute(ctx, oculto)

	inexistente := base
	inexistente.UserID = uuid.New()
	_, errInexistente := uc.Execute(ctx, inexistente)

	require.ErrorIs(t, errOculto, domain.ErrInvalidUserID)
	require.ErrorIs(t, errInexistente, domain.ErrInvalidUserID)
	require.Equal(t, errInexistente.Error(), errOculto.Error())
}

// TestAssignSobreSuperAdminComoSuperAdminFunciona: control positivo. El
// superadmin legítimo sigue pudiendo darle una membresía a otro superadmin.
func TestAssignSobreSuperAdminComoSuperAdminFunciona(t *testing.T) {
	pool := poolOrSkip(t)
	uc := newUseCase(pool)
	ctx := context.Background()

	cordoba := seedTenantAjeno(t, pool)
	superadmin := seedSuperAdmin(t, pool)

	res, err := uc.Execute(ctx, ucAssign.AssignRequest{
		UserID:        superadmin,
		TenantID:      cordoba,
		RoleID:        "cliente_operario",
		IncludeGlobal: true,
	})
	require.NoError(t, err)
	require.NotNil(t, res)
	require.Equal(t, "cliente_operario", *res.RoleID)
}

// TestAssignSobreUsuarioComunEnOtroTenantSigueFuncionando: el caso principal de
// POST /user-roles (usuario sin membresía previa en ese tenant) intacto.
func TestAssignSobreUsuarioComunEnOtroTenantSigueFuncionando(t *testing.T) {
	pool := poolOrSkip(t)
	uc := newUseCase(pool)
	ctx := context.Background()

	cordoba := seedTenantAjeno(t, pool)
	comun := seedUser(t, pool)

	res, err := uc.Execute(ctx, ucAssign.AssignRequest{
		UserID:        comun,
		TenantID:      cordoba,
		RoleID:        "cliente_operario",
		IncludeGlobal: false,
	})
	require.NoError(t, err)
	require.NotNil(t, res)
}
