package dto

import (
	"time"

	"github.com/google/uuid"
	"github.com/tu-org/embolsadora-api/internal/domain"
)

// AlarmRuleResponse es el DTO de salida para una regla de alarma.
type AlarmRuleResponse struct {
	ID          uuid.UUID `json:"id"`
	TenantID    uuid.UUID `json:"tenantId"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Metric      string    `json:"metric"`
	Operator    string    `json:"operator"`
	Threshold   float64   `json:"threshold"`
	Severity    string    `json:"severity"`
	Enabled     bool      `json:"enabled"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// FromDomain convierte un domain.AlarmRule en AlarmRuleResponse.
func FromDomain(r *domain.AlarmRule) AlarmRuleResponse {
	return AlarmRuleResponse{
		ID:          r.ID,
		TenantID:    r.TenantID,
		Name:        r.Name,
		Description: r.Description,
		Metric:      r.Metric,
		Operator:    r.Operator,
		Threshold:   r.Threshold,
		Severity:    r.Severity,
		Enabled:     r.Enabled,
		CreatedAt:   r.CreatedAt,
		UpdatedAt:   r.UpdatedAt,
	}
}
