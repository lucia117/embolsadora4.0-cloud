package get_public_tenant

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/tu-org/embolsadora-api/internal/api/usecases/tenants/get_public_tenant"
	"github.com/tu-org/embolsadora-api/internal/domain"
)

// stubRepo implements the full tenants.TenantRepository interface — only
// FindByID/FindBySubdomain matter for these tests, the rest are unused no-ops.
type stubRepo struct {
	tenant *domain.Tenant
}

func (s *stubRepo) Create(ctx context.Context, tenant *domain.Tenant) error { return nil }
func (s *stubRepo) FindAll(ctx context.Context) ([]domain.Tenant, error)    { return nil, nil }
func (s *stubRepo) Update(ctx context.Context, tenant *domain.Tenant) error { return nil }
func (s *stubRepo) Delete(ctx context.Context, id uuid.UUID) error          { return nil }
func (s *stubRepo) FindByID(ctx context.Context, id uuid.UUID) (*domain.Tenant, error) {
	return s.tenant, nil
}
func (s *stubRepo) FindBySubdomain(ctx context.Context, subdomain string) (*domain.Tenant, error) {
	return s.tenant, nil
}

func newTestRouter(tenant *domain.Tenant) *gin.Engine {
	gin.SetMode(gin.TestMode)
	uc := get_public_tenant.NewUseCase(&stubRepo{tenant: tenant})
	r := gin.Default()
	h := NewGetPublicTenantHandler(uc)
	r.GET("/api/v1/public/tenants/:idOrSubdomain", h.GetPublicTenant)
	return r
}

func TestGetPublicTenant_Found_NoBodyLeaks(t *testing.T) {
	id := uuid.New()
	tenant := &domain.Tenant{
		ID: id, Subdomain: "cordoba", Name: "Cordoba SA", CompanyName: "Cordoba SA", IsActive: true,
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	tenant.Settings.ContactEmail = "secret@cordoba.com"
	tenant.Address.Street = "Av. Secreta 123"
	r := newTestRouter(tenant)

	req, _ := http.NewRequest("GET", "/api/v1/public/tenants/"+id.String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.NotContains(t, w.Body.String(), "secret@cordoba.com")
	assert.NotContains(t, w.Body.String(), "Av. Secreta")
}

func TestGetPublicTenant_NotFound(t *testing.T) {
	r := newTestRouter(nil)

	req, _ := http.NewRequest("GET", "/api/v1/public/tenants/no-such-tenant", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestGetPublicTenant_BySubdomain(t *testing.T) {
	id := uuid.New()
	tenant := &domain.Tenant{ID: id, Subdomain: "cordoba", Name: "Cordoba SA", IsActive: true, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	r := newTestRouter(tenant)

	req, _ := http.NewRequest("GET", "/api/v1/public/tenants/cordoba", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
