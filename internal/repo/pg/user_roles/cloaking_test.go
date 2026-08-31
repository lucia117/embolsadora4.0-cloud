package user_roles_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/tu-org/embolsadora-api/internal/domain"
	"github.com/tu-org/embolsadora-api/internal/repo/pg/user_roles"
)

// Tests de regresión del Crítico 2 y del Menor de la revisión final: el dominio
// user_tenant_roles quedó entero fuera del cloaking que las Tasks 4-6 aplicaron a
// roles, users e invitations, así que la fila del super_admin seguía saliendo por
// GET /user-roles con email, nombre y el id con el que revocarla.
//
// Todo corre contra Postgres real: el cloaking vive en el WHERE de las consultas,
// y un fake no probaría nada.

var platformTenantUUID = uuid.MustParse("11b36b85-033d-4bb3-9e31-4c92161887c0")

func poolOrSkip(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	require.NoError(t, err)
	// t.Cleanup y no defer: un defer cierra el pool antes de que corran los
	// cleanups que borran las filas sembradas.
	t.Cleanup(pool.Close)
	return pool
}

type seededUTR struct {
	UserID uuid.UUID
	UTRID  uuid.UUID
	Email  string
}

// seedMembership siembra un usuario y su membresía en un tenant. roleID puede ser
// "" para una asignación pending sin rol.
func seedMembership(t *testing.T, pool *pgxpool.Pool, tenantID uuid.UUID, roleID, status string) seededUTR {
	t.Helper()
	ctx := context.Background()
	s := seededUTR{UserID: uuid.New(), UTRID: uuid.New()}
	s.Email = s.UserID.String() + "@utr-cloak.local"

	_, err := pool.Exec(ctx,
		`INSERT INTO users (id, email, name, first_name, last_name, status)
		 VALUES ($1, $2, 'UTR Cloak', 'UTR', 'Cloak', 'active')`,
		s.UserID, s.Email)
	require.NoError(t, err)

	var role *string
	if roleID != "" {
		role = &roleID
	}
	_, err = pool.Exec(ctx,
		`INSERT INTO user_tenant_roles (id, user_id, tenant_id, role_id, status, assigned_at)
		 VALUES ($1, $2, $3, $4, $5, NOW())`,
		s.UTRID, s.UserID, tenantID, role, status)
	require.NoError(t, err)

	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM user_tenant_roles WHERE user_id = $1`, s.UserID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, s.UserID)
	})
	return s
}

func utrIDs(details []domain.UserTenantRoleDetail) []uuid.UUID {
	ids := make([]uuid.UUID, 0, len(details))
	for _, d := range details {
		ids = append(ids, d.ID)
	}
	return ids
}

func statusOf(t *testing.T, pool *pgxpool.Pool, utrID uuid.UUID) string {
	t.Helper()
	var status string
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT status FROM user_tenant_roles WHERE id = $1`, utrID).Scan(&status))
	return status
}

// TestFindByTenantOcultaAsignacionesAGlobales reproduce el escenario literal del
// hallazgo: el admin de MRG abre Tenants → detalle de MRG, el frontend pega a
// GET /api/v1/user-roles?tenantId=<MRG> y la fila del super_admin salía con
// role_id, role_name, user_email y user_name.
func TestFindByTenantOcultaAsignacionesAGlobales(t *testing.T) {
	pool := poolOrSkip(t)
	repo := user_roles.NewUserRoleRepository(pool)
	ctx := context.Background()

	oculto := seedMembership(t, pool, platformTenantUUID, "super_admin", "active")
	visible := seedMembership(t, pool, platformTenantUUID, "operario", "active")

	sinCloak, err := repo.FindByTenant(ctx, platformTenantUUID, nil, false)
	require.NoError(t, err)
	ids := utrIDs(sinCloak)
	require.NotContains(t, ids, oculto.UTRID, "la membresía a un rol global no debe aparecer")
	require.Contains(t, ids, visible.UTRID, "las membresías normales siguen visibles")

	// tenant_manager es el otro rol is_global: el predicado no depende de un id
	// de rol hardcodeado.
	otroGlobal := seedMembership(t, pool, platformTenantUUID, "tenant_manager", "active")
	sinCloak, err = repo.FindByTenant(ctx, platformTenantUUID, nil, false)
	require.NoError(t, err)
	require.NotContains(t, utrIDs(sinCloak), otroGlobal.UTRID)

	conCloak, err := repo.FindByTenant(ctx, platformTenantUUID, nil, true)
	require.NoError(t, err)
	require.Contains(t, utrIDs(conCloak), oculto.UTRID, "el super_admin sí las ve")
}

