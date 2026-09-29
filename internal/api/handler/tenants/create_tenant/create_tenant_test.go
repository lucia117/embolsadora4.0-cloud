package create_tenant

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/tu-org/embolsadora-api/internal/domain"
)

// fakeUseCase implementa ucCreateTenant.UseCase y cuenta las llamadas a
// Create, para poder assertear que un 400 de validación nunca llega a
// persistir nada.
type fakeUseCase struct {
	err         error
	createCalls int
}

func (f *fakeUseCase) Create(ctx context.Context, tenant *domain.Tenant) error {
	f.createCalls++
	return f.err
}

func validTenantBody() string {
	return `{
		"name":"Demo",
		"companyName":"Demo SRL",
		"subdomain":"demo",
		"adminUser":{"email":"admin@demo.com","firstName":"A","lastName":"B","password":"password123"},
		"theme":{"primaryColor":"#000000"},
		"address":{"street":"Main 1","city":"CBA","state":"CBA","postalCode":"5000","country":"AR"}
	}`
}

func newCreateTenantRouter(uc *fakeUseCase) *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := NewCreateTenantHandler(uc)
	r := gin.New()
	r.POST("/api/v1/tenants", h.CreateTenant)
	return r
}

func doCreateTenantRequest(r *gin.Engine, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/tenants", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestCreateTenant_ValidBody_Returns201AndPersists(t *testing.T) {
	uc := &fakeUseCase{}
	r := newCreateTenantRouter(uc)

	w := doCreateTenantRequest(r, validTenantBody())

	require.Equal(t, http.StatusCreated, w.Code)
	require.Equal(t, 1, uc.createCalls)
	require.Contains(t, w.Body.String(), `"subdomain":"demo"`)
}

func TestCreateTenant_MissingRequiredField_Returns400WithoutCallingUseCase(t *testing.T) {
	uc := &fakeUseCase{}
	r := newCreateTenantRouter(uc)

	w := doCreateTenantRequest(r, `{"name":"Demo"}`)

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Equal(t, 0, uc.createCalls, "un body inválido nunca debe llegar al usecase")
}

// TestCreateTenant_UseCaseError_Returns500 cubre el error de negocio del
// usecase (p.ej. subdomain duplicado): el handler no distingue el tipo de
// error, todo cae al mismo 500 genérico -- ver nota al reportar este batch,
// no hay un 409 dedicado para slug duplicado como sí lo tienen otros
// endpoints (invitations, tenants get_tenant).
func TestCreateTenant_UseCaseError_Returns500(t *testing.T) {
	uc := &fakeUseCase{err: errors.New("duplicate key value violates unique constraint")}
	r := newCreateTenantRouter(uc)

	w := doCreateTenantRequest(r, validTenantBody())

	require.Equal(t, http.StatusInternalServerError, w.Code)
	require.Contains(t, w.Body.String(), "INTERNAL_ERROR")
}
