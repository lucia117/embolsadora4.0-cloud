package update_user_role

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/tu-org/embolsadora-api/internal/domain"
)

type fakeRepo struct {
	findByIDResult *domain.UserTenantRole
	findByIDErr    error

	updateResult *domain.UserTenantRole
	updateErr    error
}

func (f *fakeRepo) FindByTenant(context.Context, uuid.UUID, *string, bool) ([]domain.UserTenantRoleDetail, error) {
	return nil, nil
}
func (f *fakeRepo) FindByID(context.Context, uuid.UUID, bool) (*domain.UserTenantRole, error) {
	return f.findByIDResult, f.findByIDErr
}
func (f *fakeRepo) Create(context.Context, *domain.UserTenantRole, bool) (*domain.UserTenantRole, error) {
	return nil, nil
}
func (f *fakeRepo) Update(context.Context, *domain.UserTenantRole, bool) (*domain.UserTenantRole, error) {
	return f.updateResult, f.updateErr
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

type fakeRolesRepo struct {
	getByIDForTenantErr error
}

func (f *fakeRolesRepo) List(context.Context, uuid.UUID, bool) ([]*domain.Role, error) { return nil, nil }
func (f *fakeRolesRepo) GetByIDForTenant(context.Context, string, uuid.UUID, bool) (*domain.Role, error) {
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

func TestExecute_AsignacionInexistente(t *testing.T) {
	repo := &fakeRepo{}
	uc := NewUseCase(repo, &fakeRolesRepo{})

	_, err := uc.Execute(context.Background(), uuid.New(), uuid.New(), "operario", false)

	assert.ErrorIs(t, err, domain.ErrAssignmentNotFound)
}

func TestExecute_AsignacionDeOtroTenant(t *testing.T) {
	tenantID, otroTenantID := uuid.New(), uuid.New()
	repo := &fakeRepo{findByIDResult: &domain.UserTenantRole{ID: uuid.New(), TenantID: otroTenantID}}
	uc := NewUseCase(repo, &fakeRolesRepo{})

	_, err := uc.Execute(context.Background(), uuid.New(), tenantID, "operario", false)

	assert.ErrorIs(t, err, domain.ErrAssignmentNotFound)
}

func TestExecute_RolNuevoNoAsignableRechazaSinLlamarUpdate(t *testing.T) {
	tenantID := uuid.New()
	repo := &fakeRepo{findByIDResult: &domain.UserTenantRole{ID: uuid.New(), TenantID: tenantID}}
	roleRepo := &fakeRolesRepo{getByIDForTenantErr: domain.ErrRoleNotFound}
	uc := NewUseCase(repo, roleRepo)

	_, err := uc.Execute(context.Background(), uuid.New(), tenantID, "super_admin", false)

	assert.ErrorIs(t, err, domain.ErrInvalidRoleID)
}

func TestExecute_ErrorDeRepoUpdatePropaga(t *testing.T) {
	tenantID := uuid.New()
	repo := &fakeRepo{
		findByIDResult: &domain.UserTenantRole{ID: uuid.New(), TenantID: tenantID},
		updateErr:      errors.New("db down"),
	}
	uc := NewUseCase(repo, &fakeRolesRepo{})

	_, err := uc.Execute(context.Background(), uuid.New(), tenantID, "operario", false)

	assert.Error(t, err)
}

func TestExecute_UpdateDevuelveNilSinErrorEsAssignmentNotFound(t *testing.T) {
	tenantID := uuid.New()
	repo := &fakeRepo{
		findByIDResult: &domain.UserTenantRole{ID: uuid.New(), TenantID: tenantID},
		updateResult:   nil, updateErr: nil,
	}
	uc := NewUseCase(repo, &fakeRolesRepo{})

	_, err := uc.Execute(context.Background(), uuid.New(), tenantID, "operario", false)

	assert.ErrorIs(t, err, domain.ErrAssignmentNotFound, "el guard de identidad de plataforma en SQL vuelve como (nil,nil)")
}

func TestExecute_Feliz(t *testing.T) {
	tenantID := uuid.New()
	id := uuid.New()
	want := &domain.UserTenantRole{ID: id, TenantID: tenantID, RoleID: strPtr("operario")}
	repo := &fakeRepo{
		findByIDResult: &domain.UserTenantRole{ID: id, TenantID: tenantID},
		updateResult:   want,
	}
	uc := NewUseCase(repo, &fakeRolesRepo{})

	got, err := uc.Execute(context.Background(), id, tenantID, "operario", false)

	assert.NoError(t, err)
	assert.Equal(t, want, got)
}

func strPtr(s string) *string { return &s }
