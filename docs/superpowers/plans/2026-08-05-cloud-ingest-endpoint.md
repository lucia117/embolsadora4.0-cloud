# Cloud Ingest Endpoint Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implementar `POST /api/v1/consumers/events` — recibir, autenticar, validar y persistir en MongoDB los batches de mediciones que el Edge Pi Service ya está enviando, hoy contra un handler que devuelve 501.

**Architecture:** Arquitectura hexagonal existente. La identidad (tenant, device, API keys) vive en Postgres; las mediciones en MongoDB. La idempotencia la garantiza un índice único sobre `eventId` más `insertMany(ordered:false)` — no lógica de aplicación. La escritura es **síncrona**: `accepted` es un hecho consumado antes de responder, porque el Edge avanza su watermark con esa respuesta.

**Tech Stack:** Go 1.24.6, Gin, pgx/v5, `go.mongodb.org/mongo-driver/v2` v2.5.0, go-redis/v8, Zap, Prometheus, testify, golang-migrate.

**Rama:** `feat/cloud-ingest-endpoint`, creada desde `develop` en `786b4cb`.

---

## Global Constraints

Cada tarea hereda implícitamente esta sección.

- **Módulo Go:** `github.com/tu-org/embolsadora-api`. Go 1.24.0, toolchain go1.24.6.
- **Go corre nativo en este host** (`C:\Program Files\Go\bin\go.exe`, go1.24.6 windows/amd64). El `CLAUDE.md` del repo dice que hay que usar Docker porque "Go is NOT installed on macOS host" — **eso es falso en esta máquina**. Usar `go` directamente.
- **El Docker daemon está apagado.** Los tests de integración hacen skip si falta `DATABASE_URL` / `MONGO_URI` / `REDIS_URL`, siguiendo la convención ya presente en `internal/repo/pg/tenants/repository_test.go`. Para correrlos: `docker compose up -d db redis mongo`.
- **El contrato HTTP está congelado.** Fuente: `embolsadora-edge/specs/002-forwarder-influx/contracts/outbound-events.openapi.yaml`. No se puede cambiar ni una clave.
  - Request: header `X-Api-Key` obligatorio, `Idempotency-Key` opcional (≤ 64 chars). Body `{"events":[...]}`, entre 1 y 1000 elementos.
  - Evento: requeridos `eventId`, `machineId`, `ts`, `kind`, `schemaVersion`, `payload`; opcional `seq`.
  - Respuesta 200: `{"data":{"accepted":n,"rejected":m,"errors":[{"index":i,"code":"...","message":"..."}]}}`.
  - **La respuesta de este endpoint NO usa el envelope `{"success":true,"data":...}`** que usan el resto de los handlers del repo. Es `{"data":...}` a secas. Copiar el patrón de `internal/api/handler/edge_devices/create_device.go` acá sería romper el contrato.
- **Códigos de `errors[].code` — valores exactos, semántica fija.** El Edge reacciona distinto a cada uno (`embolsadora-edge/internal/app/forwarder/errors.go:96-110`):
  - `DUPLICATE` → el Edge marca ACKED.
  - `INVALID_SCHEMA` → el Edge marca **DEAD, para siempre**.
  - `VALIDATION_FAILED` → el Edge marca **DEAD, para siempre**.
  - `STORAGE_UNAVAILABLE` → cualquier código no listado arriba es retriable; este es el que usamos para fallos parciales de storage.
- **Códigos HTTP — semántica fija** (`errors.go:48-76`): 400 → **todo el batch a DEAD**; 403 → reintenta; 429 → reintenta respetando `Retry-After`; 5xx/timeout → reintenta con backoff.
- **Invariantes. Violarlos produce pérdida silenciosa de datos.**
  - **I-1** — Un error de infraestructura jamás se reporta como error de payload. Mongo caído ⇒ `STORAGE_UNAVAILABLE` o HTTP 500. **Nunca** `INVALID_SCHEMA`, `VALIDATION_FAILED` ni 400.
  - **I-2** — Los problemas de eventos individuales van por `200` + `errors[]`, nunca por 400. El 400 queda reservado a requests malformados **a nivel de sobre**.
  - **I-3** — `errors[].index` es la posición **con base 0 en el array `events` del request original**, no en ningún array filtrado.
  - **I-4** — `accepted + rejected == len(events)` en toda respuesta 200.
- **El contenido de `payload` no se valida jamás** (D-8). Se persiste tal cual llega. Validar el payload haría que cualquier cambio del catálogo AAS mandara datos reales a DEAD.
- **`tenantId` y `deviceId` salen siempre de la API key, nunca del body** (D-10).
- **Hash de API keys: SHA-256, no bcrypt/argon2** (D-4). Son secretos de alta entropía; la lentitud deliberada de bcrypt es inviable a 200 rps.
- **Sin TTL en Mongo** (D-9): retención indefinida, decisión explícita de producto.
- **Idioma:** comentarios y mensajes de commit en español, siguiendo el repo. Los identificadores en inglés.
- **Driver de Mongo: `go.mongodb.org/mongo-driver/v2` v2.5.0 — la v2, no la v1.** Ver "Relación con PR #30" más abajo. Diferencias de la v2 que afectan al código de este plan: `mongo.Connect(opts)` **no recibe `ctx`**; los paquetes cuelgan de `.../v2/...`; `options.X()` devuelve builders que se pasan como variádicos.

### Variables de entorno nuevas (valores exactos)

```ini
MONGO_URI=mongodb://localhost:27017
MONGO_DATABASE=embolsadora
MONGO_TIMEOUT=10s
INGEST_MAX_BODY_BYTES=4194304
INGEST_MAX_EVENTS=1000
INGEST_RATE_LIMIT_RPS=200
INGEST_RATE_LIMIT_BURST=1000
APIKEY_CACHE_TTL=60s
```

---

## Relación con PR #30 (`006-mongo-infra`) — leer antes de la Task 5

Hay un **PR abierto, el #30**, "feat(006): MongoDB infrastructure layer with AAS Shell CRUD", que ya introduce una capa de Mongo. **No ramificamos de ahí** (su último commit es del 2026-04-11, ~4 meses detrás de develop: usa `config.Load()` sin argumentos, cuando develop ya tiene `Load(env Environment)`), pero sí nos alineamos con sus decisiones para que el merge posterior sea barato:

- **Driver `v2` v2.5.0**, que es el que usa el #30. Los majors v1 y v2 no conviven: tienen rutas de import distintas y APIs incompatibles. Elegir v1 acá condenaría a quien mergee segundo a reescribir su capa Mongo entera.
- **`internal/platform/mongo/client.go` es el único archivo que ambos crean.** Nuestra versión es un superset de la del #30 (agrega timeouts configurables, `Database()`, `Ping()` y `Close()`). Al reconciliar, la nuestra gana.
- **Divergencia de config que hay que resolver al mergear:** el #30 usa `MongoConfig{URI, DB}` con la env var `MONGO_DB`. El diseño de la ingesta (§10) especifica `MONGO_DATABASE`. Manda el diseño, porque es el contrato con la infra de despliegue; para no romper al #30 mientras tanto, `Load()` lee `MONGO_DATABASE` y cae a `MONGO_DB` si la primera falta (Task 5, Step 2).
- **Bug a corregir si el #30 entra:** su `APIKeyAuth` responde **401** ante key inválida. El contrato congelado dice **403**. La Task 10 de este plan escribe el 403 correcto; el 401 no debe sobrevivir al merge.

Nada de esto bloquea el plan. Es la lista de puntos de fricción, para que aparezcan en un merge y no en producción.

## Prerrequisitos — ya verificados, no hay que hacerlos

- **El cambio del Edge de §5 del diseño (`aasPath` dentro de `payload`) YA ESTÁ HECHO Y COMMITEADO.** Commit `76c3805` en `embolsadora-edge`, con test en `internal/app/forwarder/grouper_test.go:85`. `grouper.go:65` emite `"aasPath": path`. **No abrir ese repo.**
- El cloud igual debe tolerar eventos **sin** `aasPath` (los que quedaron en outbox antes de ese commit): se persisten normalmente, sólo no entran al índice compuesto, que es sparse.

## Estado inicial verificado

| Pieza | Estado real |
|---|---|
| `internal/consumers/events_handler.go` | Stub, `c.String(501, "not implemented")` |
| `internal/consumers/heartbeat_handler.go` | Stub 501 — **queda así**, fuera de alcance |
| `internal/consumers/middleware/middleware.go` | 5 no-ops: `APIKeyAuth`, `RateLimit`, `Idempotency`, `NoCORS`, `Timeout` |
| `internal/consumers/router.go` | Registra rutas; `Deps{APIKeys security.APIKeyLookup}`, `Config{}` vacío |
| `internal/security/apikeys.go` | Interfaz `APIKeyLookup` + `stubAPIKeyLookup` que siempre falla |
| `internal/api/usecases/ingest/service.go` | Archivo con `package ingest` y un TODO. Sin declaraciones |
| `internal/consumers/dto/`, `internal/consumers/mapper/` | Directorios **vacíos** |
| `edge_devices` (tabla) | Existe (`migrations/000001`, línea 185), con `tenant_id`, `machine_id`, `status ∈ {ACTIVE,DISABLED}` |
| Última migración | `000007` ⇒ la nueva es **`000008`** |
| MongoDB | No existe: sin driver en `go.mod`, sin servicio en `docker-compose.yml`, sin config |

---

## Mapa de archivos

**Crear:**

| Archivo | Responsabilidad |
|---|---|
| `migrations/000008_edge_device_api_keys.up.sql` / `.down.sql` | Tabla `edge_device_api_keys` |
| `internal/domain/apikeys/apikey.go` | Entidad `APIKey`, `Credential`, `DeviceIdentity` |
| `internal/domain/apikeys/keygen.go` | `Generate`, `Parse`, `HashSecret`, `Matches` — puro, sin I/O |
| `internal/domain/apikeys/keygen_test.go` | Unit tests de formato/hash/tiempo constante |
| `internal/domain/apikeys/repository.go` | Interfaz `Repository` + `ErrKeyNotFound` |
| `internal/repo/pg/apikeys/repository.go` | Implementación Postgres |
| `internal/repo/pg/apikeys/repository_test.go` | Integración (skip sin `DATABASE_URL`) |
| `internal/security/apikeys_authenticator.go` | `Authenticator` real + caché Redis + ctx helpers |
| `internal/security/apikeys_authenticator_test.go` | Unit tests con repo fake |
| `internal/platform/mongo/client.go` | Conexión, ping, cierre |
| `internal/domain/ingest/measurement.go` | `Measurement`, `Event`, `Result`, `EventError`, `DeviceContext`, códigos |
| `internal/domain/ingest/repository.go` | Interfaz `Repository`, `InsertReport` |
| `internal/repo/mongo/measurements/repository.go` | `InsertMany(ordered:false)`, E11000 → `DUPLICATE`, índices |
| `internal/repo/mongo/measurements/repository_test.go` | Integración (skip sin `MONGO_URI`) |
| `internal/app/ingest/service.go` | `IngestBatch`: validación de sobre + mapeo de índices |
| `internal/app/ingest/validate.go` | Validación evento por evento |
| `internal/app/ingest/validate_test.go` | Tabla de casos de §8 |
| `internal/app/ingest/service_test.go` | I-3 e I-4 con repo fake |
| `internal/consumers/dto/events.go` | DTOs request/response del contrato |
| `internal/consumers/ratelimit.go` | Token bucket Lua sobre Redis |
| `internal/api/handler/edge_devices/api_keys.go` | ABM: crear / listar / revocar |
| `internal/api/handler/edge_devices/dto/api_keys.go` | DTOs del ABM |
| `internal/telemetry/ingest_metrics.go` | Contadores/histograma Prometheus |
| `scripts/genfixture/main.go` | Generador determinista del fixture |
| `internal/consumers/testdata/last-batch.json` | Fixture de 108 eventos |
| `internal/consumers/events_handler_test.go` | Contrato: SC-001, SC-002, SC-003 |
| `internal/consumers/ingest_integration_test.go` | SC-004, SC-005, SC-006, SC-007, SC-008, SC-009 |

**Modificar:**

| Archivo | Cambio |
|---|---|
| `internal/config/config.go` | `MongoConfig`, `IngestConfig` en `Config` |
| `internal/consumers/events_handler.go` | Reemplazar el 501 |
| `internal/consumers/middleware/middleware.go` | `APIKeyAuth` y `RateLimit` reales |
| `internal/consumers/router.go` | `Deps` real, cablear middlewares por ruta |
| `internal/security/apikeys.go` | Borrar `StubAPIKeyLookup`, dejar sólo lo que se usa |
| `internal/routes/url_mappings.go` | Cablear Mongo, repos, service, ABM |
| `internal/api/handler/edge_devices/routes.go` | Rutas de API keys |
| `docker-compose.yml` | Servicio `mongo` |
| `go.mod` / `go.sum` | Driver de Mongo |
| `.env.local` (si existe) | Vars nuevas |

**Borrar:**

| Archivo | Razón |
|---|---|
| `internal/api/usecases/ingest/service.go` | Stub vacío; el service real va en `internal/app/ingest/` (§7 del diseño), y dos paquetes `ingest` confunden |

---

# FASE 1 — Autenticación por API key (Postgres)

Sin esto no hay tenant ni device que resolver, y D-10 dice que salen de la key. Es el cimiento.

---

### Task 1: Migración `edge_device_api_keys`

**Files:**
- Create: `migrations/000008_edge_device_api_keys.up.sql`
- Create: `migrations/000008_edge_device_api_keys.down.sql`

**Interfaces:**
- Consumes: tablas `tenants`, `edge_devices`, `users` de `migrations/000001_initial_schema.up.sql`.
- Produces: tabla `public.edge_device_api_keys` con columnas `id, tenant_id, device_id, key_id, key_hash, name, created_at, created_by, expires_at, revoked_at, last_used_at`. Las tareas 3 y 12 dependen de estos nombres exactos.

- [ ] **Step 1: Escribir la migración up**

Crear `migrations/000008_edge_device_api_keys.up.sql`:

```sql
-- ============================================================================
-- Migration 000008: API keys de edge devices
-- ============================================================================
-- El Edge Pi Service autentica contra POST /api/v1/consumers/events con el
-- header X-Api-Key. El cloud resuelve tenant y device server-side a partir de
-- esa key: el Pi nunca manda X-Tenant-Id. Esta tabla es ese punto de anclaje.
--
-- El secreto en claro NO se guarda nunca: sólo sha256(secreto) en key_hash.
-- key_id es la parte publica e indexada, que permite el lookup directo sin
-- tener que comparar hashes contra toda la tabla.
--
-- Varias keys activas por device es intencional: es lo que permite rotar sin
-- downtime (crear la nueva, desplegarla en el Pi, recien ahi revocar la vieja).
-- ============================================================================

CREATE TABLE public.edge_device_api_keys (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id    uuid NOT NULL REFERENCES public.tenants(id)      ON DELETE CASCADE,
    device_id    uuid NOT NULL REFERENCES public.edge_devices(id) ON DELETE CASCADE,
    key_id       character varying(32) NOT NULL UNIQUE,
    key_hash     bytea NOT NULL,
    name         character varying(255),
    created_at   timestamp with time zone NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_by   uuid REFERENCES public.users(id),
    expires_at   timestamp with time zone,
    revoked_at   timestamp with time zone,
    last_used_at timestamp with time zone
);

CREATE INDEX idx_edge_device_api_keys_device ON public.edge_device_api_keys(device_id);
```

- [ ] **Step 2: Escribir la migración down**

Crear `migrations/000008_edge_device_api_keys.down.sql`:

```sql
DROP INDEX IF EXISTS idx_edge_device_api_keys_device;
DROP TABLE IF EXISTS public.edge_device_api_keys;
```

- [ ] **Step 3: Verificar que la migración aplica y revierte**

Levantar Postgres y correr up/down/up:

```bash
docker compose up -d db
export DATABASE_URL='postgres://embolsadora_user:embolsadora_password@localhost:5432/embolsadora_dev?sslmode=disable'
migrate -path migrations/ -database "$DATABASE_URL" up
migrate -path migrations/ -database "$DATABASE_URL" down 1
migrate -path migrations/ -database "$DATABASE_URL" up
```

Esperado: los tres comandos terminan sin error. Verificar la tabla:

```bash
docker compose exec db psql -U embolsadora_user -d embolsadora_dev -c '\d public.edge_device_api_keys'
```

Esperado: 11 columnas, PK sobre `id`, UNIQUE sobre `key_id`, dos FK con `ON DELETE CASCADE` y una FK a `users`.

- [ ] **Step 4: Commit**

```bash
git add migrations/000008_edge_device_api_keys.up.sql migrations/000008_edge_device_api_keys.down.sql
git commit -m "feat(ingest): tabla edge_device_api_keys para auth del Edge"
```

---

### Task 2: Generación, parseo y hash de API keys (dominio puro)

Sin I/O: es la pieza más fácil de testear exhaustivamente y la más cara de tener mal.

**Files:**
- Create: `internal/domain/apikeys/apikey.go`
- Create: `internal/domain/apikeys/keygen.go`
- Test: `internal/domain/apikeys/keygen_test.go`

**Interfaces:**
- Consumes: nada.
- Produces:
  - `apikeys.APIKey` struct (campos en Step 1).
  - `apikeys.Credential` struct (campos en Step 1).
  - `apikeys.Generate() (plaintext string, keyID string, hash []byte, err error)`
  - `apikeys.Parse(plaintext string) (keyID string, secret string, err error)`
  - `apikeys.HashSecret(secret string) []byte`
  - `apikeys.Matches(secret string, hash []byte) bool`
  - `apikeys.ErrMalformedKey` sentinel.
  - Constantes `Prefix = "emb_"`, `KeyIDLen = 12`, `SecretBytes = 32`.

- [ ] **Step 1: Escribir las entidades**

Crear `internal/domain/apikeys/apikey.go`:

```go
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
```

- [ ] **Step 2: Escribir el test de formato — debe fallar**

Crear `internal/domain/apikeys/keygen_test.go`:

```go
package apikeys_test

import (
	"crypto/sha256"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tu-org/embolsadora-api/internal/domain/apikeys"
)

func TestGenerateProducesParseableKey(t *testing.T) {
	plaintext, keyID, hash, err := apikeys.Generate()
	require.NoError(t, err)

	assert.True(t, strings.HasPrefix(plaintext, "emb_"), "la key debe llevar el prefijo emb_")
	assert.Len(t, keyID, apikeys.KeyIDLen)
	assert.Len(t, hash, sha256.Size)

	gotKeyID, secret, err := apikeys.Parse(plaintext)
	require.NoError(t, err)
	assert.Equal(t, keyID, gotKeyID)
	assert.True(t, apikeys.Matches(secret, hash))
}

func TestGenerateIsUnique(t *testing.T) {
	seen := make(map[string]struct{}, 500)
	for i := 0; i < 500; i++ {
		_, keyID, _, err := apikeys.Generate()
		require.NoError(t, err)
		_, dup := seen[keyID]
		require.False(t, dup, "key_id repetido en %d iteraciones", i)
		seen[keyID] = struct{}{}
	}
}

// El key_id es hexadecimal justamente para que no contenga "_", que es el
// separador. El secreto es base64url y SI puede contener "_": por eso Parse
// parte en el primer separador y no en el ultimo.
func TestParseHandlesUnderscoreInSecret(t *testing.T) {
	keyID, secret, err := apikeys.Parse("emb_0123456789ab_aa_bb_cc")
	require.NoError(t, err)
	assert.Equal(t, "0123456789ab", keyID)
	assert.Equal(t, "aa_bb_cc", secret)
}

func TestParseRejectsMalformed(t *testing.T) {
	cases := map[string]string{
		"vacio":              "",
		"sin prefijo":        "xxx_0123456789ab_secreto",
		"sin separador":      "emb_0123456789absecreto",
		"key_id corto":       "emb_0123_secreto",
		"key_id no hex":      "emb_zzzzzzzzzzzz_secreto",
		"secreto vacio":      "emb_0123456789ab_",
		"solo prefijo":       "emb_",
	}
	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			_, _, err := apikeys.Parse(input)
			assert.ErrorIs(t, err, apikeys.ErrMalformedKey)
		})
	}
}

func TestMatchesRejectsWrongSecret(t *testing.T) {
	_, _, hash, err := apikeys.Generate()
	require.NoError(t, err)
	assert.False(t, apikeys.Matches("secreto-incorrecto", hash))
	assert.False(t, apikeys.Matches("", hash))
	assert.False(t, apikeys.Matches("x", nil))
}
```

- [ ] **Step 3: Correr el test para verificar que falla**

Run: `go test ./internal/domain/apikeys/... -v`
Expected: FAIL — `undefined: apikeys.Generate`, `undefined: apikeys.Parse`, etc.

