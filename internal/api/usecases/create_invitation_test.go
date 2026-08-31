package usecases

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tu-org/embolsadora-api/internal/domain"
	"github.com/tu-org/embolsadora-api/internal/platform"
	"github.com/tu-org/embolsadora-api/internal/platform/supabase"
	"github.com/tu-org/embolsadora-api/internal/security"
)

// fakeInvRepoForCreate implementa invitations.InvitationRepository al completo
// (los métodos que CreateInvitation no usa devuelven cero) y registra si
// Create fue invocado, para poder afirmar que un intento de escalada nunca
// llega a persistir nada.
type fakeInvRepoForCreate struct {
	createCalled bool
	pendingErr   error
}

func (f *fakeInvRepoForCreate) Create(ctx context.Context, inv *domain.UserInvitation) (*domain.UserInvitation, error) {
	f.createCalled = true
	inv.ID = "new-inv-id"
	return inv, nil
}

func (f *fakeInvRepoForCreate) GetPendingByEmailAndTenant(ctx context.Context, email, tenantID string, includeGlobal bool) (*domain.UserInvitation, error) {
	if f.pendingErr != nil {
		return nil, f.pendingErr
	}
	return nil, domain.ErrNotFound
}

func (f *fakeInvRepoForCreate) ListPendingByEmail(ctx context.Context, email string) ([]domain.UserInvitation, error) {
	return nil, nil
}

func (f *fakeInvRepoForCreate) GetByID(ctx context.Context, id, tenantID string, includeGlobal bool) (*domain.UserInvitation, error) {
	return nil, nil
}

func (f *fakeInvRepoForCreate) ListByTenant(ctx context.Context, tenantID string, status *string, includeGlobal bool) ([]domain.UserInvitation, error) {
	return nil, nil
}

func (f *fakeInvRepoForCreate) UpdateStatus(ctx context.Context, id string, status domain.InvitationStatus) error {
	return nil
}

// fakeAdminClientForCreate registra si se mandó un mail de invitación, para
// verificar que un intento de escalada bloqueado no dispara ningún efecto
// observable (mismo patrón que ForcePasswordChange en Task 5).
type fakeAdminClientForCreate struct {
	inviteCalled bool
}

func (f *fakeAdminClientForCreate) InviteUserByEmail(ctx context.Context, p supabase.InviteParams) error {
	f.inviteCalled = true
	return nil
}

func (f *fakeAdminClientForCreate) SendPasswordResetEmail(ctx context.Context, userEmail, redirectTo string) error {
	return nil
}

func ctxForCreateInvitation(roleName string) context.Context {
	ctx := context.Background()
	ctx = platform.WithTenantID(ctx, testTenantID)
	ctx = platform.WithDomainUser(ctx, &domain.User{ID: uuid.New().String(), Name: "Caller"})
	ctx = security.WithRole(ctx, roleName)
	return ctx
}

// TestCreateInvitation_RoleIDGlobalOcultoEsRechazado cubre la escalada de
// privilegios que el brief pide auditar explícitamente: un platform_admin
// (includeGlobal=false) pidiendo role_id="super_admin" no puede terminar
// creando la invitación. roleRepo simula el cloaking ya implementado en
// GetByIDForTenant (Task 4): con includeGlobal=false, un rol is_global
// devuelve ErrRoleNotFound exactamente como un role_id inexistente.
func TestCreateInvitation_RoleIDGlobalOcultoEsRechazado(t *testing.T) {
	invRepo := &fakeInvRepoForCreate{}
	roleRepo := fakeRoleLookup{err: domain.ErrRoleNotFound}
	admin := &fakeAdminClientForCreate{}

	uc := NewInvitationUsecase(invRepo, nil, nil, fakeTenantLookup{}, roleRepo, admin, nil, "https://app.example.com", 100)

	ctx := ctxForCreateInvitation("platform_admin")
	_, err := uc.CreateInvitation(ctx, "target@example.com", "super_admin")

	require.ErrorIs(t, err, domain.ErrRoleNotFound)
	assert.False(t, invRepo.createCalled, "una escalada bloqueada no debe persistir la invitación")
	assert.False(t, admin.inviteCalled, "una escalada bloqueada no debe mandar mail")
}

// TestCreateInvitation_SuperAdminPuedeInvitarARolGlobal es el control
// positivo: el mismo role_id, con includeGlobal=true (super_admin en
// contexto), sí debe poder crear la invitación.
func TestCreateInvitation_SuperAdminPuedeInvitarARolGlobal(t *testing.T) {
	invRepo := &fakeInvRepoForCreate{}
	roleRepo := fakeRoleLookup{role: &domain.Role{ID: "super_admin", Name: "Super Admin", IsGlobal: true}}
	admin := &fakeAdminClientForCreate{}

	uc := NewInvitationUsecase(invRepo, nil, nil, fakeTenantLookup{}, roleRepo, admin, nil, "https://app.example.com", 100)

	ctx := ctxForCreateInvitation("super_admin")
	inv, err := uc.CreateInvitation(ctx, "target@example.com", "super_admin")

	require.NoError(t, err)
	require.NotNil(t, inv)
	assert.True(t, invRepo.createCalled)
	assert.True(t, admin.inviteCalled)
}

// TestCreateInvitation_RoleIDInexistenteEsRechazado prueba que un role_id que
// directamente no existe (no un global oculto) recibe el mismo tratamiento:
// hoy la tabla no tiene FK sobre role_id, así que sin este chequeo cualquier
// string pasaba.
func TestCreateInvitation_RoleIDInexistenteEsRechazado(t *testing.T) {
	invRepo := &fakeInvRepoForCreate{}
	roleRepo := fakeRoleLookup{err: domain.ErrRoleNotFound}
	admin := &fakeAdminClientForCreate{}

	uc := NewInvitationUsecase(invRepo, nil, nil, fakeTenantLookup{}, roleRepo, admin, nil, "https://app.example.com", 100)

	ctx := ctxForCreateInvitation("admin")
	_, err := uc.CreateInvitation(ctx, "target@example.com", "no-existe")

	require.ErrorIs(t, err, domain.ErrRoleNotFound)
	assert.False(t, invRepo.createCalled)
	assert.False(t, admin.inviteCalled)
}
