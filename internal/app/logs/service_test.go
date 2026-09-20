package logs_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	app "github.com/tu-org/embolsadora-api/internal/app/logs"
	"github.com/tu-org/embolsadora-api/internal/domain"
	logsRepo "github.com/tu-org/embolsadora-api/internal/repo/pg/logs"
)

type fakeRepo struct {
	listResult     []domain.LogEntry
	listTotal      int
	listErr        error
	lastListParams logsRepo.ListParams

	getResult *domain.LogEntry
	getErr    error

	getContextBefore []domain.LogEntry
	getContextAnchor *domain.LogEntry
	getContextAfter  []domain.LogEntry
	getContextErr    error
	lastWindowSize   int

	exportResult     []domain.LogEntry
	exportTotal      int
	exportErr        error
	lastExportParams logsRepo.ExportParams

	retentionResult *domain.RetentionPolicy
	retentionErr    error

	upsertRetentionResult *domain.RetentionPolicy
	upsertRetentionErr    error

	writeResult *domain.LogEntry
	writeErr    error
}

func (f *fakeRepo) Write(_ context.Context, entry *domain.LogEntry) (*domain.LogEntry, error) {
	if f.writeErr != nil {
		return nil, f.writeErr
	}
	if f.writeResult != nil {
		return f.writeResult, nil
	}
	return entry, nil
}
func (f *fakeRepo) List(_ context.Context, params logsRepo.ListParams) ([]domain.LogEntry, int, error) {
	f.lastListParams = params
	return f.listResult, f.listTotal, f.listErr
}
func (f *fakeRepo) Get(context.Context, uuid.UUID, uuid.UUID) (*domain.LogEntry, error) {
	return f.getResult, f.getErr
}
func (f *fakeRepo) GetContext(_ context.Context, _ uuid.UUID, _ uuid.UUID, windowSize int) ([]domain.LogEntry, *domain.LogEntry, []domain.LogEntry, error) {
	f.lastWindowSize = windowSize
	return f.getContextBefore, f.getContextAnchor, f.getContextAfter, f.getContextErr
}
func (f *fakeRepo) Export(_ context.Context, params logsRepo.ExportParams) ([]domain.LogEntry, int, error) {
	f.lastExportParams = params
	return f.exportResult, f.exportTotal, f.exportErr
}
func (f *fakeRepo) GetRetention(context.Context, uuid.UUID) (*domain.RetentionPolicy, error) {
	return f.retentionResult, f.retentionErr
}
func (f *fakeRepo) UpsertRetention(context.Context, *domain.RetentionPolicy) (*domain.RetentionPolicy, error) {
	return f.upsertRetentionResult, f.upsertRetentionErr
}

func TestList(t *testing.T) {
	t.Run("limit menor o igual a 0 clampea a 50", func(t *testing.T) {
		repo := &fakeRepo{}
		svc := app.New(repo, zap.NewNop())
		_, err := svc.List(context.Background(), logsRepo.ListParams{Limit: 0})
		require.NoError(t, err)
		require.Equal(t, 50, repo.lastListParams.Limit)
	})
	t.Run("limit mayor a 100 clampea a 50", func(t *testing.T) {
		repo := &fakeRepo{}
		svc := app.New(repo, zap.NewNop())
		_, err := svc.List(context.Background(), logsRepo.ListParams{Limit: 500})
		require.NoError(t, err)
		require.Equal(t, 50, repo.lastListParams.Limit)
	})
	t.Run("trae exactamente el limit: hay NextCursor", func(t *testing.T) {
		entries := make([]domain.LogEntry, 10)
		for i := range entries {
			entries[i] = domain.LogEntry{ID: uuid.New(), CreatedAt: time.Now()}
		}
		repo := &fakeRepo{listResult: entries, listTotal: 100}
		svc := app.New(repo, zap.NewNop())
		got, err := svc.List(context.Background(), logsRepo.ListParams{Limit: 10})
		require.NoError(t, err)
		require.NotNil(t, got.NextCursor)
		require.Equal(t, 100, got.Total)
	})
	t.Run("trae menos que el limit: no hay NextCursor", func(t *testing.T) {
		repo := &fakeRepo{listResult: []domain.LogEntry{{ID: uuid.New(), CreatedAt: time.Now()}}}
		svc := app.New(repo, zap.NewNop())
		got, err := svc.List(context.Background(), logsRepo.ListParams{Limit: 50})
		require.NoError(t, err)
		require.Nil(t, got.NextCursor)
	})
	t.Run("error de repo", func(t *testing.T) {
		svc := app.New(&fakeRepo{listErr: errors.New("db down")}, zap.NewNop())
		_, err := svc.List(context.Background(), logsRepo.ListParams{})
		require.Error(t, err)
	})
}