// TestFindByTenantConStatusOcultaAsignacionesAGlobales cubre la segunda consulta,
// la que usa Usuarios → Pendientes (pending-users-list.tsx).
func TestFindByTenantConStatusOcultaAsignacionesAGlobales(t *testing.T) {
	pool := poolOrSkip(t)
	repo := user_roles.NewUserRoleRepository(pool)
	ctx := context.Background()

	oculto := seedMembership(t, pool, platformTenantUUID, "super_admin", "pending")
	visible := seedMembership(t, pool, platformTenantUUID, "operario", "pending")

	pending := "pending"
	res, err := repo.FindByTenant(ctx, platformTenantUUID, &pending, false)
	require.NoError(t, err)
	ids := utrIDs(res)
	require.NotContains(t, ids, oculto.UTRID)
	require.Contains(t, ids, visible.UTRID)

	res, err = repo.FindByTenant(ctx, platformTenantUUID, &pending, true)
	require.NoError(t, err)
	require.Contains(t, utrIDs(res), oculto.UTRID)
}

// TestFindByTenantNoOcultaPendingSinRol: role_id es NULL en las asignaciones
// pending creadas por el flujo de invitación, y el LEFT JOIN no devuelve fila de
// roles. El COALESCE del predicado tiene que dejarlas pasar; si no, el cloaking
// se comería usuarios pendientes legítimos.
func TestFindByTenantNoOcultaPendingSinRol(t *testing.T) {
	pool := poolOrSkip(t)
	repo := user_roles.NewUserRoleRepository(pool)
	ctx := context.Background()

	sinRol := seedMembership(t, pool, platformTenantUUID, "", "pending")

	res, err := repo.FindByTenant(ctx, platformTenantUUID, nil, false)
	require.NoError(t, err)
	require.Contains(t, utrIDs(res), sinRol.UTRID, "una asignación pending sin rol no es una interna de plataforma")
}

// TestFindByIDOcultaAsignacionAGlobal es lo que hace que DELETE y PUT
// /user-roles/:id devuelvan 404 sobre la membresía del superadmin: los dos
// usecases resuelven con FindByID antes de mutar.
func TestFindByIDOcultaAsignacionAGlobal(t *testing.T) {
	pool := poolOrSkip(t)
	repo := user_roles.NewUserRoleRepository(pool)
	ctx := context.Background()

	oculto := seedMembership(t, pool, platformTenantUUID, "super_admin", "active")

	got, err := repo.FindByID(ctx, oculto.UTRID, false)
	require.NoError(t, err)
	require.Nil(t, got, "misma respuesta que un id inexistente → ErrAssignmentNotFound → 404")

	inexistente, err := repo.FindByID(ctx, uuid.New(), false)
	require.NoError(t, err)
	require.Nil(t, inexistente)

	got, err = repo.FindByID(ctx, oculto.UTRID, true)
	require.NoError(t, err)
	require.NotNil(t, got)
}

// TestRevokeNoMutaAsignacionOculta: el precheck del usecase ya devuelve 404, pero
// la mutación tiene que negarse sola. Es la lección de DeleteUser (Task 5): un
// efecto observable sobre algo invisible delata su existencia igual que un 403.
func TestRevokeNoMutaAsignacionOculta(t *testing.T) {
	pool := poolOrSkip(t)
	repo := user_roles.NewUserRoleRepository(pool)
	ctx := context.Background()

	oculto := seedMembership(t, pool, platformTenantUUID, "super_admin", "active")

	res, err := repo.Revoke(ctx, oculto.UTRID, platformTenantUUID, false)
	require.NoError(t, err)
	require.Nil(t, res)
	require.Equal(t, "active", statusOf(t, pool, oculto.UTRID),
		"la membresía del superadmin no puede quedar revocada por un caller que no la ve")

	res, err = repo.Revoke(ctx, oculto.UTRID, platformTenantUUID, true)
	require.NoError(t, err)
	require.NotNil(t, res)
	require.Equal(t, "revoked", statusOf(t, pool, oculto.UTRID))
}

