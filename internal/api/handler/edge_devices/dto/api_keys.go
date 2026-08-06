package dto

import "time"

// CreateAPIKeyRequest es el body para generar una key.
type CreateAPIKeyRequest struct {
	Name      string     `json:"name"`
	ExpiresAt *time.Time `json:"expiresAt"`
}

// APIKeyResponse describe una key SIN su secreto.
type APIKeyResponse struct {
	ID        string     `json:"id"`
	KeyID     string     `json:"keyId"`
	Name      *string    `json:"name"`
	CreatedAt time.Time  `json:"createdAt"`
	ExpiresAt *time.Time `json:"expiresAt"`
	RevokedAt *time.Time `json:"revokedAt"`
	// LastUsedAt: ultima vez que la key autentico un request de ingesta.
	LastUsedAt *time.Time `json:"lastUsedAt"`
	// Active responde SOLO "¿la key en si no esta revocada ni vencida?"
	// (domainapikeys.APIKey.IsActive). NO significa "esta key autentica
	// ingesta hoy": el autenticador (internal/security) ademas exige que el
	// device este ACTIVE, y esta respuesta no lo reflejaba — una key con
	// Active=true en un device DISABLED se muestra activa en el ABM mientras
	// la ingesta la rechaza con 403, sin que el operador tenga forma de ver
	// por que. DeviceStatus completa esa lectura: para saber si la key
	// realmente sirve, hay que mirar los dos campos juntos.
	Active bool `json:"active"`
	// DeviceStatus es el status del device al que pertenece la key
	// ("ACTIVE" | "DISABLED"), no de la key. Ver el comentario de Active.
	DeviceStatus string `json:"deviceStatus"`
}

// CreateAPIKeyResponse es la UNICA respuesta que incluye el secreto en claro.
// No se puede volver a consultar: no se persiste en ningun lado, solo su hash.
type CreateAPIKeyResponse struct {
	APIKeyResponse
	// Key es el valor completo emb_<keyId>_<secreto>. Se muestra una sola vez.
	Key string `json:"key"`
}
