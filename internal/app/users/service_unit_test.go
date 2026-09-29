package users_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	app "github.com/tu-org/embolsadora-api/internal/app/users"
	"github.com/tu-org/embolsadora-api/internal/domain"
	domainUsers "github.com/tu-org/embolsadora-api/internal/domain/users"
)

// fakeUsersRepo implementa usersRepo.Repository.
type fakeUsersRepo struct {
	listResult []*domainUsers.User
	listTotal  int64
	listErr    error

	getResult *domainUsers.User
	getErr    error

	getWithRolesResult *domainUsers.UserWithRoles
	getWithRolesErr    error

	listPendingResult []*domainUsers.User
	listPendingErr    error

	deleteErr   error
	deleteCalls []struct{ tenantID, userID string }

	updateResult *domainUsers.User
	updateErr    error
}

func (f *fakeUsersRepo) ListByTenant(ctx context.Context, tenantID string, limit, offset int, includeGlobal bool) ([]*domainUsers.User, int64, error) {
	return f.listResult, f.listTotal, f.listErr
}
func (f *fakeUsersRepo) GetByID(ctx context.Context, tenantID, userID string, crossTenant, includeGlobal bool) (*domainUsers.User, error) {
	return f.getResult, f.getErr
}
func (f *fakeUsersRepo) GetByIDWithRoles(ctx context.Context, tenantID, userID string, crossTenant, includeGlobal bool) (*domainUsers.UserWithRoles, error) {
	return f.getWithRolesResult, f.getWithRolesErr
}
func (f *fakeUsersRepo) ListPendingByTenant(ctx context.Context, tenantID string, includeGlobal bool) ([]*domainUsers.User, error) {
	return f.listPendingResult, f.listPendingErr
}
func (f *fakeUsersRepo) Create(ctx context.Context, user *domainUsers.User) (*domainUsers.User, error) {
	return user, nil
}
func (f *fakeUsersRepo) CreateWithRole(ctx context.Context, user *domainUsers.User, utr *domain.UserTenantRole) (*domainUsers.User, error) {
	return user, nil
}
func (f *fakeUsersRepo) Update(ctx context.Context, user *domainUsers.User) (*domainUsers.User, error) {
	return f.updateResult, f.updateErr
}
func (f *fakeUsersRepo) Delete(ctx context.Context, tenantID, userID string) error {
	f.deleteCalls = append(f.deleteCalls, struct{ tenantID, userID string }{tenantID, userID})
	return f.deleteErr
}

