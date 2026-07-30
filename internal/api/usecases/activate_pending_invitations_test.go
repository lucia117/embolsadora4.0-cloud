package usecases

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tu-org/embolsadora-api/internal/domain"
)

// Fakes below implement only the interfaces ActivatePendingInvitations needs
// (invitations.InvitationRepository, users.UserRepository,
// user_roles.UserRoleRepository). Methods this usecase never calls just
// return zero values; the ones it does call record their arguments so tests
// can assert on them, following the fake style used in invite_metadata_test.go.

type fakeInvRepoForActivation struct {
	pending []domain.UserInvitation

	updateStatusCalls []struct {
		id     string
		status domain.InvitationStatus
	}
}

func (f *fakeInvRepoForActivation) Create(ctx context.Context, inv *domain.UserInvitation) (*domain.UserInvitation, error) {
	return nil, nil
}

func (f *fakeInvRepoForActivation) GetPendingByEmailAndTenant(ctx context.Context, email, tenantID string) (*domain.UserInvitation, error) {
	return nil, nil
}

func (f *fakeInvRepoForActivation) ListPendingByEmail(ctx context.Context, email string) ([]domain.UserInvitation, error) {
	return f.pending, nil
}

func (f *fakeInvRepoForActivation) GetByID(ctx context.Context, id, tenantID string) (*domain.UserInvitation, error) {
	return nil, nil
}

func (f *fakeInvRepoForActivation) ListByTenant(ctx context.Context, tenantID string, status *string) ([]domain.UserInvitation, error) {
	return nil, nil
}

func (f *fakeInvRepoForActivation) UpdateStatus(ctx context.Context, id string, status domain.InvitationStatus) error {
	f.updateStatusCalls = append(f.updateStatusCalls, struct {
		id     string
		status domain.InvitationStatus
	}{id, status})
	return nil
}

type fakeUserRoleRepoForActivation struct {
	createErr error
}

func (f *fakeUserRoleRepoForActivation) FindByTenant(ctx context.Context, tenantID uuid.UUID, status *string) ([]domain.UserTenantRoleDetail, error) {
	return nil, nil
}

func (f *fakeUserRoleRepoForActivation) FindByID(ctx context.Context, id uuid.UUID) (*domain.UserTenantRole, error) {
	return nil, nil
}

func (f *fakeUserRoleRepoForActivation) Create(ctx context.Context, utr *domain.UserTenantRole) (*domain.UserTenantRole, error) {
	if f.createErr != nil {
		return nil, f.createErr
	}
	return utr, nil
}

func (f *fakeUserRoleRepoForActivation) Update(ctx context.Context, utr *domain.UserTenantRole) (*domain.UserTenantRole, error) {
	return nil, nil
}

func (f *fakeUserRoleRepoForActivation) Revoke(ctx context.Context, id uuid.UUID, tenantID uuid.UUID) (*domain.UserTenantRole, error) {
	return nil, nil
}

func (f *fakeUserRoleRepoForActivation) BulkCreate(ctx context.Context, utrs []domain.UserTenantRole) ([]domain.UserTenantRole, error) {
	return nil, nil
}

func (f *fakeUserRoleRepoForActivation) FindByUser(ctx context.Context, userID uuid.UUID) ([]domain.UserRoleWithContext, error) {
	return nil, nil
}

func (f *fakeUserRoleRepoForActivation) UpdateStatus(ctx context.Context, userID, tenantID uuid.UUID, status domain.UserRoleStatus) (*domain.UserTenantRole, error) {
	return nil, nil
}

type fakeUserRepoForActivation struct {
	setStatusCalls                 []domain.UserStatus
	setPasswordChangeRequiredCalls []bool
}

func (f *fakeUserRepoForActivation) UpsertBySupabaseID(ctx context.Context, supabaseUserID, email string) (*domain.User, error) {
	return nil, nil
}

func (f *fakeUserRepoForActivation) GetBySupabaseID(ctx context.Context, supabaseUserID string) (*domain.User, error) {
	return nil, nil
}

func (f *fakeUserRepoForActivation) GetByID(ctx context.Context, id string) (*domain.User, error) {
	return nil, nil
}

func (f *fakeUserRepoForActivation) SetStatus(ctx context.Context, userID string, status domain.UserStatus) error {
	f.setStatusCalls = append(f.setStatusCalls, status)
	return nil
}

func (f *fakeUserRepoForActivation) SetPasswordChangeRequired(ctx context.Context, userID string, value bool) error {
	f.setPasswordChangeRequiredCalls = append(f.setPasswordChangeRequiredCalls, value)
	return nil
}

