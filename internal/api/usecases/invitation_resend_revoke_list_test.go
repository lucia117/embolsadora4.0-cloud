package usecases

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tu-org/embolsadora-api/internal/domain"
	"github.com/tu-org/embolsadora-api/internal/platform/supabase"
)

// fakeInvRepoForResendRevoke implementa invitations.InvitationRepository al
// completo (los métodos que Resend/Revoke/List no usan devuelven cero).
type fakeInvRepoForResendRevoke struct {
	getByIDResult *domain.UserInvitation
	getByIDErr    error

	updateStatusErr   error
	updateStatusCalls []struct {
		id     string
		status domain.InvitationStatus
	}

	listByTenantResult []domain.UserInvitation
	listByTenantErr    error
}

func (f *fakeInvRepoForResendRevoke) Create(context.Context, *domain.UserInvitation) (*domain.UserInvitation, error) {
	return nil, nil
}
func (f *fakeInvRepoForResendRevoke) GetPendingByEmailAndTenant(context.Context, string, string, bool) (*domain.UserInvitation, error) {
	return nil, nil
}
func (f *fakeInvRepoForResendRevoke) ListPendingByEmail(context.Context, string) ([]domain.UserInvitation, error) {
	return nil, nil
}
func (f *fakeInvRepoForResendRevoke) GetByID(context.Context, string, string, bool) (*domain.UserInvitation, error) {
	return f.getByIDResult, f.getByIDErr
}
func (f *fakeInvRepoForResendRevoke) ListByTenant(context.Context, string, *string, bool) ([]domain.UserInvitation, error) {
	return f.listByTenantResult, f.listByTenantErr
}
func (f *fakeInvRepoForResendRevoke) UpdateStatus(_ context.Context, id string, status domain.InvitationStatus) error {
	f.updateStatusCalls = append(f.updateStatusCalls, struct {
		id     string
		status domain.InvitationStatus
	}{id, status})
	return f.updateStatusErr
}

type fakeAdminClientForResendRevoke struct {
	inviteCalls []supabase.InviteParams
	inviteErr   error
}

func (f *fakeAdminClientForResendRevoke) InviteUserByEmail(_ context.Context, p supabase.InviteParams) error {
	f.inviteCalls = append(f.inviteCalls, p)
	return f.inviteErr
}
func (f *fakeAdminClientForResendRevoke) SendPasswordResetEmail(context.Context, string, string) error {
	return nil
}

func TestResendInvitation_ErrorDeGetByIDPropaga(t *testing.T) {
	invRepo := &fakeInvRepoForResendRevoke{getByIDErr: domain.ErrNotFound}
	uc := NewInvitationUsecase(invRepo, nil, nil, fakeTenantLookup{}, fakeRoleLookup{}, &fakeAdminClientForResendRevoke{}, nil, "https://app.example.com", 100)

	err := uc.ResendInvitation(ctxForCreateInvitation("admin"), "inv1", false)

	require.ErrorIs(t, err, domain.ErrNotFound)
}

func TestResendInvitation_NoPending(t *testing.T) {
	invRepo := &fakeInvRepoForResendRevoke{getByIDResult: &domain.UserInvitation{ID: "inv1", Status: domain.InvitationStatusAccepted}}
	uc := NewInvitationUsecase(invRepo, nil, nil, fakeTenantLookup{}, fakeRoleLookup{}, &fakeAdminClientForResendRevoke{}, nil, "https://app.example.com", 100)

	err := uc.ResendInvitation(ctxForCreateInvitation("admin"), "inv1", false)

	require.ErrorIs(t, err, domain.ErrInvitationNotPending)
}

func TestResendInvitation_InviterNameEsQuienReenviaNoElOriginal(t *testing.T) {
	invRepo := &fakeInvRepoForResendRevoke{getByIDResult: &domain.UserInvitation{
		ID: "inv1", Email: "target@example.com", Status: domain.InvitationStatusPending, RoleID: "operario", InvitedBy: "otro-user-id",
	}}
	admin := &fakeAdminClientForResendRevoke{}
	uc := NewInvitationUsecase(invRepo, nil, nil, fakeTenantLookup{}, fakeRoleLookup{role: &domain.Role{ID: "operario", Name: "Operario"}}, admin, nil, "https://app.example.com", 100)

	err := uc.ResendInvitation(ctxForCreateInvitation("admin"), "inv1", false)

	require.NoError(t, err)
	require.Len(t, admin.inviteCalls, 1)
	assert.Equal(t, "Caller", admin.inviteCalls[0].InviterName, "InviterName debe ser quien reenvía (el user del contexto), no InvitedBy")
	assert.Equal(t, "target@example.com", admin.inviteCalls[0].Email)
}

