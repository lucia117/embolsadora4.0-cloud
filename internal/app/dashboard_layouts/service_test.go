package dashboard_layouts_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	app "github.com/tu-org/embolsadora-api/internal/app/dashboard_layouts"
	domain "github.com/tu-org/embolsadora-api/internal/domain/dashboard_layouts"
)

type fakeRepo struct {
	listResult []*domain.DashboardLayout
	listErr    error

	getResult *domain.DashboardLayout
	getErr    error

	createErr   error
	createCalls []*domain.DashboardLayout

	updateErr   error
	updateCalls []*domain.DashboardLayout

	softDeleteErr error
}

func (f *fakeRepo) List(context.Context, uuid.UUID, uuid.UUID) ([]*domain.DashboardLayout, error) {
	return f.listResult, f.listErr
}
func (f *fakeRepo) GetByID(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (*domain.DashboardLayout, error) {
	return f.getResult, f.getErr
}
func (f *fakeRepo) CountByTenantUser(context.Context, uuid.UUID, uuid.UUID) (int, error) {
	return 0, nil
}
func (f *fakeRepo) Create(_ context.Context, l *domain.DashboardLayout) error {
	f.createCalls = append(f.createCalls, l)
	return f.createErr
}
func (f *fakeRepo) Update(_ context.Context, l *domain.DashboardLayout) error {
	f.updateCalls = append(f.updateCalls, l)
	return f.updateErr
}
func (f *fakeRepo) SoftDelete(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) error {
	return f.softDeleteErr
}

func TestListLayouts(t *testing.T) {
	t.Run("feliz", func(t *testing.T) {
		want := []*domain.DashboardLayout{{ID: uuid.New()}}
		svc := app.NewService(&fakeRepo{listResult: want}, zap.NewNop())
		got, err := svc.ListLayouts(context.Background(), uuid.New(), uuid.New())
		require.NoError(t, err)
		require.Equal(t, want, got)
	})
	t.Run("error de repo", func(t *testing.T) {
		svc := app.NewService(&fakeRepo{listErr: errors.New("db down")}, zap.NewNop())
		_, err := svc.ListLayouts(context.Background(), uuid.New(), uuid.New())
		require.Error(t, err)
	})
}

func TestGetLayout(t *testing.T) {
	t.Run("feliz", func(t *testing.T) {
		want := &domain.DashboardLayout{ID: uuid.New()}
		svc := app.NewService(&fakeRepo{getResult: want}, zap.NewNop())
		got, err := svc.GetLayout(context.Background(), uuid.New(), uuid.New(), want.ID)
		require.NoError(t, err)
		require.Equal(t, want, got)
	})
	t.Run("no encontrado", func(t *testing.T) {
		svc := app.NewService(&fakeRepo{getErr: domain.ErrLayoutNotFound}, zap.NewNop())
		_, err := svc.GetLayout(context.Background(), uuid.New(), uuid.New(), uuid.New())
		require.ErrorIs(t, err, domain.ErrLayoutNotFound)
	})
}

func TestCreateLayout(t *testing.T) {
	tenantID, userID := uuid.New(), uuid.New()

	t.Run("widgets nil se normaliza a slice vacio, no nil", func(t *testing.T) {
		repo := &fakeRepo{}
		svc := app.NewService(repo, zap.NewNop())
		got, err := svc.CreateLayout(context.Background(), tenantID, userID, domain.CreateLayoutCommand{Name: "Mi layout"})
		require.NoError(t, err)
		require.NotNil(t, got.Widgets)
		require.Empty(t, got.Widgets)
	})
	t.Run("limite alcanzado propaga sin loguear como error", func(t *testing.T) {
		repo := &fakeRepo{createErr: domain.ErrLimitReached}
		svc := app.NewService(repo, zap.NewNop())
		_, err := svc.CreateLayout(context.Background(), tenantID, userID, domain.CreateLayoutCommand{Name: "x"})
		require.ErrorIs(t, err, domain.ErrLimitReached)
	})
	t.Run("nombre duplicado", func(t *testing.T) {
		repo := &fakeRepo{createErr: domain.ErrDuplicateName}
		svc := app.NewService(repo, zap.NewNop())
		_, err := svc.CreateLayout(context.Background(), tenantID, userID, domain.CreateLayoutCommand{Name: "x"})
		require.ErrorIs(t, err, domain.ErrDuplicateName)
	})
	t.Run("feliz preserva los widgets enviados", func(t *testing.T) {
		repo := &fakeRepo{}
		svc := app.NewService(repo, zap.NewNop())
		widgets := []domain.Widget{{ID: "w1", Type: "chart"}}
		got, err := svc.CreateLayout(context.Background(), tenantID, userID, domain.CreateLayoutCommand{Name: "x", Widgets: widgets})
		require.NoError(t, err)
		require.Equal(t, widgets, got.Widgets)
		require.Equal(t, tenantID, got.TenantID)
		require.Equal(t, userID, got.UserID)
	})
}

func TestUpdateLayout(t *testing.T) {
	tenantID, userID, layoutID := uuid.New(), uuid.New(), uuid.New()
	existing := &domain.DashboardLayout{ID: layoutID, TenantID: tenantID, UserID: userID, Name: "Vieja"}

	t.Run("no encontrado en el GetByID inicial", func(t *testing.T) {
		repo := &fakeRepo{getErr: domain.ErrLayoutNotFound}
		svc := app.NewService(repo, zap.NewNop())
		_, err := svc.UpdateLayout(context.Background(), tenantID, userID, layoutID, domain.UpdateLayoutCommand{})
		require.ErrorIs(t, err, domain.ErrLayoutNotFound)
	})
	t.Run("nombre duplicado en el Update", func(t *testing.T) {
		repo := &fakeRepo{getResult: existing, updateErr: domain.ErrDuplicateName}
		svc := app.NewService(repo, zap.NewNop())
		_, err := svc.UpdateLayout(context.Background(), tenantID, userID, layoutID, domain.UpdateLayoutCommand{Name: "Otra"})
		require.ErrorIs(t, err, domain.ErrDuplicateName)
	})
	t.Run("feliz normaliza widgets nil", func(t *testing.T) {
		repo := &fakeRepo{getResult: existing}
		svc := app.NewService(repo, zap.NewNop())
		got, err := svc.UpdateLayout(context.Background(), tenantID, userID, layoutID, domain.UpdateLayoutCommand{Name: "Nueva"})
		require.NoError(t, err)
		require.Equal(t, "Nueva", got.Name)
		require.NotNil(t, got.Widgets)
	})
}

func TestDeleteLayout(t *testing.T) {
	t.Run("no encontrado", func(t *testing.T) {
		svc := app.NewService(&fakeRepo{softDeleteErr: domain.ErrLayoutNotFound}, zap.NewNop())
		err := svc.DeleteLayout(context.Background(), uuid.New(), uuid.New(), uuid.New())
		require.ErrorIs(t, err, domain.ErrLayoutNotFound)
	})
	t.Run("no se puede borrar el ultimo layout", func(t *testing.T) {
		svc := app.NewService(&fakeRepo{softDeleteErr: domain.ErrCannotDeleteLastLayout}, zap.NewNop())
		err := svc.DeleteLayout(context.Background(), uuid.New(), uuid.New(), uuid.New())
		require.ErrorIs(t, err, domain.ErrCannotDeleteLastLayout)
	})
	t.Run("feliz", func(t *testing.T) {
		svc := app.NewService(&fakeRepo{}, zap.NewNop())
		err := svc.DeleteLayout(context.Background(), uuid.New(), uuid.New(), uuid.New())
		require.NoError(t, err)
	})
}
