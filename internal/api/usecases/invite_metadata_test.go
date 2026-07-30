package usecases

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/tu-org/embolsadora-api/internal/domain"
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

func (f fakeRoleLookup) GetByIDForTenant(ctx context.Context, id string, tenantID uuid.UUID) (*domain.Role, error) {
	return f.role, f.err
}

func TestResolveInviteDisplayNames_ResuelveAmbosNombres(t *testing.T) {
	names := resolveInviteDisplayNames(
		context.Background(),
		fakeTenantLookup{tenant: &domain.Tenant{Name: "MRG SRL"}},
		fakeRoleLookup{role: &domain.Role{Name: "Operador"}},
		testTenantID, "operator",
	)
	assert.Equal(t, "MRG SRL", names.TenantName)
	assert.Equal(t, "Operador", names.RoleName)
}

func TestResolveInviteDisplayNames_FallaDeTenantNoPierdeElRol(t *testing.T) {
	names := resolveInviteDisplayNames(
		context.Background(),
		fakeTenantLookup{err: errors.New("db caida")},
		fakeRoleLookup{role: &domain.Role{Name: "Operador"}},
		testTenantID, "operator",
	)
	assert.Empty(t, names.TenantName)
	assert.Equal(t, "Operador", names.RoleName, "una falla de tenant no puede arrastrar al rol")
}

func TestResolveInviteDisplayNames_FallaDeRolNoPierdeElTenant(t *testing.T) {
	names := resolveInviteDisplayNames(
		context.Background(),
		fakeTenantLookup{tenant: &domain.Tenant{Name: "MRG SRL"}},
		fakeRoleLookup{err: errors.New("db caida")},
		testTenantID, "operator",
	)
	assert.Equal(t, "MRG SRL", names.TenantName)
	assert.Empty(t, names.RoleName)
}

func TestResolveInviteDisplayNames_TenantIDInvalidoDevuelveVacio(t *testing.T) {
	names := resolveInviteDisplayNames(
		context.Background(),
		fakeTenantLookup{tenant: &domain.Tenant{Name: "MRG SRL"}},
		fakeRoleLookup{role: &domain.Role{Name: "Operador"}},
		"no-es-un-uuid", "operator",
	)
	assert.Empty(t, names.TenantName)
	assert.Empty(t, names.RoleName)
}

func TestResolveInviteDisplayNames_RoleIDVacioNoConsultaElRepo(t *testing.T) {
	names := resolveInviteDisplayNames(
		context.Background(),
		fakeTenantLookup{tenant: &domain.Tenant{Name: "MRG SRL"}},
		fakeRoleLookup{err: errors.New("no deberia llamarse")},
		testTenantID, "",
	)
	assert.Equal(t, "MRG SRL", names.TenantName)
	assert.Empty(t, names.RoleName)
}
