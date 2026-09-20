package notifications_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	app "github.com/tu-org/embolsadora-api/internal/app/notifications"
	"github.com/tu-org/embolsadora-api/internal/domain"
	notifRepo "github.com/tu-org/embolsadora-api/internal/repo/pg/notifications"
)

type fakeRepo struct {
	listResult []*domain.Notification
	listTotal  int
	listErr    error
	lastListParams notifRepo.ListParams

	countUnreadResult int
	countUnreadErr    error

	getResult *domain.Notification
	getErr    error

	ackResult *domain.Notification
	ackErr    error

	closeResult *domain.Notification
	closeErr    error
}

func (f *fakeRepo) List(_ context.Context, _ uuid.UUID, params notifRepo.ListParams) ([]*domain.Notification, int, error) {
	f.lastListParams = params
	return f.listResult, f.listTotal, f.listErr
}
func (f *fakeRepo) CountUnread(context.Context, uuid.UUID) (int, error) {
	return f.countUnreadResult, f.countUnreadErr
}
func (f *fakeRepo) GetByID(context.Context, uuid.UUID, uuid.UUID) (*domain.Notification, error) {
	return f.getResult, f.getErr
}
func (f *fakeRepo) Ack(context.Context, uuid.UUID, uuid.UUID) (*domain.Notification, error) {
	return f.ackResult, f.ackErr
}
func (f *fakeRepo) Close(context.Context, uuid.UUID, uuid.UUID) (*domain.Notification, error) {
	return f.closeResult, f.closeErr
}

func TestList(t *testing.T) {
	tests := []struct {
		name       string
		in         notifRepo.ListParams
		wantLimit  int
		wantOffset int
	}{
		{"limit <= 0 clampea a 20", notifRepo.ListParams{Limit: 0}, 20, 0},
		{"limit > 100 clampea a 100", notifRepo.ListParams{Limit: 500}, 100, 0},
		{"offset negativo clampea a 0", notifRepo.ListParams{Limit: 10, Offset: -5}, 10, 0},
		{"dentro de rango se respeta", notifRepo.ListParams{Limit: 30, Offset: 10}, 30, 10},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeRepo{}
			svc := app.New(repo, zap.NewNop())
			_, _, err := svc.List(context.Background(), uuid.New(), tt.in)
			require.NoError(t, err)
			require.Equal(t, tt.wantLimit, repo.lastListParams.Limit)
			require.Equal(t, tt.wantOffset, repo.lastListParams.Offset)
		})
	}
	t.Run("error de repo", func(t *testing.T) {
		svc := app.New(&fakeRepo{listErr: errors.New("db down")}, zap.NewNop())
		_, _, err := svc.List(context.Background(), uuid.New(), notifRepo.ListParams{})
		require.Error(t, err)
	})
}

func TestCountUnread(t *testing.T) {
	t.Run("feliz", func(t *testing.T) {
		svc := app.New(&fakeRepo{countUnreadResult: 7}, zap.NewNop())
		got, err := svc.CountUnread(context.Background(), uuid.New())
		require.NoError(t, err)
		require.Equal(t, 7, got)
	})
	t.Run("error", func(t *testing.T) {
		svc := app.New(&fakeRepo{countUnreadErr: errors.New("db down")}, zap.NewNop())
		_, err := svc.CountUnread(context.Background(), uuid.New())
		require.Error(t, err)
	})
}

func TestGetAckClose(t *testing.T) {
	notif := &domain.Notification{ID: uuid.New(), Status: domain.StatusAcknowledged}

	cases := []struct {
		name string
		call func(*app.Service, *fakeRepo) (*domain.Notification, error)
		setResult func(*fakeRepo, *domain.Notification)
		setErr    func(*fakeRepo, error)
	}{
		{"Get", func(s *app.Service, r *fakeRepo) (*domain.Notification, error) {
			return s.Get(context.Background(), notif.ID, uuid.New())
		}, func(r *fakeRepo, n *domain.Notification) { r.getResult = n }, func(r *fakeRepo, e error) { r.getErr = e }},
		{"Ack", func(s *app.Service, r *fakeRepo) (*domain.Notification, error) {
			return s.Ack(context.Background(), notif.ID, uuid.New())
		}, func(r *fakeRepo, n *domain.Notification) { r.ackResult = n }, func(r *fakeRepo, e error) { r.ackErr = e }},
		{"Close", func(s *app.Service, r *fakeRepo) (*domain.Notification, error) {
			return s.Close(context.Background(), notif.ID, uuid.New())
		}, func(r *fakeRepo, n *domain.Notification) { r.closeResult = n }, func(r *fakeRepo, e error) { r.closeErr = e }},
	}

	for _, tc := range cases {
		t.Run(tc.name+"/feliz", func(t *testing.T) {
			repo := &fakeRepo{}
			tc.setResult(repo, notif)
			svc := app.New(repo, zap.NewNop())
			got, err := tc.call(svc, repo)
			require.NoError(t, err)
			require.Equal(t, notif, got)
		})
		t.Run(tc.name+"/no_encontrada", func(t *testing.T) {
			repo := &fakeRepo{}
			tc.setErr(repo, domain.ErrNotificationNotFound)
			svc := app.New(repo, zap.NewNop())
			_, err := tc.call(svc, repo)
			require.ErrorIs(t, err, domain.ErrNotificationNotFound)
		})
		t.Run(tc.name+"/otro_error", func(t *testing.T) {
			repo := &fakeRepo{}
			tc.setErr(repo, errors.New("db down"))
			svc := app.New(repo, zap.NewNop())
			_, err := tc.call(svc, repo)
			require.Error(t, err)
			require.NotErrorIs(t, err, domain.ErrNotificationNotFound)
		})
	}
}
