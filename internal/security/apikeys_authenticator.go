package security

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"

	"github.com/tu-org/embolsadora-api/internal/domain/apikeys"
)

// ErrAPIKeyInvalid es el unico error que Authenticate devuelve ante cualquier
// fallo de credencial: formato malo, key inexistente, secreto incorrecto,
// revocada, vencida o device deshabilitado.
//
// Colapsarlos es deliberado. El Edge trata al 403 como "reintentar" en todos
// los casos (asume rotacion de key en curso), asi que no gana nada con el
// detalle; y un atacante que pudiera distinguir "no existe" de "revocada"
// tendria un oraculo para enumerar key_ids validos. El motivo real se registra
// en el log, del lado del servidor.
var ErrAPIKeyInvalid = errors.New("security: API key invalida")

// Authenticator resuelve una API key en claro a la identidad del device.
type Authenticator interface {
	Authenticate(ctx context.Context, plaintext string) (*apikeys.DeviceIdentity, error)
}

// APIKeyAuthenticator implementa Authenticator contra Postgres, con una cache
// de lecturas en Redis para no golpear la base en cada uno de los hasta 200
// requests por segundo que admite el rate limit.
type APIKeyAuthenticator struct {
	repo apikeys.Repository
	rdb  *redis.Client
	ttl  time.Duration
	log  *zap.Logger
}

// NewAPIKeyAuthenticator construye el autenticador. `rdb` puede ser nil: sin
// Redis la cache simplemente no opera y cada request va a Postgres. Es un modo
// degradado, no un fallo — igual que el resto del repo trata a Redis.
func NewAPIKeyAuthenticator(repo apikeys.Repository, rdb *redis.Client, ttl time.Duration, log *zap.Logger) *APIKeyAuthenticator {
	if ttl <= 0 {
		ttl = time.Minute
	}
	return &APIKeyAuthenticator{repo: repo, rdb: rdb, ttl: ttl, log: log}
}

const (
	cachePrefix = "apikey:v1:"
	touchPrefix = "apikey:touch:"
	touchEvery  = time.Minute
)

// Authenticate valida la key y devuelve la identidad del device.
func (a *APIKeyAuthenticator) Authenticate(ctx context.Context, plaintext string) (*apikeys.DeviceIdentity, error) {
	keyID, secret, err := apikeys.Parse(plaintext)
	if err != nil {
		return nil, ErrAPIKeyInvalid
	}

	cred, err := a.lookup(ctx, keyID)
	if err != nil {
		if errors.Is(err, apikeys.ErrKeyNotFound) {
			return nil, ErrAPIKeyInvalid
		}
		// Postgres caido no es una credencial invalida. Se propaga tal cual para
		// que el middleware lo traduzca a 500 y el Edge reintente, en vez de a
		// 403. Es la misma logica que I-1 aplicada a la capa de auth.
		return nil, err
	}

	// La comparacion del secreto va primero y siempre, incluso si la key esta
	// revocada: asi el costo del camino de fallo no depende de por que fallo.
	if !apikeys.Matches(secret, cred.KeyHash) {
		a.log.Warn("api key con secreto incorrecto", zap.String("key_id", keyID))
		return nil, ErrAPIKeyInvalid
	}

	now := time.Now().UTC()
	switch {
	case cred.RevokedAt != nil:
		a.log.Info("api key revocada", zap.String("key_id", keyID))
		return nil, ErrAPIKeyInvalid
	case cred.ExpiresAt != nil && !cred.ExpiresAt.After(now):
		a.log.Info("api key vencida", zap.String("key_id", keyID))
		return nil, ErrAPIKeyInvalid
	case cred.DeviceStatus != "ACTIVE":
		a.log.Info("api key de device deshabilitado",
			zap.String("key_id", keyID), zap.String("device_status", cred.DeviceStatus))
		return nil, ErrAPIKeyInvalid
	}

	a.touch(ctx, cred)

	return &apikeys.DeviceIdentity{
		TenantID:  cred.TenantID,
		DeviceID:  cred.DeviceID,
		MachineID: cred.MachineID,
		KeyPK:     cred.KeyPK,
		KeyID:     cred.KeyID,
	}, nil
}

// lookup resuelve el key_id contra la cache y, si no esta, contra Postgres.
// Solo se cachean los hits: cachear los misses convertiria un barrido de keys
// inventadas en basura acumulada en Redis.
func (a *APIKeyAuthenticator) lookup(ctx context.Context, keyID string) (*apikeys.Credential, error) {
	if a.rdb != nil {
		if raw, err := a.rdb.Get(ctx, cachePrefix+keyID).Bytes(); err == nil {
			var cached apikeys.Credential
			if json.Unmarshal(raw, &cached) == nil {
				return &cached, nil
			}
		}
	}

	cred, err := a.repo.GetByKeyID(ctx, keyID)
	if err != nil {
		return nil, err
	}

	if a.rdb != nil {
		if raw, err := json.Marshal(cred); err == nil {
			// Un fallo de escritura en cache no puede tumbar el request: el dato
			// ya se resolvio contra la fuente de verdad.
			if err := a.rdb.Set(ctx, cachePrefix+keyID, raw, a.ttl).Err(); err != nil {
				a.log.Debug("no se pudo cachear la api key", zap.Error(err))
			}
		}
	}
	return cred, nil
}

// touch actualiza last_used_at como mucho una vez por minuto por key. A 200 rps
// un UPDATE por request serian 200 escrituras por segundo sobre la misma fila,
// que es peor que no tener el dato. El SETNX en Redis es el throttle; sin Redis
// se omite el update por completo, que es la degradacion correcta: last_used_at
// es diagnostico, no funcional.
func (a *APIKeyAuthenticator) touch(ctx context.Context, cred *apikeys.Credential) {
	if a.rdb == nil {
		return
	}
	ok, err := a.rdb.SetNX(ctx, touchPrefix+cred.KeyID, 1, touchEvery).Result()
	if err != nil || !ok {
		return
	}
	if err := a.repo.TouchLastUsed(ctx, cred.KeyPK); err != nil {
		a.log.Debug("no se pudo actualizar last_used_at", zap.Error(err))
	}
}

// InvalidateAPIKeyCache borra la entrada cacheada de una key. La revocacion
// tiene que llamarlo: sin esto, una key revocada seguiria autenticando hasta
// que venza el TTL.
func InvalidateAPIKeyCache(ctx context.Context, rdb *redis.Client, keyID string) error {
	if rdb == nil {
		return nil
	}
	return rdb.Del(ctx, cachePrefix+keyID).Err()
}

type deviceIdentityKeyType struct{}

var deviceIdentityKey = deviceIdentityKeyType{}

// WithDeviceIdentity guarda la identidad resuelta en el contexto del request.
func WithDeviceIdentity(ctx context.Context, id *apikeys.DeviceIdentity) context.Context {
	return context.WithValue(ctx, deviceIdentityKey, id)
}

// DeviceIdentityFrom extrae la identidad del device. Devuelve nil si el request
// no paso por APIKeyAuth.
func DeviceIdentityFrom(ctx context.Context) *apikeys.DeviceIdentity {
	if id, ok := ctx.Value(deviceIdentityKey).(*apikeys.DeviceIdentity); ok {
		return id
	}
	return nil
}
