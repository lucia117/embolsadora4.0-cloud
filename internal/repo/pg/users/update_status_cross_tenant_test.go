package users_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	appUsers "github.com/tu-org/embolsadora-api/internal/app/users"
	rolesRepo "github.com/tu-org/embolsadora-api/internal/repo/pg/roles"
	userRolesRepo "github.com/tu-org/embolsadora-api/internal/repo/pg/user_roles"
	usersRepo "github.com/tu-org/embolsadora-api/internal/repo/pg/users"
)

// TestUpdateUserStatusCrossTenant cubre la extensión de Hallazgo C acordada a
// UpdateUserStatus: mismo root cause, con una capa extra de bug — la
// mutación real (userRoleRepo.UpdateStatus) usaba el tenantID de la request
// en vez del tenant real del target, así que incluso resolviendo el precheck
// no alcanzaba (ver el fix en service.go para el detalle).
const dummyCallerID = "00000000-0000-0000-0000-000000000000"

func TestUpdateUserStatusCrossTenantFalseDaNotFoundParaUsuarioDeOtroTenant(t *testing.T) {
	pool := poolOrSkip(t)
	repo := usersRepo.NewPostgresRepository(pool)
	urRepo := userRolesRepo.NewUserRoleRepository(pool)
	svc := appUsers.NewService(repo, urRepo, rolesRepo.NewPostgresRepository(pool), zap.NewNop())
	ctx := context.Background()

	tenantA := seedTenant(t, pool)
	tenantB := seedTenant(t, pool)
	userInB := seedUserInTenant(t, pool, tenantB)

	_, err := svc.UpdateUserStatus(ctx, tenantA, userInB, dummyCallerID, "suspended", false, false)
	require.Error(t, err, "sin crossTenant, un usuario de otro tenant debe seguir devolviendo 404")
}

func TestUpdateUserStatusCrossTenantTrueActualizaUsuarioDeOtroTenant(t *testing.T) {
	pool := poolOrSkip(t)
	repo := usersRepo.NewPostgresRepository(pool)
	urRepo := userRolesRepo.NewUserRoleRepository(pool)
	svc := appUsers.NewService(repo, urRepo, rolesRepo.NewPostgresRepository(pool), zap.NewNop())
	ctx := context.Background()

	tenantA := seedTenant(t, pool)
	tenantB := seedTenant(t, pool)
	userInB := seedUserInTenant(t, pool, tenantB)

	updated, err := svc.UpdateUserStatus(ctx, tenantA, userInB, dummyCallerID, "suspended", true, false)
	require.NoError(t, err, "con crossTenant=true, un super_admin en tenantA debe poder suspender a un usuario de tenantB")
	require.Equal(t, tenantB, updated.TenantID)

	// Verificación directa en SQL: el bug de la capa extra (Hallazgo C
	// extendido) era que la mutación real (userRoleRepo.UpdateStatus) usaba
	// el tenantID de la request (tenantA) en vez del tenant real del target
	// (tenantB), así que el precheck resolvía bien pero el UPDATE pegaba en
	// la fila equivocada (o en ninguna). Sin esta lectura independiente,
	// updated.TenantID == tenantB solo prueba lo que el propio código de
	// retorno afirma, no que la columna se haya persistido de verdad.
	var status string
	err = pool.QueryRow(ctx,
		`SELECT status FROM user_tenant_roles WHERE user_id = $1 AND tenant_id = $2`,
		userInB, tenantB,
	).Scan(&status)
	require.NoError(t, err)
	require.Equal(t, "suspended", status, "el status debe haberse persistido en la membresía real de tenantB")
}