func TestResendInvitation_ErrorDeInviteUserByEmailPropaga(t *testing.T) {
	invRepo := &fakeInvRepoForResendRevoke{getByIDResult: &domain.UserInvitation{ID: "inv1", Status: domain.InvitationStatusPending}}
	admin := &fakeAdminClientForResendRevoke{inviteErr: errors.New("supabase down")}
	uc := NewInvitationUsecase(invRepo, nil, nil, fakeTenantLookup{}, fakeRoleLookup{}, admin, nil, "https://app.example.com", 100)

	err := uc.ResendInvitation(ctxForCreateInvitation("admin"), "inv1", false)

	require.Error(t, err)
}

func TestRevokeInvitation_ErrorDeGetByIDPropaga(t *testing.T) {
	invRepo := &fakeInvRepoForResendRevoke{getByIDErr: domain.ErrNotFound}
	uc := NewInvitationUsecase(invRepo, nil, nil, nil, nil, nil, nil, "", 100)

	_, err := uc.RevokeInvitation(ctxForCreateInvitation("admin"), "inv1", false)

	require.ErrorIs(t, err, domain.ErrNotFound)
}

func TestRevokeInvitation_ErrorDeUpdateStatusPropaga(t *testing.T) {
	invRepo := &fakeInvRepoForResendRevoke{
		getByIDResult:   &domain.UserInvitation{ID: "inv1", Status: domain.InvitationStatusPending},
		updateStatusErr: errors.New("db down"),
	}
	uc := NewInvitationUsecase(invRepo, nil, nil, nil, nil, nil, nil, "", 100)

	_, err := uc.RevokeInvitation(ctxForCreateInvitation("admin"), "inv1", false)

	require.Error(t, err)
}

func TestRevokeInvitation_FelizDevuelveElObjetoConStatusRevocadoAunqueElFakeDevuelvaOtro(t *testing.T) {
	invRepo := &fakeInvRepoForResendRevoke{
		getByIDResult: &domain.UserInvitation{ID: "inv1", Status: domain.InvitationStatusPending, Email: "x@example.com"},
	}
	uc := NewInvitationUsecase(invRepo, nil, nil, nil, nil, nil, nil, "", 100)

	got, err := uc.RevokeInvitation(ctxForCreateInvitation("admin"), "inv1", false)

	require.NoError(t, err)
	assert.Equal(t, domain.InvitationStatusRevoked, got.Status, "el status se muta localmente, no se vuelve a leer de la DB")
	require.Len(t, invRepo.updateStatusCalls, 1)
	assert.Equal(t, domain.InvitationStatusRevoked, invRepo.updateStatusCalls[0].status)
}

func TestListInvitations_Feliz(t *testing.T) {
	want := []domain.UserInvitation{{ID: "inv1"}}
	invRepo := &fakeInvRepoForResendRevoke{listByTenantResult: want}
	uc := NewInvitationUsecase(invRepo, nil, nil, nil, nil, nil, nil, "", 100)

	got, err := uc.ListInvitations(ctxForCreateInvitation("admin"), nil, true)

	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestListInvitations_ErrorDeRepoPropaga(t *testing.T) {
	invRepo := &fakeInvRepoForResendRevoke{listByTenantErr: errors.New("db down")}
	uc := NewInvitationUsecase(invRepo, nil, nil, nil, nil, nil, nil, "", 100)

	_, err := uc.ListInvitations(ctxForCreateInvitation("admin"), nil, false)

	require.Error(t, err)
}

func TestCheckRateLimit_RedisNilFailaAbierto(t *testing.T) {
	uc := NewInvitationUsecase(nil, nil, nil, nil, nil, nil, nil, "", 100)

	err := uc.checkRateLimit(context.Background(), testTenantID)

	require.NoError(t, err, "sin Redis, el rate limit debe fallar abierto (deshabilitado), nunca bloquear")
}
