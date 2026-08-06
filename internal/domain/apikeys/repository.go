package apikeys

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

// ErrKeyNotFound indica que no existe ninguna key con ese key_id.
var ErrKeyNotFound = errors.New("apikeys: key no encontrada")

// Repository persiste y resuelve API keys de edge devices.
type Repository interface {
	// GetByKeyID resuelve la parte publica de la key a una credencial completa,
	// con el estado del device incluido. Devuelve ErrKeyNotFound si no existe.
	// NO filtra por revocada/vencida: esas verificaciones son del autenticador,
	// para que pueda distinguirlas en las metricas y en el log.
	GetByKeyID(ctx context.Context, keyID string) (*Credential, error)

	// Create persiste una key nueva.
	Create(ctx context.Context, k *APIKey) error

	// ListByDevice devuelve todas las keys de un device, nuevas primero.
	// Incluye las revocadas y vencidas: el ABM muestra el historial.
	ListByDevice(ctx context.Context, tenantID, deviceID uuid.UUID) ([]*APIKey, error)

	// Revoke marca la key como revocada. Es idempotente: revocar una key ya
	// revocada no cambia el revoked_at original. Devuelve ErrKeyNotFound si la
	// key no existe o no pertenece al tenant.
	Revoke(ctx context.Context, tenantID, keyPK uuid.UUID) error

	// TouchLastUsed actualiza last_used_at. Se llama de forma diferida y fuera
	// del camino critico: a 200 rps, un UPDATE por request serian 200 escrituras
	// por segundo sobre la misma fila.
	TouchLastUsed(ctx context.Context, keyPK uuid.UUID) error
}
