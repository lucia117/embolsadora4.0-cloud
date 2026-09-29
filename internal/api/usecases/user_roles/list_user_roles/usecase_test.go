package list_user_roles

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tu-org/embolsadora-api/internal/domain"
)

type findByTenantCall struct {
	tenantID      uuid.UUID
	status        *string
	includeGlobal bool
}

type fakeRepo struct {
	findByTenantResult []domain.UserTenantRoleDetail
	findByTenantErr    error
	lastIncludeGlobal  bool
	findByTenantCalls  []findByTenantCall
}

func (f *fakeRepo) FindByTenant(_ context.Context, tenantID uuid.UUID, status *string, includeGlobal bool) ([]domain.UserTenantRoleDetail, error) {
	f.lastIncludeGlobal = includeGlobal
	f.findByTenantCalls = append(f.findByTenantCalls, findByTenantCall{tenantID, status, includeGlobal})
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

	tenantID := uuid.New()
	got, err := uc.Execute(context.Background(), tenantID, nil, true)

	assert.NoError(t, err)
	assert.Equal(t, want, got)
	assert.True(t, repo.lastIncludeGlobal)

	require.Len(t, repo.findByTenantCalls, 1)
	assert.Equal(t, tenantID, repo.findByTenantCalls[0].tenantID, "el tenantID pasado a Execute debe llegar sin alterar al repo")
}

func TestExecute_ErrorDeRepoPropaga(t *testing.T) {
	repo := &fakeRepo{findByTenantErr: errors.New("db down")}
	uc := NewUseCase(repo)

	_, err := uc.Execute(context.Background(), uuid.New(), nil, false)

	assert.Error(t, err)
}
