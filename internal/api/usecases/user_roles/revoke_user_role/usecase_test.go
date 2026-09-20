package revoke_user_role

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

	revokeResult *domain.UserTenantRole
	revokeErr    error

	revokeCalls []uuid.UUID
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
	return nil, nil
}
func (f *fakeRepo) Revoke(_ context.Context, id uuid.UUID, _ uuid.UUID, _ bool) (*domain.UserTenantRole, error) {
	f.revokeCalls = append(f.revokeCalls, id)
	return f.revokeResult, f.revokeErr
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

func TestExecute_AsignacionInexistente(t *testing.T) {
	repo := &fakeRepo{findByIDResult: nil, findByIDErr: nil}
	uc := NewUseCase(repo)

	_, err := uc.Execute(context.Background(), uuid.New(), uuid.New(), false)

	assert.ErrorIs(t, err, domain.ErrAssignmentNotFound)
}

func TestExecute_AsignacionDeOtroTenantMismoErrorQueInexistente(t *testing.T) {
	tenantID, otroTenantID := uuid.New(), uuid.New()
	repo := &fakeRepo{findByIDResult: &domain.UserTenantRole{ID: uuid.New(), TenantID: otroTenantID}}
	uc := NewUseCase(repo)

	_, err := uc.Execute(context.Background(), uuid.New(), tenantID, false)

	assert.ErrorIs(t, err, domain.ErrAssignmentNotFound, "misma respuesta que inexistente, para no revelar que la asignación existe en otro tenant")
	assert.Empty(t, repo.revokeCalls, "una asignación de otro tenant no debe llegar a Revoke")
}

func TestExecute_ErrorDeFindByIDPropaga(t *testing.T) {
	dbErr := errors.New("db down")
	repo := &fakeRepo{findByIDErr: dbErr}
	uc := NewUseCase(repo)

	_, err := uc.Execute(context.Background(), uuid.New(), uuid.New(), false)

	assert.ErrorIs(t, err, dbErr)
}

func TestExecute_ErrorDeRevokePropaga(t *testing.T) {
	tenantID := uuid.New()
	repo := &fakeRepo{
		findByIDResult: &domain.UserTenantRole{ID: uuid.New(), TenantID: tenantID},
		revokeErr:      errors.New("db down"),
	}
	uc := NewUseCase(repo)

	_, err := uc.Execute(context.Background(), uuid.New(), tenantID, false)

	assert.Error(t, err)
}

func TestExecute_RevokeDevuelveNilSinErrorEsAssignmentNotFound(t *testing.T) {
	tenantID := uuid.New()
	repo := &fakeRepo{
		findByIDResult: &domain.UserTenantRole{ID: uuid.New(), TenantID: tenantID},
		revokeResult:   nil, revokeErr: nil,
	}
	uc := NewUseCase(repo)

	_, err := uc.Execute(context.Background(), uuid.New(), tenantID, false)

	assert.ErrorIs(t, err, domain.ErrAssignmentNotFound, "el guard de identidad de plataforma en SQL vuelve como (nil,nil), se mapea al mismo 404")
}

func TestExecute_Feliz(t *testing.T) {
	tenantID := uuid.New()
	want := &domain.UserTenantRole{ID: uuid.New(), TenantID: tenantID, Status: domain.UserRoleStatusRevoked}
	repo := &fakeRepo{
		findByIDResult: &domain.UserTenantRole{ID: want.ID, TenantID: tenantID},
		revokeResult:   want,
	}
	uc := NewUseCase(repo)

	got, err := uc.Execute(context.Background(), want.ID, tenantID, false)

	assert.NoError(t, err)
	assert.Equal(t, want, got)
}
