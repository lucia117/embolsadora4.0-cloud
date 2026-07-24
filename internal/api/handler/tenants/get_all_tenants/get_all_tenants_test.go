package get_all_tenants

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/tu-org/embolsadora-api/internal/api/handler/tenants/get_all_tenants/models"
	"github.com/tu-org/embolsadora-api/internal/api/usecases/tenants/get_all_tenants"
	"github.com/tu-org/embolsadora-api/internal/domain"
	"github.com/tu-org/embolsadora-api/internal/platform"
	"github.com/tu-org/embolsadora-api/internal/security"
)

type fakeRepo struct{}

func (f *fakeRepo) Create(ctx context.Context, tenant *domain.Tenant) error { return nil }
func (f *fakeRepo) FindAll(ctx context.Context) ([]domain.Tenant, error) {
	return []domain.Tenant{{Name: "A"}, {Name: "B"}, {Name: "C"}}, nil
}
func (f *fakeRepo) FindByID(ctx context.Context, id uuid.UUID) (*domain.Tenant, error) {
	return &domain.Tenant{ID: id, Name: "Own Tenant"}, nil
}
func (f *fakeRepo) Update(ctx context.Context, tenant *domain.Tenant) error { return nil }
func (f *fakeRepo) Delete(ctx context.Context, id uuid.UUID) error         { return nil }

func withActorContext(req *http.Request, role, tenantID string) *http.Request {
	ctx := security.WithRole(req.Context(), role)
	ctx = platform.WithTenantID(ctx, tenantID)
	return req.WithContext(ctx)
}

func newTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	uc := get_all_tenants.NewUseCase(&fakeRepo{})
	h := NewGetAllTenantsHandler(uc)
	r := gin.Default()
	r.GET("/api/v1/tenants", h.GetAllTenants)
	return r
}

func TestGetAllTenants_CrossTenantRole_ReturnsFullList(t *testing.T) {
	r := newTestRouter()

	req, _ := http.NewRequest("GET", "/api/v1/tenants", nil)
	req = withActorContext(req, "super_admin", uuid.New().String())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp models.GetAllTenantsResponse
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Len(t, resp, 3)
}

func TestGetAllTenants_NonGlobalRole_ReturnsOnlyOwnTenant(t *testing.T) {
	r := newTestRouter()
	actorTenantID := uuid.New()

	req, _ := http.NewRequest("GET", "/api/v1/tenants", nil)
	req = withActorContext(req, "admin", actorTenantID.String())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp models.GetAllTenantsResponse
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Len(t, resp, 1)
	assert.Equal(t, actorTenantID.String(), resp[0].ID)
}
