package permissions_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	app "github.com/tu-org/embolsadora-api/internal/app/permissions"
	"github.com/tu-org/embolsadora-api/internal/domain"
)

type fakeRepo struct {
	listResult []*domain.Permission
	listErr    error

	// getByIDResults se consume en orden: la 1ra llamada devuelve
	// getByIDResults[0], la 2da (el roundtrip post-Create/Update) getByIDResults[1].
	// Si está vacío, devuelve getResult/getErr fijos para toda llamada.
	getByIDResults []*domain.Permission
	getByIDErrs    []error
	getByIDCalls   int

	getResult *domain.Permission
	getErr    error

	createErr   error
	createCalls []*domain.Permission

	updateErr   error
	updateCalls []*domain.Permission

	deleteErr error
}

func (f *fakeRepo) List(context.Context, uuid.UUID) ([]*domain.Permission, error) {
	return f.listResult, f.listErr
}
func (f *fakeRepo) GetByID(context.Context, string, uuid.UUID) (*domain.Permission, error) {
	if len(f.getByIDResults) > 0 {
		i := f.getByIDCalls
		f.getByIDCalls++
		// Clamp cada slice de forma independiente: getByIDResults y getByIDErrs
		// pueden tener longitudes distintas (los tests solo llenan hasta donde les
		// importa), así que un único índice compartido podía indexar fuera de rango
		// del más corto de los dos y panicar.
		resultIdx := i
		if resultIdx >= len(f.getByIDResults) {
			resultIdx = len(f.getByIDResults) - 1
		}
		var err error
		if len(f.getByIDErrs) > 0 {
			errIdx := i
			if errIdx >= len(f.getByIDErrs) {
				errIdx = len(f.getByIDErrs) - 1
			}
			err = f.getByIDErrs[errIdx]
		}
		return f.getByIDResults[resultIdx], err
	}
	f.getByIDCalls++
	return f.getResult, f.getErr
}
func (f *fakeRepo) Create(_ context.Context, p *domain.Permission) error {
	f.createCalls = append(f.createCalls, p)
	return f.createErr
}
func (f *fakeRepo) Update(_ context.Context, p *domain.Permission) error {
	f.updateCalls = append(f.updateCalls, p)
	return f.updateErr
}
func (f *fakeRepo) Delete(context.Context, string, uuid.UUID) error { return f.deleteErr }

func TestListPermissions(t *testing.T) {
	t.Run("feliz", func(t *testing.T) {
		want := []*domain.Permission{{ID: "perm_x"}}
		svc := app.NewService(&fakeRepo{listResult: want}, zap.NewNop())
		got, err := svc.ListPermissions(context.Background(), uuid.New())
		require.NoError(t, err)
		require.Equal(t, want, got)
	})
	t.Run("error de repo", func(t *testing.T) {
		svc := app.NewService(&fakeRepo{listErr: errors.New("db down")}, zap.NewNop())
		_, err := svc.ListPermissions(context.Background(), uuid.New())
		require.Error(t, err)
	})
}

func TestGetPermission(t *testing.T) {
	t.Run("feliz", func(t *testing.T) {
		want := &domain.Permission{ID: "perm_x"}
		svc := app.NewService(&fakeRepo{getResult: want}, zap.NewNop())
		got, err := svc.GetPermission(context.Background(), "perm_x", uuid.New())
		require.NoError(t, err)
		require.Equal(t, want, got)
	})
	t.Run("no encontrado", func(t *testing.T) {
		svc := app.NewService(&fakeRepo{getErr: domain.ErrPermissionNotFound}, zap.NewNop())
		_, err := svc.GetPermission(context.Background(), "perm_x", uuid.New())
		require.ErrorIs(t, err, domain.ErrPermissionNotFound)
	})
}