// TestUpdateNoMutaAsignacionOculta: mismo criterio para PUT /user-roles/:id.
func TestUpdateNoMutaAsignacionOculta(t *testing.T) {
	pool := poolOrSkip(t)
	repo := user_roles.NewUserRoleRepository(pool)
	ctx := context.Background()

	oculto := seedMembership(t, pool, platformTenantUUID, "super_admin", "active")
	nuevoRol := "operario"

	utr := &domain.UserTenantRole{ID: oculto.UTRID, TenantID: platformTenantUUID, RoleID: &nuevoRol}
	res, err := repo.Update(ctx, utr, false)
	require.NoError(t, err)
	require.Nil(t, res)

	var roleID string
	require.NoError(t, pool.QueryRow(ctx, `SELECT role_id FROM user_tenant_roles WHERE id = $1`, oculto.UTRID).Scan(&roleID))
	require.Equal(t, "super_admin", roleID, "el rol de una asignación oculta no puede cambiarse")
}

// TestUpdateStatusNoMutaAsignacionOculta cubre la vía PATCH /users/:id/status.
func TestUpdateStatusNoMutaAsignacionOculta(t *testing.T) {
	pool := poolOrSkip(t)
	repo := user_roles.NewUserRoleRepository(pool)
	ctx := context.Background()

	oculto := seedMembership(t, pool, platformTenantUUID, "super_admin", "active")

	_, err := repo.UpdateStatus(ctx, oculto.UserID, platformTenantUUID, domain.UserRoleStatusSuspended, false)
	require.ErrorIs(t, err, domain.ErrNoActiveAssignment,
		"misma respuesta que un usuario sin membresía activa")
	require.Equal(t, "active", statusOf(t, pool, oculto.UTRID))

	_, err = repo.UpdateStatus(ctx, oculto.UserID, platformTenantUUID, domain.UserRoleStatusSuspended, true)
	require.NoError(t, err)
	require.Equal(t, "suspended", statusOf(t, pool, oculto.UTRID))
}

// TestFindByUserAcotaPorTenantYCloakea es el Crítico 3 a nivel de datos:
// GET /users/:id/roles devolvía TODAS las membresías de cualquier usuario, sin
// scope de tenant ni cloaking, y sin RBAC en la ruta.
func TestFindByUserAcotaPorTenantYCloakea(t *testing.T) {
	pool := poolOrSkip(t)
	repo := user_roles.NewUserRoleRepository(pool)
	ctx := context.Background()

	otroTenant := seedTenant(t, pool)

	// Un mismo usuario con dos membresías: super_admin en plataforma (oculta) y
	// cliente_operario en otro tenant (visible solo para callers cross-tenant).
	// cliente_operario (no operario: desde la migración 000010 operario es
	// platform-only y no se puede asignar fuera de MRG) es la variante
	// tenant-scoped, irrelevante para lo que este test verifica.
	s := seedMembership(t, pool, platformTenantUUID, "super_admin", "active")
	otraUTR := uuid.New()
	_, err := pool.Exec(ctx,
		`INSERT INTO user_tenant_roles (id, user_id, tenant_id, role_id, status, assigned_at)
		 VALUES ($1, $2, $3, 'cliente_operario', 'active', NOW())`,
		otraUTR, s.UserID, otroTenant)
	require.NoError(t, err)

	// Caller sin cross-tenant parado en el tenant plataforma: no ve nada, porque
	// lo único del usuario en ese tenant es la membresía global.
	res, err := repo.FindByUser(ctx, s.UserID, platformTenantUUID, false, false)
	require.NoError(t, err)
	require.Empty(t, res, "un usuario totalmente cloakeado responde igual que uno inexistente")

	// Caller cross-tenant sin ver internas de plataforma (platform_admin): ve la
	// membresía del otro tenant, no la global.
	res, err = repo.FindByUser(ctx, s.UserID, platformTenantUUID, true, false)
	require.NoError(t, err)
	require.Len(t, res, 1)
	require.Equal(t, "cliente_operario", res[0].RoleID)

	// super_admin: ve las dos.
	res, err = repo.FindByUser(ctx, s.UserID, platformTenantUUID, true, true)
	require.NoError(t, err)
	require.Len(t, res, 2)

	// Un usuario inexistente devuelve lo mismo que el cloakeado: lista vacía.
	res, err = repo.FindByUser(ctx, uuid.New(), platformTenantUUID, false, false)
	require.NoError(t, err)
	require.Empty(t, res)
}

