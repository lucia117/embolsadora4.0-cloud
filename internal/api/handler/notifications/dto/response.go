package dto

import (
	"time"

	"github.com/tu-org/embolsadora-api/internal/domain"
)

// NotificationResponse es la representación HTTP de una notificación.
type NotificationResponse struct {
	ID             string     `json:"id"`
	TenantID       string     `json:"tenant_id"`
	Title          string     `json:"title"`
	Message        string     `json:"message"`
	Severity       string     `json:"severity"`
	Status         string     `json:"status"`
	AlarmRuleID    *string    `json:"alarm_rule_id"`
	MachineID      *string    `json:"machine_id"`
	CreatedAt      time.Time  `json:"created_at"`
	AcknowledgedAt *time.Time `json:"acknowledged_at"`
	ClosedAt       *time.Time `json:"closed_at"`
}

// FromDomain convierte un domain.Notification a NotificationResponse.
func FromDomain(n *domain.Notification) NotificationResponse {
	resp := NotificationResponse{
		ID:             n.ID.String(),
		TenantID:       n.TenantID.String(),
		Title:          n.Title,
		Message:        n.Message,
		Severity:       string(n.Severity),
		Status:         string(n.Status),
		CreatedAt:      n.CreatedAt,
		AcknowledgedAt: n.AcknowledgedAt,
		ClosedAt:       n.ClosedAt,
	}
	if n.AlarmRuleID != nil {
		s := n.AlarmRuleID.String()
		resp.AlarmRuleID = &s
	}
	if n.MachineID != nil {
		s := n.MachineID.String()
		resp.MachineID = &s
	}
	return resp
}

// NotificationListResponse es la respuesta paginada del listado de notificaciones.
type NotificationListResponse struct {
	Data   []NotificationResponse `json:"data"`
	Total  int                    `json:"total"`
	Limit  int                    `json:"limit"`
	Offset int                    `json:"offset"`
}

// NotificationCountResponse es la respuesta del conteo de no leídas.
type NotificationCountResponse struct {
	Unread int `json:"unread"`
}
