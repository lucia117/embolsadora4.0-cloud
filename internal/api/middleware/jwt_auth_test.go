package middleware_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	jwt "github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"

	apimw "github.com/tu-org/embolsadora-api/internal/api/middleware"
	"github.com/tu-org/embolsadora-api/internal/api/usecases"
	"github.com/tu-org/embolsadora-api/internal/domain"
	"github.com/tu-org/embolsadora-api/internal/platform"
	"github.com/tu-org/embolsadora-api/internal/repo/pg/users"
	"github.com/tu-org/embolsadora-api/internal/security"
)

// claimsVerifier devuelve claims fijas para cualquier token, o el err
// configurado -- deja elegir por test qué rama de JWTAuth se ejercita sin
// depender de un JWKS real.
type claimsVerifier struct {
	sub   string
	email string
	err   error
}

func (v claimsVerifier) Verify(tokenString string) (*jwt.Token, error) {
	if v.err != nil {
		return nil, v.err
	}
	return &jwt.Token{Claims: jwt.MapClaims{"sub": v.sub, "email": v.email}}, nil
}

// statusUserRepo devuelve un usuario con el Status configurado desde
// UpsertBySupabaseID. El resto de los métodos no los llama JWTAuth cuando no
// hay activator (nil).
type statusUserRepo struct {
	status domain.UserStatus
}

func (r statusUserRepo) UpsertBySupabaseID(ctx context.Context, supabaseUserID, email string) (*domain.User, error) {
	return &domain.User{ID: "test-user-id", Email: email, Status: r.status}, nil
}
func (statusUserRepo) GetBySupabaseID(ctx context.Context, supabaseUserID string) (*domain.User, error) {
	return nil, errors.New("not implemented")
}
func (statusUserRepo) GetByID(ctx context.Context, id string) (*domain.User, error) {
	return nil, errors.New("not implemented")
}
func (statusUserRepo) SetStatus(ctx context.Context, userID string, status domain.UserStatus) error {
	return nil
}
func (statusUserRepo) SetPasswordChangeRequired(ctx context.Context, userID string, value bool) error {
	return nil
}
func (statusUserRepo) IsActiveMemberOfTenant(ctx context.Context, userID, tenantID string) (bool, error) {
	return false, nil
}

func newJWTAuthRouter(verifier security.Verifier, repo users.UserRepository) *gin.Engine {
	gin.SetMode(gin.TestMode)
	authUC := usecases.NewAuthUsecase(repo)
	r := gin.New()
	r.Use(apimw.JWTAuth(verifier, authUC, nil))
	r.GET("/probe", func(c *gin.Context) { c.Status(http.StatusOK) })
	return r
}

func doJWTAuthRequest(r *gin.Engine, authHeader string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/probe", nil)
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestJWTAuth_MissingAuthorizationHeader_Returns401(t *testing.T) {
	r := newJWTAuthRouter(claimsVerifier{sub: "s", email: "e@x.com"}, statusUserRepo{status: domain.UserStatusActive})
	w := doJWTAuthRequest(r, "")
	require.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestJWTAuth_AuthorizationHeaderWithoutBearerPrefix_Returns401(t *testing.T) {
	r := newJWTAuthRouter(claimsVerifier{sub: "s", email: "e@x.com"}, statusUserRepo{status: domain.UserStatusActive})
	w := doJWTAuthRequest(r, "Token abc")
	require.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestJWTAuth_InvalidToken_Returns401(t *testing.T) {
	r := newJWTAuthRouter(claimsVerifier{err: errors.New("bad signature")}, statusUserRepo{status: domain.UserStatusActive})
	w := doJWTAuthRequest(r, "Bearer bad-token")
	require.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestJWTAuth_JWKSUnavailable_Returns503(t *testing.T) {
	r := newJWTAuthRouter(
		claimsVerifier{err: fmt.Errorf("%w: dial tcp timeout", security.ErrJWKSUnavailable)},
		statusUserRepo{status: domain.UserStatusActive},
	)
	w := doJWTAuthRequest(r, "Bearer any-token")
	require.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestJWTAuth_MissingSubClaim_Returns401(t *testing.T) {
	r := newJWTAuthRouter(claimsVerifier{sub: "", email: "e@x.com"}, statusUserRepo{status: domain.UserStatusActive})
	w := doJWTAuthRequest(r, "Bearer any-token")
	require.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestJWTAuth_RevokedUser_Returns403(t *testing.T) {
	r := newJWTAuthRouter(claimsVerifier{sub: "s", email: "e@x.com"}, statusUserRepo{status: domain.UserStatusRevoked})
	w := doJWTAuthRequest(r, "Bearer any-token")
	require.Equal(t, http.StatusForbidden, w.Code)
}

func TestJWTAuth_DisabledUser_Returns403(t *testing.T) {
	r := newJWTAuthRouter(claimsVerifier{sub: "s", email: "e@x.com"}, statusUserRepo{status: domain.UserStatusDisabled})
	w := doJWTAuthRequest(r, "Bearer any-token")
	require.Equal(t, http.StatusForbidden, w.Code)
}

// TestJWTAuth_ActiveUser_NoActivator_SetsDomainUserInContext cubre el camino
// feliz sin InvitationActivator (nil): el único otro test de este paquete que
// llega a 200 (jwt_auth_activation_test.go) siempre pasa un activator no-nil.
func TestJWTAuth_ActiveUser_NoActivator_SetsDomainUserInContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	authUC := usecases.NewAuthUsecase(statusUserRepo{status: domain.UserStatusActive})
	r := gin.New()
	r.Use(apimw.JWTAuth(claimsVerifier{sub: "s", email: "e@x.com"}, authUC, nil))

	var seenID string
	r.GET("/probe", func(c *gin.Context) {
		u, _ := platform.DomainUser(c.Request.Context()).(*domain.User)
		require.NotNil(t, u)
		seenID = u.ID
		c.Status(http.StatusOK)
	})

	w := doJWTAuthRequest(r, "Bearer any-token")
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "test-user-id", seenID)
}
