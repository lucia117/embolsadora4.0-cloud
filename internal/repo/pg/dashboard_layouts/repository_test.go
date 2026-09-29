package dashboard_layouts_test

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domain "github.com/tu-org/embolsadora-api/internal/domain/dashboard_layouts"
	dashboardlayouts "github.com/tu-org/embolsadora-api/internal/repo/pg/dashboard_layouts"
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

// seedTenantAndUser crea un tenant y un usuario descartables.
func seedTenantAndUser(t *testing.T, pool *pgxpool.Pool) (tenantID, userID uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	tenantID, userID = uuid.New(), uuid.New()

	_, err := pool.Exec(ctx,
		`INSERT INTO tenants (id, name, company_name, subdomain) VALUES ($1, 'layouts-test', 'layouts-test', $2)`,
		tenantID, "layouts-test-"+uuid.NewString()[:8])
	require.NoError(t, err)

	_, err = pool.Exec(ctx,
		`INSERT INTO users (id, email, name, status) VALUES ($1, $2, 'Layout Tester', 'active')`,
		userID, userID.String()+"@layouts.local")
	require.NoError(t, err)

	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, userID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM tenants WHERE id = $1`, tenantID)
	})
	return tenantID, userID
}

func newLayout(tenantID, userID uuid.UUID, name string) *domain.DashboardLayout {
	return &domain.DashboardLayout{
		ID:       uuid.New(),
		TenantID: tenantID,
		UserID:   userID,
		Name:     name,
		Widgets: []domain.Widget{
			{ID: "w1", Type: "chart", Name: "cpu", Title: "CPU", Category: "system", Icon: "cpu",
				Position: domain.Position{X: 0, Y: 0, W: 2, H: 2}},
		},
	}
}

func TestCreateAndGetByID(t *testing.T) {
	pool := newPool(t)
	repo := dashboardlayouts.NewPostgresRepository(pool)
	tenantID, userID := seedTenantAndUser(t, pool)

	layout := newLayout(tenantID, userID, "Principal")
	require.NoError(t, repo.Create(context.Background(), layout))
	require.False(t, layout.CreatedAt.IsZero())

	got, err := repo.GetByID(context.Background(), tenantID, userID, layout.ID)
	require.NoError(t, err)
	assert.Equal(t, "Principal", got.Name)
	require.Len(t, got.Widgets, 1)
	assert.Equal(t, "cpu", got.Widgets[0].Name)
}

func TestGetByIDNotFound(t *testing.T) {
	pool := newPool(t)
	repo := dashboardlayouts.NewPostgresRepository(pool)
	tenantID, userID := seedTenantAndUser(t, pool)

	_, err := repo.GetByID(context.Background(), tenantID, userID, uuid.New())
	assert.ErrorIs(t, err, domain.ErrLayoutNotFound)
}

func TestCreateRespetaElLimiteDeTresPorUsuario(t *testing.T) {
	pool := newPool(t)
	repo := dashboardlayouts.NewPostgresRepository(pool)
	tenantID, userID := seedTenantAndUser(t, pool)

	for i := 0; i < domain.MaxLayoutsPerUser; i++ {
		l := newLayout(tenantID, userID, "layout-"+uuid.NewString()[:8])
		require.NoError(t, repo.Create(context.Background(), l))
	}

	fourth := newLayout(tenantID, userID, "el-cuarto")
	err := repo.Create(context.Background(), fourth)
	assert.ErrorIs(t, err, domain.ErrLimitReached)

	count, err := repo.CountByTenantUser(context.Background(), tenantID, userID)
	require.NoError(t, err)
	assert.Equal(t, domain.MaxLayoutsPerUser, count)
}

func TestCreateNombreDuplicadoEnElMismoTenantUser(t *testing.T) {
	pool := newPool(t)
	repo := dashboardlayouts.NewPostgresRepository(pool)
	tenantID, userID := seedTenantAndUser(t, pool)

	require.NoError(t, repo.Create(context.Background(), newLayout(tenantID, userID, "Duplicado")))

	err := repo.Create(context.Background(), newLayout(tenantID, userID, "Duplicado"))
	assert.ErrorIs(t, err, domain.ErrDuplicateName)
}

func TestListSoloDevuelveActivosOrdenadosPorCreatedAt(t *testing.T) {
	pool := newPool(t)
	repo := dashboardlayouts.NewPostgresRepository(pool)
	tenantID, userID := seedTenantAndUser(t, pool)

	first := newLayout(tenantID, userID, "primero")
	require.NoError(t, repo.Create(context.Background(), first))
	second := newLayout(tenantID, userID, "segundo")
	require.NoError(t, repo.Create(context.Background(), second))
	require.NoError(t, repo.SoftDelete(context.Background(), tenantID, userID, first.ID))

	list, err := repo.List(context.Background(), tenantID, userID)
	require.NoError(t, err)
	require.Len(t, list, 1, "el soft-deleted no debe listarse")
	assert.Equal(t, "segundo", list[0].Name)
}

func TestUpdateHappyPathAndNotFound(t *testing.T) {
	pool := newPool(t)
	repo := dashboardlayouts.NewPostgresRepository(pool)
	tenantID, userID := seedTenantAndUser(t, pool)

	layout := newLayout(tenantID, userID, "original")
	require.NoError(t, repo.Create(context.Background(), layout))

	layout.Name = "renombrado"
	layout.Widgets = append(layout.Widgets, domain.Widget{ID: "w2", Type: "gauge", Name: "ram"})
	require.NoError(t, repo.Update(context.Background(), layout))

	got, err := repo.GetByID(context.Background(), tenantID, userID, layout.ID)
	require.NoError(t, err)
	assert.Equal(t, "renombrado", got.Name)
	assert.Len(t, got.Widgets, 2)

	ghost := newLayout(tenantID, userID, "fantasma")
	err = repo.Update(context.Background(), ghost)
	assert.ErrorIs(t, err, domain.ErrLayoutNotFound)
}

func TestSoftDeleteNoPermiteBorrarElUltimo(t *testing.T) {
	pool := newPool(t)
	repo := dashboardlayouts.NewPostgresRepository(pool)
	tenantID, userID := seedTenantAndUser(t, pool)

	only := newLayout(tenantID, userID, "unico")
	require.NoError(t, repo.Create(context.Background(), only))

	err := repo.SoftDelete(context.Background(), tenantID, userID, only.ID)
	assert.ErrorIs(t, err, domain.ErrCannotDeleteLastLayout)
}

func TestSoftDeleteNotFound(t *testing.T) {
	pool := newPool(t)
	repo := dashboardlayouts.NewPostgresRepository(pool)
	tenantID, userID := seedTenantAndUser(t, pool)

	require.NoError(t, repo.Create(context.Background(), newLayout(tenantID, userID, "a")))
	require.NoError(t, repo.Create(context.Background(), newLayout(tenantID, userID, "b")))

	err := repo.SoftDelete(context.Background(), tenantID, userID, uuid.New())
	assert.ErrorIs(t, err, domain.ErrLayoutNotFound)
}