- [ ] **Step 4: Implementar la generación**

Crear `internal/domain/apikeys/keygen.go`:

```go
package apikeys

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
)

const (
	// Prefix marca visualmente el secreto en logs, dumps y variables de entorno,
	// para que sea obvio que es una credencial y no un identificador cualquiera.
	Prefix = "emb_"

	// KeyIDLen es el largo del key_id en caracteres hexadecimales (6 bytes).
	// Hexadecimal —y no base64url— porque el alfabeto base64url incluye "_",
	// que es justamente el separador del formato: un key_id con "_" adentro
	// haria ambiguo el parseo.
	KeyIDLen = 12

	// SecretBytes es la entropia del secreto. 32 bytes es lo que justifica usar
	// SHA-256 y no bcrypt (D-4): no hay diccionario que atacar.
	SecretBytes = 32
)

// ErrMalformedKey indica que el string recibido no tiene la forma
// emb_<key_id>_<secreto>. Se devuelve sin detalle de que parte fallo: el
// llamador lo traduce a 403 y no debe filtrar en que se equivoco el cliente.
var ErrMalformedKey = errors.New("apikeys: formato de key invalido")

// Generate produce una API key nueva. Devuelve el texto en claro —que se le
// muestra al usuario UNA sola vez y no se persiste jamas—, el key_id publico
// que va indexado en Postgres, y el sha256 del secreto que si se guarda.
func Generate() (plaintext string, keyID string, hash []byte, err error) {
	idBytes := make([]byte, KeyIDLen/2)
	if _, err = rand.Read(idBytes); err != nil {
		return "", "", nil, err
	}
	keyID = hex.EncodeToString(idBytes)

	secretBytes := make([]byte, SecretBytes)
	if _, err = rand.Read(secretBytes); err != nil {
		return "", "", nil, err
	}
	secret := base64.RawURLEncoding.EncodeToString(secretBytes)

	return Prefix + keyID + "_" + secret, keyID, HashSecret(secret), nil
}

// Parse separa una key en claro en su key_id y su secreto.
func Parse(plaintext string) (keyID string, secret string, err error) {
	rest, ok := strings.CutPrefix(plaintext, Prefix)
	if !ok {
		return "", "", ErrMalformedKey
	}

	// SplitN con limite 2: el secreto es base64url y puede contener "_",
	// asi que solo el PRIMER separador cuenta.
	parts := strings.SplitN(rest, "_", 2)
	if len(parts) != 2 {
		return "", "", ErrMalformedKey
	}
	keyID, secret = parts[0], parts[1]

	if len(keyID) != KeyIDLen || secret == "" {
		return "", "", ErrMalformedKey
	}
	if _, err := hex.DecodeString(keyID); err != nil {
		return "", "", ErrMalformedKey
	}
	return keyID, secret, nil
}

// HashSecret devuelve sha256(secreto). Es lo unico que se persiste.
func HashSecret(secret string) []byte {
	sum := sha256.Sum256([]byte(secret))
	return sum[:]
}

// Matches compara el secreto contra un hash almacenado en tiempo constante.
// El tiempo constante importa aunque el hash no sea secreto: sin el, la
// latencia de la comparacion filtra cuantos bytes iniciales acerto un atacante.
func Matches(secret string, hash []byte) bool {
	return subtle.ConstantTimeCompare(HashSecret(secret), hash) == 1
}
```

- [ ] **Step 5: Correr los tests para verificar que pasan**

Run: `go test ./internal/domain/apikeys/... -v`
Expected: PASS — los 5 tests en verde.

- [ ] **Step 6: Commit**

```bash
git add internal/domain/apikeys/
git commit -m "feat(ingest): generacion, parseo y hash de API keys de edge device"
```

---

### Task 3: Repositorio Postgres de API keys

**Files:**
- Create: `internal/domain/apikeys/repository.go`
- Create: `internal/repo/pg/apikeys/repository.go`
- Test: `internal/repo/pg/apikeys/repository_test.go`

**Interfaces:**
- Consumes: `apikeys.APIKey`, `apikeys.Credential` (Task 2); tabla de Task 1.
- Produces:
  - `apikeys.Repository` interfaz con `GetByKeyID`, `Create`, `ListByDevice`, `Revoke`, `TouchLastUsed`.
  - `apikeys.ErrKeyNotFound` sentinel.
  - `pgapikeys.NewRepository(db *pgxpool.Pool) *Repository`.

- [ ] **Step 1: Definir la interfaz de dominio**

Crear `internal/domain/apikeys/repository.go`:

```go
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
```

- [ ] **Step 2: Escribir el test de integración — debe fallar**

Crear `internal/repo/pg/apikeys/repository_test.go`:

```go
package apikeys_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domainapikeys "github.com/tu-org/embolsadora-api/internal/domain/apikeys"
	pgapikeys "github.com/tu-org/embolsadora-api/internal/repo/pg/apikeys"
)

// newPool sigue la convencion del repo (ver internal/repo/pg/tenants/repository_test.go):
// sin DATABASE_URL el test hace skip en vez de fallar.
func newPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL no seteada; se omite el test de integracion")
	}
	pool, err := pgxpool.New(context.Background(), url)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	return pool
}

// seedDevice crea un tenant y un edge device descartables y devuelve sus IDs.
func seedDevice(t *testing.T, pool *pgxpool.Pool) (tenantID, deviceID uuid.UUID, machineID string) {
	t.Helper()
	ctx := context.Background()
	tenantID, deviceID = uuid.New(), uuid.New()
	machineID = "EMB-TEST-" + uuid.NewString()[:8]

	_, err := pool.Exec(ctx,
		`INSERT INTO tenants (id, name, company_name, subdomain, is_active)
		 VALUES ($1, $2, $2, $3, true)`,
		tenantID, "apikeys-test", "apikeys-"+uuid.NewString()[:8])
	require.NoError(t, err)

	_, err = pool.Exec(ctx,
		`INSERT INTO edge_devices (id, tenant_id, name, machine_id, edge_type, raspberry_base_url, status)
		 VALUES ($1, $2, 'test device', $3, 'RASPBERRY_PLC', 'http://localhost:9000', 'ACTIVE')`,
		deviceID, tenantID, machineID)
	require.NoError(t, err)

	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM tenants WHERE id = $1`, tenantID)
	})
	return tenantID, deviceID, machineID
}

func newKey(tenantID, deviceID uuid.UUID) (*domainapikeys.APIKey, string) {
	plaintext, keyID, hash, err := domainapikeys.Generate()
	if err != nil {
		panic(err)
	}
	name := "clave de test"
	return &domainapikeys.APIKey{
		ID:        uuid.New(),
		TenantID:  tenantID,
		DeviceID:  deviceID,
		KeyID:     keyID,
		KeyHash:   hash,
		Name:      &name,
		CreatedAt: time.Now().UTC(),
	}, plaintext
}

func TestCreateAndGetByKeyID(t *testing.T) {
	pool := newPool(t)
	repo := pgapikeys.NewRepository(pool)
	ctx := context.Background()

	tenantID, deviceID, machineID := seedDevice(t, pool)
	key, plaintext := newKey(tenantID, deviceID)
	require.NoError(t, repo.Create(ctx, key))

	cred, err := repo.GetByKeyID(ctx, key.KeyID)
	require.NoError(t, err)

	assert.Equal(t, key.ID, cred.KeyPK)
	assert.Equal(t, tenantID, cred.TenantID)
	assert.Equal(t, deviceID, cred.DeviceID)
	assert.Equal(t, machineID, cred.MachineID, "el JOIN debe traer el machine_id del device")
	assert.Equal(t, "ACTIVE", cred.DeviceStatus)
	assert.Nil(t, cred.RevokedAt)

	// El hash persistido tiene que validar contra el secreto original.
	_, secret, err := domainapikeys.Parse(plaintext)
	require.NoError(t, err)
	assert.True(t, domainapikeys.Matches(secret, cred.KeyHash))
}

func TestGetByKeyIDNotFound(t *testing.T) {
	repo := pgapikeys.NewRepository(newPool(t))
	_, err := repo.GetByKeyID(context.Background(), "deadbeefcafe")
	assert.ErrorIs(t, err, domainapikeys.ErrKeyNotFound)
}

func TestRevokeIsIdempotent(t *testing.T) {
	pool := newPool(t)
	repo := pgapikeys.NewRepository(pool)
	ctx := context.Background()

	tenantID, deviceID, _ := seedDevice(t, pool)
	key, _ := newKey(tenantID, deviceID)
	require.NoError(t, repo.Create(ctx, key))

	require.NoError(t, repo.Revoke(ctx, tenantID, key.ID))
	first, err := repo.GetByKeyID(ctx, key.KeyID)
	require.NoError(t, err)
	require.NotNil(t, first.RevokedAt)

	// Revocar de nuevo no debe mover el timestamp original.
	require.NoError(t, repo.Revoke(ctx, tenantID, key.ID))
	second, err := repo.GetByKeyID(ctx, key.KeyID)
	require.NoError(t, err)
	assert.Equal(t, first.RevokedAt.UnixNano(), second.RevokedAt.UnixNano())
}

func TestRevokeRejectsForeignTenant(t *testing.T) {
	pool := newPool(t)
	repo := pgapikeys.NewRepository(pool)
	ctx := context.Background()

	tenantID, deviceID, _ := seedDevice(t, pool)
	key, _ := newKey(tenantID, deviceID)
	require.NoError(t, repo.Create(ctx, key))

	err := repo.Revoke(ctx, uuid.New(), key.ID)
	assert.ErrorIs(t, err, domainapikeys.ErrKeyNotFound)
}

func TestListByDeviceNewestFirst(t *testing.T) {
	pool := newPool(t)
	repo := pgapikeys.NewRepository(pool)
	ctx := context.Background()

	tenantID, deviceID, _ := seedDevice(t, pool)
	older, _ := newKey(tenantID, deviceID)
	older.CreatedAt = time.Now().UTC().Add(-time.Hour)
	newer, _ := newKey(tenantID, deviceID)
	require.NoError(t, repo.Create(ctx, older))
	require.NoError(t, repo.Create(ctx, newer))

	list, err := repo.ListByDevice(ctx, tenantID, deviceID)
	require.NoError(t, err)
	require.Len(t, list, 2)
	assert.Equal(t, newer.ID, list[0].ID, "la mas nueva va primero")
}

func TestTouchLastUsed(t *testing.T) {
	pool := newPool(t)
	repo := pgapikeys.NewRepository(pool)
	ctx := context.Background()

	tenantID, deviceID, _ := seedDevice(t, pool)
	key, _ := newKey(tenantID, deviceID)
	require.NoError(t, repo.Create(ctx, key))

	list, err := repo.ListByDevice(ctx, tenantID, deviceID)
	require.NoError(t, err)
	require.Nil(t, list[0].LastUsedAt)

	require.NoError(t, repo.TouchLastUsed(ctx, key.ID))

	list, err = repo.ListByDevice(ctx, tenantID, deviceID)
	require.NoError(t, err)
	assert.NotNil(t, list[0].LastUsedAt)
}
```

- [ ] **Step 3: Correr el test para verificar que falla**

```bash
docker compose up -d db
export DATABASE_URL='postgres://embolsadora_user:embolsadora_password@localhost:5432/embolsadora_dev?sslmode=disable'
go test ./internal/repo/pg/apikeys/... -v
```

Expected: FAIL — el paquete no compila, `undefined: pgapikeys.NewRepository`.

- [ ] **Step 4: Implementar el repositorio**

Crear `internal/repo/pg/apikeys/repository.go`:

```go
package apikeys

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	domainapikeys "github.com/tu-org/embolsadora-api/internal/domain/apikeys"
)

// Repository implementa domainapikeys.Repository sobre Postgres.
type Repository struct {
	db *pgxpool.Pool
}