// seedTenant crea un tenant descartable (no plataforma).
func seedTenant(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	id := uuid.New()
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM tenants WHERE id = $1`, id)
	})
	_, err := pool.Exec(context.Background(), `
		INSERT INTO tenants (id, name, company_name, subdomain)
		VALUES ($1, 'Tenant UTR test', 'Tenant UTR test', $2)
	`, id, "utr-"+id.String())
	require.NoError(t, err)
	return id
}

// ── Crítico 1, capa repo ────────────────────────────────────────────────────
//
// checkRoleAllowedForTenant preguntaba solo tenant_can_use_role(), que dentro del
// tenant plataforma devuelve TRUE para is_global: o sea que el backstop del repo
// aceptaba crear una UTR a super_admin. El predicado del caller lo cierra.

func TestCreateRechazaRolGlobalParaCallerSinVisibilidad(t *testing.T) {
	pool := poolOrSkip(t)
	repo := user_roles.NewUserRoleRepository(pool)
	ctx := context.Background()

	s := seedMembership(t, pool, platformTenantUUID, "operario", "revoked")
	rol := "super_admin"
	now := time.Now()
	utr := &domain.UserTenantRole{
		ID: uuid.New(), UserID: s.UserID, TenantID: platformTenantUUID,
		RoleID: &rol, Status: domain.UserRoleStatusActive, AssignedAt: &now,
	}

	created, err := repo.Create(ctx, utr, false)
	require.Nil(t, created)
	require.ErrorIs(t, err, domain.ErrInvalidRoleID,
		"un rol global invisible tiene que responder igual que un rol inexistente")

	var count int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM user_tenant_roles WHERE user_id = $1 AND role_id = 'super_admin'`,
		s.UserID).Scan(&count))
	require.Zero(t, count, "no debe haber quedado ninguna UTR de super_admin")

	// Control: el rol inexistente da exactamente el mismo error.
	inexistente := "rol_que_no_existe"
	utr.RoleID = &inexistente
	_, err = repo.Create(ctx, utr, false)
	require.ErrorIs(t, err, domain.ErrInvalidRoleID)
}

func TestCreatePermiteRolGlobalAlSuperadmin(t *testing.T) {
	pool := poolOrSkip(t)
	repo := user_roles.NewUserRoleRepository(pool)
	ctx := context.Background()

	s := seedMembership(t, pool, platformTenantUUID, "operario", "revoked")
	rol := "super_admin"
	now := time.Now()
	utr := &domain.UserTenantRole{
		ID: uuid.New(), UserID: s.UserID, TenantID: platformTenantUUID,
		RoleID: &rol, Status: domain.UserRoleStatusActive, AssignedAt: &now,
	}

	created, err := repo.Create(ctx, utr, true)
	require.NoError(t, err, "el superadmin sí puede asignar roles globales")
	require.NotNil(t, created)
}

