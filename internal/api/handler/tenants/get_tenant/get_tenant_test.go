package get_tenant

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/tu-org/embolsadora-api/internal/api/usecases/tenants/get_tenant"
	"github.com/tu-org/embolsadora-api/internal/domain"
	"github.com/tu-org/embolsadora-api/internal/platform"
	"github.com/tu-org/embolsadora-api/internal/security"
)

// fakeRepo returns tenant (with its ID overwritten to whatever ID was requested)
// for every FindByID call, so tests can focus purely on the scoping check.
type fakeRepo struct {
	tenant *domain.Tenant
}

func (f *fakeRepo) Create(ctx context.Context, tenant *domain.Tenant) error { return nil }
func (f *fakeRepo) FindAll(ctx context.Context) ([]domain.Tenant, error)    { return nil, nil }
func (f *fakeRepo) FindByID(ctx context.Context, id uuid.UUID) (*domain.Tenant, error) {
	if f.tenant == nil {
		return nil, nil
	}
	t := *f.tenant
	t.ID = id
	return &t, nil
}
func (f *fakeRepo) FindBySubdomain(ctx context.Context, subdomain string) (*domain.Tenant, error) {
	if f.tenant == nil {
		return nil, nil
	}
	t := *f.tenant
	t.Subdomain = subdomain
	return &t, nil
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
	uc := get_tenant.NewUseCase(&fakeRepo{tenant: &domain.Tenant{Name: "Demo", CreatedAt: time.Now(), UpdatedAt: time.Now()}})
	r := gin.Default()
	h := NewGetTenantHandler(uc)
	r.GET("/api/v1/tenants/:tenantId", h.GetTenant)
	return r
}

func TestGetTenantHandler_OwnTenant_NonGlobalRole_Allowed(t *testing.T) {
	id := uuid.New()
	r := newTestRouter()

	req, _ := http.NewRequest("GET", "/api/v1/tenants/"+id.String(), nil)
	req = withActorContext(req, "admin", id.String())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGetTenantHandler_ForeignTenant_NonGlobalRole_Forbidden(t *testing.T) {
	id := uuid.New()
	otherTenantID := uuid.New()
	r := newTestRouter()

	req, _ := http.NewRequest("GET", "/api/v1/tenants/"+id.String(), nil)
	req = withActorContext(req, "admin", otherTenantID.String())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestGetTenantHandler_ForeignTenant_CrossTenantRole_Allowed(t *testing.T) {
	id := uuid.New()
	actorTenantID := uuid.New()
	r := newTestRouter()

	req, _ := http.NewRequest("GET", "/api/v1/tenants/"+id.String(), nil)
	req = withActorContext(req, "super_admin", actorTenantID.String())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGetTenantHandler_NoActorContext_Forbidden(t *testing.T) {
	id := uuid.New()
	r := newTestRouter()

	req, _ := http.NewRequest("GET", "/api/v1/tenants/"+id.String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestGetTenantHandler_InvalidID(t *testing.T) {
	r := newTestRouter()

	req, _ := http.NewRequest("GET", "/api/v1/tenants/invalid-id", nil)
	req = withActorContext(req, "super_admin", uuid.New().String())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
