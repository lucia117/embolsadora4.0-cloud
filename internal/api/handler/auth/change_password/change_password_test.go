package change_password

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/tu-org/embolsadora-api/internal/api/usecases"
	"github.com/tu-org/embolsadora-api/internal/domain"
	"github.com/tu-org/embolsadora-api/internal/platform"
	"github.com/tu-org/embolsadora-api/internal/platform/supabase"
	"github.com/tu-org/embolsadora-api/internal/repo/pg/users"
)

// fakeUserRepo implementa users.UserRepository con el mínimo necesario para
// ClearPasswordChangeRequired: SetPasswordChangeRequired configurable. El
// resto de los métodos no los llama este flujo.
type fakeUserRepo struct {
	setErr error

	setCalls   int
	lastUserID string
	lastValue  bool
}

func (fakeUserRepo) UpsertBySupabaseID(ctx context.Context, supabaseUserID, email string) (*domain.User, error) {
	return nil, errors.New("not implemented")
}
func (fakeUserRepo) GetBySupabaseID(ctx context.Context, supabaseUserID string) (*domain.User, error) {
	return nil, errors.New("not implemented")
}
func (fakeUserRepo) GetByID(ctx context.Context, id string) (*domain.User, error) {
	return nil, errors.New("not implemented")
}
func (fakeUserRepo) SetStatus(ctx context.Context, userID string, status domain.UserStatus) error {
	return nil
}
func (r *fakeUserRepo) SetPasswordChangeRequired(ctx context.Context, userID string, value bool) error {
	r.setCalls++
	r.lastUserID = userID
	r.lastValue = value
	return r.setErr
}
func (fakeUserRepo) IsActiveMemberOfTenant(ctx context.Context, userID, tenantID string) (bool, error) {
	return false, nil
}

// fakeAdminClient no lo usa ClearPasswordChangeRequired -- solo hace falta
// para satisfacer la firma de NewPasswordUsecase.
type fakeAdminClient struct{}

func (fakeAdminClient) InviteUserByEmail(ctx context.Context, p supabase.InviteParams) error {
	return errors.New("not implemented")
}
func (fakeAdminClient) SendPasswordResetEmail(ctx context.Context, userEmail, redirectTo string) error {
	return errors.New("not implemented")
}

func newChangePasswordRouter(repo users.UserRepository, user *domain.User) *gin.Engine {
	gin.SetMode(gin.TestMode)
	uc := usecases.NewPasswordUsecase(repo, nil, fakeAdminClient{}, "https://embolsadora.site", nil)
	h := NewHandler(uc)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		if user != nil {
			ctx := platform.WithDomainUser(c.Request.Context(), user)
			c.Request = c.Request.WithContext(ctx)
		}
		c.Next()
	})
	r.POST("/api/v1/auth/change-password", h.Handle)
	return r
}

func doChangePasswordRequest(r *gin.Engine) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/change-password", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestHandle_AuthenticatedUser_ClearsFlagAndReturns200(t *testing.T) {
	repo := &fakeUserRepo{}
	r := newChangePasswordRouter(repo, &domain.User{ID: "u1"})
	w := doChangePasswordRequest(r)
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, 1, repo.setCalls)
	require.Equal(t, "u1", repo.lastUserID)
	require.False(t, repo.lastValue)
}

func TestHandle_NoDomainUserInContext_Returns401(t *testing.T) {
	r := newChangePasswordRouter(&fakeUserRepo{}, nil)
	w := doChangePasswordRequest(r)
	require.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestHandle_RepoError_Returns500(t *testing.T) {
	r := newChangePasswordRouter(&fakeUserRepo{setErr: errors.New("db down")}, &domain.User{ID: "u1"})
	w := doChangePasswordRequest(r)
	require.Equal(t, http.StatusInternalServerError, w.Code)
}