// TestCheckRoleRechazaRolCustomDeOtroTenant cierra la mitad del minor diferido en
// Task 4: antes de la migración 000010, tenant_can_use_role devolvía TRUE
// incondicionalmente para is_global=false, así que sin el predicado de tenant un
// admin podía asignar el rol custom de otro tenant conociendo su id.
func TestCheckRoleRechazaRolCustomDeOtroTenant(t *testing.T) {
	pool := poolOrSkip(t)
	repo := user_roles.NewUserRoleRepository(pool)
	ctx := context.Background()

	tenantA := seedTenant(t, pool)
	rolA := "custom_" + uuid.New().String()[:8]
	_, err := pool.Exec(ctx,
		`INSERT INTO roles (id, name, description, is_system_role, is_global, tenant_id, permissions)
		 VALUES ($1, 'Custom A', '', FALSE, FALSE, $2, '[]'::jsonb)`, rolA, tenantA)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM roles WHERE id = $1`, rolA)
	})

	s := seedMembership(t, pool, platformTenantUUID, "operario", "revoked")
	now := time.Now()
	utr := &domain.UserTenantRole{
		ID: uuid.New(), UserID: s.UserID, TenantID: platformTenantUUID,
		RoleID: &rolA, Status: domain.UserRoleStatusActive, AssignedAt: &now,
	}

	_, err = repo.Create(ctx, utr, false)
	require.ErrorIs(t, err, domain.ErrInvalidRoleID,
		"un rol custom de otro tenant no existe para este tenant")
}

// ── El Menor: el oráculo por constraint ─────────────────────────────────────

// TestCreateSobreUsuarioOcultoRespondeComoUsuarioInexistente es la regresión del
// hallazgo Menor. idx_utr_active_unique no sabe de cloaking: ve la membresía activa
// del superadmin igual que cualquier otra. Sin el fix, POST /user-roles distinguía
// 409 (usuario oculto con membresía activa) de 400 (user_id inexistente) — un
// oráculo que sobrevivía intacto al cloaking de las listas.
func TestCreateSobreUsuarioOcultoRespondeComoUsuarioInexistente(t *testing.T) {
	pool := poolOrSkip(t)
	repo := user_roles.NewUserRoleRepository(pool)
	ctx := context.Background()

	oculto := seedMembership(t, pool, platformTenantUUID, "super_admin", "active")

	rol := "operario"
	now := time.Now()
	nuevo := func(userID uuid.UUID) *domain.UserTenantRole {
		return &domain.UserTenantRole{
			ID: uuid.New(), UserID: userID, TenantID: platformTenantUUID,
			RoleID: &rol, Status: domain.UserRoleStatusActive, AssignedAt: &now,
		}
	}

	_, errOculto := repo.Create(ctx, nuevo(oculto.UserID), false)
	_, errInexistente := repo.Create(ctx, nuevo(uuid.New()), false)

	require.ErrorIs(t, errOculto, domain.ErrInvalidUserID)
	require.ErrorIs(t, errInexistente, domain.ErrInvalidUserID)
	require.Equal(t, errInexistente.Error(), errOculto.Error(),
		"mismo error y mismo mensaje: el cuerpo de la respuesta tiene que ser byte-idéntico")
}

// TestCreateSobreDuplicadoVisibleSigueDando409 es el control positivo: el caso
// legítimo no se degrada. Un duplicado que el caller SÍ ve tiene que seguir
// devolviendo ErrUserAlreadyHasActiveRole (409), que es accionable.
func TestCreateSobreDuplicadoVisibleSigueDando409(t *testing.T) {
	pool := poolOrSkip(t)
	repo := user_roles.NewUserRoleRepository(pool)
	ctx := context.Background()

	visible := seedMembership(t, pool, platformTenantUUID, "operario", "active")

	rol := "admin"
	now := time.Now()
	utr := &domain.UserTenantRole{
		ID: uuid.New(), UserID: visible.UserID, TenantID: platformTenantUUID,
		RoleID: &rol, Status: domain.UserRoleStatusActive, AssignedAt: &now,
	}

	_, err := repo.Create(ctx, utr, false)
	require.ErrorIs(t, err, domain.ErrUserAlreadyHasActiveRole)
}

// ── El residuo: cloaking por fila vs. cloaking por identidad ────────────────
//
// Todo lo de arriba cloakea POR FILA: el predicado mira el rol de la fila que se
// lee o se muta. Pero POST /user-roles ESCRIBE la fila, y el rol lo elige el
// atacante. Un admin de un tenant cliente que consiga el uuid del super_admin
// hacía POST {"userId": <super_admin>, "roleId": "operario"} contra su propio
// tenant: no hay conflicto con idx_utr_active_unique (el superadmin no tiene
// membresía ahí), el rol pedido no es global, y el INSERT completaba. La fila
// resultante es visible para él —su rol no es global— con user_email y user_name
// reales. Des-anonimización completa y persistente, escrita por la víctima.
//
// La regla que cierra esto es por IDENTIDAD y no por fila: un caller que no ve a
// un usuario tampoco puede escribirle una membresía. "No lo ve" acá significa que
// el destino es una identidad de plataforma — tiene una membresía no revocada a un
// rol is_global en ALGÚN tenant.

// seedPlatformIdentity siembra una cuenta interna de plataforma: usuario + membresía
// a un rol global en el tenant plataforma, con el status pedido.
func seedPlatformIdentity(t *testing.T, pool *pgxpool.Pool, status string) seededUTR {
	t.Helper()
	return seedMembership(t, pool, platformTenantUUID, "super_admin", status)
}

func nuevaUTR(userID, tenantID uuid.UUID, roleID string) *domain.UserTenantRole {
	now := time.Now()
	return &domain.UserTenantRole{
		ID: uuid.New(), UserID: userID, TenantID: tenantID,
		RoleID: &roleID, Status: domain.UserRoleStatusActive, AssignedAt: &now,
	}
}

func countUTRs(t *testing.T, pool *pgxpool.Pool, userID, tenantID uuid.UUID) int {
	t.Helper()
	var n int
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM user_tenant_roles WHERE user_id = $1 AND tenant_id = $2`,
		userID, tenantID).Scan(&n))
	return n
}

