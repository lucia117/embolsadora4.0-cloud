package alarm_rules

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"github.com/tu-org/embolsadora-api/internal/domain"
	alarmRepo "github.com/tu-org/embolsadora-api/internal/repo/pg/alarm_rules"
)

// ErrInvalidOperator se devuelve cuando el operador no es válido.
var ErrInvalidOperator = errors.New("operator debe ser uno de: gt, lt, gte, lte, eq")

// ErrInvalidSeverity se devuelve cuando la severidad no es válida.
var ErrInvalidSeverity = errors.New("severity debe ser uno de: info, warning, critical")

// ErrNameRequired se devuelve cuando el nombre está vacío.
var ErrNameRequired = errors.New("el campo 'name' es requerido")

// ErrMetricRequired se devuelve cuando la métrica está vacía.
var ErrMetricRequired = errors.New("el campo 'metric' es requerido")

// Service contiene la lógica de negocio para gestión de reglas de alarma.
type Service struct {
	repo   alarmRepo.Repository
	logger *zap.Logger
}

// NewService crea un nuevo servicio de alarm rules.
func NewService(repo alarmRepo.Repository, logger *zap.Logger) *Service {
	return &Service{repo: repo, logger: logger}
}

// ListAlarmRules devuelve todas las reglas del tenant.
func (s *Service) ListAlarmRules(ctx context.Context, tenantID uuid.UUID) ([]*domain.AlarmRule, error) {
	rules, err := s.repo.List(ctx, tenantID)
	if err != nil {
		s.logger.Error("error listando alarm rules", zap.String("tenant_id", tenantID.String()), zap.Error(err))
		return nil, err
	}
	return rules, nil
}

// GetAlarmRule devuelve una regla por ID verificando el tenant.
func (s *Service) GetAlarmRule(ctx context.Context, id uuid.UUID, tenantID uuid.UUID) (*domain.AlarmRule, error) {
	rule, err := s.repo.GetByID(ctx, id, tenantID)
	if err != nil {
		if !errors.Is(err, domain.ErrAlarmRuleNotFound) {
			s.logger.Error("error obteniendo alarm rule", zap.String("rule_id", id.String()), zap.Error(err))
		}
		return nil, err
	}
	return rule, nil
}

// CreateAlarmRuleInput contiene los campos para crear una regla.
type CreateAlarmRuleInput struct {
	Name        string
	Description string
	Metric      string
	Operator    string
	Threshold   float64
	Severity    string
	Enabled     bool
}

// CreateAlarmRule valida y persiste una nueva regla.
func (s *Service) CreateAlarmRule(ctx context.Context, tenantID uuid.UUID, input CreateAlarmRuleInput) (*domain.AlarmRule, error) {
	if input.Name == "" {
		return nil, ErrNameRequired
	}
	if input.Metric == "" {
		return nil, ErrMetricRequired
	}
	if !domain.ValidateOperator(input.Operator) {
		return nil, ErrInvalidOperator
	}
	if !domain.ValidateSeverity(input.Severity) {
		return nil, ErrInvalidSeverity
	}

	rule := &domain.AlarmRule{
		TenantID:    tenantID,
		Name:        input.Name,
		Description: input.Description,
		Metric:      input.Metric,
		Operator:    input.Operator,
		Threshold:   input.Threshold,
		Severity:    input.Severity,
		Enabled:     input.Enabled,
	}

	if err := s.repo.Create(ctx, rule); err != nil {
		s.logger.Error("error creando alarm rule", zap.String("tenant_id", tenantID.String()), zap.Error(err))
		return nil, err
	}

	s.logger.Info("alarm rule creada", zap.String("rule_id", rule.ID.String()), zap.String("tenant_id", tenantID.String()))
	return rule, nil
}

// UpdateAlarmRuleInput contiene los campos opcionales para actualizar una regla.
// Los punteros nil indican "campo no enviado — no actualizar".
type UpdateAlarmRuleInput struct {
	Name        *string
	Description *string
	Metric      *string
	Operator    *string
	Threshold   *float64
	Severity    *string
	Enabled     *bool
}

// UpdateAlarmRule aplica los campos no-nil sobre la regla existente.
func (s *Service) UpdateAlarmRule(ctx context.Context, id uuid.UUID, tenantID uuid.UUID, input UpdateAlarmRuleInput) (*domain.AlarmRule, error) {
	rule, err := s.repo.GetByID(ctx, id, tenantID)
	if err != nil {
		return nil, err
	}

	if input.Name != nil {
		if *input.Name == "" {
			return nil, ErrNameRequired
		}
		rule.Name = *input.Name
	}
	if input.Description != nil {
		rule.Description = *input.Description
	}
	if input.Metric != nil {
		if *input.Metric == "" {
			return nil, ErrMetricRequired
		}
		rule.Metric = *input.Metric
	}
	if input.Operator != nil {
		if !domain.ValidateOperator(*input.Operator) {
			return nil, ErrInvalidOperator
		}
		rule.Operator = *input.Operator
	}
	if input.Threshold != nil {
		rule.Threshold = *input.Threshold
	}
	if input.Severity != nil {
		if !domain.ValidateSeverity(*input.Severity) {
			return nil, ErrInvalidSeverity
		}
		rule.Severity = *input.Severity
	}
	if input.Enabled != nil {
		rule.Enabled = *input.Enabled
	}

	if err := s.repo.Update(ctx, rule); err != nil {
		if !errors.Is(err, domain.ErrAlarmRuleNotFound) {
			s.logger.Error("error actualizando alarm rule", zap.String("rule_id", id.String()), zap.Error(err))
		}
		return nil, err
	}

	s.logger.Info("alarm rule actualizada", zap.String("rule_id", id.String()))
	return rule, nil
}

// DeleteAlarmRule elimina permanentemente una regla del tenant.
func (s *Service) DeleteAlarmRule(ctx context.Context, id uuid.UUID, tenantID uuid.UUID) error {
	if err := s.repo.Delete(ctx, id, tenantID); err != nil {
		if !errors.Is(err, domain.ErrAlarmRuleNotFound) {
			s.logger.Error("error eliminando alarm rule", zap.String("rule_id", id.String()), zap.Error(err))
		}
		return err
	}
	s.logger.Info("alarm rule eliminada", zap.String("rule_id", id.String()))
	return nil
}
