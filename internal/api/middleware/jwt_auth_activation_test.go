package middleware_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	jwt "github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apimw "github.com/tu-org/embolsadora-api/internal/api/middleware"
	"github.com/tu-org/embolsadora-api/internal/api/usecases"
	"github.com/tu-org/embolsadora-api/internal/domain"
	"github.com/tu-org/embolsadora-api/internal/platform"
)

// fakeVerifier evita depender de un JWKS real: cualquier token string
// "válido" (no vacío) devuelve claims fijas.
type fakeVerifier struct{}

func (fakeVerifier) Verify(tokenString string) (*jwt.Token, error) {
	if tokenString == "" {
		return nil, errors.New("empty token")
	}
	return &jwt.Token{Claims: jwt.MapClaims{
		"sub":   "test-supabase-sub",
		"email": "invited@example.com",
	}}, nil
}

// fakeUserRepo implementa users.UserRepository con el mínimo necesario:
// UpsertBySupabaseID (llamado por ProvisionUser) devuelve un usuario
// 'invited' fijo. El resto de los métodos no los llama JWTAuth en este flujo.
type fakeUserRepo struct{}

func (fakeUserRepo) UpsertBySupabaseID(ctx context.Context, supabaseUserID, email string) (*domain.User, error) {
	return &domain.User{ID: "test-user-id", Email: email, Status: domain.UserStatusInvited}, nil
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
func (fakeUserRepo) SetPasswordChangeRequired(ctx context.Context, userID string, value bool) error {
	return nil
}
func (fakeUserRepo) IsActiveMemberOfTenant(ctx context.Context, userID, tenantID string) (bool, error) {
	return false, nil
}

// failingActivator siempre falla — simula el caso real que dejó 5
// invitaciones activadas con rol incorrecto en producción.
type failingActivator struct{}

func (failingActivator) ActivatePendingInvitations(ctx context.Context, email, userID string) error {
	return errors.New("simulated activation failure")
}

func runJWTAuth(t *testing.T, activator usecases.InvitationActivator) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)

	authUC := usecases.NewAuthUsecase(fakeUserRepo{})
	r := gin.New()
	r.Use(apimw.JWTAuth(fakeVerifier{}, authUC, activator))
	r.GET("/probe", func(c *gin.Context) {
		user := platform.DomainUser(c.Request.Context())
		require.NotNil(t, user)
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/probe", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestJWTAuth_ActivacionFallaAbortaElRequest(t *testing.T) {
	w := runJWTAuth(t, failingActivator{})
	assert.Equal(t, http.StatusInternalServerError, w.Code,
		"si ActivatePendingInvitations falla, el request debe abortar en vez de continuar con el usuario 'invited' en un estado inconsistente")
}

// succeedingActivator no falla — control positivo: el camino feliz (sin
// invitaciones pendientes, o activación exitosa) no debe verse afectado.
type succeedingActivator struct{}

func (succeedingActivator) ActivatePendingInvitations(ctx context.Context, email, userID string) error {
	return nil
}

func TestJWTAuth_ActivacionExitosaSigueDeLargo(t *testing.T) {
	w := runJWTAuth(t, succeedingActivator{})
	assert.Equal(t, http.StatusOK, w.Code, "una activación exitosa no debe abortar el request")
}
