package create_tenant

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/tu-org/embolsadora-api/internal/domain"
)

type fakeRepo struct {
	createErr   error
	createCalls []*domain.Tenant
}

func (f *fakeRepo) Create(ctx context.Context, tenant *domain.Tenant) error {
	f.createCalls = append(f.createCalls, tenant)
	return f.createErr
}
func (f *fakeRepo) FindAll(ctx context.Context) ([]domain.Tenant, error) { return nil, nil }
func (f *fakeRepo) FindByID(ctx context.Context, id uuid.UUID) (*domain.Tenant, error) {
	return nil, nil
}
func (f *fakeRepo) FindBySubdomain(ctx context.Context, subdomain string) (*domain.Tenant, error) {
	return nil, nil
}
func (f *fakeRepo) Update(ctx context.Context, tenant *domain.Tenant) error { return nil }
func (f *fakeRepo) Delete(ctx context.Context, id uuid.UUID) error          { return nil }

func TestCreate_Feliz(t *testing.T) {
	repo := &fakeRepo{}
	uc := NewUseCase(repo)

	tenant := &domain.Tenant{ID: uuid.New(), Name: "Acme"}
	err := uc.Create(context.Background(), tenant)

	assert.NoError(t, err)
	assert.Len(t, repo.createCalls, 1)
	assert.Same(t, tenant, repo.createCalls[0], "debe pasar el mismo puntero intacto al repo")
}

func TestCreate_ErrorDeRepo(t *testing.T) {
	repo := &fakeRepo{createErr: errors.New("subdomain duplicado")}
	uc := NewUseCase(repo)

	err := uc.Create(context.Background(), &domain.Tenant{})

	assert.Error(t, err)
}