func (f *fakeUserRepoForActivation) IsActiveMemberOfTenant(ctx context.Context, userID, tenantID string) (bool, error) {
	return false, nil
}

// TestActivatePendingInvitations_ActivaYRequierePassword cubre el gap real: un
// usuario invitado por Supabase nunca tiene password, asi que al activarse su
// primera invitacion pendiente el flag password_change_required debe quedar
// en true para que el frontend lo redirija a la pantalla de cambio de clave.
func TestActivatePendingInvitations_ActivaYRequierePassword(t *testing.T) {
	userID := uuid.New().String()
	tenantID := uuid.New().String()
	invitedBy := uuid.New().String()

	invRepo := &fakeInvRepoForActivation{
		pending: []domain.UserInvitation{
			{
				ID:        "inv-1",
				TenantID:  tenantID,
				Email:     "invited@example.com",
				RoleID:    "operator",
				Status:    domain.InvitationStatusPending,
				InvitedBy: invitedBy,
				ExpiresAt: time.Now().Add(24 * time.Hour),
			},
		},
	}
	userRoleRepo := &fakeUserRoleRepoForActivation{}
	userRepo := &fakeUserRepoForActivation{}

	uc := &InvitationUsecase{
		invRepo:      invRepo,
		userRepo:     userRepo,
		userRoleRepo: userRoleRepo,
	}

	err := uc.ActivatePendingInvitations(context.Background(), "invited@example.com", userID)
	require.NoError(t, err)

	require.Len(t, userRepo.setStatusCalls, 1)
	assert.Equal(t, domain.UserStatusActive, userRepo.setStatusCalls[0])

	require.Len(t, userRepo.setPasswordChangeRequiredCalls, 1, "una activacion real debe forzar el cambio de password")
	assert.True(t, userRepo.setPasswordChangeRequiredCalls[0])

	require.Len(t, invRepo.updateStatusCalls, 1)
	assert.Equal(t, domain.InvitationStatusAccepted, invRepo.updateStatusCalls[0].status)
}

// TestActivatePendingInvitations_SinInvitacionesPendientesNoTocaFlag cubre el
// caso negativo pedido explicitamente: si no hay nada que activar, el flag no
// debe tocarse (un login normal no puede empezar a exigir cambio de clave).
func TestActivatePendingInvitations_SinInvitacionesPendientesNoTocaFlag(t *testing.T) {
	invRepo := &fakeInvRepoForActivation{pending: nil}
	userRoleRepo := &fakeUserRoleRepoForActivation{}
	userRepo := &fakeUserRepoForActivation{}

	uc := &InvitationUsecase{
		invRepo:      invRepo,
		userRepo:     userRepo,
		userRoleRepo: userRoleRepo,
	}

	err := uc.ActivatePendingInvitations(context.Background(), "nobody@example.com", uuid.New().String())
	require.NoError(t, err)

	assert.Empty(t, userRepo.setStatusCalls)
	assert.Empty(t, userRepo.setPasswordChangeRequiredCalls, "sin invitaciones activadas, el flag no debe tocarse")
}

// TestActivatePendingInvitations_TodasExpiradasNoTocaFlag cubre la rama donde
// hay invitaciones pendientes en la fila pero ninguna sobrevive: activated
// queda en 0 igual que en el caso sin invitaciones, y el flag no debe tocarse.
func TestActivatePendingInvitations_TodasExpiradasNoTocaFlag(t *testing.T) {
	invRepo := &fakeInvRepoForActivation{
		pending: []domain.UserInvitation{
			{
				ID:        "inv-expired",
				TenantID:  uuid.New().String(),
				Email:     "invited@example.com",
				RoleID:    "operator",
				Status:    domain.InvitationStatusPending,
				InvitedBy: uuid.New().String(),
				ExpiresAt: time.Now().Add(-24 * time.Hour),
			},
		},
	}
	userRoleRepo := &fakeUserRoleRepoForActivation{}
	userRepo := &fakeUserRepoForActivation{}

	uc := &InvitationUsecase{
		invRepo:      invRepo,
		userRepo:     userRepo,
		userRoleRepo: userRoleRepo,
	}

	err := uc.ActivatePendingInvitations(context.Background(), "invited@example.com", uuid.New().String())
	require.NoError(t, err)

	assert.Empty(t, userRepo.setStatusCalls)
	assert.Empty(t, userRepo.setPasswordChangeRequiredCalls)
}
