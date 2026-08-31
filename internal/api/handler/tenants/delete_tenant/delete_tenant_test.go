package delete_tenant

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	ucDeleteTenant "github.com/tu-org/embolsadora-api/internal/api/usecases/tenants/delete_tenant"
	"github.com/tu-org/embolsadora-api/internal/domain"
	"github.com/tu-org/embolsadora-api/internal/platform"
	"github.com/tu-org/embolsadora-api/internal/security"
)

type mockRepo struct{}

func (m *mockRepo) Create(ctx context.Context, tenant *domain.Tenant) error { return nil }
func (m *mockRepo) FindAll(ctx context.Context) ([]domain.Tenant, error)    { return nil, nil }
func (m *mockRepo) FindByID(ctx context.Context, id uuid.UUID) (*domain.Tenant, error) {
	return &domain.Tenant{ID: id}, nil
}
func (m *mockRepo) FindBySubdomain(ctx context.Context, subdomain string) (*domain.Tenant, error) {
	return nil, nil
}
func (m *mockRepo) Update(ctx context.Context, tenant *domain.Tenant) error { return nil }
func (m *mockRepo) Delete(ctx context.Context, id uuid.UUID) error         { return nil }

func withActorContext(req *http.Request, role, tenantID string) *http.Request {
	// Los 4 archivos de test de este paquete solo ejercitan "super_admin" (global)
	// y "admin" (no-global) — misma señal que crossTenantRoles tenía hardcodeada
	// antes de que IsCrossTenantRole pasara a leer RoleContext.IsGlobal.
	ctx := security.WithRoleContext(req.Context(), security.RoleContext{Name: role, IsGlobal: role == "super_admin"})
	ctx = platform.WithTenantID(ctx, tenantID)
	return req.WithContext(ctx)
}

func newTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	useCase := ucDeleteTenant.NewUseCase(&mockRepo{})
	h := NewDeleteTenantHandler(useCase)
	r := gin.Default()
	r.DELETE("/api/v1/tenants/:tenantId", h.DeleteTenant)
	return r
}

func TestDeleteTenantHandler_CrossTenantRole_Allowed(t *testing.T) {
	r := newTestRouter()
	id := uuid.New().String()

	req, _ := http.NewRequest("DELETE", "/api/v1/tenants/"+id, nil)
	req = withActorContext(req, "super_admin", uuid.New().String())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestDeleteTenantHandler_OwnTenant_NonGlobalRole_Allowed(t *testing.T) {
	r := newTestRouter()
	id := uuid.New().String()

	req, _ := http.NewRequest("DELETE", "/api/v1/tenants/"+id, nil)
	req = withActorContext(req, "admin", id)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestDeleteTenantHandler_ForeignTenant_NonGlobalRole_Forbidden(t *testing.T) {
	r := newTestRouter()
	id := uuid.New().String()
	otherTenantID := uuid.New().String()

	req, _ := http.NewRequest("DELETE", "/api/v1/tenants/"+id, nil)
	req = withActorContext(req, "admin", otherTenantID)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestDeleteTenantHandler_InvalidID(t *testing.T) {
	r := newTestRouter()

	req, _ := http.NewRequest("DELETE", "/api/v1/tenants/invalid-id", nil)
	req = withActorContext(req, "super_admin", uuid.New().String())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