// TestCreateNoEscribeMembresiaAUsuarioDePlataforma es el escenario del hallazgo,
// tal cual: el admin de Córdoba le asigna 'operario' al super_admin en Córdoba.
func TestCreateNoEscribeMembresiaAUsuarioDePlataforma(t *testing.T) {
	pool := poolOrSkip(t)
	repo := user_roles.NewUserRoleRepository(pool)
	ctx := context.Background()

	cordoba := seedTenant(t, pool)
	superadmin := seedPlatformIdentity(t, pool, "active")

	// cliente_operario, no operario: desde la migración 000010 operario es
	// platform-only y checkRoleAllowedForTenant lo rechazaría antes de llegar
	// al guard de identidad que este test ejercita.
	created, err := repo.Create(ctx, nuevaUTR(superadmin.UserID, cordoba, "cliente_operario"), false)
	require.Nil(t, created)
	require.ErrorIs(t, err, domain.ErrInvalidUserID,
		"el destino es invisible para este caller: misma respuesta que un userId inexistente")
	require.Zero(t, countUTRs(t, pool, superadmin.UserID, cordoba),
		"no puede quedar ninguna UTR escrita sobre la cuenta de plataforma")
}

// TestCreateSobreUsuarioDePlataformaConvergeConInexistente: la respuesta tiene que
// ser byte-idéntica a la del usuario que no existe. Si no, sigue siendo un oráculo.
func TestCreateSobreUsuarioDePlataformaConvergeConInexistente(t *testing.T) {
	pool := poolOrSkip(t)
	repo := user_roles.NewUserRoleRepository(pool)
	ctx := context.Background()

	cordoba := seedTenant(t, pool)
	superadmin := seedPlatformIdentity(t, pool, "active")

	_, errOculto := repo.Create(ctx, nuevaUTR(superadmin.UserID, cordoba, "cliente_operario"), false)
	_, errInexistente := repo.Create(ctx, nuevaUTR(uuid.New(), cordoba, "cliente_operario"), false)

	require.ErrorIs(t, errOculto, domain.ErrInvalidUserID)
	require.ErrorIs(t, errInexistente, domain.ErrInvalidUserID)
	require.Equal(t, errInexistente.Error(), errOculto.Error())
}

// TestCreateCloakeaTambienAlPendiente: un super_admin invitado y todavía sin
// activar está cloakeado en ListPendingUsers y en las invitaciones (Tasks 5 y 6);
// tiene que estarlo también acá, o el vector se reabre contra el mismo usuario un
// minuto antes de que se loguee.
func TestCreateCloakeaTambienAlPendiente(t *testing.T) {
	pool := poolOrSkip(t)
	repo := user_roles.NewUserRoleRepository(pool)
	ctx := context.Background()

	cordoba := seedTenant(t, pool)
	pendiente := seedPlatformIdentity(t, pool, "pending")

	_, err := repo.Create(ctx, nuevaUTR(pendiente.UserID, cordoba, "cliente_operario"), false)
	require.ErrorIs(t, err, domain.ErrInvalidUserID)
}

