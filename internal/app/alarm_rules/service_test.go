package alarm_rules_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	app "github.com/tu-org/embolsadora-api/internal/app/alarm_rules"
	"github.com/tu-org/embolsadora-api/internal/domain"
)

type fakeRepo struct {
	listResult []*domain.AlarmRule
	listErr    error

	getResult *domain.AlarmRule
	getErr    error

	createErr   error
	createCalls []*domain.AlarmRule

	updateErr   error
	updateCalls []*domain.AlarmRule

	deleteErr error
}

func (f *fakeRepo) List(context.Context, uuid.UUID) ([]*domain.AlarmRule, error) {
	return f.listResult, f.listErr
}
func (f *fakeRepo) GetByID(context.Context, uuid.UUID, uuid.UUID) (*domain.AlarmRule, error) {
	return f.getResult, f.getErr
}
func (f *fakeRepo) Create(_ context.Context, r *domain.AlarmRule) error {
	f.createCalls = append(f.createCalls, r)
	return f.createErr
}
func (f *fakeRepo) Update(_ context.Context, r *domain.AlarmRule) error {
	f.updateCalls = append(f.updateCalls, r)
	return f.updateErr
}
func (f *fakeRepo) Delete(context.Context, uuid.UUID, uuid.UUID) error { return f.deleteErr }

func validInput() app.CreateAlarmRuleInput {
	return app.CreateAlarmRuleInput{
		Name: "Temp alta", Metric: "temperature", Operator: "gt", Threshold: 80, Severity: "critical", Enabled: true,
	}
}

func TestListAlarmRules(t *testing.T) {
	t.Run("feliz", func(t *testing.T) {
		want := []*domain.AlarmRule{{ID: uuid.New()}}
		svc := app.NewService(&fakeRepo{listResult: want}, zap.NewNop())
		got, err := svc.ListAlarmRules(context.Background(), uuid.New())
		require.NoError(t, err)
		require.Equal(t, want, got)
	})
	t.Run("error de repo", func(t *testing.T) {
		svc := app.NewService(&fakeRepo{listErr: errors.New("db down")}, zap.NewNop())
		_, err := svc.ListAlarmRules(context.Background(), uuid.New())
		require.Error(t, err)
	})
}

func TestGetAlarmRule(t *testing.T) {
	t.Run("feliz", func(t *testing.T) {
		want := &domain.AlarmRule{ID: uuid.New()}
		svc := app.NewService(&fakeRepo{getResult: want}, zap.NewNop())
		got, err := svc.GetAlarmRule(context.Background(), want.ID, uuid.New())
		require.NoError(t, err)
		require.Equal(t, want, got)
	})
	t.Run("no encontrada", func(t *testing.T) {
		svc := app.NewService(&fakeRepo{getErr: domain.ErrAlarmRuleNotFound}, zap.NewNop())
		_, err := svc.GetAlarmRule(context.Background(), uuid.New(), uuid.New())
		require.ErrorIs(t, err, domain.ErrAlarmRuleNotFound)
	})
}

func TestCreateAlarmRule(t *testing.T) {
	tenantID := uuid.New()

	tests := []struct {
		name    string
		mutate  func(*app.CreateAlarmRuleInput)
		wantErr error
	}{
		{"nombre vacio", func(i *app.CreateAlarmRuleInput) { i.Name = "" }, app.ErrNameRequired},
		{"metrica vacia", func(i *app.CreateAlarmRuleInput) { i.Metric = "" }, app.ErrMetricRequired},
		{"operador invalido", func(i *app.CreateAlarmRuleInput) { i.Operator = "contains" }, app.ErrInvalidOperator},
		{"severidad invalida", func(i *app.CreateAlarmRuleInput) { i.Severity = "urgent" }, app.ErrInvalidSeverity},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := validInput()
			tt.mutate(&input)
			repo := &fakeRepo{}
			svc := app.NewService(repo, zap.NewNop())
			_, err := svc.CreateAlarmRule(context.Background(), tenantID, input)
			require.ErrorIs(t, err, tt.wantErr)
			require.Empty(t, repo.createCalls, "una validacion fallida no debe llegar al repo")
		})
	}

	t.Run("feliz", func(t *testing.T) {
		repo := &fakeRepo{}
		svc := app.NewService(repo, zap.NewNop())
		got, err := svc.CreateAlarmRule(context.Background(), tenantID, validInput())
		require.NoError(t, err)
		require.Equal(t, tenantID, got.TenantID)
		require.Equal(t, "gt", got.Operator)
		require.Len(t, repo.createCalls, 1)
	})

	t.Run("error de repo", func(t *testing.T) {
		repo := &fakeRepo{createErr: errors.New("db down")}
		svc := app.NewService(repo, zap.NewNop())
		_, err := svc.CreateAlarmRule(context.Background(), tenantID, validInput())
		require.Error(t, err)
	})
}

