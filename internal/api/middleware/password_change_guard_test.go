package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/tu-org/embolsadora-api/internal/domain"
	"github.com/tu-org/embolsadora-api/internal/platform"
)

func newPasswordGuardRouter(user *domain.User) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		ctx := platform.WithDomainUser(c.Request.Context(), user)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	r.GET("/api/v1/dashboards", PasswordChangeGuard(), func(c *gin.Context) { c.Status(http.StatusOK) })
	r.GET("/api/v1/me", PasswordChangeGuard(), func(c *gin.Context) { c.Status(http.StatusOK) })
	r.POST("/api/v1/auth/change-password", PasswordChangeGuard(), func(c *gin.Context) { c.Status(http.StatusOK) })
	return r
}

func doPasswordGuardRequest(r *gin.Engine, method, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestPasswordChangeGuard_PasswordChangeRequired_BlocksNonExemptRoute(t *testing.T) {
	r := newPasswordGuardRouter(&domain.User{ID: "u1", PasswordChangeRequired: true})
	w := doPasswordGuardRequest(r, http.MethodGet, "/api/v1/dashboards")
	require.Equal(t, http.StatusForbidden, w.Code)
	require.Contains(t, w.Body.String(), "password_change_required")
}

func TestPasswordChangeGuard_ExemptGetMe_AllowsThroughEvenWhenRequired(t *testing.T) {
	r := newPasswordGuardRouter(&domain.User{ID: "u1", PasswordChangeRequired: true})
	w := doPasswordGuardRequest(r, http.MethodGet, "/api/v1/me")
	require.Equal(t, http.StatusOK, w.Code)
}

func TestPasswordChangeGuard_ExemptChangePassword_AllowsThroughEvenWhenRequired(t *testing.T) {
	r := newPasswordGuardRouter(&domain.User{ID: "u1", PasswordChangeRequired: true})
	w := doPasswordGuardRequest(r, http.MethodPost, "/api/v1/auth/change-password")
	require.Equal(t, http.StatusOK, w.Code)
}

func TestPasswordChangeGuard_NotRequired_AllowsThrough(t *testing.T) {
	r := newPasswordGuardRouter(&domain.User{ID: "u1", PasswordChangeRequired: false})
	w := doPasswordGuardRequest(r, http.MethodGet, "/api/v1/dashboards")
	require.Equal(t, http.StatusOK, w.Code)
}