// NewRepository construye el repositorio de API keys.
func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// GetByKeyID resuelve el key_id publico a una credencial completa. El JOIN con
// edge_devices trae machine_id y status en la misma ida: el camino caliente de
// la ingesta hace un solo roundtrip.
func (r *Repository) GetByKeyID(ctx context.Context, keyID string) (*domainapikeys.Credential, error) {
	const q = `
		SELECT k.id, k.tenant_id, k.device_id, k.key_id, k.key_hash,
		       k.expires_at, k.revoked_at,
		       d.machine_id, d.status
		  FROM edge_device_api_keys k
		  JOIN edge_devices d ON d.id = k.device_id
		 WHERE k.key_id = $1`

	var c domainapikeys.Credential
	err := r.db.QueryRow(ctx, q, keyID).Scan(
		&c.KeyPK, &c.TenantID, &c.DeviceID, &c.KeyID, &c.KeyHash,
		&c.ExpiresAt, &c.RevokedAt,
		&c.MachineID, &c.DeviceStatus,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domainapikeys.ErrKeyNotFound
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// Create persiste una key nueva.
func (r *Repository) Create(ctx context.Context, k *domainapikeys.APIKey) error {
	const q = `
		INSERT INTO edge_device_api_keys
		       (id, tenant_id, device_id, key_id, key_hash, name, created_at, created_by, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`

	_, err := r.db.Exec(ctx, q,
		k.ID, k.TenantID, k.DeviceID, k.KeyID, k.KeyHash,
		k.Name, k.CreatedAt, k.CreatedBy, k.ExpiresAt,
	)
	return err
}

// ListByDevice devuelve todas las keys del device, nuevas primero.
func (r *Repository) ListByDevice(ctx context.Context, tenantID, deviceID uuid.UUID) ([]*domainapikeys.APIKey, error) {
	const q = `
		SELECT id, tenant_id, device_id, key_id, key_hash, name,
		       created_at, created_by, expires_at, revoked_at, last_used_at
		  FROM edge_device_api_keys
		 WHERE tenant_id = $1 AND device_id = $2
		 ORDER BY created_at DESC`

	rows, err := r.db.Query(ctx, q, tenantID, deviceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*domainapikeys.APIKey
	for rows.Next() {
		var k domainapikeys.APIKey
		if err := rows.Scan(
			&k.ID, &k.TenantID, &k.DeviceID, &k.KeyID, &k.KeyHash, &k.Name,
			&k.CreatedAt, &k.CreatedBy, &k.ExpiresAt, &k.RevokedAt, &k.LastUsedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, &k)
	}
	return out, rows.Err()
}

// Revoke marca la key como revocada. El `AND revoked_at IS NULL` la hace
// idempotente: una segunda revocacion no pisa el timestamp original.
func (r *Repository) Revoke(ctx context.Context, tenantID, keyPK uuid.UUID) error {
	const q = `
		UPDATE edge_device_api_keys
		   SET revoked_at = CURRENT_TIMESTAMP
		 WHERE id = $1 AND tenant_id = $2 AND revoked_at IS NULL`

	tag, err := r.db.Exec(ctx, q, keyPK, tenantID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		// O no existe, o no es del tenant, o ya estaba revocada. Distinguirlas
		// obliga a un SELECT extra sin ganancia: chequeamos existencia y listo.
		return r.assertExists(ctx, tenantID, keyPK)
	}
	return nil
}

func (r *Repository) assertExists(ctx context.Context, tenantID, keyPK uuid.UUID) error {
	const q = `SELECT 1 FROM edge_device_api_keys WHERE id = $1 AND tenant_id = $2`
	var one int
	err := r.db.QueryRow(ctx, q, keyPK, tenantID).Scan(&one)
	if errors.Is(err, pgx.ErrNoRows) {
		return domainapikeys.ErrKeyNotFound
	}
	return err
}

// TouchLastUsed actualiza last_used_at. El llamador se encarga de no invocarlo
// mas de una vez por minuto por key (ver el throttle en security.Authenticator).
func (r *Repository) TouchLastUsed(ctx context.Context, keyPK uuid.UUID) error {
	const q = `UPDATE edge_device_api_keys SET last_used_at = CURRENT_TIMESTAMP WHERE id = $1`
	_, err := r.db.Exec(ctx, q, keyPK)
	return err
}
```

- [ ] **Step 5: Correr los tests para verificar que pasan**

Run: `go test ./internal/repo/pg/apikeys/... -v`
Expected: PASS — los 6 tests en verde.

- [ ] **Step 6: Verificar que todo el módulo compila**

Run: `go build ./...`
Expected: sin salida (éxito).

- [ ] **Step 7: Commit**

```bash
git add internal/domain/apikeys/repository.go internal/repo/pg/apikeys/
git commit -m "feat(ingest): repositorio Postgres de API keys de edge device"
```

---

### Task 4: Autenticador de API keys con caché en Redis

Reemplaza `StubAPIKeyLookup`. Es el punto donde D-10 se hace cumplir: de acá sale el `DeviceIdentity` y de ningún otro lado.

**Files:**
- Create: `internal/security/apikeys_authenticator.go`
- Test: `internal/security/apikeys_authenticator_test.go`
- Modify: `internal/security/apikeys.go`

**Interfaces:**
- Consumes: `apikeys.Repository`, `apikeys.Credential`, `apikeys.DeviceIdentity`, `apikeys.Parse`, `apikeys.Matches`, `apikeys.ErrKeyNotFound` (Tasks 2-3).
- Produces:
  - `security.Authenticator` interfaz: `Authenticate(ctx, plaintext string) (*apikeys.DeviceIdentity, error)`.
  - `security.NewAPIKeyAuthenticator(repo apikeys.Repository, rdb *redis.Client, ttl time.Duration, log *zap.Logger) *APIKeyAuthenticator`.
  - `security.ErrAPIKeyInvalid` sentinel.
  - `security.WithDeviceIdentity(ctx, *apikeys.DeviceIdentity) context.Context`.
  - `security.DeviceIdentityFrom(ctx) *apikeys.DeviceIdentity`.
  - `security.InvalidateAPIKeyCache(ctx, rdb *redis.Client, keyID string) error`.

- [ ] **Step 1: Escribir el test — debe fallar**

Crear `internal/security/apikeys_authenticator_test.go`:

```go
package security_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	domainapikeys "github.com/tu-org/embolsadora-api/internal/domain/apikeys"
	"github.com/tu-org/embolsadora-api/internal/security"
)

// fakeRepo implementa domainapikeys.Repository en memoria. El autenticador se
// testea sin Postgres ni Redis: sus reglas son de decision, no de storage.
type fakeRepo struct {
	byKeyID map[string]*domainapikeys.Credential
	touched []uuid.UUID
	calls   int
}

func (f *fakeRepo) GetByKeyID(_ context.Context, keyID string) (*domainapikeys.Credential, error) {
	f.calls++
	c, ok := f.byKeyID[keyID]
	if !ok {
		return nil, domainapikeys.ErrKeyNotFound
	}
	return c, nil
}
func (f *fakeRepo) Create(context.Context, *domainapikeys.APIKey) error { return nil }
func (f *fakeRepo) ListByDevice(context.Context, uuid.UUID, uuid.UUID) ([]*domainapikeys.APIKey, error) {
	return nil, nil
}
func (f *fakeRepo) Revoke(context.Context, uuid.UUID, uuid.UUID) error { return nil }
func (f *fakeRepo) TouchLastUsed(_ context.Context, keyPK uuid.UUID) error {
	f.touched = append(f.touched, keyPK)
	return nil
}

// newAuth arma un autenticador con una key valida y devuelve el texto en claro.
func newAuth(t *testing.T, mutate func(*domainapikeys.Credential)) (*security.APIKeyAuthenticator, string, *fakeRepo) {
	t.Helper()
	plaintext, keyID, hash, err := domainapikeys.Generate()
	require.NoError(t, err)

	cred := &domainapikeys.Credential{
		KeyPK:        uuid.New(),
		TenantID:     uuid.New(),
		DeviceID:     uuid.New(),
		KeyID:        keyID,
		KeyHash:      hash,
		MachineID:    "EMB-DEV-001",
		DeviceStatus: "ACTIVE",
	}
	if mutate != nil {
		mutate(cred)
	}
	repo := &fakeRepo{byKeyID: map[string]*domainapikeys.Credential{keyID: cred}}
	// rdb nil: el autenticador debe funcionar sin Redis (fail-open de la cache).
	auth := security.NewAPIKeyAuthenticator(repo, nil, time.Minute, zap.NewNop())
	return auth, plaintext, repo
}

func TestAuthenticateHappyPath(t *testing.T) {
	auth, plaintext, repo := newAuth(t, nil)

	id, err := auth.Authenticate(context.Background(), plaintext)
	require.NoError(t, err)

	want := repo.byKeyID[id.KeyID]
	assert.Equal(t, want.TenantID, id.TenantID)
	assert.Equal(t, want.DeviceID, id.DeviceID)
	assert.Equal(t, "EMB-DEV-001", id.MachineID)
	assert.Equal(t, want.KeyPK, id.KeyPK)
}

// Todos los modos de fallo colapsan al mismo error. El handler los traduce a un
// 403 identico: si el cliente pudiera distinguir "no existe" de "revocada", la
// respuesta seria un oraculo para enumerar keys.
func TestAuthenticateFailureModes(t *testing.T) {
	past := time.Now().UTC().Add(-time.Hour)

	t.Run("formato invalido", func(t *testing.T) {
		auth, _, _ := newAuth(t, nil)
		_, err := auth.Authenticate(context.Background(), "no-es-una-key")
		assert.ErrorIs(t, err, security.ErrAPIKeyInvalid)
	})

	t.Run("key inexistente", func(t *testing.T) {
		auth, _, _ := newAuth(t, nil)
		_, err := auth.Authenticate(context.Background(), "emb_ffffffffffff_secreto")
		assert.ErrorIs(t, err, security.ErrAPIKeyInvalid)
	})

	t.Run("secreto incorrecto", func(t *testing.T) {
		auth, plaintext, _ := newAuth(t, nil)
		keyID, _, err := domainapikeys.Parse(plaintext)
		require.NoError(t, err)
		_, err = auth.Authenticate(context.Background(), "emb_"+keyID+"_secreto-que-no-es")
		assert.ErrorIs(t, err, security.ErrAPIKeyInvalid)
	})

	t.Run("key revocada", func(t *testing.T) {
		auth, plaintext, _ := newAuth(t, func(c *domainapikeys.Credential) { c.RevokedAt = &past })
		_, err := auth.Authenticate(context.Background(), plaintext)
		assert.ErrorIs(t, err, security.ErrAPIKeyInvalid)
	})

	t.Run("key vencida", func(t *testing.T) {
		auth, plaintext, _ := newAuth(t, func(c *domainapikeys.Credential) { c.ExpiresAt = &past })
		_, err := auth.Authenticate(context.Background(), plaintext)
		assert.ErrorIs(t, err, security.ErrAPIKeyInvalid)
	})

	t.Run("device deshabilitado", func(t *testing.T) {
		auth, plaintext, _ := newAuth(t, func(c *domainapikeys.Credential) { c.DeviceStatus = "DISABLED" })
		_, err := auth.Authenticate(context.Background(), plaintext)
		assert.ErrorIs(t, err, security.ErrAPIKeyInvalid)
	})
}

func TestAuthenticateAcceptsFutureExpiry(t *testing.T) {
	future := time.Now().UTC().Add(time.Hour)
	auth, plaintext, _ := newAuth(t, func(c *domainapikeys.Credential) { c.ExpiresAt = &future })

	_, err := auth.Authenticate(context.Background(), plaintext)
	assert.NoError(t, err)
}

func TestDeviceIdentityRoundTripsThroughContext(t *testing.T) {
	want := &domainapikeys.DeviceIdentity{
		TenantID:  uuid.New(),
		DeviceID:  uuid.New(),
		MachineID: "EMB-DEV-001",
	}
	ctx := security.WithDeviceIdentity(context.Background(), want)

	got := security.DeviceIdentityFrom(ctx)
	require.NotNil(t, got)
	assert.Equal(t, want.TenantID, got.TenantID)
	assert.Equal(t, want.MachineID, got.MachineID)

	assert.Nil(t, security.DeviceIdentityFrom(context.Background()))
}
```

- [ ] **Step 2: Correr el test para verificar que falla**

Run: `go test ./internal/security/... -run TestAuthenticate -v`
Expected: FAIL — `undefined: security.NewAPIKeyAuthenticator`.

- [ ] **Step 3: Implementar el autenticador**

Crear `internal/security/apikeys_authenticator.go`:

```go
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
```

- [ ] **Step 4: Borrar el stub viejo**

Reemplazar **todo** el contenido de `internal/security/apikeys.go` por:

```go
package security

// La resolucion real de API keys vive en apikeys_authenticator.go.
// La interfaz APIKeyLookup y su stub fueron removidos: eran un placeholder que
// devolvia siempre "no autorizado" y su firma (tenantID string, scopes []string)
// no representa el modelo real, donde una key resuelve a un tenant Y un device
// concretos y no maneja scopes.
```

- [ ] **Step 5: Ajustar el consumidor del stub**

`internal/consumers/router.go` referencia `security.APIKeyLookup` en `Deps`. Cambiar esa línea para que compile — el cableado real llega en la Task 10:

```go
// TODO(Task 10): cablear middlewares reales.
type Deps struct {
	Auth security.Authenticator
}
```

- [ ] **Step 6: Correr los tests y el build**

```bash
go test ./internal/security/... -v
go build ./...
```

Expected: PASS en los 4 tests nuevos (más los de `jwt_test.go` y `rbac_test.go` ya existentes), y build sin errores.

- [ ] **Step 7: Commit**

```bash
git add internal/security/ internal/consumers/router.go
git commit -m "feat(ingest): autenticador real de API keys con cache en Redis"
```

---

# FASE 2 — MongoDB

MongoDB es infraestructura nueva para este repo. Todo el conocimiento de que existe queda confinado a `internal/platform/mongo/` y `internal/repo/mongo/measurements/` (mitigación del riesgo de §14).

---

### Task 5: Provisioning de MongoDB — driver, config, cliente y compose

**Files:**
- Create: `internal/platform/mongo/client.go`
- Modify: `internal/config/config.go`
- Modify: `docker-compose.yml`
- Modify: `go.mod`, `go.sum`

**Interfaces:**
- Consumes: nada.
- Produces:
  - `config.MongoConfig{URI, Database string; Timeout time.Duration}` y `config.IngestConfig{MaxBodyBytes int64; MaxEvents, RateLimitRPS, RateLimitBurst int; APIKeyCacheTTL time.Duration}`, ambos como campos `Mongo` e `Ingest` de `config.Config`.
  - `mongoplatform.Connect(ctx context.Context, cfg config.MongoConfig) (*Client, error)`
  - `(*Client).Database() *mongodriver.Database`, `(*Client).Ping(ctx) error`, `(*Client).Close(ctx) error`

- [ ] **Step 1: Agregar el driver**

Se fija la **v2**, que es la que usa el PR #30 abierto. Ver "Relación con PR #30".

```bash
go get go.mongodb.org/mongo-driver/v2@v2.5.0
go mod tidy
```

Verificar: `grep mongo-driver go.mod` debe mostrar `go.mongodb.org/mongo-driver/v2 v2.5.0` en el bloque `require` directo, y **ninguna** entrada de `go.mongodb.org/mongo-driver` sin `/v2`.

- [ ] **Step 2: Agregar la configuración**

En `internal/config/config.go`, agregar los dos structs después de `RedisConfig` (línea ~63):

```go
type MongoConfig struct {
	URI      string
	Database string
	Timeout  time.Duration
}

type IngestConfig struct {
	MaxBodyBytes   int64
	MaxEvents      int
	RateLimitRPS   int
	RateLimitBurst int
	APIKeyCacheTTL time.Duration
}
```

Agregar los campos al struct `Config`:

```go
type Config struct {
	Env           Environment
	HTTP          HTTPConfig
	DB            DBConfig
	Redis         RedisConfig
	Mongo         MongoConfig
	Ingest        IngestConfig
	Supabase      SupabaseConfig
	Observability ObservabilityConfig
}
```

Y dentro de `Load()`, después del bloque `Redis:` (línea ~117):

```go
		Mongo: MongoConfig{
			URI: getEnv("MONGO_URI", "mongodb://localhost:27017"),
			// MONGO_DATABASE es el nombre que fija el diseno de la ingesta (§10)
			// y el que usa la infra de despliegue. El fallback a MONGO_DB existe
			// solo para no romper al PR #30, que uso ese nombre; cuando ese PR
			// se reconcilie, el fallback se borra.
			Database: getEnv("MONGO_DATABASE", getEnv("MONGO_DB", "embolsadora")),
			Timeout:  getDurationEnv("MONGO_TIMEOUT", 10*time.Second),
		},
		Ingest: IngestConfig{
			// 4 MiB de tope de lectura, no 2. El Edge corta sus batches en
			// BATCH_MAX_BYTES=2097152 exactos; rechazar a partir de 2 MiB
			// inclusive mandaria a DEAD hasta 1000 eventos por un byte de
			// desajuste. El limite que se valida como regla de negocio es el de
			// 1000 eventos; el de bytes existe solo contra abuso (§9.2).
			MaxBodyBytes:   int64(getIntEnv("INGEST_MAX_BODY_BYTES", 4194304)),
			MaxEvents:      getIntEnv("INGEST_MAX_EVENTS", 1000),
			RateLimitRPS:   getIntEnv("INGEST_RATE_LIMIT_RPS", 200),
			RateLimitBurst: getIntEnv("INGEST_RATE_LIMIT_BURST", 1000),
			APIKeyCacheTTL: getDurationEnv("APIKEY_CACHE_TTL", 60*time.Second),
		},
```

- [ ] **Step 3: Escribir el cliente de Mongo**

Crear `internal/platform/mongo/client.go`:

```go
// Package mongo encapsula el ciclo de vida de la conexion a MongoDB.
// Es, junto con internal/repo/mongo/measurements, el unico lugar del codigo que
// sabe que MongoDB existe: el resto del sistema habla con interfaces de dominio.
package mongo

import (
	"context"
	"fmt"

	mongodriver "go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"

	"github.com/tu-org/embolsadora-api/internal/config"
)

// Client envuelve la conexion y la base de datos ya seleccionada.
type Client struct {
	client *mongodriver.Client
	db     *mongodriver.Database
}

// Connect abre la conexion y verifica que el servidor responda. Falla rapido:
// si Mongo no esta disponible al arrancar, la ingesta no puede cumplir su
// contrato y es preferible no levantar a aceptar eventos que se van a perder.
func Connect(ctx context.Context, cfg config.MongoConfig) (*Client, error) {
	pingCtx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()

	opts := options.Client().
		ApplyURI(cfg.URI).
		SetConnectTimeout(cfg.Timeout).
		SetServerSelectionTimeout(cfg.Timeout)

	// OJO: en el driver v2, Connect NO recibe context — a diferencia de la v1.
	// Solo crea el cliente; la conexion real se establece de forma perezosa, y
	// por eso el Ping de abajo es lo que de verdad valida que Mongo responde.
	cli, err := mongodriver.Connect(opts)
	if err != nil {
		return nil, fmt.Errorf("mongo: no se pudo conectar: %w", err)
	}
	if err := cli.Ping(pingCtx, readpref.Primary()); err != nil {
		_ = cli.Disconnect(context.Background())
		return nil, fmt.Errorf("mongo: ping fallido: %w", err)
	}
	return &Client{client: cli, db: cli.Database(cfg.Database)}, nil
}

// Database devuelve la base ya seleccionada.
func (c *Client) Database() *mongodriver.Database { return c.db }

// Ping verifica que el primario responda. Lo usa el health check.
func (c *Client) Ping(ctx context.Context) error {
	return c.client.Ping(ctx, readpref.Primary())
}

// Close cierra la conexion.
func (c *Client) Close(ctx context.Context) error { return c.client.Disconnect(ctx) }
```

- [ ] **Step 4: Agregar el servicio a docker-compose**

En `docker-compose.yml`, insertar después del bloque `redis:` (antes de `api:`):

```yaml
  mongo:
    image: mongo:7
    container_name: embolsadora_mongo
    volumes:
      - mongo_data:/data/db
    ports:
      - "27017:27017"
    healthcheck:
      test: ["CMD", "mongosh", "--quiet", "--eval", "db.adminCommand('ping')"]
      interval: 10s
      timeout: 5s
      retries: 5
    restart: unless-stopped
    networks:
      - embolsadora_network
```

En el servicio `api:`, agregar a `depends_on:`:

```yaml
      mongo:
        condition: service_healthy
```

Y a su bloque `environment:`:

```yaml
      MONGO_URI: mongodb://mongo:27017
      MONGO_DATABASE: embolsadora
      MONGO_TIMEOUT: 10s
      INGEST_MAX_BODY_BYTES: 4194304
      INGEST_MAX_EVENTS: 1000
      INGEST_RATE_LIMIT_RPS: 200
      INGEST_RATE_LIMIT_BURST: 1000
      APIKEY_CACHE_TTL: 60s
```

Al final, agregar el volumen junto a `postgres_data` y `redis_data`:

```yaml
  mongo_data:
```

- [ ] **Step 5: Verificar que compila y que Mongo levanta**

```bash
go build ./...
docker compose up -d mongo
docker compose ps mongo
```

Expected: build sin salida; el contenedor `embolsadora_mongo` en estado `healthy`.

- [ ] **Step 6: Verificar la conexión de punta a punta**

```bash
docker compose exec mongo mongosh --quiet --eval 'db.adminCommand("ping")'
```

Expected: `{ ok: 1 }`.

- [ ] **Step 7: Commit**

```bash
git add go.mod go.sum internal/config/config.go internal/platform/mongo/ docker-compose.yml
git commit -m "feat(ingest): provisioning de MongoDB (driver, cliente, config, compose)"
```

---

### Task 6: Tipos de dominio de la ingesta

Todo lo que el resto del sistema necesita saber sobre mediciones, sin una sola referencia a Mongo.

**Files:**
- Create: `internal/domain/ingest/measurement.go`
- Create: `internal/domain/ingest/repository.go`

**Interfaces:**
- Consumes: nada.
- Produces:
  - Constantes `ingest.CodeDuplicate`, `CodeInvalidSchema`, `CodeValidationFailed`, `CodeStorageUnavailable`.
  - `ingest.KindMetric/KindAlarm/KindHeartbeat`, `ingest.MaxSchemaVersion = 1`.
  - `ingest.Measurement`, `ingest.EventError`, `ingest.Result`, `ingest.DeviceContext`.
  - `ingest.Repository` interfaz con `InsertMany` y `Ping`.
  - `ingest.InsertReport{Duplicated map[int]struct{}; Failed map[int]string}`.

- [ ] **Step 1: Escribir los tipos**

Crear `internal/domain/ingest/measurement.go`:

```go
// Package ingest contiene el modelo de dominio de la ingesta de mediciones.
// No conoce Mongo, ni Gin, ni Postgres.
package ingest

import "time"

// Codigos del contrato con el Edge. Los valores son literales fijos: el Edge
// decide con ellos si un evento se ACKea, se reintenta o se manda a DEAD
// (embolsadora-edge/internal/app/forwarder/errors.go:96-110). Cambiar uno de
// estos strings, o usarlo con otra semantica, es perder datos.
const (
	// CodeDuplicate: el eventId ya existia. El Edge lo ACKea — es el resultado
	// esperado de un reintento y no es un error.
	CodeDuplicate = "DUPLICATE"

	// CodeInvalidSchema: el sobre del evento esta mal formado. El Edge lo manda
	// a DEAD y no lo reintenta NUNCA MAS. Solo para errores realmente
	// irrecuperables del dato.
	CodeInvalidSchema = "INVALID_SCHEMA"

	// CodeValidationFailed: el sobre es valido pero el evento no corresponde.
	// El Edge lo manda a DEAD.
	CodeValidationFailed = "VALIDATION_FAILED"

	// CodeStorageUnavailable: no se pudo escribir por una causa de
	// infraestructura. No esta en la lista de codigos terminales del Edge, asi
	// que cae en el default: reintentar con backoff. Es el codigo que sostiene
	// el invariante I-1.
	CodeStorageUnavailable = "STORAGE_UNAVAILABLE"
)

// Kinds admitidos por el contrato.
const (
	KindMetric    = "metric"
	KindAlarm     = "alarm"
	KindHeartbeat = "heartbeat"
)

// MaxSchemaVersion es la version de sobre mas alta que este cloud entiende.
// Un evento con una version mayor se rechaza con VALIDATION_FAILED, no con
// INVALID_SCHEMA: el sobre esta bien formado, simplemente es de un futuro que
// esta build no conoce.
const MaxSchemaVersion = 1

// DeviceContext es la identidad ya resuelta por la capa de auth. El service la
// recibe en vez de leerla del body: tenantId y deviceId nunca salen del
// request (D-10), y machineId solo se acepta si coincide con el de aca.
type DeviceContext struct {
	TenantID  string
	DeviceID  string
	MachineID string
}

// Measurement es el documento que se persiste, uno por evento aceptado.
type Measurement struct {
	EventID       string         `bson:"eventId"`
	TenantID      string         `bson:"tenantId"`
	DeviceID      string         `bson:"deviceId"`
	MachineID     string         `bson:"machineId"`
	Ts            time.Time      `bson:"ts"`
	Seq           *int64         `bson:"seq,omitempty"`
	Kind          string         `bson:"kind"`
	SchemaVersion int            `bson:"schemaVersion"`
	Payload       map[string]any `bson:"payload"`
	ReceivedAt    time.Time      `bson:"receivedAt"`
}

// EventError es un elemento de errors[] en la respuesta.
type EventError struct {
	// Index es la posicion, con base 0, en el array `events` del request
	// ORIGINAL — no en ningun array filtrado (invariante I-3). El Edge usa este
	// numero para elegir que fila de su outbox marcar como DEAD: un desfasaje
	// mata eventos sanos y da por buenos los rotos.
	Index   int    `json:"index"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Result es el contenido de `data` en la respuesta 200.
type Result struct {
	Accepted int          `json:"accepted"`
	Rejected int          `json:"rejected"`
	Errors   []EventError `json:"errors,omitempty"`
}
```

- [ ] **Step 2: Escribir la interfaz del repositorio**

Crear `internal/domain/ingest/repository.go`:

```go
package ingest

import "context"

// Repository persiste mediciones.
//
// La idempotencia es responsabilidad de la implementacion —un indice unico
// sobre eventId— y no del service. Resolverla en la aplicacion (leer, decidir,
// escribir) tendria una condicion de carrera entre dos batches concurrentes con
// el mismo evento; la base no la tiene.
type Repository interface {
	// InsertMany inserta el lote completo sin abortar ante el primer fallo y
	// reporta el desenlace de cada documento.
	//
	// Devuelve error SOLO si la operacion fallo entera (Mongo inalcanzable,
	// timeout): ese caso se traduce a HTTP 500 y el Edge reintenta. Los fallos
	// por documento van en el InsertReport, nunca en el error.
	InsertMany(ctx context.Context, docs []Measurement) (InsertReport, error)

	// Ping verifica la conectividad. Lo usa el health check.
	Ping(ctx context.Context) error
}

// InsertReport describe el desenlace de un InsertMany parcialmente exitoso.
//
// Los indices de ambos mapas son posiciones dentro del slice `docs` que se le
// paso a InsertMany, NO del array `events` del request. Traducirlos es
// responsabilidad del service, que es quien conoce esa correspondencia
// (invariante I-3).
type InsertReport struct {
	// Duplicated: indices que Mongo rechazo por violar el indice unico (E11000).
	Duplicated map[int]struct{}

	// Failed: indices que fallaron por cualquier otra causa, con su mensaje.
	// Se reportan como STORAGE_UNAVAILABLE para que el Edge reintente.
	Failed map[int]string
}

// Inserted devuelve cuantos documentos de un lote de tamano `total` entraron.
func (r InsertReport) Inserted(total int) int {
	return total - len(r.Duplicated) - len(r.Failed)
}
```

- [ ] **Step 3: Verificar que compila**

Run: `go build ./internal/domain/ingest/...`
Expected: sin salida.

- [ ] **Step 4: Commit**

```bash
git add internal/domain/ingest/
git commit -m "feat(ingest): tipos de dominio de la ingesta de mediciones"
```

---

### Task 7: Repositorio Mongo de mediciones

Acá vive toda la idempotencia del sistema. Es la tarea crítica del plan.

**Files:**
- Create: `internal/repo/mongo/measurements/repository.go`
- Test: `internal/repo/mongo/measurements/repository_test.go`

**Interfaces:**
- Consumes: `ingest.Measurement`, `ingest.InsertReport`, `ingest.Repository` (Task 6); `mongoplatform.Client` (Task 5).
- Produces:
  - `measurements.New(db *mongodriver.Database) *Repository` — implementa `ingest.Repository`.
  - `(*Repository).EnsureIndexes(ctx context.Context) error`
  - `measurements.CollectionName = "measurements"`

- [ ] **Step 1: Escribir el test de integración — debe fallar**

Crear `internal/repo/mongo/measurements/repository_test.go`:

```go
package measurements_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
	mongodriver "go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/tu-org/embolsadora-api/internal/domain/ingest"
	"github.com/tu-org/embolsadora-api/internal/repo/mongo/measurements"
)

// newDB abre una base descartable por test. Sin MONGO_URI hace skip, igual que
// los tests de Postgres del repo.
func newDB(t *testing.T) *mongodriver.Database {
	t.Helper()
	uri := os.Getenv("MONGO_URI")
	if uri == "" {
		t.Skip("MONGO_URI no seteada; se omite el test de integracion")
	}
	// Driver v2: Connect no recibe context.
	cli, err := mongodriver.Connect(options.Client().ApplyURI(uri))
	require.NoError(t, err)

	db := cli.Database("embolsadora_test_" + uuid.NewString()[:8])
	t.Cleanup(func() {
		_ = db.Drop(context.Background())
		_ = cli.Disconnect(context.Background())
	})
	return db
}

func newRepo(t *testing.T) (*measurements.Repository, *mongodriver.Database) {
	t.Helper()
	db := newDB(t)
	repo := measurements.New(db)
	require.NoError(t, repo.EnsureIndexes(context.Background()))
	return repo, db
}

func doc(eventID string) ingest.Measurement {
	seq := int64(1)
	return ingest.Measurement{
		EventID:       eventID,
		TenantID:      uuid.NewString(),
		DeviceID:      uuid.NewString(),
		MachineID:     "EMB-DEV-001",
		Ts:            time.Now().UTC().Truncate(time.Millisecond),
		Seq:           &seq,
		Kind:          ingest.KindMetric,
		SchemaVersion: 1,
		Payload: map[string]any{
			"aasPath": "Operativos/Pesada/peso", "value": 1.0,
			"unit": "kg", "valueType": "xs:float",
		},
		ReceivedAt: time.Now().UTC(),
	}
}

func count(t *testing.T, db *mongodriver.Database) int64 {
	t.Helper()
	n, err := db.Collection(measurements.CollectionName).CountDocuments(context.Background(), bson.D{})
	require.NoError(t, err)
	return n
}

func TestInsertManyCleanBatch(t *testing.T) {
	repo, db := newRepo(t)

	docs := []ingest.Measurement{doc("a1"), doc("a2"), doc("a3")}
	report, err := repo.InsertMany(context.Background(), docs)
	require.NoError(t, err)

	assert.Empty(t, report.Duplicated)
	assert.Empty(t, report.Failed)
	assert.Equal(t, 3, report.Inserted(len(docs)))
	assert.EqualValues(t, 3, count(t, db))
}

// El nucleo de la idempotencia: reinsertar el mismo lote no crea documentos
// nuevos y reporta cada eventId repetido como DUPLICATE.
func TestInsertManyReportsDuplicates(t *testing.T) {
	repo, db := newRepo(t)
	ctx := context.Background()

	docs := []ingest.Measurement{doc("b1"), doc("b2")}
	_, err := repo.InsertMany(ctx, docs)
	require.NoError(t, err)

	report, err := repo.InsertMany(ctx, docs)
	require.NoError(t, err, "un lote 100% duplicado NO es un error de operacion")

	assert.Len(t, report.Duplicated, 2)
	assert.Contains(t, report.Duplicated, 0)
	assert.Contains(t, report.Duplicated, 1)
	assert.Empty(t, report.Failed)
	assert.EqualValues(t, 2, count(t, db), "no se duplicaron documentos")
}

// ordered:false es lo que hace que un duplicado en el medio no aborte el resto.
// Con ordered:true, "c2" frenaria el lote y "c3" se perderia.
func TestInsertManyIsUnorderedAndReportsCorrectIndices(t *testing.T) {
	repo, db := newRepo(t)
	ctx := context.Background()

	_, err := repo.InsertMany(ctx, []ingest.Measurement{doc("c2")})
	require.NoError(t, err)

	docs := []ingest.Measurement{doc("c1"), doc("c2"), doc("c3")}
	report, err := repo.InsertMany(ctx, docs)
	require.NoError(t, err)

	require.Len(t, report.Duplicated, 1)
	assert.Contains(t, report.Duplicated, 1, "el indice reportado es la posicion dentro de docs")
	assert.Equal(t, 2, report.Inserted(len(docs)))
	assert.EqualValues(t, 3, count(t, db))
}

func TestInsertManyEmptyIsNoop(t *testing.T) {
	repo, _ := newRepo(t)
	report, err := repo.InsertMany(context.Background(), nil)
	require.NoError(t, err)
	assert.Equal(t, 0, report.Inserted(0))
}

// Un evento sin aasPath se persiste igual: el indice compuesto es sparse. Esto
// cubre los eventos que quedaron en el outbox del Pi antes del commit 76c3805.
func TestInsertManyAcceptsPayloadWithoutAASPath(t *testing.T) {
	repo, db := newRepo(t)

	d := doc("d1")
	d.Payload = map[string]any{"value": 1.0, "unit": "kg"}
	_, err := repo.InsertMany(context.Background(), []ingest.Measurement{d})
	require.NoError(t, err)
	assert.EqualValues(t, 1, count(t, db))
}

func TestEnsureIndexesIsIdempotent(t *testing.T) {
	repo, db := newRepo(t)
	ctx := context.Background()

	require.NoError(t, repo.EnsureIndexes(ctx), "correrlo dos veces no debe fallar")

	cur, err := db.Collection(measurements.CollectionName).Indexes().List(ctx)
	require.NoError(t, err)
	var idx []bson.M
	require.NoError(t, cur.All(ctx, &idx))

	// _id + los 3 del diseno (§6.3).
	assert.Len(t, idx, 4)

	var foundUnique bool
	for _, i := range idx {
		if i["name"] == "uq_eventId" {
			foundUnique = true
			assert.Equal(t, true, i["unique"], "el indice de eventId DEBE ser unico")
		}
	}
	assert.True(t, foundUnique, "falta el indice unico sobre eventId")
}

func TestPing(t *testing.T) {
	repo, _ := newRepo(t)
	assert.NoError(t, repo.Ping(context.Background()))
}
```

- [ ] **Step 2: Correr el test para verificar que falla**

```bash
docker compose up -d mongo
export MONGO_URI='mongodb://localhost:27017'
go test ./internal/repo/mongo/measurements/... -v
```

Expected: FAIL — `undefined: measurements.New`.

- [ ] **Step 3: Implementar el repositorio**

Crear `internal/repo/mongo/measurements/repository.go`:

```go
// Package measurements es la unica implementacion de ingest.Repository, y el
// unico lugar del codigo —junto con internal/platform/mongo— que sabe que
// MongoDB existe.
package measurements

import (
	"context"
	"errors"

	"go.mongodb.org/mongo-driver/v2/bson"
	mongodriver "go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"

	"github.com/tu-org/embolsadora-api/internal/domain/ingest"
)

// CollectionName es la coleccion de mediciones.
const CollectionName = "measurements"

// duplicateKeyCode es el codigo de error de MongoDB para violacion de indice
// unico. Es la senal de idempotencia: no es un fallo, es "ya lo tenia".
const duplicateKeyCode = 11000

// Repository implementa ingest.Repository sobre MongoDB.
type Repository struct {
	coll *mongodriver.Collection
	db   *mongodriver.Database
}

// New construye el repositorio de mediciones.
func New(db *mongodriver.Database) *Repository {
	return &Repository{coll: db.Collection(CollectionName), db: db}
}

// EnsureIndexes crea los indices de §6.3 si no existen. Es idempotente, asi que
// se puede llamar en cada arranque.
func (r *Repository) EnsureIndexes(ctx context.Context) error {
	_, err := r.coll.Indexes().CreateMany(ctx, []mongodriver.IndexModel{
		{
			// El indice critico: es toda la idempotencia del sistema. Sin el,
			// cada reintento del Pi duplicaria mediciones en silencio.
			Keys:    bson.D{{Key: "eventId", Value: 1}},
			Options: options.Index().SetName("uq_eventId").SetUnique(true),
		},
		{
			// Sirve las dos consultas previstas del frontend: "ultimo valor de
			// esta propiedad" (igualdad en los tres primeros campos + limit 1) y
			// "esta propiedad en las ultimas 24 hs" (igualdad + rango sobre ts).
			//
			// Sparse porque los eventos anteriores al commit 76c3805 del Edge no
			// traen payload.aasPath: se persisten igual, solo no entran al
			// indice. Es la razon por la que el deploy de los dos repos no
			// necesita coordinarse.
			Keys: bson.D{
				{Key: "tenantId", Value: 1},
				{Key: "machineId", Value: 1},
				{Key: "payload.aasPath", Value: 1},
				{Key: "ts", Value: -1},
			},
			Options: options.Index().SetName("ix_tenant_machine_path_ts").SetSparse(true),
		},
		{
			// Barridos por tenant y lectura de lo reciente por el futuro motor
			// de alarmas.
			Keys:    bson.D{{Key: "tenantId", Value: 1}, {Key: "ts", Value: -1}},
			Options: options.Index().SetName("ix_tenant_ts"),
		},
	})
	return err
}

// InsertMany inserta el lote con ordered:false y traduce el resultado a un
// InsertReport.
//
// ordered:false es esencial y no una optimizacion: con ordered:true el primer
// duplicado abortaria el resto del lote, y en un reintento del Pi —donde el
// primer evento casi siempre ya existe— se perderian los 999 siguientes.
func (r *Repository) InsertMany(ctx context.Context, docs []ingest.Measurement) (ingest.InsertReport, error) {
	report := ingest.InsertReport{
		Duplicated: make(map[int]struct{}),
		Failed:     make(map[int]string),
	}
	if len(docs) == 0 {
		return report, nil
	}

	raw := make([]any, len(docs))
	for i := range docs {
		raw[i] = docs[i]
	}

	_, err := r.coll.InsertMany(ctx, raw, options.InsertMany().SetOrdered(false))
	if err == nil {
		return report, nil
	}

	// BulkWriteException significa que la operacion llego al servidor y este
	// respondio por documento. Todo lo demas —conexion caida, timeout, seleccion
	// de servidor— es un fallo total que se propaga para terminar en un 500.
	var bwe mongodriver.BulkWriteException
	if !errors.As(err, &bwe) {
		return ingest.InsertReport{}, err
	}

	for _, we := range bwe.WriteErrors {
		if we.Code == duplicateKeyCode {
			report.Duplicated[we.Index] = struct{}{}
			continue
		}
		report.Failed[we.Index] = we.Message
	}

	// Un write concern error afecta al lote entero, no a documentos puntuales:
	// no sabemos que se persistio. Se propaga como fallo total (I-1).
	if bwe.WriteConcernError != nil {
		return ingest.InsertReport{}, err
	}
	return report, nil
}

// Ping verifica la conectividad con el primario.
func (r *Repository) Ping(ctx context.Context) error {
	return r.db.Client().Ping(ctx, readpref.Primary())
}
```

- [ ] **Step 4: Correr los tests para verificar que pasan**

Run: `go test ./internal/repo/mongo/measurements/... -v`
Expected: PASS — los 7 tests en verde. Prestar atención especial a `TestInsertManyReportsDuplicates` y `TestInsertManyIsUnorderedAndReportsCorrectIndices`.

- [ ] **Step 5: Verificar el build completo**

Run: `go build ./...`
Expected: sin salida.

- [ ] **Step 6: Commit**

```bash
git add internal/repo/mongo/
git commit -m "feat(ingest): repositorio Mongo con idempotencia por indice unico sobre eventId"
```

---

# FASE 3 — Pipeline de ingesta

Donde viven los cuatro invariantes. La Task 8 es la más delicada del plan.

---

### Task 8: Service de ingesta — validación del sobre y mapeo de índices

**Files:**
- Create: `internal/app/ingest/validate.go`
- Create: `internal/app/ingest/service.go`
- Test: `internal/app/ingest/validate_test.go`
- Test: `internal/app/ingest/service_test.go`
- Delete: `internal/api/usecases/ingest/service.go`

**Interfaces:**
- Consumes: `ingest.Measurement`, `ingest.Result`, `ingest.EventError`, `ingest.DeviceContext`, `ingest.Repository`, `ingest.InsertReport`, códigos y kinds (Task 6).
- Produces:
  - `ingestapp.NewService(repo ingest.Repository, log *zap.Logger) *Service`
  - `(*Service).IngestBatch(ctx context.Context, dev ingest.DeviceContext, raw []json.RawMessage) (ingest.Result, error)` — devuelve error **solo** ante fallo total de storage.
  - `ingestapp.ValidateEvent(raw json.RawMessage, dev ingest.DeviceContext, now time.Time) (ingest.Measurement, *ingest.EventError)` — exportada para poder testearla sola; el `Index` del `EventError` que devuelve queda en 0 y lo setea el llamador.

- [ ] **Step 1: Escribir el test de validación — debe fallar**

Crear `internal/app/ingest/validate_test.go`:

```go
package ingest_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	ingestapp "github.com/tu-org/embolsadora-api/internal/app/ingest"
	"github.com/tu-org/embolsadora-api/internal/domain/ingest"
)

var dev = ingest.DeviceContext{
	TenantID:  "11111111-1111-1111-1111-111111111111",
	DeviceID:  "22222222-2222-2222-2222-222222222222",
	MachineID: "EMB-DEV-001",
}

const validEvent = `{
	"eventId": "c2a7a6a240e21113474b88c5352cec7ddbb3e942b29651b06173371afc09186f",
	"machineId": "EMB-DEV-001",
	"ts": "2026-07-31T01:06:37.147166Z",
	"seq": 6,
	"kind": "metric",
	"schemaVersion": 1,
	"payload": {"aasPath":"Operativos/Pesada/peso","value":1,"unit":"kg","valueType":"xs:float"}
}`

func TestValidateEventHappyPath(t *testing.T) {
	now := time.Now().UTC()
	m, evErr := ingestapp.ValidateEvent(json.RawMessage(validEvent), dev, now)
	require.Nil(t, evErr)

	assert.Equal(t, "c2a7a6a240e21113474b88c5352cec7ddbb3e942b29651b06173371afc09186f", m.EventID)
	assert.Equal(t, dev.TenantID, m.TenantID, "tenantId sale de la API key, no del body")
	assert.Equal(t, dev.DeviceID, m.DeviceID)
	assert.Equal(t, "EMB-DEV-001", m.MachineID)
	assert.Equal(t, 2026, m.Ts.Year())
	require.NotNil(t, m.Seq)
	assert.EqualValues(t, 6, *m.Seq)
	assert.Equal(t, ingest.KindMetric, m.Kind)
	assert.Equal(t, 1, m.SchemaVersion)
	assert.Equal(t, now, m.ReceivedAt)
	assert.Equal(t, "Operativos/Pesada/peso", m.Payload["aasPath"])
}

// seq es opcional: el primer evento de cada instante no lo trae.
func TestValidateEventWithoutSeq(t *testing.T) {
	doc := `{"eventId":"e1","machineId":"EMB-DEV-001","ts":"2026-07-31T01:06:37Z",
	         "kind":"metric","schemaVersion":1,"payload":{"value":1}}`
	m, evErr := ingestapp.ValidateEvent(json.RawMessage(doc), dev, time.Now().UTC())
	require.Nil(t, evErr)
	assert.Nil(t, m.Seq)
}

// El contenido de payload NO se valida jamas (D-8). Un payload vacio, con claves
// desconocidas o con tipos raros se persiste igual: validarlo haria que un
// cambio del catalogo AAS mandara datos reales a DEAD.
func TestValidateEventNeverValidatesPayloadContents(t *testing.T) {
	for _, payload := range []string{
		`{}`,
		`{"clave-desconocida":"lo que sea"}`,
		`{"value":null}`,
		`{"anidado":{"profundo":[1,2,{"x":true}]}}`,
	} {
		doc := `{"eventId":"e1","machineId":"EMB-DEV-001","ts":"2026-07-31T01:06:37Z",
		         "kind":"metric","schemaVersion":1,"payload":` + payload + `}`
		_, evErr := ingestapp.ValidateEvent(json.RawMessage(doc), dev, time.Now().UTC())
		assert.Nil(t, evErr, "payload %s no deberia rechazarse", payload)
	}
}

func TestValidateEventInvalidSchema(t *testing.T) {
	cases := map[string]string{
		"json roto":            `{"eventId":`,
		"no es objeto":         `"soy un string"`,
		"sin eventId":          `{"machineId":"EMB-DEV-001","ts":"2026-07-31T01:06:37Z","kind":"metric","schemaVersion":1,"payload":{}}`,
		"eventId vacio":        `{"eventId":"","machineId":"EMB-DEV-001","ts":"2026-07-31T01:06:37Z","kind":"metric","schemaVersion":1,"payload":{}}`,
		"sin machineId":        `{"eventId":"e1","ts":"2026-07-31T01:06:37Z","kind":"metric","schemaVersion":1,"payload":{}}`,
		"sin ts":               `{"eventId":"e1","machineId":"EMB-DEV-001","kind":"metric","schemaVersion":1,"payload":{}}`,
		"ts no RFC3339":        `{"eventId":"e1","machineId":"EMB-DEV-001","ts":"31/07/2026","kind":"metric","schemaVersion":1,"payload":{}}`,
		"ts numerico":          `{"eventId":"e1","machineId":"EMB-DEV-001","ts":1753923997,"kind":"metric","schemaVersion":1,"payload":{}}`,
		"sin kind":             `{"eventId":"e1","machineId":"EMB-DEV-001","ts":"2026-07-31T01:06:37Z","schemaVersion":1,"payload":{}}`,
		"kind fuera del enum":  `{"eventId":"e1","machineId":"EMB-DEV-001","ts":"2026-07-31T01:06:37Z","kind":"telemetria","schemaVersion":1,"payload":{}}`,
		"sin schemaVersion":    `{"eventId":"e1","machineId":"EMB-DEV-001","ts":"2026-07-31T01:06:37Z","kind":"metric","payload":{}}`,
		"schemaVersion 0":      `{"eventId":"e1","machineId":"EMB-DEV-001","ts":"2026-07-31T01:06:37Z","kind":"metric","schemaVersion":0,"payload":{}}`,
		"schemaVersion string": `{"eventId":"e1","machineId":"EMB-DEV-001","ts":"2026-07-31T01:06:37Z","kind":"metric","schemaVersion":"1","payload":{}}`,
		"schemaVersion float":  `{"eventId":"e1","machineId":"EMB-DEV-001","ts":"2026-07-31T01:06:37Z","kind":"metric","schemaVersion":1.5,"payload":{}}`,
		"sin payload":          `{"eventId":"e1","machineId":"EMB-DEV-001","ts":"2026-07-31T01:06:37Z","kind":"metric","schemaVersion":1}`,
		"payload es array":     `{"eventId":"e1","machineId":"EMB-DEV-001","ts":"2026-07-31T01:06:37Z","kind":"metric","schemaVersion":1,"payload":[1,2]}`,
		"payload es string":    `{"eventId":"e1","machineId":"EMB-DEV-001","ts":"2026-07-31T01:06:37Z","kind":"metric","schemaVersion":1,"payload":"x"}`,
		"seq negativo":         `{"eventId":"e1","machineId":"EMB-DEV-001","ts":"2026-07-31T01:06:37Z","kind":"metric","schemaVersion":1,"seq":-1,"payload":{}}`,
		"seq string":           `{"eventId":"e1","machineId":"EMB-DEV-001","ts":"2026-07-31T01:06:37Z","kind":"metric","schemaVersion":1,"seq":"6","payload":{}}`,
		"seq float":            `{"eventId":"e1","machineId":"EMB-DEV-001","ts":"2026-07-31T01:06:37Z","kind":"metric","schemaVersion":1,"seq":1.5,"payload":{}}`,
	}
	for name, doc := range cases {
		t.Run(name, func(t *testing.T) {
			_, evErr := ingestapp.ValidateEvent(json.RawMessage(doc), dev, time.Now().UTC())
			require.NotNil(t, evErr, "deberia rechazarse")
			assert.Equal(t, ingest.CodeInvalidSchema, evErr.Code)
		})
	}
}

// El sobre esta bien formado, pero el evento no corresponde a este device o a
// esta version. Es VALIDATION_FAILED, no INVALID_SCHEMA.
func TestValidateEventValidationFailed(t *testing.T) {
	cases := map[string]string{
		"machineId de otra maquina": `{"eventId":"e1","machineId":"EMB-OTRA-999","ts":"2026-07-31T01:06:37Z","kind":"metric","schemaVersion":1,"payload":{}}`,
		"schemaVersion del futuro":  `{"eventId":"e1","machineId":"EMB-DEV-001","ts":"2026-07-31T01:06:37Z","kind":"metric","schemaVersion":99,"payload":{}}`,
	}
	for name, doc := range cases {
		t.Run(name, func(t *testing.T) {
			_, evErr := ingestapp.ValidateEvent(json.RawMessage(doc), dev, time.Now().UTC())
			require.NotNil(t, evErr)
			assert.Equal(t, ingest.CodeValidationFailed, evErr.Code)
		})
	}
}

func TestValidateEventAcceptsAllKinds(t *testing.T) {
	for _, kind := range []string{ingest.KindMetric, ingest.KindAlarm, ingest.KindHeartbeat} {
		doc := `{"eventId":"e1","machineId":"EMB-DEV-001","ts":"2026-07-31T01:06:37Z",
		         "kind":"` + kind + `","schemaVersion":1,"payload":{}}`
		_, evErr := ingestapp.ValidateEvent(json.RawMessage(doc), dev, time.Now().UTC())
		assert.Nil(t, evErr, "kind %q es del enum del contrato", kind)
	}
}
```

- [ ] **Step 2: Correr el test para verificar que falla**

Run: `go test ./internal/app/ingest/... -v`
Expected: FAIL — `undefined: ingestapp.ValidateEvent`.

- [ ] **Step 3: Implementar la validación**

Crear `internal/app/ingest/validate.go`:

```go
// Package ingest implementa el caso de uso de ingesta de mediciones.
package ingest

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	domain "github.com/tu-org/embolsadora-api/internal/domain/ingest"
)

// rawEvent es el sobre tal como llega. Todos los campos son punteros para poder
// distinguir "ausente" de "presente con el valor cero": sin esa distincion, un
// schemaVersion faltante y un schemaVersion 0 serian el mismo caso.
//
// Los numericos son json.RawMessage y no int: si fueran int, un valor de tipo
// equivocado abortaria el decode del evento entero con un mensaje generico, y
// perderiamos la posibilidad de decir cual campo estaba mal.
type rawEvent struct {
	EventID       *string          `json:"eventId"`
	MachineID     *string          `json:"machineId"`
	Ts            *string          `json:"ts"`
	Seq           *json.RawMessage `json:"seq"`
	Kind          *string          `json:"kind"`
	SchemaVersion *json.RawMessage `json:"schemaVersion"`
	Payload       *map[string]any  `json:"payload"`
}

// jsonInt parsea un entero JSON de forma estricta.
//
// El chequeo del primer byte no es paranoia: encoding/json acepta el STRING
// "1" dentro de un json.Number sin devolver error, asi que un
// `"schemaVersion": "1"` pasaria por valido si confiaramos en el decoder.
// Tambien rechaza 1.5 y 1e3, que el contrato no admite.
func jsonInt(raw json.RawMessage) (int64, bool) {
	s := strings.TrimSpace(string(raw))
	if s == "" || s[0] == '"' {
		return 0, false
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, false
	}
	return n, true
}

func invalid(format string, args ...any) *domain.EventError {
	return &domain.EventError{Code: domain.CodeInvalidSchema, Message: fmt.Sprintf(format, args...)}
}

func failed(format string, args ...any) *domain.EventError {
	return &domain.EventError{Code: domain.CodeValidationFailed, Message: fmt.Sprintf(format, args...)}
}

// ValidateEvent valida el SOBRE de un evento y lo convierte en Measurement.
//
// El contenido de `payload` no se valida nunca (D-8): se copia tal cual. Si
// validaramos el payload, cualquier cambio del catalogo AAS del Edge produciria
// INVALID_SCHEMA y el Edge mandaria mediciones reales a DEAD, sin reintento.
//
// El campo Index del EventError devuelto queda en 0: lo setea el llamador, que
// es el unico que conoce la posicion en el array original (invariante I-3).
func ValidateEvent(raw json.RawMessage, dev domain.DeviceContext, now time.Time) (domain.Measurement, *domain.EventError) {
	var e rawEvent
	if err := json.Unmarshal(raw, &e); err != nil {
		return domain.Measurement{}, invalid("evento no deserializable: %v", err)
	}

	switch {
	case e.EventID == nil || *e.EventID == "":
		return domain.Measurement{}, invalid("falta el campo requerido eventId")
	case e.MachineID == nil || *e.MachineID == "":
		return domain.Measurement{}, invalid("falta el campo requerido machineId")
	case e.Ts == nil:
		return domain.Measurement{}, invalid("falta el campo requerido ts")
	case e.Kind == nil:
		return domain.Measurement{}, invalid("falta el campo requerido kind")
	case e.SchemaVersion == nil:
		return domain.Measurement{}, invalid("falta el campo requerido schemaVersion")
	case e.Payload == nil:
		return domain.Measurement{}, invalid("falta el campo requerido payload")
	}

	ts, err := time.Parse(time.RFC3339, *e.Ts)
	if err != nil {
		return domain.Measurement{}, invalid("ts no es RFC3339: %q", *e.Ts)
	}

	switch *e.Kind {
	case domain.KindMetric, domain.KindAlarm, domain.KindHeartbeat:
	default:
		return domain.Measurement{}, invalid("kind fuera del enum: %q", *e.Kind)
	}

	schemaVersion, ok := jsonInt(*e.SchemaVersion)
	if !ok || schemaVersion < 1 {
		return domain.Measurement{}, invalid("schemaVersion debe ser un entero >= 1")
	}

	var seq *int64
	if e.Seq != nil {
		n, ok := jsonInt(*e.Seq)
		if !ok || n < 0 {
			return domain.Measurement{}, invalid("seq debe ser un entero >= 0")
		}
		seq = &n
	}

	// A partir de aca el sobre es valido; lo que sigue son rechazos de
	// VALIDATION_FAILED, que el Edge tambien manda a DEAD pero que significan
	// otra cosa: el dato esta bien formado y aun asi no corresponde.

	// Un Pi comprometido no puede escribir en nombre de otra maquina: el
	// machineId del body solo se acepta si coincide con el device de la key.
	if *e.MachineID != dev.MachineID {
		return domain.Measurement{}, failed("machineId %q no corresponde al device de la API key", *e.MachineID)
	}
	if schemaVersion > domain.MaxSchemaVersion {
		return domain.Measurement{}, failed("schemaVersion %d no soportada (maxima: %d)", schemaVersion, domain.MaxSchemaVersion)
	}

	return domain.Measurement{
		EventID: *e.EventID,
		// tenantId y deviceId salen SIEMPRE de la API key resuelta (D-10).
		TenantID:      dev.TenantID,
		DeviceID:      dev.DeviceID,
		MachineID:     *e.MachineID,
		Ts:            ts.UTC(),
		Seq:           seq,
		Kind:          *e.Kind,
		SchemaVersion: int(schemaVersion),
		Payload:       *e.Payload,
		ReceivedAt:    now,
	}, nil
}
```

- [ ] **Step 4: Correr el test de validación**

Run: `go test ./internal/app/ingest/... -run TestValidateEvent -v`
Expected: PASS — los 5 tests, incluidos los 19 subcasos de `TestValidateEventInvalidSchema`.

- [ ] **Step 5: Escribir el test del service — debe fallar**

Crear `internal/app/ingest/service_test.go`:

```go
package ingest_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	ingestapp "github.com/tu-org/embolsadora-api/internal/app/ingest"
	"github.com/tu-org/embolsadora-api/internal/domain/ingest"
)

// fakeRepo permite guionar el desenlace de InsertMany por posicion.
type fakeRepo struct {
	report   ingest.InsertReport
	err      error
	received []ingest.Measurement
}

func (f *fakeRepo) InsertMany(_ context.Context, docs []ingest.Measurement) (ingest.InsertReport, error) {
	f.received = docs
	if f.err != nil {
		return ingest.InsertReport{}, f.err
	}
	r := f.report
	if r.Duplicated == nil {
		r.Duplicated = map[int]struct{}{}
	}
	if r.Failed == nil {
		r.Failed = map[int]string{}
	}
	return r, nil
}
func (f *fakeRepo) Ping(context.Context) error { return nil }

func rawEvents(docs ...string) []json.RawMessage {
	out := make([]json.RawMessage, len(docs))
	for i, d := range docs {
		out[i] = json.RawMessage(d)
	}
	return out
}

func evt(id string) string {
	return `{"eventId":"` + id + `","machineId":"EMB-DEV-001","ts":"2026-07-31T01:06:37Z",
	         "kind":"metric","schemaVersion":1,"payload":{"value":1}}`
}

func TestIngestBatchAllValid(t *testing.T) {
	repo := &fakeRepo{}
	svc := ingestapp.NewService(repo, zap.NewNop())

	res, err := svc.IngestBatch(context.Background(), dev, rawEvents(evt("a"), evt("b"), evt("c")))
	require.NoError(t, err)

	assert.Equal(t, 3, res.Accepted)
	assert.Equal(t, 0, res.Rejected)
	assert.Empty(t, res.Errors)
	assert.Len(t, repo.received, 3)
}

// SC-003 exacto: [valido, valido, sin ts, valido, ya existente] debe reportar
// los indices 2 y 4 — posiciones del array ORIGINAL, no del filtrado.
//
// Este es el test del invariante I-3. Si el service reportara indices del slice
// que le pasa al repo, el duplicado (posicion 3 de ese slice, que no existe)
// caeria en el lugar equivocado y el Edge mataria el evento sano.
func TestIngestBatchReportsOriginalIndices(t *testing.T) {
	// El evento valido en la posicion 3 del array original es el indice 2 del
	// slice filtrado, y ese es el que el repo marca como duplicado.
	repo := &fakeRepo{report: ingest.InsertReport{Duplicated: map[int]struct{}{3: {}}}}
	svc := ingestapp.NewService(repo, zap.NewNop())

	sinTs := `{"eventId":"c","machineId":"EMB-DEV-001","kind":"metric","schemaVersion":1,"payload":{}}`
	batch := rawEvents(evt("a"), evt("b"), sinTs, evt("d"), evt("e"))

	res, err := svc.IngestBatch(context.Background(), dev, batch)
	require.NoError(t, err)

	assert.Equal(t, 3, res.Accepted)
	assert.Equal(t, 2, res.Rejected)
	require.Len(t, res.Errors, 2)

	assert.Equal(t, 2, res.Errors[0].Index)
	assert.Equal(t, ingest.CodeInvalidSchema, res.Errors[0].Code)

	assert.Equal(t, 4, res.Errors[1].Index)
	assert.Equal(t, ingest.CodeDuplicate, res.Errors[1].Code)
}

// I-4 sobre una combinacion amplia de casos.
func TestIngestBatchAccountingInvariant(t *testing.T) {
	cases := []struct {
		name   string
		batch  []json.RawMessage
		report ingest.InsertReport
	}{
		{"todos validos", rawEvents(evt("a"), evt("b")), ingest.InsertReport{}},
		{"todos duplicados", rawEvents(evt("a"), evt("b")),
			ingest.InsertReport{Duplicated: map[int]struct{}{0: {}, 1: {}}}},
		{"todos invalidos", rawEvents(`{}`, `{"x":1}`), ingest.InsertReport{}},
		{"mixto con storage", rawEvents(evt("a"), `{}`, evt("c")),
			ingest.InsertReport{Failed: map[int]string{1: "disco lleno"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := ingestapp.NewService(&fakeRepo{report: tc.report}, zap.NewNop())
			res, err := svc.IngestBatch(context.Background(), dev, tc.batch)
			require.NoError(t, err)
			assert.Equal(t, len(tc.batch), res.Accepted+res.Rejected,
				"I-4: accepted + rejected debe ser igual a len(events)")
			assert.Equal(t, res.Rejected, len(res.Errors))
		})
	}
}

// I-1: un fallo de storage se reporta como STORAGE_UNAVAILABLE, que el Edge
// reintenta. Nunca como INVALID_SCHEMA ni VALIDATION_FAILED.
func TestIngestBatchPartialStorageFailure(t *testing.T) {
	repo := &fakeRepo{report: ingest.InsertReport{Failed: map[int]string{1: "write concern"}}}
	svc := ingestapp.NewService(repo, zap.NewNop())

	res, err := svc.IngestBatch(context.Background(), dev, rawEvents(evt("a"), evt("b")))
	require.NoError(t, err)

	assert.Equal(t, 1, res.Accepted)
	require.Len(t, res.Errors, 1)
	assert.Equal(t, 1, res.Errors[0].Index)
	assert.Equal(t, ingest.CodeStorageUnavailable, res.Errors[0].Code)
}

// I-1 en su forma fuerte: si Mongo esta caido, el service propaga el error para
// que el handler devuelva 500. No inventa un resultado parcial ni marca los
// eventos como invalidos.
func TestIngestBatchTotalStorageFailurePropagates(t *testing.T) {
	boom := errors.New("mongo inalcanzable")
	svc := ingestapp.NewService(&fakeRepo{err: boom}, zap.NewNop())

	_, err := svc.IngestBatch(context.Background(), dev, rawEvents(evt("a")))
	assert.ErrorIs(t, err, boom)
}

// Si TODOS los eventos son invalidos no hay nada que insertar: no se debe
// llamar al repo, y menos aun devolver 500 por un batch de basura.
func TestIngestBatchAllInvalidSkipsRepo(t *testing.T) {
	repo := &fakeRepo{err: errors.New("no deberia llamarse")}
	svc := ingestapp.NewService(repo, zap.NewNop())

	res, err := svc.IngestBatch(context.Background(), dev, rawEvents(`{}`, `{}`))
	require.NoError(t, err)
	assert.Equal(t, 0, res.Accepted)
	assert.Equal(t, 2, res.Rejected)
	assert.Nil(t, repo.received)
}
```

- [ ] **Step 6: Correr el test para verificar que falla**

Run: `go test ./internal/app/ingest/... -run TestIngestBatch -v`
Expected: FAIL — `undefined: ingestapp.NewService`.

- [ ] **Step 7: Implementar el service**

Crear `internal/app/ingest/service.go`:

```go
package ingest

import (
	"context"
	"encoding/json"
	"sort"
	"time"

	"go.uber.org/zap"

	domain "github.com/tu-org/embolsadora-api/internal/domain/ingest"
)

// Service orquesta la ingesta de un batch.
type Service struct {
	repo domain.Repository
	log  *zap.Logger
	now  func() time.Time
}

// NewService construye el service de ingesta.
func NewService(repo domain.Repository, log *zap.Logger) *Service {
	return &Service{repo: repo, log: log, now: func() time.Time { return time.Now().UTC() }}
}

// IngestBatch valida el sobre de cada evento y persiste los validos.
//
// Devuelve error SOLO si la escritura fallo entera (Mongo inalcanzable): ese
// caso lo traduce el handler a HTTP 500 y el Edge reintenta con backoff. Todo
// lo demas —eventos invalidos, duplicados, fallos parciales— viaja dentro del
// Result como un 200 con errors[], porque un 400 mandaria el batch entero a
// DEAD (invariante I-2).
func (s *Service) IngestBatch(ctx context.Context, dev domain.DeviceContext, raw []json.RawMessage) (domain.Result, error) {
	now := s.now()

	valid := make([]domain.Measurement, 0, len(raw))
	// origIndex[j] es la posicion en `raw` del documento que quedo en valid[j].
	// Esta correspondencia es lo unico que sostiene el invariante I-3: el repo
	// reporta indices sobre `valid`, y el Edge espera indices sobre `raw`.
	origIndex := make([]int, 0, len(raw))
	errs := make([]domain.EventError, 0)

	for i, item := range raw {
		m, evErr := ValidateEvent(item, dev, now)
		if evErr != nil {
			evErr.Index = i
			errs = append(errs, *evErr)
			continue
		}
		valid = append(valid, m)
		origIndex = append(origIndex, i)
	}

	if len(valid) > 0 {
		report, err := s.repo.InsertMany(ctx, valid)
		if err != nil {
			s.log.Error("fallo total al persistir el batch",
				zap.Error(err),
				zap.String("tenant_id", dev.TenantID),
				zap.String("machine_id", dev.MachineID),
				zap.Int("eventos", len(valid)),
			)
			return domain.Result{}, err
		}

		for j := range valid {
			if _, dup := report.Duplicated[j]; dup {
				errs = append(errs, domain.EventError{
					Index:   origIndex[j],
					Code:    domain.CodeDuplicate,
					Message: "el evento ya habia sido ingerido",
				})
				continue
			}
			if msg, bad := report.Failed[j]; bad {
				// I-1: esto es infraestructura, no payload. El codigo tiene que
				// ser retriable o el Edge dara el evento por muerto.
				errs = append(errs, domain.EventError{
					Index:   origIndex[j],
					Code:    domain.CodeStorageUnavailable,
					Message: msg,
				})
			}
		}
	}

	// Orden por indice: la respuesta es determinista y el mapa de errores del
	// repo no impone su orden de iteracion, que en Go es aleatorio.
	sort.Slice(errs, func(a, b int) bool { return errs[a].Index < errs[b].Index })

	res := domain.Result{
		// I-4 por construccion: cada evento rechazado aporta exactamente una
		// entrada a errs, asi que accepted + rejected == len(raw) siempre.
		Accepted: len(raw) - len(errs),
		Rejected: len(errs),
	}
	if len(errs) > 0 {
		res.Errors = errs
	}
	return res, nil
}
```

- [ ] **Step 8: Borrar el stub viejo**

```bash
git rm internal/api/usecases/ingest/service.go
```

Era un archivo con `package ingest` y un TODO, sin declaraciones. El service real vive en `internal/app/ingest/` (§7 del diseño); dejar los dos invita a importar el equivocado.

- [ ] **Step 9: Correr todos los tests del paquete y el build**

```bash
go test ./internal/app/ingest/... -v
go build ./...
```

Expected: PASS en los 11 tests. Verificar especialmente `TestIngestBatchReportsOriginalIndices` (I-3) y `TestIngestBatchAccountingInvariant` (I-4).

- [ ] **Step 10: Commit**

```bash
git add internal/app/ingest/
git commit -m "feat(ingest): service de ingesta con validacion de sobre e indices originales"
```

---

### Task 9: Handler HTTP y DTOs

**Files:**
- Create: `internal/consumers/dto/events.go`
- Modify: `internal/consumers/events_handler.go`

**Interfaces:**
- Consumes: `ingestapp.Service` (Task 8), `security.DeviceIdentityFrom` (Task 4), `ingest.Result` (Task 6).
- Produces:
  - `dto.BatchEventsRequest{Events []json.RawMessage}`, `dto.BatchEventsResponse{Data ingest.Result}`.
  - `consumers.HandlerConfig{MaxBodyBytes int64; MaxEvents int}`.
  - `consumers.IngestEvents(svc *ingestapp.Service, cfg HandlerConfig, log *zap.Logger) gin.HandlerFunc` — **cambia de firma**: antes era `func(c *gin.Context)`.

- [ ] **Step 1: Escribir los DTOs**

Crear `internal/consumers/dto/events.go`:

```go
// Package dto contiene los tipos de transporte del contrato con el Edge.
package dto

import (
	"encoding/json"

	"github.com/tu-org/embolsadora-api/internal/domain/ingest"
)

// BatchEventsRequest es el body del POST /api/v1/consumers/events.
//
// Events es []json.RawMessage y no []Event a proposito: si fuera una lista de
// structs tipados, UN evento con un tipo equivocado —un ts numerico, por
// ejemplo— haria fallar el Unmarshal del body entero. Eso obligaria a devolver
// 400, y el Edge mandaria los hasta 1000 eventos del batch a DEAD por culpa de
// uno solo (invariante I-2). Difiriendo el decode a cada elemento, el evento
// roto se rechaza solo, con su indice.
type BatchEventsRequest struct {
	Events []json.RawMessage `json:"events"`
}

// BatchEventsResponse es la respuesta 200.
//
// El contrato pide {"data": {...}} pelado. NO lleva el envelope
// {"success": true, "data": ...} que usa el resto de los handlers del repo:
// el parser del Edge esta congelado contra esta forma.
type BatchEventsResponse struct {
	Data ingest.Result `json:"data"`
}
```

- [ ] **Step 2: Implementar el handler**

Reemplazar **todo** el contenido de `internal/consumers/events_handler.go`:

```go
package consumers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	ingestapp "github.com/tu-org/embolsadora-api/internal/app/ingest"
	"github.com/tu-org/embolsadora-api/internal/consumers/dto"
	"github.com/tu-org/embolsadora-api/internal/security"
	"github.com/tu-org/embolsadora-api/internal/telemetry"
)

// HandlerConfig son los limites de la ingesta.
type HandlerConfig struct {
	MaxBodyBytes int64
	MaxEvents    int
}

// IngestEvents recibe un batch de eventos del Edge Pi Service.
//
// Sobre los codigos de respuesta, que no son intercambiables:
//   - 400 significa "todo el batch es irrecuperable" y el Edge manda hasta 1000
//     eventos a DEAD. Queda reservado a requests rotos a nivel de SOBRE.
//   - Los problemas de eventos individuales van por 200 + errors[] (I-2).
//   - 500 es "no pude guardarlo ahora"; el Edge reintenta con backoff.
func IngestEvents(svc *ingestapp.Service, cfg HandlerConfig, log *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		identity := security.DeviceIdentityFrom(c.Request.Context())
		if identity == nil {
			// Solo puede pasar si el middleware APIKeyAuth no esta cableado.
			log.Error("IngestEvents sin identidad en contexto: APIKeyAuth no esta en la cadena")
			c.JSON(http.StatusInternalServerError, gin.H{"message": "error interno"})
			return
		}

		// El tope de bytes se aplica ANTES de parsear: un body de 2 GB no debe
		// poder consumir memoria mientras se deserializa.
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, cfg.MaxBodyBytes)
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			var tooLarge *http.MaxBytesError
			if errors.As(err, &tooLarge) {
				log.Warn("batch por encima del tope de bytes",
					zap.String("machine_id", identity.MachineID),
					zap.Int64("limite", cfg.MaxBodyBytes))
				c.JSON(http.StatusBadRequest, gin.H{"message": "body demasiado grande"})
				return
			}
			c.JSON(http.StatusBadRequest, gin.H{"message": "no se pudo leer el body"})
			return
		}

		var req dto.BatchEventsRequest
		if err := json.Unmarshal(body, &req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "JSON malformado"})
			return
		}

		switch {
		case len(req.Events) == 0:
			c.JSON(http.StatusBadRequest, gin.H{"message": "events es obligatorio y no puede estar vacio"})
			return
		case len(req.Events) > cfg.MaxEvents:
			c.JSON(http.StatusBadRequest, gin.H{"message": "events excede el maximo de elementos"})
			return
		}

		// El Idempotency-Key se registra para trazabilidad pero no decide nada
		// (D-6): la garantia fuerte la da el indice unico sobre eventId. Una
		// cache de respuestas seria una segunda fuente de verdad con casos borde
		// —request en vuelo, Redis caido— para un beneficio acotado.
		idemKey := c.GetHeader("Idempotency-Key")
		if len(idemKey) > 64 {
			idemKey = idemKey[:64]
		}

		res, err := svc.IngestBatch(c.Request.Context(), identity.ToDeviceContext(), req.Events)
		if err != nil {
			// I-1: infraestructura caida es 500, jamas 400 ni INVALID_SCHEMA.
			// Un 400 aca convertiria una caida de diez minutos en perdida
			// permanente de datos.
			telemetry.IngestBatchesTotal.WithLabelValues("error").Inc()
			c.JSON(http.StatusInternalServerError, gin.H{"message": "no se pudo persistir el batch"})
			return
		}

		telemetry.IngestBatchesTotal.WithLabelValues("ok").Inc()
		telemetry.IngestEventsAcceptedTotal.Add(float64(res.Accepted))
		for _, e := range res.Errors {
			telemetry.IngestEventsRejectedTotal.WithLabelValues(e.Code).Inc()
		}

		log.Info("batch ingerido",
			zap.String("machine_id", identity.MachineID),
			zap.String("tenant_id", identity.TenantID.String()),
			zap.String("idempotency_key", idemKey),
			zap.Int("recibidos", len(req.Events)),
			zap.Int("aceptados", res.Accepted),
			zap.Int("rechazados", res.Rejected),
		)

		c.JSON(http.StatusOK, dto.BatchEventsResponse{Data: res})
	}
}
```

- [ ] **Step 3: Agregar el conversor de identidad**

`security.DeviceIdentity` (que es `apikeys.DeviceIdentity`) usa `uuid.UUID`; `ingest.DeviceContext` usa `string`. Agregar el conversor al final de `internal/domain/apikeys/apikey.go`:

```go
// ToDeviceContext convierte la identidad a la forma que espera el dominio de
// ingesta, que trabaja con strings porque no le interesa que los ids sean UUID.
func (d *DeviceIdentity) ToDeviceContext() ingest.DeviceContext {
	return ingest.DeviceContext{
		TenantID:  d.TenantID.String(),
		DeviceID:  d.DeviceID.String(),
		MachineID: d.MachineID,
	}
}
```

Y agregar el import `"github.com/tu-org/embolsadora-api/internal/domain/ingest"` a ese archivo. No hay ciclo: `domain/ingest` no importa `domain/apikeys`.

- [ ] **Step 4: Verificar que compila**

Run: `go build ./...`
Expected: falla con `undefined: telemetry.IngestBatchesTotal` — las métricas llegan en la Task 13. Crear ahora `internal/telemetry/ingest_metrics.go` con el contenido de la Task 13, Step 1, y volver a correr.

- [ ] **Step 5: Commit**

```bash
git add internal/consumers/ internal/domain/apikeys/apikey.go internal/telemetry/ingest_metrics.go
git commit -m "feat(ingest): handler HTTP del endpoint de eventos, reemplaza el 501"
```

---

### Task 10: Middlewares reales — APIKeyAuth y RateLimit

**Files:**
- Create: `internal/consumers/ratelimit.go`
- Modify: `internal/consumers/middleware/middleware.go`
- Modify: `internal/consumers/router.go`

**Interfaces:**
- Consumes: `security.Authenticator`, `security.ErrAPIKeyInvalid`, `security.WithDeviceIdentity` (Task 4); `consumers.IngestEvents` (Task 9).
- Produces:
  - `consumermw.APIKeyAuth(auth security.Authenticator, log *zap.Logger) gin.HandlerFunc`
  - `consumermw.RateLimit(limiter *consumers.RateLimiter) gin.HandlerFunc`
  - `consumers.NewRateLimiter(rdb *redis.Client, rps, burst int) *RateLimiter`, con `Allow(ctx, key string) (bool, int, error)`.
  - `consumers.Deps{Auth, Ingest, Limiter, Log}`, `consumers.Config{MaxBodyBytes, MaxEvents}`.

- [ ] **Step 1: Implementar el rate limiter**

Crear `internal/consumers/ratelimit.go`:

```go
package consumers

import (
	"context"
	"math"

	"github.com/go-redis/redis/v8"
)

// tokenBucketScript implementa un token bucket en Redis.
//
// Va en Lua y no en Go porque leer-decidir-escribir desde la aplicacion tiene
// una condicion de carrera entre replicas: dos instancias podrian ver el mismo
// saldo de tokens y ambas dejar pasar. El script se ejecuta atomicamente.
//
// Devuelve {permitido, retry_after_segundos}.
const tokenBucketScript = `
local rate      = tonumber(ARGV[1])
local burst     = tonumber(ARGV[2])
local now       = tonumber(ARGV[3])
local requested = tonumber(ARGV[4])

local data   = redis.call('HMGET', KEYS[1], 'tokens', 'ts')
local tokens = tonumber(data[1])
local ts     = tonumber(data[2])
if tokens == nil then
  tokens = burst
  ts = now
end

local delta = math.max(0, now - ts) / 1000.0
tokens = math.min(burst, tokens + delta * rate)

local allowed = 0
if tokens >= requested then
  tokens = tokens - requested
  allowed = 1
end

redis.call('HSET', KEYS[1], 'tokens', tokens, 'ts', now)
redis.call('PEXPIRE', KEYS[1], math.ceil((burst / rate) * 1000) + 1000)

local retry_after = 0
if allowed == 0 then
  retry_after = math.ceil((requested - tokens) / rate)
  if retry_after < 1 then retry_after = 1 end
end

return {allowed, retry_after}
`

// RateLimiter limita requests por API key con un token bucket distribuido.
type RateLimiter struct {
	rdb   *redis.Client
	rps   int
	burst int
	script *redis.Script
}

// NewRateLimiter construye el limitador. `rdb` puede ser nil: sin Redis no se
// limita nada. Es la misma politica de fail-open que el resto del repo — un
// Redis caido no puede cortar la ingesta, porque el costo de rechazar datos
// reales es mayor que el de no limitar por un rato.
func NewRateLimiter(rdb *redis.Client, rps, burst int) *RateLimiter {
	if rps <= 0 {
		rps = 200
	}
	if burst <= 0 {
		burst = 1000
	}
	return &RateLimiter{rdb: rdb, rps: rps, burst: burst, script: redis.NewScript(tokenBucketScript)}
}

// Allow consume un token para `key`. Devuelve si se permite el request y,
// si no, cuantos segundos esperar. Ante cualquier error de Redis permite pasar.
func (l *RateLimiter) Allow(ctx context.Context, key string) (bool, int, error) {
	if l.rdb == nil {
		return true, 0, nil
	}
	now := timeNowMillis()
	res, err := l.script.Run(ctx, l.rdb, []string{"ratelimit:v1:" + key},
		l.rps, l.burst, now, 1).Result()
	if err != nil {
		return true, 0, err
	}
	vals, ok := res.([]any)
	if !ok || len(vals) != 2 {
		return true, 0, nil
	}
	allowed, _ := vals[0].(int64)
	retryAfter, _ := vals[1].(int64)
	return allowed == 1, int(math.Max(float64(retryAfter), 0)), nil
}
```

Agregar también, en el mismo archivo:

```go
import "time"

func timeNowMillis() int64 { return time.Now().UnixMilli() }
```

(consolidar los imports en un solo bloque).

- [ ] **Step 2: Implementar los middlewares**

Reemplazar **todo** el contenido de `internal/consumers/middleware/middleware.go`:

```go
package middleware

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/tu-org/embolsadora-api/internal/consumers"
	"github.com/tu-org/embolsadora-api/internal/security"
	"github.com/tu-org/embolsadora-api/internal/telemetry"
)

// APIKeyAuth resuelve el header X-Api-Key a la identidad del device.
//
// Ante credencial invalida responde 403, no 401. El codigo esta fijado por el
// contrato: el Edge trata al 403 como "reintentar, probablemente haya una
// rotacion de key en curso". Un 401 no esta en su tabla y no aporta nada.
func APIKeyAuth(auth security.Authenticator, log *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.GetHeader("X-Api-Key")
		if key == "" {
			telemetry.IngestAuthTotal.WithLabelValues("missing").Inc()
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"message": "falta el header X-Api-Key"})
			return
		}

		identity, err := auth.Authenticate(c.Request.Context(), key)
		if err != nil {
			if errors.Is(err, security.ErrAPIKeyInvalid) {
				telemetry.IngestAuthTotal.WithLabelValues("invalid").Inc()
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"message": "API key invalida"})
				return
			}
			// Postgres caido al resolver la key no es una credencial invalida.
			// Es un 500 para que el Edge reintente, no un 403 (I-1 aplicado a auth).
			log.Error("error resolviendo la API key", zap.Error(err))
			telemetry.IngestAuthTotal.WithLabelValues("error").Inc()
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "error de autenticacion"})
			return
		}

		telemetry.IngestAuthTotal.WithLabelValues("ok").Inc()
		c.Request = c.Request.WithContext(security.WithDeviceIdentity(c.Request.Context(), identity))
		c.Next()
	}
}

// RateLimit aplica el token bucket por key_id.
//
// Va DESPUES de APIKeyAuth para poder limitar por key y no por IP: todos los Pi
// de una misma planta pueden salir por la misma IP.
func RateLimit(limiter *consumers.RateLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		identity := security.DeviceIdentityFrom(c.Request.Context())
		if identity == nil {
			c.Next()
			return
		}

		allowed, retryAfter, _ := limiter.Allow(c.Request.Context(), identity.KeyID)
		if !allowed {
			telemetry.IngestRateLimitedTotal.Inc()
			// El Retry-After no es decorativo: el Edge lo lee y espera ese
			// tiempo exacto antes de reintentar.
			c.Header("Retry-After", strconv.Itoa(retryAfter))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"message": "rate limit excedido"})
			return
		}
		c.Next()
	}
}

// NoCORS marca la superficie de consumers como no navegable desde un browser:
// la consumen dispositivos, no paginas. No emitir headers CORS hace que
// cualquier preflight falle, que es exactamente lo que se quiere.
func NoCORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusMethodNotAllowed)
			return
		}
		c.Next()
	}
}
```

`Idempotency()` y `Timeout()` se **eliminan**: el `Idempotency-Key` lo maneja el handler (D-6) y el timeout de la escritura lo aplica el driver de Mongo vía `MONGO_TIMEOUT`. Un middleware que no hace nada es peor que ninguno, porque sugiere una garantía que no existe.

- [ ] **Step 3: Cablear el router**

Reemplazar **todo** el contenido de `internal/consumers/router.go`:

```go
package consumers

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	ingestapp "github.com/tu-org/embolsadora-api/internal/app/ingest"
)

// Deps son las dependencias de la superficie de consumers.
type Deps struct {
	Ingest *ingestapp.Service
	Log    *zap.Logger
}

// Config son los limites de la ingesta.
type Config struct {
	MaxBodyBytes int64
	MaxEvents    int
}

// RegisterConsumerRoutes registra las rutas bajo el grupo dado
// (ej. /api/v1/consumers). El grupo ya trae APIKeyAuth y RateLimit aplicados.
func RegisterConsumerRoutes(g *gin.RouterGroup, deps Deps, cfg Config) {
	g.POST("/events", IngestEvents(deps.Ingest, HandlerConfig{
		MaxBodyBytes: cfg.MaxBodyBytes,
		MaxEvents:    cfg.MaxEvents,
	}, deps.Log))

	// Sigue siendo 501: el feature 002 del Edge no lo usa (events.KindHeartbeat
	// esta reservado pero no se emite). Fuera del alcance de este plan.
	g.POST("/heartbeat", Heartbeat)
}
```

> **Ciclo de imports:** `consumers/middleware` importa `consumers` (por `RateLimiter`), así que `consumers` **no puede** importar `consumers/middleware`. El cableado de los middlewares ocurre en `internal/routes/url_mappings.go` (Task 11), no acá. Verificar con `go build ./...` al terminar.

- [ ] **Step 4: Verificar que compila**

Run: `go build ./...`
Expected: falla en `internal/routes/url_mappings.go`, que todavía llama a `consumers.RegisterConsumerRoutes(c1, consumers.Deps{}, consumers.Config{})` y a los middlewares viejos. Se arregla en la Task 11.

- [ ] **Step 5: Commit**

```bash
git add internal/consumers/
git commit -m "feat(ingest): middlewares reales de API key y rate limit para consumers"
```

---

### Task 11: Cableado, health check y arranque

**Files:**
- Modify: `internal/routes/url_mappings.go`
- Modify: `cmd/api/main.go`

**Interfaces:**
- Consumes: todo lo de las Tasks 1-10.
- Produces: `RegisterURLMappings` con Mongo cableado; `GET /health` con el estado de Postgres, Redis y Mongo.

- [ ] **Step 1: Cablear en url_mappings.go**

Agregar los imports:

```go
	ingestapp "github.com/tu-org/embolsadora-api/internal/app/ingest"
	mongoplatform "github.com/tu-org/embolsadora-api/internal/platform/mongo"
	measurementsRepo "github.com/tu-org/embolsadora-api/internal/repo/mongo/measurements"
	apiKeysRepo "github.com/tu-org/embolsadora-api/internal/repo/pg/apikeys"
```

Reemplazar el bloque "Consumer surface" (líneas ~162-171) por:

```go
	// ── Consumer surface (Edge Pi Service) ────────────────────────────────────
	// La ingesta necesita Mongo. Si no hay conexion, la superficie de consumers
	// NO se registra: es preferible que el Edge reciba un 404 —que reintenta—
	// a que reciba un 200 por eventos que nunca se guardaron.
	mongoClient, err := mongoplatform.Connect(context.Background(), cfg.Mongo)
	if err != nil {
		log.Fatalf("no se pudo conectar a MongoDB: %v", err)
	}

	measurementRepo := measurementsRepo.New(mongoClient.Database())
	if err := measurementRepo.EnsureIndexes(context.Background()); err != nil {
		// Sin el indice unico sobre eventId no hay idempotencia, y cada reintento
		// del Pi duplicaria mediciones en silencio. No se arranca sin el.
		log.Fatalf("no se pudieron crear los indices de measurements: %v", err)
	}

	apiKeyRepository := apiKeysRepo.NewRepository(db)
	apiKeyAuth := security.NewAPIKeyAuthenticator(apiKeyRepository, redisClient, cfg.Ingest.APIKeyCacheTTL, logger)
	ingestService := ingestapp.NewService(measurementRepo, logger)
	rateLimiter := consumers.NewRateLimiter(redisClient, cfg.Ingest.RateLimitRPS, cfg.Ingest.RateLimitBurst)

	c1 := r.Group(
		"/api/v1/consumers",
		apimw.RequestID(),
		consumermw.NoCORS(),
		consumermw.APIKeyAuth(apiKeyAuth, logger),
		consumermw.RateLimit(rateLimiter),
	)
	consumers.RegisterConsumerRoutes(c1, consumers.Deps{
		Ingest: ingestService,
		Log:    logger,
	}, consumers.Config{
		MaxBodyBytes: cfg.Ingest.MaxBodyBytes,
		MaxEvents:    cfg.Ingest.MaxEvents,
	})
```

Agregar `"context"` a los imports del archivo.

- [ ] **Step 2: Agregar el health check**

Después del handler de `/ping` (línea ~62), agregar:

```go
	// /ping se deja intacto: devuelve texto plano y es lo que sondean Koyeb y
	// Cloud Run. Cambiarlo a JSON romperia esas probes. El estado detallado va
	// en un endpoint nuevo.
	r.GET("/health", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
		defer cancel()

		checks := gin.H{"postgres": "ok", "mongo": "ok", "redis": "ok"}
		status := http.StatusOK

		if err := db.Ping(ctx); err != nil {
			checks["postgres"] = "error"
			status = http.StatusServiceUnavailable
		}
		if err := measurementRepo.Ping(ctx); err != nil {
			checks["mongo"] = "error"
			status = http.StatusServiceUnavailable
		}
		if redisClient == nil {
			checks["redis"] = "no configurado"
		} else if err := redisClient.Ping(ctx).Err(); err != nil {
			// Redis degradado no tumba el servicio: la cache de keys y el rate
			// limit fallan abiertos a proposito.
			checks["redis"] = "error"
		}

		telemetry.SetMongoUp(checks["mongo"] == "ok")
		c.JSON(status, gin.H{"status": map[bool]string{true: "ok", false: "degraded"}[status == http.StatusOK], "checks": checks})
	})
```

> Este bloque usa `measurementRepo`, que se declara más abajo en la función. Mover la construcción de Mongo (Step 1) **antes** del registro de `/health`, o capturar el repo en una variable declarada arriba. Verificar con `go build ./...`.

- [ ] **Step 3: Verificar el build y los tests**

```bash
go build ./...
go test ./... 2>&1 | grep -v "^ok\|no test files"
```

Expected: build sin salida; ningún FAIL.

- [ ] **Step 4: Levantar el servicio y probar el 403**

```bash
docker compose up -d db redis mongo
go run ./cmd/api &
sleep 3
curl -s -o /dev/null -w '%{http_code}\n' -X POST localhost:8080/api/v1/consumers/events \
  -H 'Content-Type: application/json' -H 'X-Api-Key: emb_000000000000_no-existe' \
  -d '{"events":[]}'
curl -s localhost:8080/health
```

Expected: `403` (la key no existe — se corta antes de mirar el body), y el health con `"mongo":"ok"`.

- [ ] **Step 5: Commit**

```bash
git add internal/routes/url_mappings.go cmd/api/main.go
git commit -m "feat(ingest): cablear ingesta, Mongo y health check con estado de dependencias"
```

---

# FASE 4 — ABM de API keys y observabilidad

---

### Task 12: Endpoints de administración de API keys

Sin esto no hay forma de darle una credencial a un Pi salvo insertando a mano en Postgres.

**Files:**
- Create: `internal/api/handler/edge_devices/dto/api_keys.go`
- Create: `internal/api/handler/edge_devices/api_keys.go`
- Modify: `internal/api/handler/edge_devices/routes.go`
- Modify: `internal/app/edge_devices/service.go`
- Modify: `internal/routes/url_mappings.go`

**Interfaces:**
- Consumes: `apikeys.Repository`, `apikeys.Generate` (Tasks 2-3); `security.InvalidateAPIKeyCache` (Task 4).
- Produces:
  - `edge_devices.Service` gana `CreateAPIKey`, `ListAPIKeys`, `RevokeAPIKey`.
  - Rutas `POST|GET /edge-devices/:deviceId/api-keys` y `DELETE /edge-devices/:deviceId/api-keys/:keyId`.

- [ ] **Step 1: Escribir los DTOs**

Crear `internal/api/handler/edge_devices/dto/api_keys.go`:

```go
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
```

- [ ] **Step 2: Agregar los métodos al service**

Primero, ampliar el struct y el constructor. El actual (`internal/app/edge_devices/service.go:22`) es:

```go
type Service struct {
	repo   edge_devices.Repository
	client edgeclient.EdgeDeviceClient
	logger *zap.Logger
}

func NewService(repo edge_devices.Repository, client edgeclient.EdgeDeviceClient, logger *zap.Logger) *Service {
```

Pasa a ser:

```go
type Service struct {
	repo    edge_devices.Repository
	client  edgeclient.EdgeDeviceClient
	logger  *zap.Logger
	apiKeys domainapikeys.Repository
	redis   *redis.Client
}

func NewService(
	repo edge_devices.Repository,
	client edgeclient.EdgeDeviceClient,
	logger *zap.Logger,
	apiKeys domainapikeys.Repository,
	redisClient *redis.Client,
) *Service {
	return &Service{repo: repo, client: client, logger: logger, apiKeys: apiKeys, redis: redisClient}
}
```

Actualizar la llamada en `internal/routes/url_mappings.go:177`:

```go
	edgeDeviceService := edgeDevicesApp.NewService(edgeDeviceRepository, edgeDeviceClient, logger, apiKeyRepository, redisClient)
```

> `apiKeyRepository` se construye en la Task 11, Step 1. Si esa línea quedó después de esta, moverla arriba.

Imports nuevos en `service.go`: `"github.com/go-redis/redis/v8"`, `domainapikeys "github.com/tu-org/embolsadora-api/internal/domain/apikeys"`, `"github.com/tu-org/embolsadora-api/internal/security"`, `"time"`, `"github.com/google/uuid"`.

Ahora sí, agregar los métodos al final del archivo. **Ojo: el campo del logger se llama `logger`, no `log`.**

```go
// CreateAPIKey genera una credencial nueva para el device y devuelve el valor
// en claro, que el llamador debe mostrar UNA sola vez.
func (s *Service) CreateAPIKey(ctx context.Context, tenantID, deviceID uuid.UUID, name string, expiresAt *time.Time, createdBy *uuid.UUID) (*domainapikeys.APIKey, string, error) {
	// Verificar que el device exista y sea del tenant antes de emitir nada.
	if _, err := s.repo.GetByID(ctx, tenantID, deviceID); err != nil {
		return nil, "", err
	}

	plaintext, keyID, hash, err := domainapikeys.Generate()
	if err != nil {
		return nil, "", err
	}

	key := &domainapikeys.APIKey{
		ID:        uuid.New(),
		TenantID:  tenantID,
		DeviceID:  deviceID,
		KeyID:     keyID,
		KeyHash:   hash,
		CreatedAt: time.Now().UTC(),
		CreatedBy: createdBy,
		ExpiresAt: expiresAt,
	}
	if name != "" {
		key.Name = &name
	}

	if err := s.apiKeys.Create(ctx, key); err != nil {
		return nil, "", err
	}
	s.logger.Info("api key creada",
		zap.String("device_id", deviceID.String()), zap.String("key_id", keyID))
	return key, plaintext, nil
}

// ListAPIKeys devuelve las keys del device, incluidas las revocadas.
func (s *Service) ListAPIKeys(ctx context.Context, tenantID, deviceID uuid.UUID) ([]*domainapikeys.APIKey, error) {
	if _, err := s.repo.GetByID(ctx, tenantID, deviceID); err != nil {
		return nil, err
	}
	return s.apiKeys.ListByDevice(ctx, tenantID, deviceID)
}

// RevokeAPIKey revoca una key e invalida su entrada de cache.
//
// El orden importa: primero Postgres —la fuente de verdad— y despues la cache.
// Al reves, una escritura concurrente podria recachear la version vieja entre
// medio y la key revocada seguiria autenticando hasta que venza el TTL.
func (s *Service) RevokeAPIKey(ctx context.Context, tenantID, deviceID, keyPK uuid.UUID) error {
	keys, err := s.apiKeys.ListByDevice(ctx, tenantID, deviceID)
	if err != nil {
		return err
	}
	var keyID string
	for _, k := range keys {
		if k.ID == keyPK {
			keyID = k.KeyID
			break
		}
	}
	if keyID == "" {
		return domainapikeys.ErrKeyNotFound
	}

	if err := s.apiKeys.Revoke(ctx, tenantID, keyPK); err != nil {
		return err
	}
	if err := security.InvalidateAPIKeyCache(ctx, s.redis, keyID); err != nil {
		// La revocacion ya es efectiva en Postgres; la cache expira sola por TTL.
		s.logger.Warn("no se pudo invalidar la cache de la api key",
			zap.String("key_id", keyID), zap.Error(err))
	}
	s.logger.Info("api key revocada", zap.String("key_id", keyID))
	return nil
}
```

- [ ] **Step 3: Escribir los handlers**

Crear `internal/api/handler/edge_devices/api_keys.go`:

```go
package edge_devices

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/tu-org/embolsadora-api/internal/api/handler/edge_devices/dto"
	appedge "github.com/tu-org/embolsadora-api/internal/app/edge_devices"
	domainapikeys "github.com/tu-org/embolsadora-api/internal/domain/apikeys"
	edgeerrors "github.com/tu-org/embolsadora-api/internal/domain/edge_devices"
	"github.com/tu-org/embolsadora-api/internal/platform"
)

func toAPIKeyResponse(k *domainapikeys.APIKey) dto.APIKeyResponse {
	return dto.APIKeyResponse{
		ID:         k.ID.String(),
		KeyID:      k.KeyID,
		Name:       k.Name,
		CreatedAt:  k.CreatedAt,
		ExpiresAt:  k.ExpiresAt,
		RevokedAt:  k.RevokedAt,
		LastUsedAt: k.LastUsedAt,
		Active:     k.IsActive(timeNow()),
	}
}

// CreateAPIKey genera una API key para el device.
//
// El secreto viaja en esta respuesta y en ninguna otra: no se persiste, asi que
// no hay forma de recuperarlo despues.
func CreateAPIKey(service *appedge.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := platform.TenantUUID(c.Request.Context())
		if tenantID == nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "tenant ID not found"})
			return
		}
		deviceID, err := uuid.Parse(c.Param("deviceId"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "deviceId invalido"})
			return
		}

		var req dto.CreateAPIKeyRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			// El body es opcional: una key sin nombre ni vencimiento es valida.
			req = dto.CreateAPIKeyRequest{}
		}

		key, plaintext, err := service.CreateAPIKey(
			c.Request.Context(), *tenantID, deviceID, req.Name, req.ExpiresAt, platform.UserID(c.Request.Context()))
		if err != nil {
			if errors.Is(err, edgeerrors.ErrDeviceNotFound) {
				c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "device no encontrado"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "no se pudo crear la api key"})
			return
		}

		c.JSON(http.StatusCreated, gin.H{"success": true, "data": dto.CreateAPIKeyResponse{
			APIKeyResponse: toAPIKeyResponse(key),
			Key:            plaintext,
		}})
	}
}

// ListAPIKeys devuelve las keys del device, sin secretos.
func ListAPIKeys(service *appedge.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := platform.TenantUUID(c.Request.Context())
		if tenantID == nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "tenant ID not found"})
			return
		}
		deviceID, err := uuid.Parse(c.Param("deviceId"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "deviceId invalido"})
			return
		}

		keys, err := service.ListAPIKeys(c.Request.Context(), *tenantID, deviceID)
		if err != nil {
			if errors.Is(err, edgeerrors.ErrDeviceNotFound) {
				c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "device no encontrado"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "no se pudieron listar las api keys"})
			return
		}

		out := make([]dto.APIKeyResponse, 0, len(keys))
		for _, k := range keys {
			out = append(out, toAPIKeyResponse(k))
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "data": out})
	}
}

// RevokeAPIKey revoca una key. Es idempotente.
func RevokeAPIKey(service *appedge.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := platform.TenantUUID(c.Request.Context())
		if tenantID == nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "tenant ID not found"})
			return
		}
		deviceID, err := uuid.Parse(c.Param("deviceId"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "deviceId invalido"})
			return
		}
		keyPK, err := uuid.Parse(c.Param("keyId"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "keyId invalido"})
			return
		}

		if err := service.RevokeAPIKey(c.Request.Context(), *tenantID, deviceID, keyPK); err != nil {
			if errors.Is(err, domainapikeys.ErrKeyNotFound) || errors.Is(err, edgeerrors.ErrDeviceNotFound) {
				c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "api key no encontrada"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "no se pudo revocar la api key"})
			return
		}
		c.Status(http.StatusNoContent)
	}
}
```

Agregar el helper `timeNow` en el mismo archivo:

```go
import "time"

func timeNow() time.Time { return time.Now().UTC() }
```

- [ ] **Step 4: Registrar las rutas**

En `internal/api/handler/edge_devices/routes.go`, cambiar la firma para recibir el grupo con RBAC y agregar las rutas:

```go
// RegisterRoutes registra los endpoints de edge devices.
// writeGroup lleva RBACCheck("machines:write"): emitir credenciales que dan
// acceso de escritura a la ingesta no puede ser una operacion de solo-lectura.
func RegisterRoutes(g *gin.RouterGroup, writeGroup *gin.RouterGroup, service *edge_devices.Service) {
	// ... las 10 rutas existentes, sin cambios ...

	// API keys del device
	writeGroup.POST("/edge-devices/:deviceId/api-keys", CreateAPIKey(service))
	g.GET("/edge-devices/:deviceId/api-keys", ListAPIKeys(service))
	writeGroup.DELETE("/edge-devices/:deviceId/api-keys/:keyId", RevokeAPIKey(service))
}
```

Y en `internal/routes/url_mappings.go`, reemplazar la llamada existente:

```go
	edgeDevicesWriteGroup := tenantsGroup.Group("", apimw.RBACCheck("machines:write"))
	edgeDevicesHandler.RegisterRoutes(tenantsGroup, edgeDevicesWriteGroup, edgeDeviceService)
```

- [ ] **Step 5: Verificar el build y probar el ciclo completo**

```bash
go build ./...
go test ./... 2>&1 | grep -v "^ok\|no test files"
```

Expected: build limpio, sin FAIL.

- [ ] **Step 6: Commit**

```bash
git add internal/api/handler/edge_devices/ internal/app/edge_devices/service.go internal/routes/url_mappings.go
git commit -m "feat(ingest): ABM de API keys de edge device (crear, listar, revocar)"
```

---

### Task 13: Métricas Prometheus

**Files:**
- Create: `internal/telemetry/ingest_metrics.go`

**Interfaces:**
- Produces: `telemetry.IngestBatchesTotal` (CounterVec `status`), `IngestEventsAcceptedTotal` (Counter), `IngestEventsRejectedTotal` (CounterVec `code`), `IngestAuthTotal` (CounterVec `result`), `IngestRateLimitedTotal` (Counter), `IngestBatchDuration` (Histogram), `SetMongoUp(bool)`.

> Si la Task 9 ya creó este archivo para desbloquear su build, esta tarea sólo verifica que esté completo y agrega el histograma.

- [ ] **Step 1: Escribir las métricas**

Crear `internal/telemetry/ingest_metrics.go`:

```go
package telemetry

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// IngestBatchesTotal cuenta batches procesados por desenlace (ok/error).
	IngestBatchesTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "ingest_batches_total",
		Help: "Total de batches de eventos recibidos, por desenlace",
	}, []string{"status"})

	// IngestEventsAcceptedTotal cuenta eventos efectivamente persistidos.
	IngestEventsAcceptedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "ingest_events_accepted_total",
		Help: "Total de eventos aceptados y persistidos",
	})

	// IngestEventsRejectedTotal cuenta eventos rechazados por codigo.
	//
	// Es la metrica mas importante del endpoint: un salto de INVALID_SCHEMA o
	// VALIDATION_FAILED significa que el Edge esta mandando eventos a DEAD, y
	// eso es perdida de datos permanente. Merece alerta.
	IngestEventsRejectedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "ingest_events_rejected_total",
		Help: "Total de eventos rechazados, por codigo de error",
	}, []string{"code"})

	// IngestAuthTotal cuenta intentos de autenticacion por API key.
	IngestAuthTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "ingest_auth_total",
		Help: "Intentos de autenticacion por API key, por resultado",
	}, []string{"result"})

	// IngestRateLimitedTotal cuenta requests rechazados con 429.
	IngestRateLimitedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "ingest_rate_limited_total",
		Help: "Total de requests rechazados por rate limit",
	})

	// IngestBatchDuration mide la latencia del insertMany.
	// Los buckets llegan hasta 2s porque SC-008 pide p95 < 500 ms con 1000
	// eventos: sin buckets por encima del objetivo, el p95 no se puede medir.
	IngestBatchDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "ingest_batch_duration_seconds",
		Help:    "Latencia de persistencia de un batch",
		Buckets: []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2},
	})

	// ingestMongoUp refleja el estado de la conexion a Mongo (1 arriba, 0 abajo).
	ingestMongoUp = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "ingest_mongo_up",
		Help: "1 si MongoDB responde al ping, 0 si no",
	})
)

// SetMongoUp registra el estado de Mongo. Lo llama el health check.
func SetMongoUp(up bool) {
	if up {
		ingestMongoUp.Set(1)
		return
	}
	ingestMongoUp.Set(0)
}
```

- [ ] **Step 2: Instrumentar la duración en el service**

En `internal/app/ingest/service.go`, envolver la llamada a `InsertMany`:

```go
		start := time.Now()
		report, err := s.repo.InsertMany(ctx, valid)
		telemetry.IngestBatchDuration.Observe(time.Since(start).Seconds())
```

Agregar el import de `telemetry`.

- [ ] **Step 3: Verificar que las métricas se exponen**

```bash
go build ./...
go run ./cmd/api &
sleep 3
curl -s localhost:8080/metrics | grep ingest_
```

Expected: aparecen `ingest_batches_total`, `ingest_events_accepted_total`, `ingest_events_rejected_total`, `ingest_auth_total`, `ingest_rate_limited_total`, `ingest_batch_duration_seconds`, `ingest_mongo_up`.

- [ ] **Step 4: Commit**

```bash
git add internal/telemetry/ingest_metrics.go internal/app/ingest/service.go
git commit -m "feat(ingest): metricas Prometheus de la ingesta"
```

---

# FASE 5 — Fixture y criterios de aceptación

---

### Task 14: Fixture de 108 eventos y tests de contrato

**Files:**
- Create: `scripts/genfixture/main.go`
- Create: `internal/consumers/testdata/last-batch.json`
- Test: `internal/consumers/events_handler_test.go`

**Interfaces:**
- Consumes: todo lo anterior.
- Produces: fixture versionado + tests que cubren **SC-001, SC-002, SC-003**.

> El diseño (§11) asume un batch real capturado del Pi. Ese archivo **no existe** en `embolsadora-edge`. Este generador produce uno equivalente en forma: mismo `machineId`, grupos de eventos que comparten `ts` y se distinguen por `seq`, `aasPath` reales del catálogo, y `eventId` = `sha256(machineId|aasPath|ts_nano)` como hace el Edge (`internal/app/forwarder/eventid.go`). Cuando haya un batch real, se reemplaza el archivo y los tests siguen valiendo.

- [ ] **Step 1: Escribir el generador**

Crear `scripts/genfixture/main.go`:

```go
// Command genfixture emite un batch de 108 eventos con la misma forma que los
// que manda el Pi, para usar como fixture de los tests de contrato.
//
// Es determinista: correrlo dos veces produce byte por byte el mismo archivo,
// asi que el fixture se puede versionar y los diffs son significativos.
//
//	go run ./scripts/genfixture > internal/consumers/testdata/last-batch.json
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"time"
)

const machineID = "EMB-DEV-001"

type event struct {
	EventID       string         `json:"eventId"`
	MachineID     string         `json:"machineId"`
	Ts            string         `json:"ts"`
	Seq           *int64         `json:"seq,omitempty"`
	Kind          string         `json:"kind"`
	SchemaVersion int            `json:"schemaVersion"`
	Payload       map[string]any `json:"payload"`
}

// computeEventID replica embolsadora-edge/internal/app/forwarder/eventid.go:
// sha256(machineId|aasPath|ts_nano).
func computeEventID(machineID, aasPath string, ts time.Time) string {
	h := sha256.New()
	h.Write([]byte(machineID))
	h.Write([]byte("|"))
	h.Write([]byte(aasPath))
	h.Write([]byte("|"))
	h.Write([]byte(fmt.Sprintf("%d", ts.UnixNano())))
	return hex.EncodeToString(h.Sum(nil))
}

func main() {
	props := []struct {
		path, unit, valueType string
		value                 any
	}{
		{"Operativos/Pesada/peso", "kg", "xs:float", 1.0},
		{"Operativos/Pesada/contador", "count", "xs:int", 100.0},
		{"Operativos/Sellado/temperatura", "degC", "xs:float", 82.5},
		{"Alarmas/estado", "bool", "xs:boolean", false},
	}

	base := time.Date(2026, 7, 31, 1, 6, 37, 147166000, time.UTC)
	events := make([]event, 0, 108)

	// 27 instantes x 4 propiedades = 108 eventos. Los 4 de un mismo instante
	// comparten ts y se distinguen por seq: es exactamente lo que hace el
	// historian, que escribe varias propiedades en el mismo tick.
	for tick := 0; tick < 27; tick++ {
		ts := base.Add(time.Duration(tick) * time.Second)
		for i, p := range props {
			seq := int64(i)
			events = append(events, event{
				EventID:       computeEventID(machineID, p.path, ts),
				MachineID:     machineID,
				Ts:            ts.Format(time.RFC3339Nano),
				Seq:           &seq,
				Kind:          "metric",
				SchemaVersion: 1,
				Payload: map[string]any{
					"aasPath":   p.path,
					"value":     p.value,
					"unit":      p.unit,
					"valueType": p.valueType,
				},
			})
		}
	}

	out, err := json.MarshalIndent(map[string]any{"events": events}, "", "  ")
	if err != nil {
		panic(err)
	}
	os.Stdout.Write(append(out, '\n'))
}
```

- [ ] **Step 2: Generar el fixture y verificarlo**

```bash
mkdir -p internal/consumers/testdata
go run ./scripts/genfixture > internal/consumers/testdata/last-batch.json
grep -c '"eventId"' internal/consumers/testdata/last-batch.json
```

Expected: `108`.

Verificar que los `eventId` son únicos:

```bash
grep -o '"eventId": "[a-f0-9]*"' internal/consumers/testdata/last-batch.json | sort -u | wc -l
```

Expected: `108`.

- [ ] **Step 3: Escribir el test de contrato — debe fallar**

Crear `internal/consumers/events_handler_test.go`:

```go
package consumers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	ingestapp "github.com/tu-org/embolsadora-api/internal/app/ingest"
	"github.com/tu-org/embolsadora-api/internal/consumers"
	domainapikeys "github.com/tu-org/embolsadora-api/internal/domain/apikeys"
	"github.com/tu-org/embolsadora-api/internal/domain/ingest"
	"github.com/tu-org/embolsadora-api/internal/security"
)

// memRepo es un repositorio en memoria con la MISMA semantica de idempotencia
// que Mongo: un eventId repetido se reporta como duplicado y no se guarda dos
// veces. Permite testear el contrato sin infraestructura.
type memRepo struct {
	stored map[string]ingest.Measurement
}

func newMemRepo() *memRepo { return &memRepo{stored: map[string]ingest.Measurement{}} }

func (m *memRepo) InsertMany(_ context.Context, docs []ingest.Measurement) (ingest.InsertReport, error) {
	rep := ingest.InsertReport{Duplicated: map[int]struct{}{}, Failed: map[int]string{}}
	for i, d := range docs {
		if _, dup := m.stored[d.EventID]; dup {
			rep.Duplicated[i] = struct{}{}
			continue
		}
		m.stored[d.EventID] = d
	}
	return rep, nil
}
func (m *memRepo) Ping(context.Context) error { return nil }

// newRouter monta el handler con una identidad ya inyectada, salteando
// APIKeyAuth: lo que se testea aca es el contrato del body, no la auth.
func newRouter(repo ingest.Repository) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	identity := &domainapikeys.DeviceIdentity{
		TenantID:  uuid.New(),
		DeviceID:  uuid.New(),
		MachineID: "EMB-DEV-001",
		KeyID:     "0123456789ab",
	}
	r.POST("/events", func(c *gin.Context) {
		c.Request = c.Request.WithContext(security.WithDeviceIdentity(c.Request.Context(), identity))
		c.Next()
	}, consumers.IngestEvents(
		ingestapp.NewService(repo, zap.NewNop()),
		consumers.HandlerConfig{MaxBodyBytes: 4194304, MaxEvents: 1000},
		zap.NewNop(),
	))
	return r
}

func post(t *testing.T, r *gin.Engine, body []byte) (int, ingest.Result) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/events", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		return w.Code, ingest.Result{}
	}
	var resp struct {
		Data ingest.Result `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	return w.Code, resp.Data
}

func fixture(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/last-batch.json")
	require.NoError(t, err, "correr: go run ./scripts/genfixture > internal/consumers/testdata/last-batch.json")
	return b
}

// SC-001: el batch de 108 eventos entra completo.
func TestSC001CleanBatch(t *testing.T) {
	repo := newMemRepo()
	code, res := post(t, newRouter(repo), fixture(t))

	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, 108, res.Accepted)
	assert.Equal(t, 0, res.Rejected)
	assert.Empty(t, res.Errors)
	assert.Len(t, repo.stored, 108, "deben quedar exactamente 108 documentos")
}

// SC-002: reenviar el MISMO batch no duplica nada. Es la prueba de que la
// idempotencia funciona — el escenario real de un reintento del Pi.
func TestSC002ReplayIsIdempotent(t *testing.T) {
	repo := newMemRepo()
	r := newRouter(repo)
	body := fixture(t)

	_, first := post(t, r, body)
	require.Equal(t, 108, first.Accepted)

	code, second := post(t, r, body)
	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, 0, second.Accepted)
	assert.Equal(t, 108, second.Rejected)
	require.Len(t, second.Errors, 108)
	for _, e := range second.Errors {
		assert.Equal(t, ingest.CodeDuplicate, e.Code)
	}
	assert.Len(t, repo.stored, 108, "sigue habiendo 108 documentos, no 216")
}

// SC-003: [valido, valido, sin ts, valido, ya existente] -> errores en 2 y 4.
// Es el test del invariante I-3, con los indices del array ORIGINAL.
func TestSC003MixedBatchReportsOriginalIndices(t *testing.T) {
	repo := newMemRepo()
	r := newRouter(repo)

	ev := func(id string) string {
		return `{"eventId":"` + id + `","machineId":"EMB-DEV-001","ts":"2026-07-31T01:06:37Z",` +
			`"kind":"metric","schemaVersion":1,"payload":{"value":1}}`
	}
	// Preexistente, para que el quinto elemento sea un duplicado real.
	_, pre := post(t, r, []byte(`{"events":[`+ev("ya-existente")+`]}`))
	require.Equal(t, 1, pre.Accepted)

	sinTs := `{"eventId":"c","machineId":"EMB-DEV-001","kind":"metric","schemaVersion":1,"payload":{}}`
	body := `{"events":[` + ev("a") + `,` + ev("b") + `,` + sinTs + `,` + ev("d") + `,` + ev("ya-existente") + `]}`

	code, res := post(t, r, []byte(body))
	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, 3, res.Accepted)
	assert.Equal(t, 2, res.Rejected)

	require.Len(t, res.Errors, 2)
	assert.Equal(t, 2, res.Errors[0].Index)
	assert.Equal(t, ingest.CodeInvalidSchema, res.Errors[0].Code)
	assert.Equal(t, 4, res.Errors[1].Index)
	assert.Equal(t, ingest.CodeDuplicate, res.Errors[1].Code)
}