// TestCreatePermiteUsuarioConGlobalRevocado: el corte es 'no revocada'. Un ex
// super_admin cuya membresía global ya fue revocada no es una identidad de
// plataforma, y degradarlo a operario es un flujo legítimo que no puede quedar
// bloqueado para siempre.
func TestCreatePermiteUsuarioConGlobalRevocado(t *testing.T) {
	pool := poolOrSkip(t)
	repo := user_roles.NewUserRoleRepository(pool)
	ctx := context.Background()

	cordoba := seedTenant(t, pool)
	ex := seedPlatformIdentity(t, pool, "revoked")

	created, err := repo.Create(ctx, nuevaUTR(ex.UserID, cordoba, "cliente_operario"), false)
	require.NoError(t, err)
	require.NotNil(t, created)
}

// TestCreateSobreUsuarioDePlataformaComoSuperadminFunciona: control positivo. El
// superadmin legítimo no pierde nada.
func TestCreateSobreUsuarioDePlataformaComoSuperadminFunciona(t *testing.T) {
	pool := poolOrSkip(t)
	repo := user_roles.NewUserRoleRepository(pool)
	ctx := context.Background()

	cordoba := seedTenant(t, pool)
	superadmin := seedPlatformIdentity(t, pool, "active")

	created, err := repo.Create(ctx, nuevaUTR(superadmin.UserID, cordoba, "cliente_operario"), true)
	require.NoError(t, err, "includeGlobal=true sigue pudiendo asignar sobre cualquiera")
	require.NotNil(t, created)
}

// TestCreateSobreUsuarioNormalSigueFuncionando: el caso principal de
// POST /user-roles —asignar un rol a alguien sin membresía previa en ese tenant—
// no se toca.
func TestCreateSobreUsuarioNormalSigueFuncionando(t *testing.T) {
	pool := poolOrSkip(t)
	repo := user_roles.NewUserRoleRepository(pool)
	ctx := context.Background()

	cordoba := seedTenant(t, pool)
	normal := seedMembership(t, pool, platformTenantUUID, "operario", "active")

	created, err := repo.Create(ctx, nuevaUTR(normal.UserID, cordoba, "cliente_operario"), false)
	require.NoError(t, err)
	require.NotNil(t, created)
}

// TestBulkCreateNoEscribeMembresiaAUsuarioDePlataforma: el lote comparte el mismo
// INSERT, y es all-or-nothing, así que el usuario limpio del batch tampoco entra.
func TestBulkCreateNoEscribeMembresiaAUsuarioDePlataforma(t *testing.T) {
	pool := poolOrSkip(t)
	repo := user_roles.NewUserRoleRepository(pool)
	ctx := context.Background()

	cordoba := seedTenant(t, pool)
	superadmin := seedPlatformIdentity(t, pool, "active")
	normal := seedMembership(t, pool, platformTenantUUID, "operario", "active")

	lote := []domain.UserTenantRole{
		*nuevaUTR(normal.UserID, cordoba, "cliente_operario"),
		*nuevaUTR(superadmin.UserID, cordoba, "cliente_operario"),
	}

	res, err := repo.BulkCreate(ctx, lote, false)
	require.Nil(t, res)
	require.ErrorIs(t, err, domain.ErrInvalidUserID)
	require.Zero(t, countUTRs(t, pool, superadmin.UserID, cordoba))
	require.Zero(t, countUTRs(t, pool, normal.UserID, cordoba), "la transacción es all-or-nothing")

	// Control positivo: el mismo lote con includeGlobal=true entra completo.
	res, err = repo.BulkCreate(ctx, lote, true)
	require.NoError(t, err)
	require.Len(t, res, 2)
}