// fakeUserRoleRepo implementa userRolesRepo.UserRoleRepository (solo UpdateStatus importa acá).
type fakeUserRoleRepo struct {
	updateStatusResult *domain.UserTenantRole
	updateStatusErr    error
	updateStatusCalls  []struct {
		userID, tenantID uuid.UUID
		status           domain.UserRoleStatus
	}
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
func (f *fakeUserRoleRepo) BulkCreate(context.Context, []domain.UserTenantRole, bool) ([]domain.UserTenantRole, error) {
	return nil, nil
}
func (f *fakeUserRoleRepo) FindByUser(context.Context, uuid.UUID, uuid.UUID, bool, bool) ([]domain.UserRoleWithContext, error) {
	return nil, nil
}
func (f *fakeUserRoleRepo) UpdateStatus(ctx context.Context, userID, tenantID uuid.UUID, status domain.UserRoleStatus, includeGlobal bool) (*domain.UserTenantRole, error) {
	f.updateStatusCalls = append(f.updateStatusCalls, struct {
		userID, tenantID uuid.UUID
		status           domain.UserRoleStatus
	}{userID, tenantID, status})
	return f.updateStatusResult, f.updateStatusErr
}

// fakeRolesRepoForUsers implementa rolesRepo.Repository (no se ejercita en estos 6
// métodos — ninguno llama EnsureAssignable — así que basta con satisfacer la interfaz).
type fakeRolesRepoForUsers struct{}

func (f *fakeRolesRepoForUsers) List(context.Context, uuid.UUID, bool) ([]*domain.Role, error) {
	return nil, nil
}
func (f *fakeRolesRepoForUsers) GetByIDForTenant(context.Context, string, uuid.UUID, bool) (*domain.Role, error) {
	return &domain.Role{}, nil
}
func (f *fakeRolesRepoForUsers) CountCustomByTenant(context.Context, uuid.UUID) (int, error) {
	return 0, nil
}
func (f *fakeRolesRepoForUsers) Create(context.Context, *domain.Role) error { return nil }
func (f *fakeRolesRepoForUsers) Update(context.Context, *domain.Role) error { return nil }
func (f *fakeRolesRepoForUsers) SoftDelete(context.Context, string) error   { return nil }
func (f *fakeRolesRepoForUsers) CountActiveAssignments(context.Context, string) (int, error) {
	return 0, nil
}

func newTestService(repo *fakeUsersRepo, urRepo *fakeUserRoleRepo) *app.Service {
	return app.NewService(repo, urRepo, &fakeRolesRepoForUsers{}, zap.NewNop())
}

func TestListUsers(t *testing.T) {
	t.Run("feliz", func(t *testing.T) {
		want := []*domainUsers.User{{ID: "u1"}}
		repo := &fakeUsersRepo{listResult: want, listTotal: 1}
		svc := newTestService(repo, &fakeUserRoleRepo{})
		got, total, err := svc.ListUsers(context.Background(), "tenant1", 10, 0, false)
		require.NoError(t, err)
		require.Equal(t, want, got)
		require.EqualValues(t, 1, total)
	})
	t.Run("error de repo", func(t *testing.T) {
		repo := &fakeUsersRepo{listErr: errors.New("db down")}
		svc := newTestService(repo, &fakeUserRoleRepo{})
		_, _, err := svc.ListUsers(context.Background(), "tenant1", 10, 0, false)
		require.Error(t, err)
	})
}

func TestListPendingUsers(t *testing.T) {
	t.Run("feliz", func(t *testing.T) {
		want := []*domainUsers.User{{ID: "u1"}}
		repo := &fakeUsersRepo{listPendingResult: want}
		svc := newTestService(repo, &fakeUserRoleRepo{})
		got, err := svc.ListPendingUsers(context.Background(), "tenant1", false)
		require.NoError(t, err)
		require.Equal(t, want, got)
	})
	t.Run("error de repo", func(t *testing.T) {
		repo := &fakeUsersRepo{listPendingErr: errors.New("db down")}
		svc := newTestService(repo, &fakeUserRoleRepo{})
		_, err := svc.ListPendingUsers(context.Background(), "tenant1", false)
		require.Error(t, err)
	})
}

func TestGetUser(t *testing.T) {
	t.Run("feliz", func(t *testing.T) {
		repo := &fakeUsersRepo{getResult: &domainUsers.User{ID: "u1"}}
		svc := newTestService(repo, &fakeUserRoleRepo{})
		got, err := svc.GetUser(context.Background(), "tenant1", "u1", false, false)
		require.NoError(t, err)
		require.Equal(t, "u1", got.ID)
	})
	t.Run("no encontrado", func(t *testing.T) {
		repo := &fakeUsersRepo{getErr: domainUsers.ErrNotFound}
		svc := newTestService(repo, &fakeUserRoleRepo{})
		_, err := svc.GetUser(context.Background(), "tenant1", "u1", false, false)
		require.ErrorIs(t, err, domainUsers.ErrNotFound)
	})
	t.Run("otro error", func(t *testing.T) {
		repo := &fakeUsersRepo{getErr: errors.New("db down")}
		svc := newTestService(repo, &fakeUserRoleRepo{})
		_, err := svc.GetUser(context.Background(), "tenant1", "u1", false, false)
		require.Error(t, err)
		require.NotErrorIs(t, err, domainUsers.ErrNotFound)
	})
}

func TestGetUserWithRoles(t *testing.T) {
	t.Run("feliz", func(t *testing.T) {
		want := &domainUsers.UserWithRoles{User: domainUsers.User{ID: "u1"}, Roles: []domainUsers.AssignedRole{{ID: "admin"}}}
		repo := &fakeUsersRepo{getWithRolesResult: want}
		svc := newTestService(repo, &fakeUserRoleRepo{})
		got, err := svc.GetUserWithRoles(context.Background(), "tenant1", "u1", false, false)
		require.NoError(t, err)
		require.Equal(t, want, got)
	})
	t.Run("no encontrado", func(t *testing.T) {
		repo := &fakeUsersRepo{getWithRolesErr: domainUsers.ErrNotFound}
		svc := newTestService(repo, &fakeUserRoleRepo{})
		_, err := svc.GetUserWithRoles(context.Background(), "tenant1", "u1", false, false)
		require.ErrorIs(t, err, domainUsers.ErrNotFound)
	})
}

func TestDeleteUser(t *testing.T) {
	t.Run("usuario no encontrado en el precheck nunca llama Delete", func(t *testing.T) {
		repo := &fakeUsersRepo{getErr: domainUsers.ErrNotFound}
		svc := newTestService(repo, &fakeUserRoleRepo{})
		err := svc.DeleteUser(context.Background(), "tenant-request", "u1", false, false)
		require.ErrorIs(t, err, domainUsers.ErrNotFound)
		require.Empty(t, repo.deleteCalls, "un usuario invisible en el precheck no debe llegar a Delete")
	})
	t.Run("error de Delete", func(t *testing.T) {
		repo := &fakeUsersRepo{getResult: &domainUsers.User{ID: "u1", TenantID: "tenant-real"}, deleteErr: domainUsers.ErrNotFound}
		svc := newTestService(repo, &fakeUserRoleRepo{})
		err := svc.DeleteUser(context.Background(), "tenant-request", "u1", false, false)
		require.ErrorIs(t, err, domainUsers.ErrNotFound)
	})
	t.Run("feliz borra contra el tenant REAL del target, no el de la request", func(t *testing.T) {
		repo := &fakeUsersRepo{getResult: &domainUsers.User{ID: "u1", TenantID: "tenant-real"}}
		svc := newTestService(repo, &fakeUserRoleRepo{})
		err := svc.DeleteUser(context.Background(), "tenant-request", "u1", true, false)
		require.NoError(t, err)
		require.Len(t, repo.deleteCalls, 1)
		require.Equal(t, "tenant-real", repo.deleteCalls[0].tenantID, "debe usar current.TenantID, no el tenantID de la request")
	})
}

func TestUpdateUserStatus(t *testing.T) {
	t.Run("no puede desactivarse a si mismo", func(t *testing.T) {
		svc := newTestService(&fakeUsersRepo{}, &fakeUserRoleRepo{})
		_, err := svc.UpdateUserStatus(context.Background(), "tenant1", "u1", "u1", "inactive", false, false)
		require.ErrorIs(t, err, domainUsers.ErrCannotDeactivateSelf)
	})
	t.Run("status invalido", func(t *testing.T) {
		svc := newTestService(&fakeUsersRepo{}, &fakeUserRoleRepo{})
		_, err := svc.UpdateUserStatus(context.Background(), "tenant1", "u1", "caller1", "on-vacation", false, false)
		require.ErrorIs(t, err, domainUsers.ErrInvalidStatus)
	})
	t.Run("usuario no encontrado en el precheck", func(t *testing.T) {
		repo := &fakeUsersRepo{getErr: domainUsers.ErrNotFound}
		svc := newTestService(repo, &fakeUserRoleRepo{})
		_, err := svc.UpdateUserStatus(context.Background(), "tenant1", "u1", "caller1", "active", false, false)
		require.ErrorIs(t, err, domainUsers.ErrNotFound)
	})
	t.Run("sin asignacion activa", func(t *testing.T) {
		tenantUUID := uuid.New()
		userUUID := uuid.New()
		repo := &fakeUsersRepo{getResult: &domainUsers.User{ID: userUUID.String(), TenantID: tenantUUID.String()}}
		urRepo := &fakeUserRoleRepo{updateStatusErr: domain.ErrNoActiveAssignment}
		svc := newTestService(repo, urRepo)
		_, err := svc.UpdateUserStatus(context.Background(), "tenant1", userUUID.String(), "caller1", "suspended", false, false)
		require.ErrorIs(t, err, domain.ErrNoActiveAssignment)
	})
	t.Run("feliz devuelve el snapshot pre-mutacion, no re-consulta", func(t *testing.T) {
		tenantUUID := uuid.New()
		userUUID := uuid.New()
		current := &domainUsers.User{ID: userUUID.String(), TenantID: tenantUUID.String(), FirstName: "Ana"}
		repo := &fakeUsersRepo{getResult: current}
		urRepo := &fakeUserRoleRepo{updateStatusResult: &domain.UserTenantRole{}}
		svc := newTestService(repo, urRepo)
		got, err := svc.UpdateUserStatus(context.Background(), "tenant1", userUUID.String(), "caller1", "active", false, false)
		require.NoError(t, err)
		require.Same(t, current, got, "debe devolver el mismo puntero resuelto en el precheck, sin un segundo GetByID")
		require.Len(t, urRepo.updateStatusCalls, 1)
		require.Equal(t, domain.UserRoleStatusActive, urRepo.updateStatusCalls[0].status)
		require.Equal(t, tenantUUID, urRepo.updateStatusCalls[0].tenantID, "debe usar el tenant REAL del target")
	})
}