func TestCreatePermission(t *testing.T) {
	tenantID := uuid.New()

	fieldTests := []struct {
		name                    string
		permName, section, desc string
	}{
		{"nombre muy corto", "ab", "sec", "desc"},
		{"section vacia", "nombre valido", "  ", "desc"},
		{"description vacia", "nombre valido", "sec", ""},
	}
	for _, tt := range fieldTests {
		t.Run(tt.name, func(t *testing.T) {
			svc := app.NewService(&fakeRepo{}, zap.NewNop())
			_, err := svc.CreatePermission(context.Background(), tenantID, tt.permName, tt.section, tt.desc)
			require.Error(t, err)
			var ve *app.ValidationError
			require.ErrorAs(t, err, &ve)
		})
	}

	t.Run("error de repo.Create", func(t *testing.T) {
		svc := app.NewService(&fakeRepo{createErr: errors.New("db down")}, zap.NewNop())
		_, err := svc.CreatePermission(context.Background(), tenantID, "nombre valido", "sec", "desc")
		require.Error(t, err)
	})

	t.Run("error en el GetByID del roundtrip post-create", func(t *testing.T) {
		repo := &fakeRepo{getByIDResults: []*domain.Permission{nil}, getByIDErrs: []error{errors.New("read replica lag")}}
		svc := app.NewService(repo, zap.NewNop())
		_, err := svc.CreatePermission(context.Background(), tenantID, "nombre valido", "sec", "desc")
		require.Error(t, err)
	})

	t.Run("feliz genera id, IsSystemPermission=false, y hace el roundtrip", func(t *testing.T) {
		created := &domain.Permission{ID: "generated", Name: "nombre valido"}
		repo := &fakeRepo{getByIDResults: []*domain.Permission{created}, getByIDErrs: []error{nil}}
		svc := app.NewService(repo, zap.NewNop())
		got, err := svc.CreatePermission(context.Background(), tenantID, "nombre valido", "sec", "desc")
		require.NoError(t, err)
		require.Equal(t, created, got)
		require.Equal(t, 1, repo.getByIDCalls)

		require.Len(t, repo.createCalls, 1)
		assert.Equal(t, "nombre valido", repo.createCalls[0].Name, "el Permission persistido debe llevar el name pedido")
		assert.False(t, repo.createCalls[0].IsSystemPermission, "un permiso creado por este flujo nunca es de sistema")
		require.NotNil(t, repo.createCalls[0].TenantID)
		assert.Equal(t, tenantID, *repo.createCalls[0].TenantID, "el Permission persistido debe quedar asociado al tenant correcto")
	})
}

func TestUpdatePermission(t *testing.T) {
	tenantID := uuid.New()

	t.Run("no encontrado", func(t *testing.T) {
		svc := app.NewService(&fakeRepo{getErr: domain.ErrPermissionNotFound}, zap.NewNop())
		_, err := svc.UpdatePermission(context.Background(), "perm_x", tenantID, "nombre valido", "sec", "desc")
		require.ErrorIs(t, err, domain.ErrPermissionNotFound)
	})

	t.Run("permiso de sistema rechaza ANTES de validar campos (campos invalidos no importan)", func(t *testing.T) {
		repo := &fakeRepo{getResult: &domain.Permission{ID: "perm_x", IsSystemPermission: true}}
		svc := app.NewService(repo, zap.NewNop())
		_, err := svc.UpdatePermission(context.Background(), "perm_x", tenantID, "", "", "")
		require.ErrorIs(t, err, domain.ErrPermissionIsSystem)
	})

	t.Run("campos invalidos en permiso no-sistema", func(t *testing.T) {
		repo := &fakeRepo{getResult: &domain.Permission{ID: "perm_x", IsSystemPermission: false}}
		svc := app.NewService(repo, zap.NewNop())
		_, err := svc.UpdatePermission(context.Background(), "perm_x", tenantID, "ab", "sec", "desc")
		var ve *app.ValidationError
		require.ErrorAs(t, err, &ve)
	})

	t.Run("feliz hace el roundtrip post-update", func(t *testing.T) {
		before := &domain.Permission{ID: "perm_x", IsSystemPermission: false}
		after := &domain.Permission{ID: "perm_x", Name: "nombre valido"}
		repo := &fakeRepo{getByIDResults: []*domain.Permission{before, after}, getByIDErrs: []error{nil, nil}}
		svc := app.NewService(repo, zap.NewNop())
		got, err := svc.UpdatePermission(context.Background(), "perm_x", tenantID, "nombre valido", "sec", "desc")
		require.NoError(t, err)
		require.Equal(t, after, got)
		require.Equal(t, 2, repo.getByIDCalls)

		require.Len(t, repo.updateCalls, 1)
		assert.Equal(t, "perm_x", repo.updateCalls[0].ID, "Update debe persistir el permiso correcto por id")
		assert.Equal(t, "nombre valido", repo.updateCalls[0].Name, "Update debe persistir el name nuevo pedido")
		assert.Equal(t, "sec", repo.updateCalls[0].Section)
		assert.Equal(t, "desc", repo.updateCalls[0].Description)
	})
}

func TestDeletePermission(t *testing.T) {
	t.Run("no encontrado", func(t *testing.T) {
		svc := app.NewService(&fakeRepo{deleteErr: domain.ErrPermissionNotFound}, zap.NewNop())
		err := svc.DeletePermission(context.Background(), "perm_x", uuid.New())
		require.ErrorIs(t, err, domain.ErrPermissionNotFound)
	})
	t.Run("es de sistema", func(t *testing.T) {
		svc := app.NewService(&fakeRepo{deleteErr: domain.ErrPermissionIsSystem}, zap.NewNop())
		err := svc.DeletePermission(context.Background(), "perm_x", uuid.New())
		require.ErrorIs(t, err, domain.ErrPermissionIsSystem)
	})
	t.Run("feliz", func(t *testing.T) {
		svc := app.NewService(&fakeRepo{}, zap.NewNop())
		err := svc.DeletePermission(context.Background(), "perm_x", uuid.New())
		require.NoError(t, err)
	})
}
