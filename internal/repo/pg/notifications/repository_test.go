package notifications_test

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tu-org/embolsadora-api/internal/domain"
	"github.com/tu-org/embolsadora-api/internal/repo/pg/notifications"
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

func cleanupTenant(t *testing.T, pool *pgxpool.Pool, tenantID uuid.UUID) {
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM notifications WHERE tenant_id = $1`, tenantID)
	})
}

func seedNotification(t *testing.T, pool *pgxpool.Pool, tenantID uuid.UUID, severity, status string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	_, err := pool.Exec(context.Background(),
		`INSERT INTO notifications (id, tenant_id, title, message, severity, status)
		 VALUES ($1, $2, 'Alerta', 'algo paso', $3, $4)`,
		id, tenantID, severity, status)
	require.NoError(t, err)
	return id
}

func TestListFiltraPorStatusYSeverity(t *testing.T) {
	pool := newPool(t)
	repo := notifications.New(pool)
	tenantID := uuid.New()
	cleanupTenant(t, pool, tenantID)

	seedNotification(t, pool, tenantID, "critical", "unread")
	seedNotification(t, pool, tenantID, "info", "unread")
	seedNotification(t, pool, tenantID, "critical", "closed")

	status := "unread"
	severity := "critical"
	list, total, err := repo.List(context.Background(), tenantID, notifications.ListParams{
		Status: &status, Severity: &severity, Limit: 10,
	})
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	require.Len(t, list, 1)
	assert.Equal(t, "critical", string(list[0].Severity))
}

func TestCountUnread(t *testing.T) {
	pool := newPool(t)
	repo := notifications.New(pool)
	tenantID := uuid.New()
	cleanupTenant(t, pool, tenantID)

	seedNotification(t, pool, tenantID, "info", "unread")
	seedNotification(t, pool, tenantID, "info", "unread")
	seedNotification(t, pool, tenantID, "info", "closed")

	count, err := repo.CountUnread(context.Background(), tenantID)
	require.NoError(t, err)
	assert.Equal(t, 2, count)
}

func TestGetByIDNotFound(t *testing.T) {
	pool := newPool(t)
	repo := notifications.New(pool)
	_, err := repo.GetByID(context.Background(), uuid.New(), uuid.New())
	assert.ErrorIs(t, err, domain.ErrNotificationNotFound)
}

func TestAckEsIdempotenteYNoPisaAcknowledgedAtExistente(t *testing.T) {
	pool := newPool(t)
	repo := notifications.New(pool)
	tenantID := uuid.New()
	cleanupTenant(t, pool, tenantID)
	id := seedNotification(t, pool, tenantID, "warning", "unread")

	first, err := repo.Ack(context.Background(), id, tenantID)
	require.NoError(t, err)
	assert.Equal(t, string(domain.StatusAcknowledged), string(first.Status))
	require.NotNil(t, first.AcknowledgedAt)
	firstAckAt := *first.AcknowledgedAt

	second, err := repo.Ack(context.Background(), id, tenantID)
	require.NoError(t, err)
	assert.Equal(t, string(domain.StatusAcknowledged), string(second.Status))
	require.NotNil(t, second.AcknowledgedAt)
	assert.True(t, firstAckAt.Equal(*second.AcknowledgedAt), "un segundo Ack no debe mover el timestamp original")
}

func TestCloseDesdeCualquierEstadoYEsIdempotente(t *testing.T) {
	pool := newPool(t)
	repo := notifications.New(pool)
	tenantID := uuid.New()
	cleanupTenant(t, pool, tenantID)
	id := seedNotification(t, pool, tenantID, "critical", "unread")

	closed, err := repo.Close(context.Background(), id, tenantID)
	require.NoError(t, err)
	assert.Equal(t, string(domain.StatusClosed), string(closed.Status))
	require.NotNil(t, closed.ClosedAt)
	firstClosedAt := *closed.ClosedAt

	again, err := repo.Close(context.Background(), id, tenantID)
	require.NoError(t, err)
	assert.True(t, firstClosedAt.Equal(*again.ClosedAt), "cerrar una ya cerrada no debe mover el timestamp")
}

func TestAckDeIDInexistenteEsNotFound(t *testing.T) {
	pool := newPool(t)
	repo := notifications.New(pool)
	_, err := repo.Ack(context.Background(), uuid.New(), uuid.New())
	assert.ErrorIs(t, err, domain.ErrNotificationNotFound)
}
