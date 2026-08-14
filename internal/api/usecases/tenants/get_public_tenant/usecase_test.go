package get_public_tenant

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tu-org/embolsadora-api/internal/domain"
)

type fakeRepo struct {
	byID        map[uuid.UUID]*domain.Tenant
	bySubdomain map[string]*domain.Tenant
}

func (f *fakeRepo) Create(ctx context.Context, tenant *domain.Tenant) error { return nil }
func (f *fakeRepo) FindAll(ctx context.Context) ([]domain.Tenant, error)    { return nil, nil }
func (f *fakeRepo) Update(ctx context.Context, tenant *domain.Tenant) error { return nil }
func (f *fakeRepo) Delete(ctx context.Context, id uuid.UUID) error          { return nil }
func (f *fakeRepo) FindByID(ctx context.Context, id uuid.UUID) (*domain.Tenant, error) {
	return f.byID[id], nil
}
func (f *fakeRepo) FindBySubdomain(ctx context.Context, subdomain string) (*domain.Tenant, error) {
	return f.bySubdomain[subdomain], nil
}

func activeTenant(id uuid.UUID) *domain.Tenant {
	return &domain.Tenant{ID: id, Subdomain: "cordoba", Name: "Cordoba SA", IsActive: true, CreatedAt: time.Now(), UpdatedAt: time.Now()}
}

func TestExecute_ByUUID_Found(t *testing.T) {
	id := uuid.New()
	repo := &fakeRepo{byID: map[uuid.UUID]*domain.Tenant{id: activeTenant(id)}}
	uc := NewUseCase(repo)

	tenant, err := uc.Execute(context.Background(), id.String())
	require.NoError(t, err)
	assert.Equal(t, id, tenant.ID)
}

func TestExecute_BySubdomain_Found(t *testing.T) {
	id := uuid.New()
	repo := &fakeRepo{bySubdomain: map[string]*domain.Tenant{"cordoba": activeTenant(id)}}
	uc := NewUseCase(repo)

	tenant, err := uc.Execute(context.Background(), "cordoba")
	require.NoError(t, err)
	assert.Equal(t, id, tenant.ID)
}

func TestExecute_BySubdomain_MixedCase_Found(t *testing.T) {
	id := uuid.New()
	repo := &fakeRepo{bySubdomain: map[string]*domain.Tenant{"cordoba": activeTenant(id)}}
	uc := NewUseCase(repo)

	tenant, err := uc.Execute(context.Background(), "CorDoba")
	require.NoError(t, err)
	assert.Equal(t, id, tenant.ID)
}

func TestExecute_NotFound(t *testing.T) {
	repo := &fakeRepo{}
	uc := NewUseCase(repo)

	_, err := uc.Execute(context.Background(), "no-such-tenant")
	assert.ErrorIs(t, err, ErrTenantNotFound)
}

func TestExecute_Inactive_TreatedAsNotFound(t *testing.T) {
	id := uuid.New()
	inactive := activeTenant(id)
	inactive.IsActive = false
	repo := &fakeRepo{byID: map[uuid.UUID]*domain.Tenant{id: inactive}}
	uc := NewUseCase(repo)

	_, err := uc.Execute(context.Background(), id.String())
	assert.ErrorIs(t, err, ErrTenantNotFound)
}
