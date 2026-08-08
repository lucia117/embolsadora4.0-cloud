package invitations_test

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/tu-org/embolsadora-api/internal/domain"
	invRepo "github.com/tu-org/embolsadora-api/internal/repo/pg/invitations"
)

const platformTenant = "11b36b85-033d-4bb3-9e31-4c92161887c0"

func poolOrSkip(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	// t.Cleanup, NO `defer pool.Close()`: un defer corre ANTES que los t.Cleanup
	// que registra seedInvitation, así que el pool quedaría cerrado cuando toca
	// borrar las filas y la limpieza fallaría en silencio, contaminando la DB
	// compartida. Registrado acá primero, el orden LIFO lo deja cerrando último.
	t.Cleanup(pool.Close)
	return pool
}

func seedInvitation(t *testing.T, pool *pgxpool.Pool, roleID string) (invID, email string) {
	t.Helper()
	ctx := context.Background()
	invID = uuid.New().String()
	inviterID := uuid.New().String()
	email = invID + "@inv.local"

	_, err := pool.Exec(ctx,
		`INSERT INTO users (id, email, name, status) VALUES ($1, $2, 'Inviter', 'active')`,
		inviterID, inviterID+"@inv.local")
	require.NoError(t, err)

	_, err = pool.Exec(ctx,
		`INSERT INTO user_invitations (id, tenant_id, email, role_id, status, invited_by)
		 VALUES ($1, $2, $3, $4, 'pending', $5)`,
		invID, platformTenant, email, roleID, inviterID)
	require.NoError(t, err)

	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM user_invitations WHERE id = $1`, invID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, inviterID)
	})
	return invID, email
}

func TestListByTenantOcultaInvitacionesARolesGlobales(t *testing.T) {
	pool := poolOrSkip(t)
	ctx := context.Background()
	repo := invRepo.NewInvitationRepository(pool)

	superInv, _ := seedInvitation(t, pool, "super_admin")
	adminInv, _ := seedInvitation(t, pool, "admin")

	list, err := repo.ListByTenant(ctx, platformTenant, nil, false)
	require.NoError(t, err)

	var ids []string
	for _, i := range list {
		ids = append(ids, i.ID)
	}
	require.NotContains(t, ids, superInv, "una invitación a super_admin delata el rol")
	require.Contains(t, ids, adminInv)

	all, err := repo.ListByTenant(ctx, platformTenant, nil, true)
	require.NoError(t, err)
	var allIDs []string
	for _, i := range all {
		allIDs = append(allIDs, i.ID)
	}
	require.Contains(t, allIDs, superInv, "el superadmin sí las ve")
}

// TestGetByIDDevuelveNotFoundParaInvitacionOculta cubre ResendInvitation y
// RevokeInvitation: ambas resuelven la invitación por id con GetByID antes de
// actuar (reenviar mail / cambiar status). Si GetByID no filtrara, un 200 con
// el invitation completo (RoleID incluido) delataría el rol en el cuerpo JSON,
// y un resend delataría el rol mandando el mail.
func TestGetByIDDevuelveNotFoundParaInvitacionOculta(t *testing.T) {
	pool := poolOrSkip(t)
	ctx := context.Background()
	repo := invRepo.NewInvitationRepository(pool)

	superInv, _ := seedInvitation(t, pool, "super_admin")

	_, err := repo.GetByID(ctx, superInv, platformTenant, false)
	require.Error(t, err, "404, no 403: un 403 confirmaría que la invitación existe")

	got, err := repo.GetByID(ctx, superInv, platformTenant, true)
	require.NoError(t, err)
	require.Equal(t, superInv, got.ID)
}

// TestGetPendingByEmailAndTenantOcultaRolGlobal cubre el chequeo de duplicados
// de CreateInvitation: si no ocultara la invitación pendiente a super_admin,
// un caller no-super_admin que reintentara invitar a ese mismo email con un
// rol distinto recibiría "ya pendiente" — confirmando la existencia de una
// invitación que ListByTenant ya le esconde. Debe poder crear una segunda
// invitación (no-global) para el mismo email sin chocar con la oculta.
func TestGetPendingByEmailAndTenantOcultaRolGlobal(t *testing.T) {
	pool := poolOrSkip(t)
	ctx := context.Background()
	repo := invRepo.NewInvitationRepository(pool)

	_, email := seedInvitation(t, pool, "super_admin")

	got, err := repo.GetPendingByEmailAndTenant(ctx, email, platformTenant, false)
	require.ErrorIs(t, err, domain.ErrNotFound, "la invitación pendiente a super_admin no debe confirmar duplicado a un caller no-superadmin")
	require.Nil(t, got)

	got, err = repo.GetPendingByEmailAndTenant(ctx, email, platformTenant, true)
	require.NoError(t, err)
	require.NotNil(t, got, "el superadmin sí ve el duplicado")
}
