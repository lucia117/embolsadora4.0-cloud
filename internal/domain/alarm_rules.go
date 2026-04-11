package domain

import (
	"errors"
	"slices"
	"time"

	"github.com/google/uuid"
)

// ErrAlarmRuleNotFound se devuelve cuando una regla de alarma no existe o pertenece a otro tenant.
var ErrAlarmRuleNotFound = errors.New("regla de alarma no encontrada")

// ValidOperators lista los operadores de comparación aceptados.
var ValidOperators = []string{"gt", "lt", "gte", "lte", "eq"}

// ValidSeverities lista los niveles de severidad aceptados.
var ValidSeverities = []string{"info", "warning", "critical"}

// AlarmRule representa una condición de alerta configurable para un tenant.
type AlarmRule struct {
	ID          uuid.UUID
	TenantID    uuid.UUID
	Name        string
	Description string
	Metric      string
	Operator    string
	Threshold   float64
	Severity    string
	Enabled     bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// ValidateOperator devuelve true si op es un operador válido.
func ValidateOperator(op string) bool {
	return slices.Contains(ValidOperators, op)
}

// ValidateSeverity devuelve true si s es una severidad válida.
func ValidateSeverity(s string) bool {
	return slices.Contains(ValidSeverities, s)
}
