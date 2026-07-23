package get_all_tenants

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/tu-org/embolsadora-api/internal/domain"
)

type fakeRepo struct {
	all []domain.Tenant
	one *domain.Tenant
}

func (f *fakeRepo) Create(ctx context.Context, tenant *domain.Tenant) error { return nil }
func (f *fakeRepo) FindAll(ctx context.Context) ([]domain.Tenant, error)    { return f.all, nil }
func (f *fakeRepo) FindByID(ctx context.Context, id uuid.UUID) (*domain.Tenant, error) {
	if f.one == nil {
		return nil, nil
	}
	t := *f.one
	t.ID = id
	return &t, nil
}
func (f *fakeRepo) Update(ctx context.Context, tenant *domain.Tenant) error { return nil }
func (f *fakeRepo) Delete(ctx context.Context, id uuid.UUID) error         { return nil }

func TestExecute_NoScopeReturnsAll(t *testing.T) {
	repo := &fakeRepo{all: []domain.Tenant{{Name: "A"}, {Name: "B"}, {Name: "C"}}}
	uc := NewUseCase(repo)

	result, err := uc.Execute(context.Background(), nil)

	assert.NoError(t, err)
	assert.Len(t, result, 3)
}

func TestExecute_WithScopeReturnsOnlyThatTenant(t *testing.T) {
	scopeID := uuid.New()
	repo := &fakeRepo{
		all: []domain.Tenant{{Name: "A"}, {Name: "B"}},
		one: &domain.Tenant{Name: "Own Tenant"},
	}
	uc := NewUseCase(repo)

	result, err := uc.Execute(context.Background(), &scopeID)

	assert.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "Own Tenant", result[0].Name)
	assert.Equal(t, scopeID, result[0].ID)
}
