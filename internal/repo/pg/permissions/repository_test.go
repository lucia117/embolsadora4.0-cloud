package permissions_test

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tu-org/embolsadora-api/internal/domain"
	"github.com/tu-org/embolsadora-api/internal/repo/pg/permissions"
)

func newPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL no seteada; se omite el test de integracion")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	return pool
}

func seedTenant(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	id := uuid.New()
	_, err := pool.Exec(context.Background(),
		`INSERT INTO tenants (id, name, company_name, subdomain) VALUES ($1, 'perms-test', 'perms-test', $2)`,
		id, "perms-test-"+uuid.NewString()[:8])
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM tenants WHERE id = $1`, id)
	})
	return id
}

func newCustomPermission(tenantID uuid.UUID) *domain.Permission {
	id := "perm_custom_" + uuid.NewString()[:8]
	return &domain.Permission{
		ID:          id,
		Name:        "Permiso de prueba",
		Section:     "testing",
		Description: "usado por repository_test.go",
		TenantID:    &tenantID,
	}
}

func TestCreateAndGetByID(t *testing.T) {
	pool := newPool(t)
	repo := permissions.NewPostgresRepository(pool)
	tenantID := seedTenant(t, pool)

	p := newCustomPermission(tenantID)
	require.NoError(t, repo.Create(context.Background(), p))

	got, err := repo.GetByID(context.Background(), p.ID, tenantID)
	require.NoError(t, err)
	assert.Equal(t, p.Name, got.Name)
	assert.False(t, got.IsSystemPermission)
	require.NotNil(t, got.TenantID)
	assert.Equal(t, tenantID, *got.TenantID)
}

func TestGetByIDDeOtroTenantEsNotFound(t *testing.T) {
	pool := newPool(t)
	repo := permissions.NewPostgresRepository(pool)
	tenantA := seedTenant(t, pool)
	tenantB := seedTenant(t, pool)

	p := newCustomPermission(tenantA)
	require.NoError(t, repo.Create(context.Background(), p))

	_, err := repo.GetByID(context.Background(), p.ID, tenantB)
	assert.ErrorIs(t, err, domain.ErrPermissionNotFound)
}

func TestListIncluyeSistemaYCustomDelTenant(t *testing.T) {
	pool := newPool(t)
	repo := permissions.NewPostgresRepository(pool)
	tenantA := seedTenant(t, pool)
	tenantB := seedTenant(t, pool)

	custom := newCustomPermission(tenantA)
	require.NoError(t, repo.Create(context.Background(), custom))
	otherCustom := newCustomPermission(tenantB)
	require.NoError(t, repo.Create(context.Background(), otherCustom))

	list, err := repo.List(context.Background(), tenantA)
	require.NoError(t, err)

	var ids []string
	for _, p := range list {
		ids = append(ids, p.ID)
	}
	assert.Contains(t, ids, custom.ID)
	assert.NotContains(t, ids, otherCustom.ID, "el permiso custom de otro tenant no debe listarse")
}

func TestUpdateHappyPathAndNotFound(t *testing.T) {
	pool := newPool(t)
	repo := permissions.NewPostgresRepository(pool)
	tenantID := seedTenant(t, pool)

	p := newCustomPermission(tenantID)
	require.NoError(t, repo.Create(context.Background(), p))

	p.Name = "Nombre actualizado"
	p.Section = "otra-seccion"
	require.NoError(t, repo.Update(context.Background(), p))

	got, err := repo.GetByID(context.Background(), p.ID, tenantID)
	require.NoError(t, err)
	assert.Equal(t, "Nombre actualizado", got.Name)

	ghost := newCustomPermission(tenantID)
	err = repo.Update(context.Background(), ghost)
	assert.ErrorIs(t, err, domain.ErrPermissionNotFound)
}

func TestDeleteHappyPathAndNotFound(t *testing.T) {
	pool := newPool(t)
	repo := permissions.NewPostgresRepository(pool)
	tenantID := seedTenant(t, pool)

	p := newCustomPermission(tenantID)
	require.NoError(t, repo.Create(context.Background(), p))

	require.NoError(t, repo.Delete(context.Background(), p.ID, tenantID))

	_, err := repo.GetByID(context.Background(), p.ID, tenantID)
	assert.ErrorIs(t, err, domain.ErrPermissionNotFound)

	err = repo.Delete(context.Background(), p.ID, tenantID)
	assert.ErrorIs(t, err, domain.ErrPermissionNotFound)
}

// TestDeleteDePermisoDeSistemaFalla siembra un permiso de sistema directo por
// SQL (is_system_permission=true exige tenant_id NULL — chk_system_perm_no_tenant
// — asi que no pasa por repo.Create, pensado para permisos custom con tenant).
func TestDeleteDePermisoDeSistemaFalla(t *testing.T) {
	pool := newPool(t)
	repo := permissions.NewPostgresRepository(pool)
	tenantID := seedTenant(t, pool)

	sysID := "perm_system_test_" + uuid.NewString()[:8]
	_, err := pool.Exec(context.Background(),
		`INSERT INTO permissions (id, name, section, description, is_system_permission, tenant_id)
		 VALUES ($1, 'Sistema', 'testing', 'permiso de sistema de prueba', TRUE, NULL)`,
		sysID)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM permissions WHERE id = $1`, sysID)
	})

	err = repo.Delete(context.Background(), sysID, tenantID)
	assert.ErrorIs(t, err, domain.ErrPermissionIsSystem)
}
