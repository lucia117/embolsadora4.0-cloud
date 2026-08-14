package users

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	appUsers "github.com/tu-org/embolsadora-api/internal/app/users"
	"github.com/tu-org/embolsadora-api/internal/domain"
	domainUsers "github.com/tu-org/embolsadora-api/internal/domain/users"
	"github.com/tu-org/embolsadora-api/internal/platform"
)

// fakeUserRepo es un doble mínimo de users.Repository. UpdateMe solo ejercita
// GetByID y Update — el resto de los métodos existen únicamente para satisfacer
// la interfaz.
type fakeUserRepo struct {
	stored *domainUsers.User
}

func (f *fakeUserRepo) ListByTenant(ctx context.Context, tenantID string, limit, offset int, includeGlobal bool) ([]*domainUsers.User, int64, error) {
	return nil, 0, nil
}
func (f *fakeUserRepo) GetByID(ctx context.Context, tenantID, userID string, includeGlobal bool) (*domainUsers.User, error) {
	if f.stored == nil || f.stored.ID != userID {
		return nil, domainUsers.ErrNotFound
	}
	cp := *f.stored
	return &cp, nil
}
func (f *fakeUserRepo) GetByIDWithRoles(ctx context.Context, tenantID, userID string, includeGlobal bool) (*domainUsers.UserWithRoles, error) {
	return nil, domainUsers.ErrNotFound
}
func (f *fakeUserRepo) ListPendingByTenant(ctx context.Context, tenantID string, includeGlobal bool) ([]*domainUsers.User, error) {
	return nil, nil
}
func (f *fakeUserRepo) Create(ctx context.Context, user *domainUsers.User) (*domainUsers.User, error) {
	return user, nil
}
func (f *fakeUserRepo) CreateWithRole(ctx context.Context, user *domainUsers.User, utr *domain.UserTenantRole) (*domainUsers.User, error) {
	return user, nil
}
func (f *fakeUserRepo) Update(ctx context.Context, user *domainUsers.User) (*domainUsers.User, error) {
	f.stored = user
	return user, nil
}
func (f *fakeUserRepo) Delete(ctx context.Context, tenantID, userID string) error { return nil }

func newTestRouterForUpdateMe(repo *fakeUserRepo) *gin.Engine {
	gin.SetMode(gin.TestMode)
	svc := appUsers.NewService(repo, nil, nil, zap.NewNop())
	h := NewHandler(svc, zap.NewNop())
	r := gin.New()
	r.PATCH("/api/v1/users/me", h.UpdateMe)
	return r
}

func withDomainUserAndTenant(req *http.Request, userID, tenantID string) *http.Request {
	ctx := platform.WithDomainUser(req.Context(), &domain.User{ID: userID})
	ctx = platform.WithTenantID(ctx, tenantID)
	return req.WithContext(ctx)
}

// TestUpdateMe_SinPermisoRBAC_Funciona es el regression test de B-002: antes,
// este flujo pegaba a PATCH /users/:id (gateado por RBACCheck("users:write")) y
// devolvía 403 para cualquier rol sin ese permiso. Esta ruta no tiene RBACCheck
// en absoluto — el test lo prueba registrando SOLO el handler, sin ningún
// middleware de por medio, y confirma que igual funciona.
func TestUpdateMe_SinPermisoRBAC_Funciona(t *testing.T) {
	userID := "11111111-1111-1111-1111-111111111111"
	repo := &fakeUserRepo{stored: &domainUsers.User{ID: userID, FirstName: "Viejo", LastName: "Nombre"}}
	r := newTestRouterForUpdateMe(repo)

	body, _ := json.Marshal(map[string]string{"firstName": "Nuevo", "lastName": "Apellido"})
	req, _ := http.NewRequest("PATCH", "/api/v1/users/me", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = withDomainUserAndTenant(req, userID, "22222222-2222-2222-2222-222222222222")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, "Nuevo", repo.stored.FirstName)
	assert.Equal(t, "Apellido", repo.stored.LastName)
}

// TestUpdateMe_IgnoraCampoRoleEnElBody confirma que aunque un cliente mande
// "role" en el JSON (UpdateUserRequest sí lo acepta, UpdateMeRequest no), el
// campo se ignora silenciosamente — no hay forma de que llegue a
// UpdateUserCommand.Role porque el DTO no lo tiene.
func TestUpdateMe_IgnoraCampoRoleEnElBody(t *testing.T) {
	userID := "11111111-1111-1111-1111-111111111111"
	repo := &fakeUserRepo{stored: &domainUsers.User{ID: userID, Role: "operario"}}
	r := newTestRouterForUpdateMe(repo)

	body, _ := json.Marshal(map[string]string{"firstName": "X", "role": "super_admin"})
	req, _ := http.NewRequest("PATCH", "/api/v1/users/me", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = withDomainUserAndTenant(req, userID, "22222222-2222-2222-2222-222222222222")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, "operario", repo.stored.Role, "role no debe cambiar vía este endpoint")
}

func TestUpdateMe_SinDomainUserEnContexto_Unauthorized(t *testing.T) {
	r := newTestRouterForUpdateMe(&fakeUserRepo{})

	body, _ := json.Marshal(map[string]string{"firstName": "X"})
	req, _ := http.NewRequest("PATCH", "/api/v1/users/me", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
