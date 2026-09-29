package edge_devices_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domain "github.com/tu-org/embolsadora-api/internal/domain/edge_devices"
	edgedevices "github.com/tu-org/embolsadora-api/internal/repo/pg/edge_devices"
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
		`INSERT INTO tenants (id, name, company_name, subdomain) VALUES ($1, 'edge-test', 'edge-test', $2)`,
		id, "edge-test-"+uuid.NewString()[:8])
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM tenants WHERE id = $1`, id)
	})
	return id
}

func newDevice(tenantID uuid.UUID) *domain.EdgeDevice {
	return &domain.EdgeDevice{
		ID:               uuid.New(),
		TenantID:         tenantID,
		Name:             "Edge de test",
		MachineID:        "EMB-TEST-" + uuid.NewString()[:8],
		EdgeType:         "RASPBERRY_PLC",
		RaspberryBaseURL: "http://localhost:9000",
		Status:           "ACTIVE",
	}
}

func TestCreateAndGetByID(t *testing.T) {
	pool := newPool(t)
	repo := edgedevices.NewPostgresRepository(pool)
	tenantID := seedTenant(t, pool)

	device := newDevice(tenantID)
	require.NoError(t, repo.Create(context.Background(), device))
	require.False(t, device.CreatedAt.IsZero())

	got, err := repo.GetByID(context.Background(), tenantID, device.ID)
	require.NoError(t, err)
	assert.Equal(t, device.MachineID, got.MachineID)
	assert.Equal(t, "ACTIVE", got.Status)
	assert.Equal(t, "UNKNOWN", got.LastHealthStatus, "default de la columna, no lo setea el repo")
}

func TestCreateMachineIDDuplicadoEnElMismoTenant(t *testing.T) {
	pool := newPool(t)
	repo := edgedevices.NewPostgresRepository(pool)
	tenantID := seedTenant(t, pool)

	first := newDevice(tenantID)
	require.NoError(t, repo.Create(context.Background(), first))

	dup := newDevice(tenantID)
	dup.MachineID = first.MachineID
	err := repo.Create(context.Background(), dup)
	assert.ErrorIs(t, err, domain.ErrMachineIDConflict)
}

func TestGetByIDNotFound(t *testing.T) {
	pool := newPool(t)
	repo := edgedevices.NewPostgresRepository(pool)
	tenantID := seedTenant(t, pool)

	_, err := repo.GetByID(context.Background(), tenantID, uuid.New())
	assert.ErrorIs(t, err, domain.ErrDeviceNotFound)
}

func TestListScopeaPorTenant(t *testing.T) {
	pool := newPool(t)
	repo := edgedevices.NewPostgresRepository(pool)
	tenantA := seedTenant(t, pool)
	tenantB := seedTenant(t, pool)

	require.NoError(t, repo.Create(context.Background(), newDevice(tenantA)))
	require.NoError(t, repo.Create(context.Background(), newDevice(tenantB)))

	list, err := repo.List(context.Background(), tenantA)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, tenantA, list[0].TenantID)
}

func TestUpdateHappyPathAndNotFound(t *testing.T) {
	pool := newPool(t)
	repo := edgedevices.NewPostgresRepository(pool)
	tenantID := seedTenant(t, pool)

	device := newDevice(tenantID)
	require.NoError(t, repo.Create(context.Background(), device))

	device.Name = "Edge renombrado"
	device.Status = "DISABLED"
	require.NoError(t, repo.Update(context.Background(), device))

	got, err := repo.GetByID(context.Background(), tenantID, device.ID)
	require.NoError(t, err)
	assert.Equal(t, "Edge renombrado", got.Name)
	assert.Equal(t, "DISABLED", got.Status)

	ghost := newDevice(tenantID)
	err = repo.Update(context.Background(), ghost)
	assert.ErrorIs(t, err, domain.ErrDeviceNotFound)
}

func TestSetStatus(t *testing.T) {
	pool := newPool(t)
	repo := edgedevices.NewPostgresRepository(pool)
	tenantID := seedTenant(t, pool)
	device := newDevice(tenantID)
	require.NoError(t, repo.Create(context.Background(), device))

	got, err := repo.SetStatus(context.Background(), tenantID, device.ID, "DISABLED")
	require.NoError(t, err)
	assert.Equal(t, "DISABLED", got.Status)

	_, err = repo.SetStatus(context.Background(), tenantID, uuid.New(), "ACTIVE")
	assert.ErrorIs(t, err, domain.ErrDeviceNotFound)
}

func TestUpdateHealthStateAndTouchLastSeen(t *testing.T) {
	pool := newPool(t)
	repo := edgedevices.NewPostgresRepository(pool)
	tenantID := seedTenant(t, pool)
	device := newDevice(tenantID)
	require.NoError(t, repo.Create(context.Background(), device))

	require.NoError(t, repo.UpdateHealthState(context.Background(), tenantID, device.ID, "OK", "todo bien"))
	got, err := repo.GetByID(context.Background(), tenantID, device.ID)
	require.NoError(t, err)
	assert.Equal(t, "OK", got.LastHealthStatus)
	require.NotNil(t, got.LastHealthSummary)
	assert.Equal(t, "todo bien", *got.LastHealthSummary)
	require.NotNil(t, got.LastHealthCheckAt)

	require.Nil(t, got.LastSeenAt, "todavia no se toco")
	require.NoError(t, repo.TouchLastSeen(context.Background(), tenantID, device.ID))
	got2, err := repo.GetByID(context.Background(), tenantID, device.ID)
	require.NoError(t, err)
	require.NotNil(t, got2.LastSeenAt)

	err = repo.UpdateHealthState(context.Background(), tenantID, uuid.New(), "OK", "x")
	assert.ErrorIs(t, err, domain.ErrDeviceNotFound)
	err = repo.TouchLastSeen(context.Background(), tenantID, uuid.New())
	assert.ErrorIs(t, err, domain.ErrDeviceNotFound)
}

func TestSaveEventAndListEventsNewestFirst(t *testing.T) {
	pool := newPool(t)
	repo := edgedevices.NewPostgresRepository(pool)
	tenantID := seedTenant(t, pool)
	device := newDevice(tenantID)
	require.NoError(t, repo.Create(context.Background(), device))

	userID := uuid.New()
	now := time.Now().UTC()
	older := &domain.DeviceEvent{
		ID: uuid.New(), DeviceID: device.ID, TenantID: tenantID,
		CheckType: "STATUS", OverallStatus: "OK", CheckedAt: now,
		UserID: userID, UserEmail: "op@test.local",
		Details: map[string]interface{}{},
	}
	require.NoError(t, repo.SaveEvent(context.Background(), older))

	newer := &domain.DeviceEvent{
		ID: uuid.New(), DeviceID: device.ID, TenantID: tenantID,
		CheckType: "HEALTH_CHECK", OverallStatus: "DEGRADED", CheckedAt: now.Add(time.Second),
		UserID: userID, UserEmail: "op@test.local",
		Details: map[string]interface{}{"cpu": 95.5},
	}
	require.NoError(t, repo.SaveEvent(context.Background(), newer))

	events, err := repo.ListEvents(context.Background(), tenantID, device.ID)
	require.NoError(t, err)
	require.Len(t, events, 2)
	assert.Equal(t, "HEALTH_CHECK", events[0].CheckType, "checked_at mas reciente va primero")
	assert.Equal(t, "STATUS", events[1].CheckType)
	assert.Equal(t, 95.5, events[0].Details["cpu"])
}