// I-2: los requests rotos a nivel de SOBRE son 400. Estos son los unicos casos
// en que el Edge manda el batch entero a DEAD, asi que la lista es cerrada.
func TestEnvelopeErrorsReturn400(t *testing.T) {
	r := newRouter(newMemRepo())
	cases := map[string]string{
		"json malformado": `{"events":`,
		"sin events":      `{}`,
		"events vacio":    `{"events":[]}`,
		"events null":     `{"events":null}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			code, _ := post(t, r, []byte(body))
			assert.Equal(t, http.StatusBadRequest, code)
		})
	}
}

// Un batch de mas de 1000 eventos es 400: es el limite de negocio del contrato.
func TestTooManyEventsReturns400(t *testing.T) {
	var buf bytes.Buffer
	buf.WriteString(`{"events":[`)
	for i := 0; i < 1001; i++ {
		if i > 0 {
			buf.WriteByte(',')
		}
		fmt.Fprintf(&buf, `{"eventId":"e%d","machineId":"EMB-DEV-001","ts":"2026-07-31T01:06:37Z",`+
			`"kind":"metric","schemaVersion":1,"payload":{}}`, i)
	}
	buf.WriteString(`]}`)

	code, _ := post(t, newRouter(newMemRepo()), buf.Bytes())
	assert.Equal(t, http.StatusBadRequest, code)
}

// I-4 sobre el fixture real.
func TestAccountingInvariantOnFixture(t *testing.T) {
	_, res := post(t, newRouter(newMemRepo()), fixture(t))
	assert.Equal(t, 108, res.Accepted+res.Rejected)
}

// La respuesta NO lleva el envelope {"success":true,...} del resto del repo.
// El parser del Edge esta congelado contra {"data":{...}}.
func TestResponseShapeMatchesFrozenContract(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/events", bytes.NewReader(fixture(t)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	newRouter(newMemRepo()).ServeHTTP(w, req)

	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &raw))
	assert.Contains(t, raw, "data")
	assert.NotContains(t, raw, "success", "este endpoint no usa el envelope success del repo")

	var data map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw["data"], &data))
	assert.Contains(t, data, "accepted")
	assert.Contains(t, data, "rejected")
}
```

Agregar `"fmt"` a los imports.

- [ ] **Step 4: Correr los tests**

Run: `go test ./internal/consumers/... -v`
Expected: PASS en los 8 tests. `TestSC002ReplayIsIdempotent` es el determinante.

- [ ] **Step 5: Commit**

```bash
git add scripts/genfixture/ internal/consumers/testdata/ internal/consumers/events_handler_test.go
git commit -m "test(ingest): fixture de 108 eventos y tests de contrato SC-001..SC-003"
```

---

### Task 15: Tests de integración de los criterios restantes

**Files:**
- Test: `internal/consumers/ingest_integration_test.go`

**Interfaces:**
- Consumes: todo. Cubre **SC-004, SC-005, SC-007, SC-008, SC-009**.

> SC-006 (429 con `Retry-After`) queda cubierto por un test del `RateLimiter` con Redis real. SC-004 exige detener Mongo a mano: se documenta como paso manual verificable y el test automatiza la mitad que puede automatizarse (un repo que falla).

- [ ] **Step 1: Escribir los tests**

Crear `internal/consumers/ingest_integration_test.go`:

```go
package consumers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
	mongodriver "go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	ingestapp "github.com/tu-org/embolsadora-api/internal/app/ingest"
	"github.com/tu-org/embolsadora-api/internal/consumers"
	"github.com/tu-org/embolsadora-api/internal/domain/ingest"
	"github.com/tu-org/embolsadora-api/internal/repo/mongo/measurements"
)

// deadRepo simula Mongo caido: TODA operacion falla.
type deadRepo struct{}

func (deadRepo) InsertMany(context.Context, []ingest.Measurement) (ingest.InsertReport, error) {
	return ingest.InsertReport{}, errors.New("mongo inalcanzable")
}
func (deadRepo) Ping(context.Context) error { return errors.New("mongo inalcanzable") }

// SC-004 — el criterio mas importante del plan.
//
// Con el storage caido, la respuesta NUNCA puede contener INVALID_SCHEMA,
// VALIDATION_FAILED ni un 400: cualquiera de los tres haria que el Edge marque
// los eventos como DEAD y no los reintente jamas. Una caida de diez minutos se
// convertiria en perdida permanente.
func TestSC004StorageDownNeverReportsPayloadErrors(t *testing.T) {
	r := newRouter(deadRepo{})
	body := fixture(t)

	code, _ := post(t, r, body)
	assert.Equal(t, http.StatusInternalServerError, code,
		"storage caido debe ser 500 —que el Edge reintenta—, nunca 400")

	// Y la respuesta no debe mencionar ningun codigo terminal.
	raw := postRaw(t, r, body)
	for _, terminal := range []string{ingest.CodeInvalidSchema, ingest.CodeValidationFailed} {
		assert.NotContains(t, string(raw), terminal,
			"un fallo de infraestructura no puede reportarse como error de payload (I-1)")
	}
}

// SC-004, segunda mitad: al volver el storage, el reintento persiste todo sin
// duplicar. Se simula reusando el mismo repo en memoria.
func TestSC004RetryAfterRecoveryPersistsWithoutDuplicates(t *testing.T) {
	repo := newMemRepo()
	body := fixture(t)

	// Primer intento con storage caido.
	code, _ := post(t, newRouter(deadRepo{}), body)
	require.Equal(t, http.StatusInternalServerError, code)

	// El Edge reintenta contra el storage ya recuperado.
	code, res := post(t, newRouter(repo), body)
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, 108, res.Accepted)
	assert.Len(t, repo.stored, 108)

	// Y un tercer reintento tampoco duplica.
	_, third := post(t, newRouter(repo), body)
	assert.Equal(t, 108, third.Rejected)
	assert.Len(t, repo.stored, 108)
}

// SC-007: un batch cuyo machineId no es el del device escribe CERO documentos.
func TestSC007ForeignMachineIDWritesNothing(t *testing.T) {
	repo := newMemRepo()

	body := `{"events":[{"eventId":"x1","machineId":"EMB-OTRA-999","ts":"2026-07-31T01:06:37Z",` +
		`"kind":"metric","schemaVersion":1,"payload":{"value":1}}]}`

	code, res := post(t, newRouter(repo), []byte(body))
	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, 0, res.Accepted)
	assert.Equal(t, 1, res.Rejected)
	require.Len(t, res.Errors, 1)
	assert.Equal(t, ingest.CodeValidationFailed, res.Errors[0].Code)
	assert.Empty(t, repo.stored, "no se escribio ningun documento")
}

// SC-008: 1000 eventos por debajo de 500 ms.
func TestSC008ThousandEventsLatency(t *testing.T) {
	if os.Getenv("MONGO_URI") == "" {
		t.Skip("MONGO_URI no seteada; se omite el benchmark")
	}
	repo, _ := newMongoRepo(t)
	r := newRouter(repo)

	body := makeBatch(t, 1000)
	start := time.Now()
	code, res := post(t, r, body)
	elapsed := time.Since(start)

	require.Equal(t, http.StatusOK, code)
	require.Equal(t, 1000, res.Accepted)
	assert.Less(t, elapsed, 500*time.Millisecond, "SC-008: 1000 eventos en menos de 500 ms")
	t.Logf("1000 eventos en %v", elapsed)
}

// SC-009: la consulta "ultimo valor de una propiedad" resuelve por IXSCAN.
// Sin este test es facil terminar con cientos de GB que hay que escanear
// enteros para dibujar un grafico.
func TestSC009LatestValueQueryUsesIndex(t *testing.T) {
	repo, db := newMongoRepo(t)
	ctx := context.Background()

	require.NoError(t, seedFixture(t, repo))

	coll := db.Collection(measurements.CollectionName)
	cmd := bson.D{
		{Key: "explain", Value: bson.D{
			{Key: "find", Value: measurements.CollectionName},
			{Key: "filter", Value: bson.D{
				{Key: "tenantId", Value: "t1"},
				{Key: "machineId", Value: "EMB-DEV-001"},
				{Key: "payload.aasPath", Value: "Operativos/Pesada/peso"},
			}},
			{Key: "sort", Value: bson.D{{Key: "ts", Value: -1}}},
			{Key: "limit", Value: 1},
		}},
		{Key: "verbosity", Value: "queryPlanner"},
	}
	var out bson.M
	require.NoError(t, coll.Database().RunCommand(ctx, cmd).Decode(&out))

	plan, err := json.Marshal(out)
	require.NoError(t, err)
	assert.Contains(t, string(plan), "IXSCAN", "la consulta debe resolver por indice")
	assert.NotContains(t, string(plan), "COLLSCAN", "no puede escanear la coleccion entera")
}

// SC-006: superado el limite, 429 con Retry-After.
func TestSC006RateLimitReturns429WithRetryAfter(t *testing.T) {
	url := os.Getenv("REDIS_URL")
	if url == "" {
		t.Skip("REDIS_URL no seteada; se omite el test de rate limit")
	}
	opt, err := redis.ParseURL(url)
	require.NoError(t, err)
	rdb := redis.NewClient(opt)
	t.Cleanup(func() { _ = rdb.Close() })

	// rps y burst chicos para agotar el bucket sin mandar 200 requests.
	limiter := consumers.NewRateLimiter(rdb, 1, 3)
	key := "test-" + time.Now().Format("150405.000")
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		allowed, _, err := limiter.Allow(ctx, key)
		require.NoError(t, err)
		require.True(t, allowed, "los primeros %d deben pasar (burst)", 3)
	}

	allowed, retryAfter, err := limiter.Allow(ctx, key)
	require.NoError(t, err)
	assert.False(t, allowed, "agotado el burst, se rechaza")
	assert.GreaterOrEqual(t, retryAfter, 1, "Retry-After debe ser >= 1 segundo")
}

// El limitador sin Redis deja pasar todo: fail-open deliberado. Un Redis caido
// no puede cortar la ingesta de datos reales.
func TestRateLimiterFailsOpenWithoutRedis(t *testing.T) {
	limiter := consumers.NewRateLimiter(nil, 1, 1)
	for i := 0; i < 10; i++ {
		allowed, _, err := limiter.Allow(context.Background(), "k")
		require.NoError(t, err)
		assert.True(t, allowed)
	}
}
```

Agregar al final del archivo los helpers:

```go
func newMongoRepo(t *testing.T) (*measurements.Repository, *mongodriver.Database) {
	t.Helper()
	uri := os.Getenv("MONGO_URI")
	if uri == "" {
		t.Skip("MONGO_URI no seteada; se omite el test de integracion")
	}
	cli, err := mongodriver.Connect(options.Client().ApplyURI(uri))
	require.NoError(t, err)
	db := cli.Database("embolsadora_ingest_test")
	t.Cleanup(func() {
		_ = db.Drop(context.Background())
		_ = cli.Disconnect(context.Background())
	})
	repo := measurements.New(db)
	require.NoError(t, repo.EnsureIndexes(context.Background()))
	return repo, db
}

// postRaw devuelve el body crudo de la respuesta.
func postRaw(t *testing.T, r *gin.Engine, body []byte) []byte {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/events", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w.Body.Bytes()
}

// makeBatch arma un batch de n eventos validos y distintos.
func makeBatch(t *testing.T, n int) []byte {
	t.Helper()
	var buf bytes.Buffer
	buf.WriteString(`{"events":[`)
	for i := 0; i < n; i++ {
		if i > 0 {
			buf.WriteByte(',')
		}
		fmt.Fprintf(&buf, `{"eventId":"bench-%d-%d","machineId":"EMB-DEV-001",`+
			`"ts":"2026-07-31T01:06:37Z","kind":"metric","schemaVersion":1,`+
			`"payload":{"aasPath":"Operativos/Pesada/peso","value":%d}}`, time.Now().UnixNano(), i, i)
	}
	buf.WriteString(`]}`)
	return buf.Bytes()
}

// seedFixture carga el fixture en el repo dado, con tenantId "t1".
func seedFixture(t *testing.T, repo ingest.Repository) error {
	t.Helper()
	var batch struct {
		Events []json.RawMessage `json:"events"`
	}
	require.NoError(t, json.Unmarshal(fixture(t), &batch))

	docs := make([]ingest.Measurement, 0, len(batch.Events))
	for _, raw := range batch.Events {
		m, evErr := ingestapp.ValidateEvent(raw,
			ingest.DeviceContext{TenantID: "t1", DeviceID: "d1", MachineID: "EMB-DEV-001"},
			time.Now().UTC())
		require.Nil(t, evErr)
		docs = append(docs, m)
	}
	_, err := repo.InsertMany(context.Background(), docs)
	return err
}
```

- [ ] **Step 2: Correr los tests con toda la infraestructura**

```bash
docker compose up -d db redis mongo
export MONGO_URI='mongodb://localhost:27017'
export REDIS_URL='redis://:embolsadora_redis_pass@localhost:6379/0'
export DATABASE_URL='postgres://embolsadora_user:embolsadora_password@localhost:5432/embolsadora_dev?sslmode=disable'
go test ./internal/consumers/... -v
```

Expected: PASS en todos. Si alguno hace skip, falta la variable de entorno correspondiente.

- [ ] **Step 3: Verificar SC-004 y SC-005 a mano contra el servicio real**

Este paso no se puede automatizar del todo: exige detener Mongo de verdad.

```bash
# Crear un tenant, un device y una key por el ABM, y guardar la key en $KEY.
# Despues:
docker compose stop mongo
curl -s -i -X POST localhost:8080/api/v1/consumers/events \
  -H "X-Api-Key: $KEY" -H 'Content-Type: application/json' \
  --data @internal/consumers/testdata/last-batch.json | head -20
```

Expected (**SC-004**): HTTP **500**. El body **no** contiene `INVALID_SCHEMA`, `VALIDATION_FAILED` ni es un 400.

```bash
docker compose start mongo
sleep 5
curl -s -X POST localhost:8080/api/v1/consumers/events \
  -H "X-Api-Key: $KEY" -H 'Content-Type: application/json' \
  --data @internal/consumers/testdata/last-batch.json
docker compose exec mongo mongosh embolsadora --quiet \
  --eval 'db.measurements.countDocuments({})'
```

Expected: `accepted: 108` y exactamente **108** documentos.

```bash
# SC-005: revocar la key y reintentar.
curl -s -X DELETE "localhost:8080/api/v1/tenants/$TENANT/edge-devices/$DEVICE/api-keys/$KEY_PK" \
  -H "Authorization: Bearer $JWT"
sleep 1
curl -s -o /dev/null -w '%{http_code}\n' -X POST localhost:8080/api/v1/consumers/events \
  -H "X-Api-Key: $KEY" -H 'Content-Type: application/json' -d '{"events":[]}'
```

Expected (**SC-005**): **403**. Inmediato, sin esperar el TTL de la caché — la revocación la invalida explícitamente.

- [ ] **Step 4: Correr la suite completa**

```bash
go build ./...
go vet ./...
go test ./... 2>&1 | grep -v "no test files"
```

Expected: sin FAIL.

- [ ] **Step 5: Commit**

```bash
git add internal/consumers/ingest_integration_test.go
git commit -m "test(ingest): criterios de aceptacion SC-004..SC-009"
```

---

## Trazabilidad — cobertura del diseño

| Sección del diseño | Dónde se implementa |
|---|---|
| §2 Alcance — endpoint completo | Tasks 8, 9, 10, 11 |
| §2 Alcance — auth por API key | Tasks 1, 2, 3, 4 |
| §2 Alcance — rate limiting | Task 10 |
| §2 Alcance — persistencia idempotente | Tasks 6, 7 |
| §2 Alcance — migración Postgres | Task 1 |
| §2 Alcance — ABM de keys | Task 12 |
| §2 Alcance — provisioning Mongo | Task 5 |
| §5 Cambio en el Edge | **Ya hecho** — commit `76c3805`, verificado |
| §6.1 Tabla + validación de keys | Tasks 1, 3, 4 |
| §6.2 Colección `measurements` | Task 6 |
| §6.3 Índices | Task 7, Step 3 |
| §6.4 Sin TTL | Task 7 — ningún índice TTL, deliberado |
| §7 Arquitectura por capas | Tasks 4-12, una capa por tarea |
| §8 Flujo del request | Tasks 9, 10 |
| §8 Validación del sobre | Task 8 |
| §9 Manejo de errores | Tasks 8, 9 |
| §9.1 I-1 | Tasks 7, 8, 9, 10 · tests SC-004 |
| §9.1 I-2 | Task 9 · `TestEnvelopeErrorsReturn400` |
| §9.1 I-3 | Task 8 (`origIndex`) · SC-003 |
| §9.1 I-4 | Task 8 · `TestIngestBatchAccountingInvariant` |
| §9.2 Límite de bytes 4 MiB | Task 5 (config), Task 9 (handler) |
| §10 Variables de entorno | Task 5 |
| §10 Observabilidad | Task 13 |
| §10 Health check | Task 11 |
| §11 Testing | Tasks 2, 3, 7, 8, 14, 15 |
| §12 SC-001…SC-009 | Tasks 14, 15 |

**Fuera de alcance, explícitamente** (§2): endpoints de lectura, motor de alarmas, `POST /consumers/heartbeat` (sigue 501), agregaciones, y la caché de idempotencia por `Idempotency-Key` (§13).

## Deudas conocidas que deja este plan

1. **El fixture es sintético, no capturado del Pi.** Tiene la forma correcta y los `eventId` se calculan con el mismo algoritmo que el Edge, pero conviene reemplazarlo por un batch real cuando haya uno. Los tests no cambian.
2. **SC-004 y SC-005 tienen una parte manual** (Task 15, Step 3): detener Mongo de verdad y revocar una key contra el servicio corriendo.
3. **`MONGO_DATABASE` con fallback a `MONGO_DB`** hasta que se reconcilie el PR #30.
4. **No hay caché de negativos** en el autenticador: un barrido con `key_id` inválidos golpea Postgres en cada intento. El rate limit lo acota, pero es por key autenticada — o sea, no aplica a keys inexistentes. Si aparece como problema, la solución es cachear los misses con TTL corto.