func strp(s string) *string   { return &s }
func f64p(f float64) *float64 { return &f }
func boolp(b bool) *bool      { return &b }

func TestUpdateAlarmRule(t *testing.T) {
	tenantID, ruleID := uuid.New(), uuid.New()
	existing := &domain.AlarmRule{ID: ruleID, TenantID: tenantID, Name: "Vieja", Metric: "temp", Operator: "gt", Severity: "info", Threshold: 10, Enabled: false}

	t.Run("regla no encontrada no llama Update", func(t *testing.T) {
		repo := &fakeRepo{getErr: domain.ErrAlarmRuleNotFound}
		svc := app.NewService(repo, zap.NewNop())
		_, err := svc.UpdateAlarmRule(context.Background(), ruleID, tenantID, app.UpdateAlarmRuleInput{})
		require.ErrorIs(t, err, domain.ErrAlarmRuleNotFound)
		require.Empty(t, repo.updateCalls)
	})

	t.Run("solo threshold y enabled, sin validacion", func(t *testing.T) {
		repo := &fakeRepo{getResult: existing}
		svc := app.NewService(repo, zap.NewNop())
		got, err := svc.UpdateAlarmRule(context.Background(), ruleID, tenantID, app.UpdateAlarmRuleInput{
			Threshold: f64p(999), Enabled: boolp(true),
		})
		require.NoError(t, err)
		require.Equal(t, 999.0, got.Threshold)
		require.True(t, got.Enabled)
		require.Equal(t, "Vieja", got.Name, "campos no enviados no cambian")
	})

	t.Run("nombre vacio explicito falla", func(t *testing.T) {
		repo := &fakeRepo{getResult: existing}
		svc := app.NewService(repo, zap.NewNop())
		_, err := svc.UpdateAlarmRule(context.Background(), ruleID, tenantID, app.UpdateAlarmRuleInput{Name: strp("")})
		require.ErrorIs(t, err, app.ErrNameRequired)
	})

	t.Run("operador invalido explicito falla", func(t *testing.T) {
		repo := &fakeRepo{getResult: existing}
		svc := app.NewService(repo, zap.NewNop())
		_, err := svc.UpdateAlarmRule(context.Background(), ruleID, tenantID, app.UpdateAlarmRuleInput{Operator: strp("contains")})
		require.ErrorIs(t, err, app.ErrInvalidOperator)
	})

	t.Run("feliz actualiza todos los campos enviados", func(t *testing.T) {
		repo := &fakeRepo{getResult: existing}
		svc := app.NewService(repo, zap.NewNop())
		got, err := svc.UpdateAlarmRule(context.Background(), ruleID, tenantID, app.UpdateAlarmRuleInput{
			Name: strp("Nueva"), Operator: strp("lte"), Severity: strp("warning"),
		})
		require.NoError(t, err)
		require.Equal(t, "Nueva", got.Name)
		require.Equal(t, "lte", got.Operator)
		require.Equal(t, "warning", got.Severity)
		require.Len(t, repo.updateCalls, 1)
	})
}

func TestDeleteAlarmRule(t *testing.T) {
	t.Run("no encontrada", func(t *testing.T) {
		svc := app.NewService(&fakeRepo{deleteErr: domain.ErrAlarmRuleNotFound}, zap.NewNop())
		err := svc.DeleteAlarmRule(context.Background(), uuid.New(), uuid.New())
		require.ErrorIs(t, err, domain.ErrAlarmRuleNotFound)
	})
	t.Run("otro error", func(t *testing.T) {
		svc := app.NewService(&fakeRepo{deleteErr: errors.New("db down")}, zap.NewNop())
		err := svc.DeleteAlarmRule(context.Background(), uuid.New(), uuid.New())
		require.Error(t, err)
	})
	t.Run("feliz", func(t *testing.T) {
		svc := app.NewService(&fakeRepo{}, zap.NewNop())
		err := svc.DeleteAlarmRule(context.Background(), uuid.New(), uuid.New())
		require.NoError(t, err)
	})
}
