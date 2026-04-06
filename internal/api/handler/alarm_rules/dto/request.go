package dto

// CreateAlarmRuleRequest es el DTO de entrada para crear una regla de alarma.
type CreateAlarmRuleRequest struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Metric      string  `json:"metric"`
	Operator    string  `json:"operator"`
	Threshold   float64 `json:"threshold"`
	Severity    string  `json:"severity"`
	Enabled     *bool   `json:"enabled"` // puntero para detectar omisión; default true
}

// UpdateAlarmRuleRequest es el DTO de entrada para actualizar una regla (PATCH parcial).
// Todos los campos son opcionales — nil significa "no actualizar".
type UpdateAlarmRuleRequest struct {
	Name        *string  `json:"name"`
	Description *string  `json:"description"`
	Metric      *string  `json:"metric"`
	Operator    *string  `json:"operator"`
	Threshold   *float64 `json:"threshold"`
	Severity    *string  `json:"severity"`
	Enabled     *bool    `json:"enabled"`
}
