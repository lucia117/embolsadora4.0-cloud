package usecases

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/tu-org/embolsadora-api/internal/domain"
	"github.com/tu-org/embolsadora-api/internal/platform"
)

const testTenantID = "11b36b85-033d-4bb3-9e31-4c92161887c0"

type fakeTenantLookup struct {
	tenant *domain.Tenant
	err    error
}

func (f fakeTenantLookup) FindByID(ctx context.Context, id uuid.UUID) (*domain.Tenant, error) {
	return f.tenant, f.err
}

type fakeRoleLookup struct {
	role *domain.Role
	err  error
}

func (f fakeRoleLookup) GetByIDForTenant(ctx context.Context, id string, tenantID uuid.UUID, includeGlobal bool) (*domain.Role, error) {
	return f.role, f.err
}

func TestResolveInviteDisplayNames_ResuelveAmbosNombres(t *testing.T) {
	names := resolveInviteDisplayNames(
		context.Background(),
		fakeTenantLookup{tenant: &domain.Tenant{Name: "MRG SRL"}},
		fakeRoleLookup{role: &domain.Role{Name: "Operador"}},
		testTenantID, "operator", false,
	)
	assert.Equal(t, "MRG SRL", names.TenantName)
	assert.Equal(t, "Operador", names.RoleName)
}

func TestResolveInviteDisplayNames_FallaDeTenantNoPierdeElRol(t *testing.T) {
	names := resolveInviteDisplayNames(
		context.Background(),
		fakeTenantLookup{err: errors.New("db caida")},
		fakeRoleLookup{role: &domain.Role{Name: "Operador"}},
		testTenantID, "operator", false,
	)
	assert.Empty(t, names.TenantName)
	assert.Equal(t, "Operador", names.RoleName, "una falla de tenant no puede arrastrar al rol")
}

func TestResolveInviteDisplayNames_FallaDeRolNoPierdeElTenant(t *testing.T) {
	names := resolveInviteDisplayNames(
		context.Background(),
		fakeTenantLookup{tenant: &domain.Tenant{Name: "MRG SRL"}},
		fakeRoleLookup{err: errors.New("db caida")},
		testTenantID, "operator", false,
	)
	assert.Equal(t, "MRG SRL", names.TenantName)
	assert.Empty(t, names.RoleName)
}

func TestResolveInviteDisplayNames_TenantIDInvalidoDevuelveVacio(t *testing.T) {
	names := resolveInviteDisplayNames(
		context.Background(),
		fakeTenantLookup{tenant: &domain.Tenant{Name: "MRG SRL"}},
		fakeRoleLookup{role: &domain.Role{Name: "Operador"}},
		"no-es-un-uuid", "operator", false,
	)
	assert.Empty(t, names.TenantName)
	assert.Empty(t, names.RoleName)
}

func TestResolveInviteDisplayNames_RoleIDVacioNoConsultaElRepo(t *testing.T) {
	names := resolveInviteDisplayNames(
		context.Background(),
		fakeTenantLookup{tenant: &domain.Tenant{Name: "MRG SRL"}},
		fakeRoleLookup{err: errors.New("no deberia llamarse")},
		testTenantID, "", false,
	)
	assert.Equal(t, "MRG SRL", names.TenantName)
	assert.Empty(t, names.RoleName)
}

// TestResolveInviteDisplayNames_TenantNoEncontradoNoPierdeElRol cubre el caso
// (t=nil, err=nil) que devuelve FindByID cuando el tenant_id es valido pero no
// existe (pgx.ErrNoRows mapeado a nil,nil en tenants.repository.go). Es el
// disparador mas realista de este best-effort: un tenant_id invalido/borrado,
// no una caida de base de datos.
func TestResolveInviteDisplayNames_TenantNoEncontradoNoPierdeElRol(t *testing.T) {
	names := resolveInviteDisplayNames(
		context.Background(),
		fakeTenantLookup{},
		fakeRoleLookup{role: &domain.Role{Name: "Operador"}},
		testTenantID, "operator", false,
	)
	assert.Empty(t, names.TenantName)
	assert.Equal(t, "Operador", names.RoleName, "un tenant no encontrado no puede arrastrar al rol")
}

// TestResolveInviteDisplayNames_AmbosLookupsNilNoPanica ejercita los guards
// `tenants != nil` / `roles != nil`: pasar nil en ambos no debe panickear y
// debe devolver ambos nombres vacios.
func TestResolveInviteDisplayNames_AmbosLookupsNilNoPanica(t *testing.T) {
	names := resolveInviteDisplayNames(
		context.Background(),
		nil,
		nil,
		testTenantID, "operator", false,
	)
	assert.Empty(t, names.TenantName)
	assert.Empty(t, names.RoleName)
}

// TestCallbackURL_UsaElOriginDelContexto cubre la rama que da sentido a toda
// la feature: si el middleware valido un origin y lo dejo en el contexto, el
// link del mail se arma con ese origin y no con el default configurado.
func TestCallbackURL_UsaElOriginDelContexto(t *testing.T) {
	ctx := platform.WithAppBaseURL(context.Background(), "http://localhost:3000")
	assert.Equal(t, "http://localhost:3000/s/"+testTenantID+"/auth/callback",
		callbackURL(ctx, "https://embolsadora.site", testTenantID))
}

// TestCallbackURL_SinContextoCaeAlFallback cubre la otra rama: sin origin en
// el contexto —header ausente o rechazado por la allow-list— se usa el
// APP_BASE_URL configurado.
func TestCallbackURL_SinContextoCaeAlFallback(t *testing.T) {
	assert.Equal(t, "https://embolsadora.site/s/"+testTenantID+"/auth/callback",
		callbackURL(context.Background(), "https://embolsadora.site", testTenantID))
}
