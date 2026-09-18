package revoke_invitation

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
	"github.com/tu-org/embolsadora-api/internal/repo/pg/invitations"
)

// fakeInvRepo implementa invitations.InvitationRepository. RevokeInvitation
// usa GetByID y UpdateStatus; el resto no se llama en este flujo.
type fakeInvRepo struct {
	inv       *domain.UserInvitation
	getErr    error
	updateErr error
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
func (f fakeInvRepo) UpdateStatus(ctx context.Context, id string, status domain.InvitationStatus) error {
	return f.updateErr
}

func newRevokeRouter(repo invitations.InvitationRepository) *gin.Engine {
	gin.SetMode(gin.TestMode)
	uc := usecases.NewInvitationUsecase(repo, nil, nil, nil, nil, nil, nil, "https://embolsadora.site", 100)
	h := NewHandler(uc)
	r := gin.New()
	r.POST("/api/v1/invitations/:id/revoke", h.Handle)
	return r
}

func doRevokeRequest(r *gin.Engine, id string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/invitations/"+id+"/revoke", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestHandle_InvalidID_Returns400(t *testing.T) {
	r := newRevokeRouter(fakeInvRepo{})
	w := doRevokeRequest(r, "not-a-uuid")
	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandle_InvitationNotFound_Returns404(t *testing.T) {
	r := newRevokeRouter(fakeInvRepo{getErr: domain.ErrNotFound})
	w := doRevokeRequest(r, uuid.NewString())
	require.Equal(t, http.StatusNotFound, w.Code)
}

func TestHandle_UpdateStatusFails_Returns500(t *testing.T) {
	inv := &domain.UserInvitation{ID: uuid.NewString(), Email: "a@b.com", Status: domain.InvitationStatusPending}
	r := newRevokeRouter(fakeInvRepo{inv: inv, updateErr: errors.New("db down")})
	w := doRevokeRequest(r, inv.ID)
	require.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandle_PendingInvitation_RevokesAndReturns200WithRevokedStatus(t *testing.T) {
	inv := &domain.UserInvitation{ID: uuid.NewString(), TenantID: uuid.NewString(), Email: "a@b.com", RoleID: "cliente_admin", Status: domain.InvitationStatusPending}
	r := newRevokeRouter(fakeInvRepo{inv: inv})
	w := doRevokeRequest(r, inv.ID)

	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `"status":"revoked"`)
}
