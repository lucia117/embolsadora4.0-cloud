package alarm_rules_test

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tu-org/embolsadora-api/internal/domain"
	alarmrules "github.com/tu-org/embolsadora-api/internal/repo/pg/alarm_rules"
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

// seedTenant crea un tenant descartable y devuelve su id.
func seedTenant(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	id := uuid.New()
	_, err := pool.Exec(context.Background(),
		`INSERT INTO tenants (id, name, company_name, subdomain) VALUES ($1, 'alarm-test', 'alarm-test', $2)`,
		id, "alarm-test-"+uuid.NewString()[:8])
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM tenants WHERE id = $1`, id)
	})
	return id
}

func newRule(tenantID uuid.UUID) *domain.AlarmRule {
	return &domain.AlarmRule{
		TenantID:    tenantID,
		Name:        "Temp alta",
		Description: "dispara si supera el umbral",
		Metric:      "temperature",
		Operator:    "gt",
		Threshold:   80,
		Severity:    "critical",
		Enabled:     true,
	}
}

func TestCreateAndGetByID(t *testing.T) {
	pool := newPool(t)
	repo := alarmrules.NewPostgresRepository(pool)
	tenantID := seedTenant(t, pool)

	rule := newRule(tenantID)
	require.NoError(t, repo.Create(context.Background(), rule))
	require.NotEqual(t, uuid.Nil, rule.ID, "Create debe poblar el ID generado por la DB")

	got, err := repo.GetByID(context.Background(), rule.ID, tenantID)
	require.NoError(t, err)
	assert.Equal(t, rule.Name, got.Name)
	assert.Equal(t, rule.Description, got.Description)
	assert.Equal(t, rule.Threshold, got.Threshold)
}

func TestCreateConDescripcionVaciaGuardaNull(t *testing.T) {
	pool := newPool(t)
	repo := alarmrules.NewPostgresRepository(pool)
	tenantID := seedTenant(t, pool)

	rule := newRule(tenantID)
	rule.Description = ""
	require.NoError(t, repo.Create(context.Background(), rule))

	got, err := repo.GetByID(context.Background(), rule.ID, tenantID)
	require.NoError(t, err)
	assert.Equal(t, "", got.Description, "una description NULL en DB debe volver como string vacio")
}

func TestGetByIDNotFound(t *testing.T) {
	pool := newPool(t)
	repo := alarmrules.NewPostgresRepository(pool)
	tenantID := seedTenant(t, pool)

	_, err := repo.GetByID(context.Background(), uuid.New(), tenantID)
	assert.ErrorIs(t, err, domain.ErrAlarmRuleNotFound)
}

func TestGetByIDDeOtroTenantEsNotFound(t *testing.T) {
	pool := newPool(t)
	repo := alarmrules.NewPostgresRepository(pool)
	tenantA := seedTenant(t, pool)
	tenantB := seedTenant(t, pool)

	rule := newRule(tenantA)
	require.NoError(t, repo.Create(context.Background(), rule))

	_, err := repo.GetByID(context.Background(), rule.ID, tenantB)
	assert.ErrorIs(t, err, domain.ErrAlarmRuleNotFound, "una regla de otro tenant no debe ser visible")
}

func TestListOrdenaPorCreatedAtYScopeaPorTenant(t *testing.T) {
	pool := newPool(t)
	repo := alarmrules.NewPostgresRepository(pool)
	tenantA := seedTenant(t, pool)
	tenantB := seedTenant(t, pool)

	first := newRule(tenantA)
	first.Name = "primera"
	require.NoError(t, repo.Create(context.Background(), first))

	second := newRule(tenantA)
	second.Name = "segunda"
	require.NoError(t, repo.Create(context.Background(), second))

	other := newRule(tenantB)
	require.NoError(t, repo.Create(context.Background(), other))

	list, err := repo.List(context.Background(), tenantA)
	require.NoError(t, err)
	require.Len(t, list, 2)
	assert.Equal(t, "primera", list[0].Name)
	assert.Equal(t, "segunda", list[1].Name)
}

func TestListSinReglasDevuelveSliceVacioNoNil(t *testing.T) {
	pool := newPool(t)
	repo := alarmrules.NewPostgresRepository(pool)
	tenantID := seedTenant(t, pool)

	list, err := repo.List(context.Background(), tenantID)
	require.NoError(t, err)
	assert.NotNil(t, list)
	assert.Empty(t, list)
}

func TestUpdateHappyPathAndNotFound(t *testing.T) {
	pool := newPool(t)
	repo := alarmrules.NewPostgresRepository(pool)
	tenantID := seedTenant(t, pool)

	rule := newRule(tenantID)
	require.NoError(t, repo.Create(context.Background(), rule))

	rule.Name = "Temp muy alta"
	rule.Threshold = 95
	rule.Enabled = false
	require.NoError(t, repo.Update(context.Background(), rule))

	got, err := repo.GetByID(context.Background(), rule.ID, tenantID)
	require.NoError(t, err)
	assert.Equal(t, "Temp muy alta", got.Name)
	assert.Equal(t, float64(95), got.Threshold)
	assert.False(t, got.Enabled)

	ghost := newRule(tenantID)
	ghost.ID = uuid.New()
	err = repo.Update(context.Background(), ghost)
	assert.ErrorIs(t, err, domain.ErrAlarmRuleNotFound)
}

func TestDeleteHappyPathAndNotFound(t *testing.T) {
	pool := newPool(t)
	repo := alarmrules.NewPostgresRepository(pool)
	tenantID := seedTenant(t, pool)

	rule := newRule(tenantID)
	require.NoError(t, repo.Create(context.Background(), rule))

	require.NoError(t, repo.Delete(context.Background(), rule.ID, tenantID))

	_, err := repo.GetByID(context.Background(), rule.ID, tenantID)
	assert.ErrorIs(t, err, domain.ErrAlarmRuleNotFound)

	err = repo.Delete(context.Background(), rule.ID, tenantID)
	assert.ErrorIs(t, err, domain.ErrAlarmRuleNotFound, "borrar de nuevo debe fallar, no ser idempotente")
}
