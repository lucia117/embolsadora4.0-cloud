package security

// La resolucion real de API keys vive en apikeys_authenticator.go.
// La interfaz APIKeyLookup y su stub fueron removidos: eran un placeholder que
// devolvia siempre "no autorizado" y su firma (tenantID string, scopes []string)
// no representa el modelo real, donde una key resuelve a un tenant Y un device
// concretos y no maneja scopes.
