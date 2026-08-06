package apikeys

import (
	"time"

	"github.com/google/uuid"
)

// APIKey es el registro persistido de una credencial de edge device.
// El secreto en claro no se almacena nunca: solo su SHA-256 en KeyHash.
type APIKey struct {
	ID         uuid.UUID
	TenantID   uuid.UUID
	DeviceID   uuid.UUID
	KeyID      string
	KeyHash    []byte
	Name       *string
	CreatedAt  time.Time
	CreatedBy  *uuid.UUID
	ExpiresAt  *time.Time
	RevokedAt  *time.Time
	LastUsedAt *time.Time
}

// IsActive reporta si la key sirve para autenticar en el instante `now`.
// Una key vencida o revocada no autentica, pero sigue existiendo en la tabla
// para que el ABM pueda mostrar el historial.
func (k *APIKey) IsActive(now time.Time) bool {
	if k.RevokedAt != nil {
		return false
	}
	if k.ExpiresAt != nil && !k.ExpiresAt.After(now) {
		return false
	}
	return true
}

// Credential es el resultado del lookup por key_id: la key mas el estado del
// device al que pertenece. Se resuelve con un unico JOIN para que el camino
// caliente de la ingesta no haga dos roundtrips a Postgres por request.
type Credential struct {
	KeyPK        uuid.UUID
	TenantID     uuid.UUID
	DeviceID     uuid.UUID
	KeyID        string
	KeyHash      []byte
	ExpiresAt    *time.Time
	RevokedAt    *time.Time
	MachineID    string
	DeviceStatus string // "ACTIVE" | "DISABLED"
}

// DeviceIdentity es la identidad resuelta que viaja en el contexto del request
// una vez que la API key fue validada. Es la unica fuente de tenant y device
// para la ingesta: el body del request nunca los aporta (D-10).
type DeviceIdentity struct {
	TenantID  uuid.UUID
	DeviceID  uuid.UUID
	MachineID string
	KeyPK     uuid.UUID
	KeyID     string
}