// TestUpdateNoMutaMembresiaDeUsuarioDePlataforma cubre PUT /user-roles/:id. La
// fila objetivo tiene un rol NO global (así que el cloaking por fila la deja
// pasar) pero su dueño es una cuenta de plataforma.
func TestUpdateNoMutaMembresiaDeUsuarioDePlataforma(t *testing.T) {
	pool := poolOrSkip(t)
	repo := user_roles.NewUserRoleRepository(pool)
	ctx := context.Background()

	cordoba := seedTenant(t, pool)
	superadmin := seedPlatformIdentity(t, pool, "active")

	// La membresía visible la escribe el superadmin legítimo (includeGlobal=true).
	// cliente_operario/cliente_admin, no operario/admin: desde la migración 000010
	// esos dos son platform-only y no se pueden asignar en cordoba (tenant cliente),
	// ni siquiera vía el bypass de identidad del superadmin que este test ejercita.
	visible, err := repo.Create(ctx, nuevaUTR(superadmin.UserID, cordoba, "cliente_operario"), true)
	require.NoError(t, err)

	nuevoRol := "cliente_admin"
	res, err := repo.Update(ctx, &domain.UserTenantRole{
		ID: visible.ID, UserID: superadmin.UserID, TenantID: cordoba, RoleID: &nuevoRol,
	}, false)
	require.NoError(t, err)
	require.Nil(t, res, "misma respuesta que un id inexistente → ErrAssignmentNotFound → 404")

	var roleID string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT role_id FROM user_tenant_roles WHERE id = $1`, visible.ID).Scan(&roleID))
	require.Equal(t, "cliente_operario", roleID, "no puede haber mutado")

	// Control positivo: el superadmin sí puede.
	res, err = repo.Update(ctx, &domain.UserTenantRole{
		ID: visible.ID, UserID: superadmin.UserID, TenantID: cordoba, RoleID: &nuevoRol,
	}, true)
	require.NoError(t, err)
	require.NotNil(t, res)
	require.Equal(t, "cliente_admin", *res.RoleID)
}

// TestUpdateSobreUsuarioNormalSigueFuncionando: control de que el PUT cotidiano
// no se rompió.
func TestUpdateSobreUsuarioNormalSigueFuncionando(t *testing.T) {
	pool := poolOrSkip(t)
	repo := user_roles.NewUserRoleRepository(pool)
	ctx := context.Background()

	normal := seedMembership(t, pool, platformTenantUUID, "operario", "active")
	nuevoRol := "admin"

	res, err := repo.Update(ctx, &domain.UserTenantRole{
		ID: normal.UTRID, UserID: normal.UserID, TenantID: platformTenantUUID, RoleID: &nuevoRol,
	}, false)
	require.NoError(t, err)
	require.NotNil(t, res)
	require.Equal(t, "admin", *res.RoleID)
}

// TestCreateNoRompeLaActivacionDeInvitacionDeSuperAdmin fija la interacción con
// ActivatePendingInvitations (invitation_usecase.go): un super_admin invitado tiene
// una UTR pending a un rol global, y al loguearse el flujo llama a Create con
// includeGlobal=true a propósito (es self-action, no un caller mirando ajeno). Sin
// ese true el guard de identidad se comería la activación — el usuario destino es,
// justamente, una identidad de plataforma.
func TestCreateNoRompeLaActivacionDeInvitacionDeSuperAdmin(t *testing.T) {
	pool := poolOrSkip(t)
	repo := user_roles.NewUserRoleRepository(pool)
	ctx := context.Background()

	invitado := seedPlatformIdentity(t, pool, "pending")

	activada, err := repo.Create(ctx, nuevaUTR(invitado.UserID, platformTenantUUID, "super_admin"), true)
	require.NoError(t, err, "la activación de la propia invitación no puede romperse")
	require.NotNil(t, activada)
	require.Equal(t, domain.UserRoleStatusActive, activada.Status)
}

// TestBulkCreateSobreUsuarioOcultoRespondeComoInexistente: la misma convergencia
// en el lote, donde el error va envuelto con el user id.
func TestBulkCreateSobreUsuarioOcultoRespondeComoInexistente(t *testing.T) {
	pool := poolOrSkip(t)
	repo := user_roles.NewUserRoleRepository(pool)
	ctx := context.Background()

	oculto := seedMembership(t, pool, platformTenantUUID, "super_admin", "active")

	rol := "operario"
	now := time.Now()
	lote := []domain.UserTenantRole{{
		ID: uuid.New(), UserID: oculto.UserID, TenantID: platformTenantUUID,
		RoleID: &rol, Status: domain.UserRoleStatusActive, AssignedAt: &now,
	}}

	_, err := repo.BulkCreate(ctx, lote, false)
	require.ErrorIs(t, err, domain.ErrInvalidUserID)
}