func TestGet(t *testing.T) {
	want := &domain.LogEntry{ID: uuid.New()}
	svc := app.New(&fakeRepo{getResult: want}, zap.NewNop())
	got, err := svc.Get(context.Background(), uuid.New(), want.ID)
	require.NoError(t, err)
	require.Equal(t, want, got)
}

func TestGetContext(t *testing.T) {
	tests := []struct {
		name  string
		input int
		want  int
	}{
		{"cero clampea a 10", 0, 10},
		{"negativo clampea a 10", -5, 10},
		{"mayor a 50 clampea a 10", 51, 10},
		{"dentro de rango se respeta", 20, 20},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeRepo{}
			svc := app.New(repo, zap.NewNop())
			_, _, _, err := svc.GetContext(context.Background(), uuid.New(), uuid.New(), tt.input)
			require.NoError(t, err)
			require.Equal(t, tt.want, repo.lastWindowSize)
		})
	}
}

func TestExport(t *testing.T) {
	t.Run("fuerza MaxRows a 50001 sin importar el input", func(t *testing.T) {
		repo := &fakeRepo{}
		svc := app.New(repo, zap.NewNop())
		_, err := svc.Export(context.Background(), logsRepo.ExportParams{MaxRows: 10})
		require.NoError(t, err)
		require.Equal(t, 50001, repo.lastExportParams.MaxRows)
	})
	t.Run("por debajo del limite no trunca", func(t *testing.T) {
		entries := make([]domain.LogEntry, 100)
		repo := &fakeRepo{exportResult: entries, exportTotal: 100}
		svc := app.New(repo, zap.NewNop())
		got, err := svc.Export(context.Background(), logsRepo.ExportParams{})
		require.NoError(t, err)
		require.False(t, got.Truncated)
		require.Len(t, got.Entries, 100)
	})
	t.Run("por encima de 50000 trunca y marca Truncated", func(t *testing.T) {
		entries := make([]domain.LogEntry, 50001)
		repo := &fakeRepo{exportResult: entries, exportTotal: 60000}
		svc := app.New(repo, zap.NewNop())
		got, err := svc.Export(context.Background(), logsRepo.ExportParams{})
		require.NoError(t, err)
		require.True(t, got.Truncated)
		require.Len(t, got.Entries, 50000)
		require.Equal(t, 60000, got.TotalAvailable)
	})
	t.Run("error de repo", func(t *testing.T) {
		svc := app.New(&fakeRepo{exportErr: errors.New("db down")}, zap.NewNop())
		_, err := svc.Export(context.Background(), logsRepo.ExportParams{})
		require.Error(t, err)
	})
}

func TestGetRetention(t *testing.T) {
	tenantID := uuid.New()
	t.Run("sin politica devuelve default 90 dias sin persistir", func(t *testing.T) {
		repo := &fakeRepo{retentionErr: domain.ErrRetentionNotFound}
		svc := app.New(repo, zap.NewNop())
		got, err := svc.GetRetention(context.Background(), tenantID)
		require.NoError(t, err)
		require.Equal(t, 90, got.RetentionDays)
		require.Equal(t, tenantID, got.TenantID)
	})
	t.Run("politica existente pasa tal cual", func(t *testing.T) {
		want := &domain.RetentionPolicy{TenantID: tenantID, RetentionDays: 30}
		repo := &fakeRepo{retentionResult: want}
		svc := app.New(repo, zap.NewNop())
		got, err := svc.GetRetention(context.Background(), tenantID)
		require.NoError(t, err)
		require.Equal(t, want, got)
	})
	t.Run("otro error propaga", func(t *testing.T) {
		repo := &fakeRepo{retentionErr: errors.New("db down")}
		svc := app.New(repo, zap.NewNop())
		_, err := svc.GetRetention(context.Background(), tenantID)
		require.Error(t, err)
	})
}

