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

// Verifica end-to-end (service+repo real contra DB) que DeleteUser ya no puede
// soft-borrar a un miembro con rol global cuando includeGlobal=false: debe dar
// ErrNotFound y el registro debe seguir sin deleted_at. Esto es la reproducción
// del side door hallado en la auditoría: antes de este fix, DeleteUser llamaba a
// repo.Delete directamente sin ningún precheck de visibilidad.
func TestDeleteUser_SideDoorFix_NoPuedeBorrarSuperadminOculto(t *testing.T) {
	pool := poolOrSkip(t)
	repo := usersRepo.NewPostgresRepository(pool)
	urRepo := userRolesRepo.NewUserRoleRepository(pool)
	svc := appUsers.NewService(repo, urRepo, rolesRepo.NewPostgresRepository(pool), zap.NewNop())
	ctx := context.Background()

	superID := seedMemberWithRole(t, pool, "super_admin", "active")

	// includeGlobal=false (caller no-superadmin): debe fallar con ErrNotFound, NO
	// debe borrar la fila.
	err := svc.DeleteUser(ctx, platformTenant, superID, false, false)
	require.Error(t, err)

	var deletedAt *string
	scanErr := pool.QueryRow(ctx, "SELECT deleted_at::text FROM users WHERE id = $1", superID).Scan(&deletedAt)
	require.NoError(t, scanErr)
	require.Nil(t, deletedAt, "el usuario oculto NO debe quedar soft-deleted por un caller no-superadmin")

	// includeGlobal=true (caller superadmin): debe poder borrarlo normalmente.
	err = svc.DeleteUser(ctx, platformTenant, superID, false, true)
	require.NoError(t, err)

	scanErr = pool.QueryRow(ctx, "SELECT deleted_at::text FROM users WHERE id = $1", superID).Scan(&deletedAt)
	require.NoError(t, scanErr)
	require.NotNil(t, deletedAt, "un superadmin caller sí debe poder borrarlo")
}
