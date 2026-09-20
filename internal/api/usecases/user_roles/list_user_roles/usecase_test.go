package list_user_roles

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/tu-org/embolsadora-api/internal/domain"
)

type fakeRepo struct {
	findByTenantResult []domain.UserTenantRoleDetail
	findByTenantErr    error
	lastIncludeGlobal  bool
}

func (f *fakeRepo) FindByTenant(_ context.Context, _ uuid.UUID, _ *string, includeGlobal bool) ([]domain.UserTenantRoleDetail, error) {
	f.lastIncludeGlobal = includeGlobal
	return f.findByTenantResult, f.findByTenantErr
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
func (f *fakeRepo) FindByUser(context.Context, uuid.UUID, uuid.UUID, bool, bool) ([]domain.UserRoleWithContext, error) {
	return nil, nil
}
func (f *fakeRepo) UpdateStatus(context.Context, uuid.UUID, uuid.UUID, domain.UserRoleStatus, bool) (*domain.UserTenantRole, error) {
	return nil, nil
}

func TestExecute_FelizPasaIncludeGlobal(t *testing.T) {
	want := []domain.UserTenantRoleDetail{{RoleName: "admin"}}
	repo := &fakeRepo{findByTenantResult: want}
	uc := NewUseCase(repo)

	got, err := uc.Execute(context.Background(), uuid.New(), nil, true)

	assert.NoError(t, err)
	assert.Equal(t, want, got)
	assert.True(t, repo.lastIncludeGlobal)
}

func TestExecute_ErrorDeRepoPropaga(t *testing.T) {
	repo := &fakeRepo{findByTenantErr: errors.New("db down")}
	uc := NewUseCase(repo)

	_, err := uc.Execute(context.Background(), uuid.New(), nil, false)

	assert.Error(t, err)
}
