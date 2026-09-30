package update_user_role

// Issue #107: un usuario no puede cambiar el rol de su propia asignación. Era
// el paso que permitía a un cliente_admin pasarse a un rol custom con
// permisos que no tenía.

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tu-org/embolsadora-api/internal/domain"
	"github.com/tu-org/embolsadora-api/internal/platform"
)

func TestExecute_NoPermiteCambiarElRolPropio(t *testing.T) {
	tenantID := uuid.New()
	callerID := uuid.New()
	rol := "cliente_admin"
	repo := &fakeRepo{
		findByIDResult: &domain.UserTenantRole{ID: uuid.New(), UserID: callerID, TenantID: tenantID, RoleID: &rol},
	}
	roleRepo := &fakeRolesRepo{}
	uc := NewUseCase(repo, roleRepo)

	ctx := platform.WithUserID(context.Background(), callerID)
	_, err := uc.Execute(ctx, uuid.New(), tenantID, "custom_abc123", false)

	require.ErrorIs(t, err, domain.ErrCannotChangeOwnRole)
	assert.Empty(t, repo.updateCalls, "no se toca la asignación")
	assert.Empty(t, roleRepo.getByIDForTenantCalls, "se rechaza antes de validar el rol nuevo")
}

func TestExecute_PermiteCambiarElRolDeOtroUsuario(t *testing.T) {
	tenantID := uuid.New()
	id := uuid.New()
	rol := "operario"
	want := &domain.UserTenantRole{ID: id, TenantID: tenantID, RoleID: strPtr("cliente_admin")}
	repo := &fakeRepo{
		findByIDResult: &domain.UserTenantRole{ID: id, UserID: uuid.New(), TenantID: tenantID, RoleID: &rol},
		updateResult:   want,
	}
	uc := NewUseCase(repo, &fakeRolesRepo{})

	ctx := platform.WithUserID(context.Background(), uuid.New())
	got, err := uc.Execute(ctx, id, tenantID, "cliente_admin", false)

	require.NoError(t, err)
	assert.Equal(t, want, got)
}
