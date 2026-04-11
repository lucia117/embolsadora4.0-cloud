package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// ErrNotificationNotFound se devuelve cuando una notificación no existe o pertenece a otro tenant.
var ErrNotificationNotFound = errors.New("notificación no encontrada")

// NotificationStatus representa el estado de atención de una notificación.
type NotificationStatus string

const (
	StatusUnread       NotificationStatus = "unread"
	StatusAcknowledged NotificationStatus = "acknowledged"
	StatusClosed       NotificationStatus = "closed"
)

// Notification representa un evento que requiere atención del operador.
// El campo Severity reutiliza el tipo Severity definido en logs.go (mismo package).
type Notification struct {
	ID             uuid.UUID
	TenantID       uuid.UUID
	Title          string
	Message        string
	Severity       Severity
	Status         NotificationStatus
	AlarmRuleID    *uuid.UUID
	MachineID      *uuid.UUID
	CreatedAt      time.Time
	AcknowledgedAt *time.Time
	ClosedAt       *time.Time
}
