package resend_invitation

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/tu-org/embolsadora-api/internal/api/usecases"
	"github.com/tu-org/embolsadora-api/internal/domain"
	"github.com/tu-org/embolsadora-api/internal/platform/supabase"
	"github.com/tu-org/embolsadora-api/internal/repo/pg/invitations"
)

// fakeInvRepo implementa invitations.InvitationRepository. ResendInvitation
// solo usa GetByID; el resto no se llama en este flujo.
type fakeInvRepo struct {
	inv    *domain.UserInvitation
	getErr error
}

func (fakeInvRepo) Create(ctx context.Context, inv *domain.UserInvitation) (*domain.UserInvitation, error) {
	return nil, errors.New("not implemented")
}
func (fakeInvRepo) GetPendingByEmailAndTenant(ctx context.Context, email, tenantID string, includeGlobal bool) (*domain.UserInvitation, error) {
	return nil, errors.New("not implemented")
}
func (fakeInvRepo) ListPendingByEmail(ctx context.Context, email string) ([]domain.UserInvitation, error) {
	return nil, errors.New("not implemented")
}
func (f fakeInvRepo) GetByID(ctx context.Context, id, tenantID string, includeGlobal bool) (*domain.UserInvitation, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	cp := *f.inv
	return &cp, nil
}
func (fakeInvRepo) ListByTenant(ctx context.Context, tenantID string, status *string, includeGlobal bool) ([]domain.UserInvitation, error) {
	return nil, errors.New("not implemented")
}
func (fakeInvRepo) UpdateStatus(ctx context.Context, id string, status domain.InvitationStatus) error {
	return nil
}

type fakeAdminClient struct {
	inviteErr error

	inviteCalls int
	lastParams  supabase.InviteParams
}

func (f *fakeAdminClient) InviteUserByEmail(ctx context.Context, p supabase.InviteParams) error {
	f.inviteCalls++
	f.lastParams = p
	return f.inviteErr
}
func (*fakeAdminClient) SendPasswordResetEmail(ctx context.Context, userEmail, redirectTo string) error {
	return errors.New("not implemented")
}

func newResendRouter(repo invitations.InvitationRepository, admin supabase.AdminClient) *gin.Engine {
	gin.SetMode(gin.TestMode)
	uc := usecases.NewInvitationUsecase(repo, nil, nil, nil, nil, admin, nil, "https://embolsadora.site", 100)
	h := NewHandler(uc)
	r := gin.New()
	r.POST("/api/v1/invitations/:id/resend", h.Handle)
	return r
}

func doResendRequest(r *gin.Engine, id string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/invitations/"+id+"/resend", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestHandle_InvalidID_Returns400(t *testing.T) {
	r := newResendRouter(fakeInvRepo{}, &fakeAdminClient{})
	w := doResendRequest(r, "not-a-uuid")
	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandle_InvitationNotFound_Returns404(t *testing.T) {
	r := newResendRouter(fakeInvRepo{getErr: domain.ErrNotFound}, &fakeAdminClient{})
	w := doResendRequest(r, uuid.NewString())
	require.Equal(t, http.StatusNotFound, w.Code)
}

// TestHandle_InvitationAlreadyAccepted_Returns409 cubre el guard de
// ResendInvitation contra invitaciones que ya salieron de 'pending' -- sin
// esto se podría reenviar el mail de una invitación ya aceptada o revocada.
func TestHandle_InvitationAlreadyAccepted_Returns409(t *testing.T) {
	inv := &domain.UserInvitation{ID: uuid.NewString(), Email: "a@b.com", Status: domain.InvitationStatusAccepted}
	r := newResendRouter(fakeInvRepo{inv: inv}, &fakeAdminClient{})
	w := doResendRequest(r, inv.ID)
	require.Equal(t, http.StatusConflict, w.Code)
}

func TestHandle_SupabaseFails_Returns500(t *testing.T) {
	inv := &domain.UserInvitation{ID: uuid.NewString(), Email: "a@b.com", Status: domain.InvitationStatusPending}
	r := newResendRouter(fakeInvRepo{inv: inv}, &fakeAdminClient{inviteErr: errors.New("smtp down")})
	w := doResendRequest(r, inv.ID)
	require.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandle_PendingInvitation_ResendsAndReturns200(t *testing.T) {
	inv := &domain.UserInvitation{ID: uuid.NewString(), Email: "a@b.com", Status: domain.InvitationStatusPending}
	admin := &fakeAdminClient{}
	r := newResendRouter(fakeInvRepo{inv: inv}, admin)
	w := doResendRequest(r, inv.ID)
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, 1, admin.inviteCalls)
	require.Equal(t, "a@b.com", admin.lastParams.Email)
}
