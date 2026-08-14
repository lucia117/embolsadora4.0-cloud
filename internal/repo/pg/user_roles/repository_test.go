package user_roles_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tu-org/embolsadora-api/internal/domain"
	"github.com/tu-org/embolsadora-api/internal/repo/pg/user_roles"
)

// mrgPlatformTenantID is the fixed MRG platform tenant seeded by migration
// 000002 (migrations/000002_seed_essentials.up.sql). Reused here instead of
// creating a throwaway tenant because it always exists, and the 'operario'
// role used below (tenant-scoped, is_global=false) is always assignable to
// it under the migration 000004 platform-role rule.
const mrgPlatformTenantID = "11b36b85-033d-4bb3-9e31-4c92161887c0"

// TestFindByTenant_ResolvesUserAndRoleAcrossJoin verifies the fix for the
// root cause in the design spec: a user auto-provisioned with
// users.tenant_id = NULL (see internal/api/usecases/auth_usecase.go's
// ProvisionUser) must still resolve correctly through the real
// user_tenant_roles relation, and role_id must resolve to a role name.
func TestFindByTenant_ResolvesUserAndRoleAcrossJoin(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set; skipping integration test")
	}

	db, err := pgxpool.New(context.Background(), dbURL)
	require.NoError(t, err)
	// t.Cleanup, NO defer: un defer cierra el pool ANTES de que corran los
	// t.Cleanup que borran las filas sembradas, y la limpieza falla en silencio
	// dejando basura en la DB compartida.
	t.Cleanup(db.Close)

	ctx := context.Background()
	repo := user_roles.NewUserRoleRepository(db)
	tenantID := uuid.MustParse(mrgPlatformTenantID)

	autoProvisionedUserID := uuid.New()
	_, err = db.Exec(ctx,
		`INSERT INTO users (id, email, tenant_id, first_name, last_name) VALUES ($1, $2, NULL, $3, $4)`,
		autoProvisionedUserID, "join-fix-test@example.com", "Join", "Fixture",
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		db.Exec(ctx, `DELETE FROM users WHERE id = $1`, autoProvisionedUserID)
	})

	activeUTRID := uuid.New()
	const roleID = "operario"
	_, err = db.Exec(ctx,
		`INSERT INTO user_tenant_roles (id, user_id, tenant_id, role_id, status) VALUES ($1, $2, $3, $4, 'active')`,
		activeUTRID, autoProvisionedUserID, tenantID, roleID,
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		db.Exec(ctx, `DELETE FROM user_tenant_roles WHERE id = $1`, activeUTRID)
	})

	pendingUTRID := uuid.New()
	_, err = db.Exec(ctx,
		`INSERT INTO user_tenant_roles (id, user_id, tenant_id, role_id, status) VALUES ($1, $2, $3, NULL, 'pending')`,
		pendingUTRID, autoProvisionedUserID, tenantID,
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		db.Exec(ctx, `DELETE FROM user_tenant_roles WHERE id = $1`, pendingUTRID)
	})

	var expectedRoleName string
	err = db.QueryRow(ctx, `SELECT name FROM roles WHERE id = $1`, roleID).Scan(&expectedRoleName)
	require.NoError(t, err)

	results, err := repo.FindByTenant(ctx, tenantID, nil, false)
	require.NoError(t, err)

	var active, pending *domain.UserTenantRoleDetail
	for i := range results {
		switch results[i].ID {
		case activeUTRID:
			active = &results[i]
		case pendingUTRID:
			pending = &results[i]
		}
	}

	require.NotNil(t, active, "active assignment should be present in FindByTenant results")
	assert.Equal(t, expectedRoleName, active.RoleName)
	assert.Equal(t, "join-fix-test@example.com", active.UserEmail)
	require.NotNil(t, active.UserFirstName)
	assert.Equal(t, "Join", *active.UserFirstName)
	require.NotNil(t, active.UserLastName)
	assert.Equal(t, "Fixture", *active.UserLastName)

	require.NotNil(t, pending, "pending assignment should be present in FindByTenant results")
	assert.Equal(t, "", pending.RoleName)
	assert.Equal(t, "join-fix-test@example.com", pending.UserEmail)
}

// TestCreateRechazaAdminEnTenantNoPlataforma cierra el gap de cobertura directa
// que la revisión final señaló: hasta acá la regla de la migración 000010 (admin/
// operario son platform-only aunque is_global=FALSE) solo tenía evidencia
// indirecta, vía fixtures que dejaron de poder usar "admin" fuera de MRG
// (ver me_usecase_test.go). Este test ejercita el mismo camino que
// TestCreateRechazaRolGlobalParaCallerSinVisibilidad (cloaking_test.go) pero para
// un rol is_global=FALSE, contra un tenant no-plataforma recién creado — usa
// seedTenant/seedMembership/poolOrSkip de cloaking_test.go, mismo paquete.
func TestCreateRechazaAdminEnTenantNoPlataforma(t *testing.T) {
	pool := poolOrSkip(t)
	repo := user_roles.NewUserRoleRepository(pool)
	ctx := context.Background()

	tenantID := seedTenant(t, pool)
	// status "revoked" y roleID "" solo para tener un user_id válido sembrado sin
	// que choque con idx_utr_active_unique cuando Create intente insertar la
	// asignación activa de abajo.
	s := seedMembership(t, pool, tenantID, "", "revoked")

	rol := "admin"
	now := time.Now()
	utr := &domain.UserTenantRole{
		ID: uuid.New(), UserID: s.UserID, TenantID: tenantID,
		RoleID: &rol, Status: domain.UserRoleStatusActive, AssignedAt: &now,
	}

	created, err := repo.Create(ctx, utr, false)
	require.Nil(t, created)
	require.ErrorIs(t, err, domain.ErrRoleNotAllowedForTenant,
		"admin es platform-only desde la migración 000010; checkRoleAllowedForTenant debe rechazarlo fuera de MRG")
}

// TestTriggerRechazaInsertRawDeAdminEnTenantNoPlataforma prueba el backstop de la
// DB en sí (trg_enforce_platform_role_tenant / enforce_platform_role_tenant()),
// sin pasar por checkRoleAllowedForTenant: un INSERT crudo contra
// user_tenant_roles asignando "admin" a un tenant no-plataforma tiene que fallar
// con check_violation (23514), igual que documenta errCodeCheckViolation en
// repository.go.
func TestTriggerRechazaInsertRawDeAdminEnTenantNoPlataforma(t *testing.T) {
	pool := poolOrSkip(t)
	ctx := context.Background()

	tenantID := seedTenant(t, pool)
	s := seedMembership(t, pool, tenantID, "", "revoked")

	_, err := pool.Exec(ctx,
		`INSERT INTO user_tenant_roles (id, user_id, tenant_id, role_id, status, assigned_at)
		 VALUES ($1, $2, $3, 'admin', 'active', NOW())`,
		uuid.New(), s.UserID, tenantID,
	)
	require.Error(t, err)

	var pgErr *pgconn.PgError
	require.True(t, errors.As(err, &pgErr), "el rechazo del trigger debe llegar como *pgconn.PgError")
	require.Equal(t, "23514", pgErr.Code,
		"trg_enforce_platform_role_tenant debe rechazar con check_violation (23514)")
}