func TestUpdateRetention(t *testing.T) {
	tenantID := uuid.New()
	t.Run("menor a 1 falla", func(t *testing.T) {
		svc := app.New(&fakeRepo{}, zap.NewNop())
		_, err := svc.UpdateRetention(context.Background(), tenantID, 0)
		require.ErrorIs(t, err, domain.ErrInvalidRetentionDays)
	})
	t.Run("mayor a 3650 falla", func(t *testing.T) {
		svc := app.New(&fakeRepo{}, zap.NewNop())
		_, err := svc.UpdateRetention(context.Background(), tenantID, 3651)
		require.ErrorIs(t, err, domain.ErrInvalidRetentionDays)
	})
	t.Run("feliz", func(t *testing.T) {
		want := &domain.RetentionPolicy{TenantID: tenantID, RetentionDays: 60}
		repo := &fakeRepo{upsertRetentionResult: want}
		svc := app.New(repo, zap.NewNop())
		got, err := svc.UpdateRetention(context.Background(), tenantID, 60)
		require.NoError(t, err)
		require.Equal(t, want, got)
	})
}

func TestSubscribePublishUnsubscribe(t *testing.T) {
	tenantID := uuid.New()
	svc := app.New(&fakeRepo{}, zap.NewNop())

	ch := svc.Subscribe(tenantID)
	entry := &domain.LogEntry{ID: uuid.New(), TenantID: tenantID}
	svc.Publish(tenantID, entry)

	select {
	case got := <-ch:
		require.Equal(t, entry, got)
	case <-time.After(time.Second):
		t.Fatal("no se recibió el entry publicado")
	}

	svc.Unsubscribe(tenantID, ch)
	svc.Publish(tenantID, &domain.LogEntry{ID: uuid.New(), TenantID: tenantID})
	select {
	case _, ok := <-ch:
		require.False(t, ok, "el channel debe estar cerrado tras Unsubscribe")
	case <-time.After(100 * time.Millisecond):
		t.Fatal("el channel debería estar cerrado, no bloqueado")
	}

	require.NotPanics(t, func() { svc.Unsubscribe(tenantID, ch) }, "un segundo Unsubscribe del mismo channel no debe panickear")
}

func TestPublishConSubscriberLentoNoBloquea(t *testing.T) {
	tenantID := uuid.New()
	svc := app.New(&fakeRepo{}, zap.NewNop())
	svc.Subscribe(tenantID) // nadie lee de este channel

	done := make(chan struct{})
	go func() {
		for i := 0; i < 100; i++ {
			svc.Publish(tenantID, &domain.LogEntry{ID: uuid.New(), TenantID: tenantID})
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Publish no debe bloquear cuando el subscriber está lleno/lento")
	}
}

func TestWritePublicaAlTenantDelEntryPersistido(t *testing.T) {
	tenantID := uuid.New()
	persisted := &domain.LogEntry{ID: uuid.New(), TenantID: tenantID}
	repo := &fakeRepo{writeResult: persisted}
	svc := app.New(repo, zap.NewNop())

	ch := svc.Subscribe(tenantID)
	err := svc.Write(context.Background(), &domain.LogEntry{TenantID: tenantID})
	require.NoError(t, err)

	select {
	case got := <-ch:
		require.Equal(t, persisted, got)
	case <-time.After(time.Second):
		t.Fatal("Write debe publicar el entry persistido a los suscriptores")
	}
}

func TestWriteErrorDeRepoNoPublica(t *testing.T) {
	svc := app.New(&fakeRepo{writeErr: errors.New("db down")}, zap.NewNop())
	err := svc.Write(context.Background(), &domain.LogEntry{})
	require.Error(t, err)
}
