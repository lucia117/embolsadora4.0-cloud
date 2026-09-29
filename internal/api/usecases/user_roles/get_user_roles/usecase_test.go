package get_user_roles

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/tu-org/embolsadora-api/internal/domain"
)

type fakeRepo struct {
	findByUserResult []domain.UserRoleWithContext
	findByUserErr    error
	lastCall         struct {
		userID, tenantID           uuid.UUID
		crossTenant, includeGlobal bool
	}
}

func (f *fakeRepo) FindByTenant(context.Context, uuid.UUID, *string, bool) ([]domain.UserTenantRoleDetail, error) {
	return nil, nil
}
func (f *fakeRepo) FindByID(context.Context, uuid.UUID, bool) (*domain.UserTenantRole, error) {
	return nil, nil
}
func (f *fakeRepo) Create(context.Context, *domain.UserTenantRole, bool) (*domain.UserTenantRole, error) {
	return nil, nil
}
func (f *fakeRepo) Update(context.Context, *domain.UserTenantRole, bool) (*domain.UserTenantRole, error) {
	return nil, nil
}
func (f *fakeRepo) Revoke(context.Context, uuid.UUID, uuid.UUID, bool) (*domain.UserTenantRole, error) {
	return nil, nil
}
func (f *fakeRepo) BulkCreate(context.Context, []domain.UserTenantRole, bool) ([]domain.UserTenantRole, error) {
	return nil, nil
}
func (f *fakeRepo) FindByUser(_ context.Context, userID, tenantID uuid.UUID, crossTenant, includeGlobal bool) ([]domain.UserRoleWithContext, error) {
	f.lastCall.userID, f.lastCall.tenantID = userID, tenantID
	f.lastCall.crossTenant, f.lastCall.includeGlobal = crossTenant, includeGlobal
	return f.findByUserResult, f.findByUserErr
}
func (f *fakeRepo) UpdateStatus(context.Context, uuid.UUID, uuid.UUID, domain.UserRoleStatus, bool) (*domain.UserTenantRole, error) {
	return nil, nil
}

func TestExecute_FelizPasaLosCuatroParametros(t *testing.T) {
	want := []domain.UserRoleWithContext{{TenantName: "Acme"}}
	repo := &fakeRepo{findByUserResult: want}
	uc := NewUseCase(repo)

	userID, tenantID := uuid.New(), uuid.New()
	got, err := uc.Execute(context.Background(), Query{
		UserID: userID, TenantID: tenantID, CrossTenant: true, IncludeGlobal: true,
	})

	assert.NoError(t, err)
	assert.Equal(t, want, got)
	assert.Equal(t, userID, repo.lastCall.userID)
	assert.Equal(t, tenantID, repo.lastCall.tenantID)
	assert.True(t, repo.lastCall.crossTenant)
	assert.True(t, repo.lastCall.includeGlobal)
}

func TestExecute_ErrorDeRepoPropaga(t *testing.T) {
	repo := &fakeRepo{findByUserErr: errors.New("db down")}
	uc := NewUseCase(repo)

	_, err := uc.Execute(context.Background(), Query{UserID: uuid.New(), TenantID: uuid.New()})

	assert.Error(t, err)
}
