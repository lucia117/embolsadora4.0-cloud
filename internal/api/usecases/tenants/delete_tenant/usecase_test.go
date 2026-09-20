package delete_tenant

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/tu-org/embolsadora-api/internal/domain"
)

type fakeRepo struct {
	deleteErr   error
	deleteCalls []uuid.UUID
}

func (f *fakeRepo) Create(ctx context.Context, tenant *domain.Tenant) error { return nil }
func (f *fakeRepo) FindAll(ctx context.Context) ([]domain.Tenant, error)   { return nil, nil }
func (f *fakeRepo) FindByID(ctx context.Context, id uuid.UUID) (*domain.Tenant, error) {
	return nil, nil
}
func (f *fakeRepo) FindBySubdomain(ctx context.Context, subdomain string) (*domain.Tenant, error) {
	return nil, nil
}
func (f *fakeRepo) Update(ctx context.Context, tenant *domain.Tenant) error { return nil }
func (f *fakeRepo) Delete(ctx context.Context, id uuid.UUID) error {
	f.deleteCalls = append(f.deleteCalls, id)
	return f.deleteErr
}

func TestDelete_Feliz(t *testing.T) {
	repo := &fakeRepo{}
	uc := NewUseCase(repo)

	id := uuid.New()
	err := uc.Delete(context.Background(), id)

	assert.NoError(t, err)
	assert.Equal(t, []uuid.UUID{id}, repo.deleteCalls)
}

func TestDelete_NoEncontradoSeMapeaAErrTenantNotFoundLocal(t *testing.T) {
	repo := &fakeRepo{deleteErr: domain.ErrNotFound}
	uc := NewUseCase(repo)

	err := uc.Delete(context.Background(), uuid.New())

	assert.ErrorIs(t, err, ErrTenantNotFound)
	assert.NotErrorIs(t, err, domain.ErrNotFound, "el caller debe ver el error local del paquete, no domain.ErrNotFound directo")
}

func TestDelete_OtroErrorPasaTalCual(t *testing.T) {
	dbErr := errors.New("constraint violation")
	repo := &fakeRepo{deleteErr: dbErr}
	uc := NewUseCase(repo)

	err := uc.Delete(context.Background(), uuid.New())

	assert.ErrorIs(t, err, dbErr)
	assert.NotErrorIs(t, err, ErrTenantNotFound)
}
