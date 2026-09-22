package logs_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tu-org/embolsadora-api/internal/domain"
	"github.com/tu-org/embolsadora-api/internal/repo/pg/logs"
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

// cleanupTenant borra todo lo escrito bajo un tenant de test al terminar
// (log_entries y log_retention_policies no tienen FK a tenants, asi que un
// uuid.New() sin seed alcanza — pero hay que limpiar manualmente).
func cleanupTenant(t *testing.T, pool *pgxpool.Pool, tenantID uuid.UUID) {
	t.Cleanup(func() {
		ctx := context.Background()
		_, _ = pool.Exec(ctx, `DELETE FROM log_entries WHERE tenant_id = $1`, tenantID)
		_, _ = pool.Exec(ctx, `DELETE FROM log_retention_policies WHERE tenant_id = $1`, tenantID)
	})
}

func TestWriteAndGet(t *testing.T) {
	pool := newPool(t)
	repo := logs.New(pool)
	tenantID := uuid.New()
	cleanupTenant(t, pool, tenantID)

	entry := &domain.LogEntry{
		TenantID:  tenantID,
		Severity:  domain.SeverityCritical,
		EventType: domain.EventTypeAlarmTriggered,
		Message:   "temperatura fuera de rango",
		Metadata:  map[string]any{"value": 95.5},
	}
	written, err := repo.Write(context.Background(), entry)
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, written.ID)

	got, err := repo.Get(context.Background(), tenantID, written.ID)
	require.NoError(t, err)
	assert.Equal(t, "temperatura fuera de rango", got.Message)
	assert.Equal(t, domain.SeverityCritical, got.Severity)
	assert.Equal(t, 95.5, got.Metadata["value"])
}

func TestGetNotFound(t *testing.T) {
	pool := newPool(t)
	repo := logs.New(pool)
	_, err := repo.Get(context.Background(), uuid.New(), uuid.New())
	assert.ErrorIs(t, err, domain.ErrLogNotFound)
}

func TestListFiltraPorSeverityYPaginaConCursor(t *testing.T) {
	pool := newPool(t)
	repo := logs.New(pool)
	tenantID := uuid.New()
	cleanupTenant(t, pool, tenantID)
	ctx := context.Background()

	for i, sev := range []domain.Severity{domain.SeverityInfo, domain.SeverityCritical, domain.SeverityCritical} {
		_, err := repo.Write(ctx, &domain.LogEntry{
			TenantID: tenantID, Severity: sev, EventType: domain.EventTypeSystem,
			Message: "entry", Metadata: map[string]any{"i": i},
		})
		require.NoError(t, err)
		time.Sleep(time.Millisecond) // created_at distinto para orden determinista
	}

	page1, total, err := repo.List(ctx, logs.ListParams{TenantID: tenantID, Severity: "critical", Limit: 1})
	require.NoError(t, err)
	assert.Equal(t, 2, total)
	require.Len(t, page1, 1)

	cursor := logs.EncodeCursor(page1[0])
	page2, _, err := repo.List(ctx, logs.ListParams{TenantID: tenantID, Severity: "critical", Limit: 1, Cursor: cursor})
	require.NoError(t, err)
	require.Len(t, page2, 1)
	assert.NotEqual(t, page1[0].ID, page2[0].ID, "la segunda pagina no debe repetir la fila del cursor")
}

func TestListCursorInvalidoDevuelveError(t *testing.T) {
	pool := newPool(t)
	repo := logs.New(pool)
	_, _, err := repo.List(context.Background(), logs.ListParams{TenantID: uuid.New(), Cursor: "invalido!!"})
	assert.ErrorIs(t, err, domain.ErrInvalidCursor)
}

func TestGetContextDevuelveVentanaAlrededorDelAncla(t *testing.T) {
	pool := newPool(t)
	repo := logs.New(pool)
	tenantID := uuid.New()
	cleanupTenant(t, pool, tenantID)
	ctx := context.Background()

	var ids []uuid.UUID
	for i := 0; i < 5; i++ {
		e, err := repo.Write(ctx, &domain.LogEntry{
			TenantID: tenantID, Severity: domain.SeverityInfo, EventType: domain.EventTypeSystem,
			Message: "e", Metadata: map[string]any{},
		})
		require.NoError(t, err)
		ids = append(ids, e.ID)
		time.Sleep(time.Millisecond)
	}

	before, anchor, after, err := repo.GetContext(ctx, tenantID, ids[2], 2)
	require.NoError(t, err)
	assert.Equal(t, ids[2], anchor.ID)
	require.Len(t, before, 2)
	require.Len(t, after, 2)
	assert.Equal(t, ids[0], before[0].ID, "before debe quedar en orden cronologico ascendente")
	assert.Equal(t, ids[1], before[1].ID)
	assert.Equal(t, ids[3], after[0].ID)
	assert.Equal(t, ids[4], after[1].ID)
}

func TestExportRespetaMaxRows(t *testing.T) {
	pool := newPool(t)
	repo := logs.New(pool)
	tenantID := uuid.New()
	cleanupTenant(t, pool, tenantID)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		_, err := repo.Write(ctx, &domain.LogEntry{
			TenantID: tenantID, Severity: domain.SeverityInfo, EventType: domain.EventTypeSystem,
			Message: "e", Metadata: map[string]any{},
		})
		require.NoError(t, err)
	}

	rows, total, err := repo.Export(ctx, logs.ExportParams{TenantID: tenantID, MaxRows: 2})
	require.NoError(t, err)
	assert.Equal(t, 3, total, "total cuenta todas las que matchean, sin el limite")
	assert.Len(t, rows, 2)
}

func TestRetentionUpsertAndGet(t *testing.T) {
	pool := newPool(t)
	repo := logs.New(pool)
	tenantID := uuid.New()
	cleanupTenant(t, pool, tenantID)
	ctx := context.Background()

	_, err := repo.GetRetention(ctx, tenantID)
	assert.ErrorIs(t, err, domain.ErrRetentionNotFound)

	created, err := repo.UpsertRetention(ctx, &domain.RetentionPolicy{TenantID: tenantID, RetentionDays: 30})
	require.NoError(t, err)
	assert.Equal(t, 30, created.RetentionDays)

	updated, err := repo.UpsertRetention(ctx, &domain.RetentionPolicy{TenantID: tenantID, RetentionDays: 90})
	require.NoError(t, err)
	assert.Equal(t, 90, updated.RetentionDays)

	got, err := repo.GetRetention(ctx, tenantID)
	require.NoError(t, err)
	assert.Equal(t, 90, got.RetentionDays)
}
