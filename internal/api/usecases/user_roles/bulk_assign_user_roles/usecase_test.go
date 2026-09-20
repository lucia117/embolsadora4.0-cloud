package bulk_assign_user_roles

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tu-org/embolsadora-api/internal/domain"
)

type fakeUserRoleRepo struct {
	bulkCreateResult []domain.UserTenantRole
	bulkCreateErr    error
	bulkCreateCalls  [][]domain.UserTenantRole
}

func (f *fakeUserRoleRepo) FindByTenant(context.Context, uuid.UUID, *string, bool) ([]domain.UserTenantRoleDetail, error) {
	return nil, nil
}
func (f *fakeUserRoleRepo) FindByID(context.Context, uuid.UUID, bool) (*domain.UserTenantRole, error) {
	return nil, nil
}
func (f *fakeUserRoleRepo) Create(context.Context, *domain.UserTenantRole, bool) (*domain.UserTenantRole, error) {
	return nil, nil
}
func (f *fakeUserRoleRepo) Update(context.Context, *domain.UserTenantRole, bool) (*domain.UserTenantRole, error) {
	return nil, nil
}
func (f *fakeUserRoleRepo) Revoke(context.Context, uuid.UUID, uuid.UUID, bool) (*domain.UserTenantRole, error) {
	return nil, nil
}
func (f *fakeUserRoleRepo) BulkCreate(_ context.Context, utrs []domain.UserTenantRole, includeGlobal bool) ([]domain.UserTenantRole, error) {
	f.bulkCreateCalls = append(f.bulkCreateCalls, utrs)
	if f.bulkCreateErr != nil {
		return nil, f.bulkCreateErr
	}
	if f.bulkCreateResult != nil {
		return f.bulkCreateResult, nil
	}
	return utrs, nil
}
func (f *fakeUserRoleRepo) FindByUser(context.Context, uuid.UUID, uuid.UUID, bool, bool) ([]domain.UserRoleWithContext, error) {
	return nil, nil
}
func (f *fakeUserRoleRepo) UpdateStatus(context.Context, uuid.UUID, uuid.UUID, domain.UserRoleStatus, bool) (*domain.UserTenantRole, error) {
	return nil, nil
}

type getByIDForTenantCall struct {
	roleID        string
	tenantID      uuid.UUID
	includeGlobal bool
}

type fakeRolesRepo struct {
	getByIDForTenantErr   error
	getByIDForTenantCalls []getByIDForTenantCall
}

func (f *fakeRolesRepo) List(context.Context, uuid.UUID, bool) ([]*domain.Role, error) {
	return nil, nil
}
func (f *fakeRolesRepo) GetByIDForTenant(_ context.Context, roleID string, tenantID uuid.UUID, includeGlobal bool) (*domain.Role, error) {
	f.getByIDForTenantCalls = append(f.getByIDForTenantCalls, getByIDForTenantCall{roleID, tenantID, includeGlobal})
	if f.getByIDForTenantErr != nil {
		return nil, f.getByIDForTenantErr
	}
	return &domain.Role{ID: "operario"}, nil
}
func (f *fakeRolesRepo) CountCustomByTenant(context.Context, uuid.UUID) (int, error) { return 0, nil }
func (f *fakeRolesRepo) Create(context.Context, *domain.Role) error                  { return nil }
func (f *fakeRolesRepo) Update(context.Context, *domain.Role) error                  { return nil }
func (f *fakeRolesRepo) SoftDelete(context.Context, string) error                    { return nil }
func (f *fakeRolesRepo) CountActiveAssignments(context.Context, string) (int, error) { return 0, nil }

func TestExecute_RolNoAsignableRechazaSinLlegarAlRepo(t *testing.T) {
	urRepo := &fakeUserRoleRepo{}
	roleRepo := &fakeRolesRepo{getByIDForTenantErr: domain.ErrRoleNotFound}
	uc := NewUseCase(urRepo, roleRepo)

	_, err := uc.Execute(context.Background(), BulkAssignRequest{
		UserIDs: []uuid.UUID{uuid.New()}, TenantID: uuid.New(), RoleID: "super_admin",
	})

	require.ErrorIs(t, err, domain.ErrInvalidRoleID)
	assert.Empty(t, urRepo.bulkCreateCalls, "una escalada bloqueada no debe llegar a BulkCreate")
}

func TestExecute_FelizArmaUnUTRPorUsuarioConElMismoRolYTenant(t *testing.T) {
	urRepo := &fakeUserRoleRepo{}
	roleRepo := &fakeRolesRepo{}
	uc := NewUseCase(urRepo, roleRepo)

	tenantID := uuid.New()
	userIDs := []uuid.UUID{uuid.New(), uuid.New(), uuid.New()}

	got, err := uc.Execute(context.Background(), BulkAssignRequest{
		UserIDs: userIDs, TenantID: tenantID, RoleID: "operario", IncludeGlobal: false,
	})

	require.NoError(t, err)
	assert.Equal(t, 3, got.Assigned)
	require.Len(t, urRepo.bulkCreateCalls, 1)
	batch := urRepo.bulkCreateCalls[0]
	require.Len(t, batch, 3)
	for i, utr := range batch {
		assert.Equal(t, userIDs[i], utr.UserID)
		assert.Equal(t, tenantID, utr.TenantID)
		assert.Equal(t, "operario", *utr.RoleID)
		assert.Equal(t, domain.UserRoleStatusActive, utr.Status)
	}
	assert.Equal(t, batch[0].AssignedAt, batch[1].AssignedAt, "todo el batch comparte el mismo timestamp")

	require.Len(t, roleRepo.getByIDForTenantCalls, 1)
	assert.Equal(t, "operario", roleRepo.getByIDForTenantCalls[0].roleID, "EnsureAssignable debe validar el rol pasado en el request")
	assert.Equal(t, tenantID, roleRepo.getByIDForTenantCalls[0].tenantID, "EnsureAssignable debe validar contra el tenant del caller")
}

func TestExecute_ErrorDeBulkCreatePropaga(t *testing.T) {
	urRepo := &fakeUserRoleRepo{bulkCreateErr: errors.New("tx rollback")}
	roleRepo := &fakeRolesRepo{}
	uc := NewUseCase(urRepo, roleRepo)

	_, err := uc.Execute(context.Background(), BulkAssignRequest{
		UserIDs: []uuid.UUID{uuid.New()}, TenantID: uuid.New(), RoleID: "operario",
	})

	assert.Error(t, err)
}
