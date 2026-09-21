package list_invitations

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
	"github.com/tu-org/embolsadora-api/internal/platform"
	"github.com/tu-org/embolsadora-api/internal/repo/pg/invitations"
)

// fakeInvRepo implementa invitations.InvitationRepository y registra los
// argumentos de la última llamada a ListByTenant, para poder assertear QUÉ
// tenant/status le llegaron -- no solo que la respuesta HTTP fue 200.
type fakeInvRepo struct {
	list []domain.UserInvitation
	err  error

	gotTenantID      string
	gotStatus        *string
	gotIncludeGlobal bool
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
func (fakeInvRepo) GetByID(ctx context.Context, id, tenantID string, includeGlobal bool) (*domain.UserInvitation, error) {
	return nil, errors.New("not implemented")
}
func (f *fakeInvRepo) ListByTenant(ctx context.Context, tenantID string, status *string, includeGlobal bool) ([]domain.UserInvitation, error) {
	f.gotTenantID = tenantID
	f.gotStatus = status
	f.gotIncludeGlobal = includeGlobal
	if f.err != nil {
		return nil, f.err
	}
	return f.list, nil
}
func (fakeInvRepo) UpdateStatus(ctx context.Context, id string, status domain.InvitationStatus) error {
	return nil
}

func newListRouter(repo invitations.InvitationRepository, tenantID string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	uc := usecases.NewInvitationUsecase(repo, nil, nil, nil, nil, nil, nil, "https://embolsadora.site", 100)
	h := NewHandler(uc)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		ctx := platform.WithTenantID(c.Request.Context(), tenantID)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	r.GET("/api/v1/invitations", h.Handle)
	return r
}

// TestHandle_ScopesQueryToTenantFromContext cubre I2 del relevamiento: el
// usecase debe pasar el tenant del contexto (resuelto por TenantFromHeader) al
// repo, nunca un tenant que el caller pudiera inyectar por otra vía.
func TestHandle_ScopesQueryToTenantFromContext(t *testing.T) {
	tenantID := uuid.NewString()
	repo := &fakeInvRepo{list: []domain.UserInvitation{{ID: "i1", TenantID: tenantID, Email: "a@b.com", Status: domain.InvitationStatusPending}}}
	r := newListRouter(repo, tenantID)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/invitations", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, tenantID, repo.gotTenantID)
	require.Contains(t, w.Body.String(), "i1")
}

func TestHandle_StatusQueryParam_PassedToRepo(t *testing.T) {
	repo := &fakeInvRepo{}
	r := newListRouter(repo, uuid.NewString())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/invitations?status=pending", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.NotNil(t, repo.gotStatus)
	require.Equal(t, "pending", *repo.gotStatus)
}

func TestHandle_NoStatusQueryParam_PassesNilFilter(t *testing.T) {
	repo := &fakeInvRepo{}
	r := newListRouter(repo, uuid.NewString())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/invitations", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Nil(t, repo.gotStatus)
}

func TestHandle_RepoError_Returns500(t *testing.T) {
	repo := &fakeInvRepo{err: errors.New("db down")}
	r := newListRouter(repo, uuid.NewString())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/invitations", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusInternalServerError, w.Code)
}
