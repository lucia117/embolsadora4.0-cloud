package get_tenant

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/tu-org/embolsadora-api/internal/domain"
)

type fakeRepo struct {
	findByIDResult *domain.Tenant
	findByIDErr    error
}

func (f *fakeRepo) Create(ctx context.Context, tenant *domain.Tenant) error { return nil }
func (f *fakeRepo) FindAll(ctx context.Context) ([]domain.Tenant, error)   { return nil, nil }
func (f *fakeRepo) FindByID(ctx context.Context, id uuid.UUID) (*domain.Tenant, error) {
	return f.findByIDResult, f.findByIDErr
}
func (f *fakeRepo) FindBySubdomain(ctx context.Context, subdomain string) (*domain.Tenant, error) {
	return nil, nil
}
func (f *fakeRepo) Update(ctx context.Context, tenant *domain.Tenant) error { return nil }
func (f *fakeRepo) Delete(ctx context.Context, id uuid.UUID) error         { return nil }

func TestExecute_Feliz(t *testing.T) {
	want := &domain.Tenant{ID: uuid.New(), Name: "Acme"}
	repo := &fakeRepo{findByIDResult: want}
	uc := NewUseCase(repo)

	got, err := uc.Execute(context.Background(), want.ID)

	assert.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestExecute_TenantNilSinErrorEsNotFound(t *testing.T) {
	repo := &fakeRepo{findByIDResult: nil, findByIDErr: nil}
	uc := NewUseCase(repo)

	_, err := uc.Execute(context.Background(), uuid.New())

	assert.ErrorIs(t, err, ErrTenantNotFound)
}

func TestExecute_ErrorDeRepoPropaga(t *testing.T) {
	dbErr := errors.New("db down")
	repo := &fakeRepo{findByIDErr: dbErr}
	uc := NewUseCase(repo)

	_, err := uc.Execute(context.Background(), uuid.New())

	assert.ErrorIs(t, err, dbErr)
}
