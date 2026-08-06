package dto

import "time"

// CreateAPIKeyRequest es el body para generar una key.
type CreateAPIKeyRequest struct {
	Name      string     `json:"name"`
	ExpiresAt *time.Time `json:"expiresAt"`
}

// APIKeyResponse describe una key SIN su secreto.
type APIKeyResponse struct {
	ID         string     `json:"id"`
	KeyID      string     `json:"keyId"`
	Name       *string    `json:"name"`
	CreatedAt  time.Time  `json:"createdAt"`
	ExpiresAt  *time.Time `json:"expiresAt"`
	RevokedAt  *time.Time `json:"revokedAt"`
	LastUsedAt *time.Time `json:"lastUsedAt"`
	Active     bool       `json:"active"`
}

// CreateAPIKeyResponse es la UNICA respuesta que incluye el secreto en claro.
// No se puede volver a consultar: no se persiste en ningun lado, solo su hash.
type CreateAPIKeyResponse struct {
	APIKeyResponse
	// Key es el valor completo emb_<keyId>_<secreto>. Se muestra una sola vez.
	Key string `json:"key"`
}
