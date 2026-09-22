# Cobertura de tests en la capa de providers — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Agregar los tests faltantes en la capa de providers (`internal/platform/*`, `internal/repo/pg/*`, `internal/repo/mongo/*` gaps) identificados en el relevamiento, siguiendo la misma convención ya usada para handlers (#84-#86) y casos de uso (`docs/superpowers/plans/2026-09-20-usecase-test-coverage.md`).

**Architecture:** Cada tarea es un paquete Go independiente (archivo de test propio, sin estado compartido con otras tareas) — se puede ejecutar en paralelo, un subagente por tarea, sin conflictos de merge. A diferencia del plan de casos de uso (que usa fakes manuscritos), los providers SON el límite de infraestructura: no hay nada que fakear. La convención real del repo para `internal/repo/pg/*` (confirmada leyendo `tenants/repository_test.go`, `roles/repository_test.go`, `apikeys/repository_test.go`, `invitations/cloaking_test.go`) es **tests de integración contra Postgres real**, gateados por `if os.Getenv("DATABASE_URL") == "" { t.Skip(...) }`, con seed/cleanup manual vía `t.Cleanup`. Para Mongo, el mismo patrón existe en `internal/repo/mongo/measurements/repository_test.go` gateado por `MONGO_URI`. Dos excepciones son 100% unitarias sin infraestructura: `platform/tenantctx.go` (funciones puras de contexto) y `platform/edgeclient/http_client.go` (HTTP client contra un `httptest.Server` local).

**Tech Stack:** Go 1.24, testify (`require`/`assert`), `pgx/v5/pgxpool`, `google/uuid`, `go.mongodb.org/mongo-driver/v2` (solo Task 3), `net/http/httptest` (solo Task 2).

**Spec:** No hay spec previa — este plan nace del relevamiento hecho en la conversación. No existe un `docs/superpowers/specs/*` asociado.

## Global Constraints

- **Nunca usar `uber/mock` / `mockgen`.** Estos son tests de integración: no hay interfaz que mockear, se ejercita el `*PostgresRepository`/`*Client` real contra la base real.
- **`testify`** (`require` para asserts que abortan el test, `assert` para el resto) — sin excepciones.
- **Gateo por variable de entorno, siempre `t.Skip`, nunca `t.Fatal`.** Todo archivo de test de integración empieza con un helper `newPool(t)` / `newDB(t)` que hace skip si `DATABASE_URL`/`MONGO_URI` no está seteada — igual que `internal/repo/pg/apikeys/repository_test.go:20-30`. Esto es lo que hace que `go test ./...` sin infraestructura levantada siga pasando en CI.
- **Cleanup con `t.Cleanup`, nunca `defer` suelto antes de un `t.Cleanup` que dependa del recurso.** Ver el comentario en `internal/repo/pg/invitations/cloaking_test.go:27-30`: el pool se cierra con `t.Cleanup(pool.Close)` registrado ANTES que cualquier cleanup de filas, así el orden LIFO cierra el pool al final.
- **IDs de test únicos vía `uuid.NewString()[:8]`** en subdominios/nombres para evitar choques con constraints UNIQUE si los tests corren en paralelo contra la misma DB compartida.
- **Seed mínimo real, no todos los repos necesitan tenant.** Confirmado por grep de `FOREIGN KEY` en `migrations/000001_initial_schema.up.sql`: `alarm_rules`, `dashboard_layouts` (+ `users`), `edge_devices` y `permissions` (solo si `tenant_id` no es NULL) tienen FK real a `tenants`/`users` — hay que sembrar esas filas. `log_entries`, `log_retention_policies` y `notifications` NO tienen FK en `tenant_id` — un `uuid.New()` suelto alcanza, no hace falta sembrar tenant.
- **Valores válidos de CHECK constraints** (la DB los rechaza si no matchean, no es opcional):
  - `alarm_rules`: `operator` ∈ {gt,lt,gte,lte,eq}, `severity` ∈ {info,warning,critical}.
  - `edge_devices`: `edge_type` = `RASPBERRY_PLC`, `status` ∈ {ACTIVE,DISABLED}, `last_health_status` ∈ {OK,DEGRADED,ERROR,UNKNOWN}.
  - `device_events`: `check_type` ∈ {STATUS,HEALTH_CHECK}, `overall_status` ∈ {OK,DEGRADED,ERROR,UNKNOWN}.
  - `notifications`: `severity` ∈ {info,warning,critical,error}, `status` ∈ {unread,acknowledged,closed}.
  - `log_entries`: `severity` ∈ {info,warning,critical,error}, `event_type` ∈ {alarm_triggered,alarm_resolved,device_connected,device_disconnected,device_state_changed,user_action,system}.
  - `permissions`: `is_system_permission=false` exige `tenant_id` no NULL (`chk_custom_perm_has_tenant`); `is_system_permission=true` exige `tenant_id` NULL (`chk_system_perm_no_tenant`); `name` ≥ 3 chars, `section`/`description` no vacíos.
- **No es TDD clásico.** Backfill de cobertura sobre código que ya está en producción — si un test da RED, es señal de un bug real, no de que falte implementar algo.
- **Comando de verificación por tarea:** el `go test` exacto de cada tarea. Verificación global al final: `go build ./...` y `go test ./...` (sin `DATABASE_URL`/`MONGO_URI` para confirmar que todo sigue skippeando limpio, y opcionalmente con ellas si hay Postgres/Mongo local levantado vía `docker compose up -d db mongo`).
- **Fuera de alcance (decidido con el usuario):** `internal/repo/pg/{events_repo,machines_repo,tenants_repo,users_repo}.go`, `internal/repo/redis/{idem_store,ratelimit_tokenbucket}.go`, `internal/platform/{backoff,clock,uuid}.go` son scaffolding sin ninguna referencia real (confirmado por grep) — no se tocan en este plan. `internal/platform/logwriter/writer.go` y `internal/platform/edgeclient/client.go` son solo interfaces (sin lógica propia que testear; sus implementaciones sí se cubren: `edgeclient.HTTPClient` en Task 2, y `logwriter.LogWriter` ya está cubierto por `internal/app/logs/service_test.go`). `internal/repo/pg/{tenants/resources.go,user_roles/resources.go}` son constantes SQL sin lógica propia — se ejercitan indirectamente vía los tests de sus repositorios hermanos.

## File Structure

| # | Paquete/archivo | Acción | Archivo de test |
|---|---|---|---|
| 1 | `internal/platform` (tenantctx.go) | Crear | `tenantctx_test.go` |
| 2 | `internal/platform/edgeclient` (http_client.go) | Crear | `http_client_test.go` |
| 3 | `internal/platform/mongo` (client.go) | Crear | `client_test.go` |
| 4 | `internal/repo/pg/alarm_rules` | Crear | `repository_test.go` |
| 5 | `internal/repo/pg/dashboard_layouts` | Crear | `repository_test.go` |
| 6 | `internal/repo/pg/edge_devices` | Crear | `repository_test.go` |
| 7 | `internal/repo/pg/logs` | Crear | `cursor_filters_test.go` (unit) + `repository_test.go` (integración) |
| 8 | `internal/repo/pg/notifications` | Crear | `repository_test.go` |
| 9 | `internal/repo/pg/permissions` | Crear | `repository_test.go` |
| 10 | `internal/repo/pg/invitations` | Crear | `repository_test.go` (complementa `cloaking_test.go`, no lo toca) |

---

## Task 1: `internal/platform/tenantctx.go` — sin ningún test

**Files:**
- Create: `internal/platform/tenantctx_test.go`

**Interfaces:**
- Consumes: nada (funciones puras de contexto, package `platform`).
- Produces: n/a (archivo de test hoja).

Es lógica pura sin dependencias externas — 100% unitario, sin gateo. `TenantMatches` es la función con más peso real (la usan los handlers de escritura tenant-scoped para comparar el tenant del path contra el del contexto), así que merece varios casos.

- [ ] **Step 1: Crear el archivo con los tests de round-trip simples**

```go
package platform_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tu-org/embolsadora-api/internal/platform"
)

func TestTenantIDRoundTrip(t *testing.T) {
	ctx := platform.WithTenantID(context.Background(), "tenant-abc")
	assert.Equal(t, "tenant-abc", platform.TenantID(ctx))
}

func TestTenantIDSinSetearDevuelveVacio(t *testing.T) {
	assert.Equal(t, "", platform.TenantID(context.Background()))
}

func TestUserIDRoundTrip(t *testing.T) {
	id := uuid.New()
	ctx := platform.WithUserID(context.Background(), id)
	got := platform.UserID(ctx)
	require.NotNil(t, got)
	assert.Equal(t, id, *got)
}

func TestUserIDSinSetearDevuelveNil(t *testing.T) {
	assert.Nil(t, platform.UserID(context.Background()))
}

func TestSupabaseSubRoundTrip(t *testing.T) {
	ctx := platform.WithSupabaseSub(context.Background(), "sub-123")
	assert.Equal(t, "sub-123", platform.SupabaseSub(ctx))
}

func TestUserEmailRoundTrip(t *testing.T) {
	ctx := platform.WithUserEmail(context.Background(), "a@b.com")
	assert.Equal(t, "a@b.com", platform.UserEmail(ctx))
}

func TestDomainUserRoundTrip(t *testing.T) {
	type fakeDomainUser struct{ Name string }
	ctx := platform.WithDomainUser(context.Background(), &fakeDomainUser{Name: "Ana"})
	got, ok := platform.DomainUser(ctx).(*fakeDomainUser)
	require.True(t, ok)
	assert.Equal(t, "Ana", got.Name)
}

func TestDomainUserSinSetearDevuelveNil(t *testing.T) {
	assert.Nil(t, platform.DomainUser(context.Background()))
}

func TestTenantUUIDRoundTrip(t *testing.T) {
	id := uuid.New()
	ctx := platform.WithTenantUUID(context.Background(), id)
	got := platform.TenantUUID(ctx)
	require.NotNil(t, got)
	assert.Equal(t, id, *got)
}

func TestTenantUUIDSinSetearDevuelveNil(t *testing.T) {
	assert.Nil(t, platform.TenantUUID(context.Background()))
}

func TestAppBaseURLRoundTrip(t *testing.T) {
	ctx := platform.WithAppBaseURL(context.Background(), "https://app.example.com")
	assert.Equal(t, "https://app.example.com", platform.AppBaseURL(ctx))
}

func TestAppBaseURLSinSetearDevuelveVacio(t *testing.T) {
	assert.Equal(t, "", platform.AppBaseURL(context.Background()))
}
```

- [ ] **Step 2: `TenantMatches` — feliz, mismatch, contexto vacío, tenantID inválido**

```go
func TestTenantMatches(t *testing.T) {
	tenantID := uuid.New()

	t.Run("feliz: mismo uuid", func(t *testing.T) {
		ctx := platform.WithTenantID(context.Background(), tenantID.String())
		assert.True(t, platform.TenantMatches(ctx, tenantID))
	})

	t.Run("mismatch: uuid distinto", func(t *testing.T) {
		ctx := platform.WithTenantID(context.Background(), tenantID.String())
		assert.False(t, platform.TenantMatches(ctx, uuid.New()))
	})

	t.Run("contexto sin tenant seteado", func(t *testing.T) {
		assert.False(t, platform.TenantMatches(context.Background(), tenantID))
	})

	t.Run("tenantID en contexto no es un uuid valido", func(t *testing.T) {
		ctx := platform.WithTenantID(context.Background(), "no-es-un-uuid")
		assert.False(t, platform.TenantMatches(ctx, tenantID))
	})

	t.Run("case insensitive: mismo uuid con distinto casing", func(t *testing.T) {
		ctx := platform.WithTenantID(context.Background(), tenantID.String())
		upper, err := uuid.Parse(tenantID.String())
		require.NoError(t, err)
		assert.True(t, platform.TenantMatches(ctx, upper))
	})
}
```

- [ ] **Step 3: Correr los tests del paquete**

Run: `go test ./internal/platform/... -run TestTenant -v`
Expected: PASS.

Run completo del paquete: `go test ./internal/platform/... -v`
Expected: PASS (incluye Task 1 y Task 2, que viven en subpaquetes distintos así que no colisionan).

- [ ] **Step 4: Commit**

```bash
git add internal/platform/tenantctx_test.go
git commit -m "test(platform): cubrir tenantctx.go (paquete no tenía ningún test)"
```

---

## Task 2: `internal/platform/edgeclient/http_client.go` — sin ningún test

**Files:**
- Create: `internal/platform/edgeclient/http_client_test.go`

**Interfaces:**
- Consumes: `net/http/httptest` para simular el dispositivo edge; `edge_devices.CheckResult`/`TelemetrySnapshot` (`internal/domain/edge_devices`).
- Produces: n/a.

100% unitario — el `HTTPClient` es un cliente HTTP genérico, se testea contra un `httptest.Server` local, sin tocar red real ni DB.

- [ ] **Step 1: Crear el archivo con `StatusCheck`/`HealthCheck` (comparten `callEndpoint`)**

```go
package edgeclient_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tu-org/embolsadora-api/internal/platform/edgeclient"
)

func TestStatusCheck(t *testing.T) {
	t.Run("feliz: parsea el CheckResult del device", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/status", r.URL.Path)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"overallStatus": "OK",
				"checkedAt":     time.Now().Format(time.RFC3339),
			})
		}))
		defer srv.Close()

		c := edgeclient.NewHTTPClient(2 * time.Second)
		got, err := c.StatusCheck(t.Context(), srv.URL)
		require.NoError(t, err)
		assert.Equal(t, "OK", got.OverallStatus)
		assert.NotNil(t, got.Details, "Details nunca debe quedar nil aunque el device no lo mande")
	})

	t.Run("status no-2xx sintetiza un CheckResult ERROR en vez de fallar", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
		defer srv.Close()

		c := edgeclient.NewHTTPClient(2 * time.Second)
		got, err := c.StatusCheck(t.Context(), srv.URL)
		require.NoError(t, err, "un 503 del device no debe propagar error, se sintetiza un resultado")
		assert.Equal(t, "ERROR", got.OverallStatus)
		require.NotNil(t, got.Summary)
		assert.Contains(t, *got.Summary, "non-2xx")
	})

	t.Run("body invalido devuelve error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("no es json"))
		}))
		defer srv.Close()

		c := edgeclient.NewHTTPClient(2 * time.Second)
		_, err := c.StatusCheck(t.Context(), srv.URL)
		require.Error(t, err)
	})

	t.Run("timeout del cliente devuelve error de transporte", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(50 * time.Millisecond)
		}))
		defer srv.Close()

		c := edgeclient.NewHTTPClient(5 * time.Millisecond)
		_, err := c.StatusCheck(t.Context(), srv.URL)
		require.Error(t, err)
	})
}

func TestHealthCheck(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/health", r.URL.Path)
		_ = json.NewEncoder(w).Encode(map[string]any{"overallStatus": "DEGRADED"})
	}))
	defer srv.Close()

	c := edgeclient.NewHTTPClient(2 * time.Second)
	got, err := c.HealthCheck(t.Context(), srv.URL)
	require.NoError(t, err)
	assert.Equal(t, "DEGRADED", got.OverallStatus)
}
```

- [ ] **Step 2: `GetTelemetry` — feliz, no-2xx propaga error (a diferencia de los checks, NO sintetiza), `CapturedAt` por defecto**

```go
func TestGetTelemetry(t *testing.T) {
	t.Run("feliz", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/telemetry", r.URL.Path)
			_ = json.NewEncoder(w).Encode(map[string]any{})
		}))
		defer srv.Close()

		c := edgeclient.NewHTTPClient(2 * time.Second)
		got, err := c.GetTelemetry(t.Context(), srv.URL)
		require.NoError(t, err)
		assert.False(t, got.CapturedAt.IsZero(), "sin capturedAt en el JSON, debe rellenarse con now()")
	})

	t.Run("status no-2xx propaga error, no sintetiza (a diferencia de StatusCheck)", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer srv.Close()

		c := edgeclient.NewHTTPClient(2 * time.Second)
		got, err := c.GetTelemetry(t.Context(), srv.URL)
		require.Error(t, err)
		assert.Nil(t, got)
	})
}

func TestNewHTTPClientTimeoutPorDefecto(t *testing.T) {
	// timeout=0 debe caer al default de 10s en vez de quedar sin límite;
	// lo verificamos indirectamente: un server que tarda 20ms debe responder bien.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(20 * time.Millisecond)
		_ = json.NewEncoder(w).Encode(map[string]any{"overallStatus": "OK"})
	}))
	defer srv.Close()

	c := edgeclient.NewHTTPClient(0)
	got, err := c.StatusCheck(t.Context(), srv.URL)
	require.NoError(t, err)
	assert.Equal(t, "OK", got.OverallStatus)
}
```

- [ ] **Step 3: Correr los tests del paquete**

Run: `go test ./internal/platform/edgeclient/... -v`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/platform/edgeclient/http_client_test.go
git commit -m "test(edgeclient): cubrir HTTPClient contra httptest.Server (paquete no tenia ningun test)"
```

---

## Task 3: `internal/platform/mongo/client.go` — sin ningún test

**Files:**
- Create: `internal/platform/mongo/client_test.go`

**Interfaces:**
- Consumes: `config.MongoConfig` (`internal/config`), gateado por `MONGO_URI` igual que `internal/repo/mongo/measurements/repository_test.go`.
- Produces: n/a.

Es el wrapper de conexión real usado en producción (`internal/routes/url_mappings.go:362`) — sin test hoy. Sigue el mismo patrón de skip que el resto de los tests de Mongo del repo.

- [ ] **Step 1: Crear el archivo**

```go
package mongo_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tu-org/embolsadora-api/internal/config"
	mongoplatform "github.com/tu-org/embolsadora-api/internal/platform/mongo"
)

func testConfig(t *testing.T) config.MongoConfig {
	t.Helper()
	uri := os.Getenv("MONGO_URI")
	if uri == "" {
		t.Skip("MONGO_URI no seteada; se omite el test de integracion")
	}
	return config.MongoConfig{
		URI:      uri,
		Database: "embolsadora_test_platform_client",
		Timeout:  5 * time.Second,
	}
}

func TestConnectPingClose(t *testing.T) {
	cfg := testConfig(t)
	client, err := mongoplatform.Connect(context.Background(), cfg)
	require.NoError(t, err)
	require.NotNil(t, client.Database())
	assert.Equal(t, cfg.Database, client.Database().Name())

	require.NoError(t, client.Ping(context.Background()))
	require.NoError(t, client.Close(context.Background()))
}

func TestConnectURIInvalidaDevuelveError(t *testing.T) {
	// No necesita Mongo real levantado: una URI con esquema invalido falla
	// en el propio Connect del driver antes de intentar red.
	_, err := mongoplatform.Connect(context.Background(), config.MongoConfig{
		URI:      "not-a-mongo-uri",
		Database: "x",
		Timeout:  2 * time.Second,
	})
	require.Error(t, err)
}

func TestConnectServidorInalcanzableFallaPorPing(t *testing.T) {
	// URI con esquema valido pero puerto sin nada escuchando: Connect() no
	// falla (conexion perezosa en el driver v2), pero el Ping interno si.
	_, err := mongoplatform.Connect(context.Background(), config.MongoConfig{
		URI:      "mongodb://127.0.0.1:1",
		Database: "x",
		Timeout:  500 * time.Millisecond,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ping fallido")
}
```

Nota: `TestConnectURIInvalidaDevuelveError` y `TestConnectServidorInalcanzableFallaPorPing` NO dependen de `MONGO_URI` (no llaman `testConfig`) — corren siempre, sin skip. Solo `TestConnectPingClose` está gateado.

- [ ] **Step 2: Correr los tests del paquete**

Run: `go test ./internal/platform/mongo/... -v`
Expected: sin `MONGO_URI`, `TestConnectPingClose` hace SKIP y los otros dos PASS. Con `MONGO_URI` seteada (`docker compose up -d mongo`), los tres PASS.

- [ ] **Step 3: Commit**

```bash
git add internal/platform/mongo/client_test.go
git commit -m "test(mongo): cubrir Connect/Ping/Close (paquete no tenia ningun test)"
```

---

## Task 4: `internal/repo/pg/alarm_rules` — sin ningún test

**Files:**
- Create: `internal/repo/pg/alarm_rules/repository_test.go`

**Interfaces:**
- Consumes: `alarm_rules.Repository` (`List`, `GetByID`, `Create`, `Update`, `Delete`), `domain.AlarmRule`, `domain.ErrAlarmRuleNotFound`.
- Produces: n/a.

Requiere un tenant real (`alarm_rules_tenant_id_fkey`). Sigue el patrón exacto de `internal/repo/pg/apikeys/repository_test.go` (seed helper + `t.Cleanup`).

- [ ] **Step 1: Crear el archivo con el seed helper y CRUD feliz**

```go
package alarm_rules_test

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tu-org/embolsadora-api/internal/domain"
	alarmrules "github.com/tu-org/embolsadora-api/internal/repo/pg/alarm_rules"
)

func newPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL no seteada; se omite el test de integracion")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	return pool
}

// seedTenant crea un tenant descartable y devuelve su id.
func seedTenant(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	id := uuid.New()
	_, err := pool.Exec(context.Background(),
		`INSERT INTO tenants (id, name, company_name, subdomain) VALUES ($1, 'alarm-test', 'alarm-test', $2)`,
		id, "alarm-test-"+uuid.NewString()[:8])
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM tenants WHERE id = $1`, id)
	})
	return id
}

func newRule(tenantID uuid.UUID) *domain.AlarmRule {
	return &domain.AlarmRule{
		TenantID:    tenantID,
		Name:        "Temp alta",
		Description: "dispara si supera el umbral",
		Metric:      "temperature",
		Operator:    "gt",
		Threshold:   80,
		Severity:    "critical",
		Enabled:     true,
	}
}

func TestCreateAndGetByID(t *testing.T) {
	pool := newPool(t)
	repo := alarmrules.NewPostgresRepository(pool)
	tenantID := seedTenant(t, pool)

	rule := newRule(tenantID)
	require.NoError(t, repo.Create(context.Background(), rule))
	require.NotEqual(t, uuid.Nil, rule.ID, "Create debe poblar el ID generado por la DB")

	got, err := repo.GetByID(context.Background(), rule.ID, tenantID)
	require.NoError(t, err)
	assert.Equal(t, rule.Name, got.Name)
	assert.Equal(t, rule.Description, got.Description)
	assert.Equal(t, rule.Threshold, got.Threshold)
}

func TestCreateConDescripcionVaciaGuardaNull(t *testing.T) {
	pool := newPool(t)
	repo := alarmrules.NewPostgresRepository(pool)
	tenantID := seedTenant(t, pool)

	rule := newRule(tenantID)
	rule.Description = ""
	require.NoError(t, repo.Create(context.Background(), rule))

	got, err := repo.GetByID(context.Background(), rule.ID, tenantID)
	require.NoError(t, err)
	assert.Equal(t, "", got.Description, "una description NULL en DB debe volver como string vacio")
}

func TestGetByIDNotFound(t *testing.T) {
	pool := newPool(t)
	repo := alarmrules.NewPostgresRepository(pool)
	tenantID := seedTenant(t, pool)

	_, err := repo.GetByID(context.Background(), uuid.New(), tenantID)
	assert.ErrorIs(t, err, domain.ErrAlarmRuleNotFound)
}

func TestGetByIDDeOtroTenantEsNotFound(t *testing.T) {
	pool := newPool(t)
	repo := alarmrules.NewPostgresRepository(pool)
	tenantA := seedTenant(t, pool)
	tenantB := seedTenant(t, pool)

	rule := newRule(tenantA)
	require.NoError(t, repo.Create(context.Background(), rule))

	_, err := repo.GetByID(context.Background(), rule.ID, tenantB)
	assert.ErrorIs(t, err, domain.ErrAlarmRuleNotFound, "una regla de otro tenant no debe ser visible")
}
```

- [ ] **Step 2: `List` — orden y scoping por tenant**

```go
func TestListOrdenaPorCreatedAtYScopeaPorTenant(t *testing.T) {
	pool := newPool(t)
	repo := alarmrules.NewPostgresRepository(pool)
	tenantA := seedTenant(t, pool)
	tenantB := seedTenant(t, pool)

	first := newRule(tenantA)
	first.Name = "primera"
	require.NoError(t, repo.Create(context.Background(), first))

	second := newRule(tenantA)
	second.Name = "segunda"
	require.NoError(t, repo.Create(context.Background(), second))

	other := newRule(tenantB)
	require.NoError(t, repo.Create(context.Background(), other))

	list, err := repo.List(context.Background(), tenantA)
	require.NoError(t, err)
	require.Len(t, list, 2)
	assert.Equal(t, "primera", list[0].Name)
	assert.Equal(t, "segunda", list[1].Name)
}

func TestListSinReglasDevuelveSliceVacioNoNil(t *testing.T) {
	pool := newPool(t)
	repo := alarmrules.NewPostgresRepository(pool)
	tenantID := seedTenant(t, pool)

	list, err := repo.List(context.Background(), tenantID)
	require.NoError(t, err)
	assert.NotNil(t, list)
	assert.Empty(t, list)
}
```

- [ ] **Step 3: `Update` y `Delete` — feliz y not-found**

```go
func TestUpdateHappyPathAndNotFound(t *testing.T) {
	pool := newPool(t)
	repo := alarmrules.NewPostgresRepository(pool)
	tenantID := seedTenant(t, pool)

	rule := newRule(tenantID)
	require.NoError(t, repo.Create(context.Background(), rule))

	rule.Name = "Temp muy alta"
	rule.Threshold = 95
	rule.Enabled = false
	require.NoError(t, repo.Update(context.Background(), rule))

	got, err := repo.GetByID(context.Background(), rule.ID, tenantID)
	require.NoError(t, err)
	assert.Equal(t, "Temp muy alta", got.Name)
	assert.Equal(t, float64(95), got.Threshold)
	assert.False(t, got.Enabled)

	ghost := newRule(tenantID)
	ghost.ID = uuid.New()
	err = repo.Update(context.Background(), ghost)
	assert.ErrorIs(t, err, domain.ErrAlarmRuleNotFound)
}

func TestDeleteHappyPathAndNotFound(t *testing.T) {
	pool := newPool(t)
	repo := alarmrules.NewPostgresRepository(pool)
	tenantID := seedTenant(t, pool)

	rule := newRule(tenantID)
	require.NoError(t, repo.Create(context.Background(), rule))

	require.NoError(t, repo.Delete(context.Background(), rule.ID, tenantID))

	_, err := repo.GetByID(context.Background(), rule.ID, tenantID)
	assert.ErrorIs(t, err, domain.ErrAlarmRuleNotFound)

	err = repo.Delete(context.Background(), rule.ID, tenantID)
	assert.ErrorIs(t, err, domain.ErrAlarmRuleNotFound, "borrar de nuevo debe fallar, no ser idempotente")
}
```

- [ ] **Step 4: Correr los tests del paquete**

Run: `go test ./internal/repo/pg/alarm_rules/... -v`
Expected: sin `DATABASE_URL`, todos SKIP. Con `DATABASE_URL` (`docker compose up -d db`), todos PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/repo/pg/alarm_rules/repository_test.go
git commit -m "test(alarm_rules): cubrir el repository completo (paquete no tenia ningun test)"
```

---

## Task 5: `internal/repo/pg/dashboard_layouts` — sin ningún test

**Files:**
- Create: `internal/repo/pg/dashboard_layouts/repository_test.go`

**Interfaces:**
- Consumes: `*dashboard_layouts.PostgresRepository` (`List`, `GetByID`, `CountByTenantUser`, `Create`, `Update`, `SoftDelete`), `domain.DashboardLayout`/`Widget`/`MaxLayoutsPerUser`/`ErrLayoutNotFound`/`ErrDuplicateName`/`ErrLimitReached`/`ErrCannotDeleteLastLayout` (`internal/domain/dashboard_layouts`).
- Produces: n/a.

Requiere tenant Y user reales (`dashboard_layouts_tenant_id_fkey`, `dashboard_layouts_user_id_fkey`). El repo tiene lógica real no trivial (límite de 3 vía `SELECT ... FOR UPDATE`, no-borrar-el-último) que vale la pena cubrir a fondo.

- [ ] **Step 1: Crear el archivo con seed helpers y CRUD feliz**

```go
package dashboard_layouts_test

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domain "github.com/tu-org/embolsadora-api/internal/domain/dashboard_layouts"
	dashboardlayouts "github.com/tu-org/embolsadora-api/internal/repo/pg/dashboard_layouts"
)

func newPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL no seteada; se omite el test de integracion")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	return pool
}

// seedTenantAndUser crea un tenant y un usuario descartables.
func seedTenantAndUser(t *testing.T, pool *pgxpool.Pool) (tenantID, userID uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	tenantID, userID = uuid.New(), uuid.New()

	_, err := pool.Exec(ctx,
		`INSERT INTO tenants (id, name, company_name, subdomain) VALUES ($1, 'layouts-test', 'layouts-test', $2)`,
		tenantID, "layouts-test-"+uuid.NewString()[:8])
	require.NoError(t, err)

	_, err = pool.Exec(ctx,
		`INSERT INTO users (id, email, name, status) VALUES ($1, $2, 'Layout Tester', 'active')`,
		userID, userID.String()+"@layouts.local")
	require.NoError(t, err)

	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, userID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM tenants WHERE id = $1`, tenantID)
	})
	return tenantID, userID
}

func newLayout(tenantID, userID uuid.UUID, name string) *domain.DashboardLayout {
	return &domain.DashboardLayout{
		ID:       uuid.New(),
		TenantID: tenantID,
		UserID:   userID,
		Name:     name,
		Widgets: []domain.Widget{
			{ID: "w1", Type: "chart", Name: "cpu", Title: "CPU", Category: "system", Icon: "cpu",
				Position: domain.Position{X: 0, Y: 0, W: 2, H: 2}},
		},
	}
}

func TestCreateAndGetByID(t *testing.T) {
	pool := newPool(t)
	repo := dashboardlayouts.NewPostgresRepository(pool)
	tenantID, userID := seedTenantAndUser(t, pool)

	layout := newLayout(tenantID, userID, "Principal")
	require.NoError(t, repo.Create(context.Background(), layout))
	require.False(t, layout.CreatedAt.IsZero())

	got, err := repo.GetByID(context.Background(), tenantID, userID, layout.ID)
	require.NoError(t, err)
	assert.Equal(t, "Principal", got.Name)
	require.Len(t, got.Widgets, 1)
	assert.Equal(t, "cpu", got.Widgets[0].Name)
}

func TestGetByIDNotFound(t *testing.T) {
	pool := newPool(t)
	repo := dashboardlayouts.NewPostgresRepository(pool)
	tenantID, userID := seedTenantAndUser(t, pool)

	_, err := repo.GetByID(context.Background(), tenantID, userID, uuid.New())
	assert.ErrorIs(t, err, domain.ErrLayoutNotFound)
}
```

- [ ] **Step 2: Límite de 3 layouts y nombre duplicado**

```go
func TestCreateRespetaElLimiteDeTresPorUsuario(t *testing.T) {
	pool := newPool(t)
	repo := dashboardlayouts.NewPostgresRepository(pool)
	tenantID, userID := seedTenantAndUser(t, pool)

	for i := 0; i < domain.MaxLayoutsPerUser; i++ {
		l := newLayout(tenantID, userID, "layout-"+uuid.NewString()[:8])
		require.NoError(t, repo.Create(context.Background(), l))
	}

	fourth := newLayout(tenantID, userID, "el-cuarto")
	err := repo.Create(context.Background(), fourth)
	assert.ErrorIs(t, err, domain.ErrLimitReached)

	count, err := repo.CountByTenantUser(context.Background(), tenantID, userID)
	require.NoError(t, err)
	assert.Equal(t, domain.MaxLayoutsPerUser, count)
}

func TestCreateNombreDuplicadoEnElMismoTenantUser(t *testing.T) {
	pool := newPool(t)
	repo := dashboardlayouts.NewPostgresRepository(pool)
	tenantID, userID := seedTenantAndUser(t, pool)

	require.NoError(t, repo.Create(context.Background(), newLayout(tenantID, userID, "Duplicado")))

	err := repo.Create(context.Background(), newLayout(tenantID, userID, "Duplicado"))
	assert.ErrorIs(t, err, domain.ErrDuplicateName)
}

func TestListSoloDevuelveActivosOrdenadosPorCreatedAt(t *testing.T) {
	pool := newPool(t)
	repo := dashboardlayouts.NewPostgresRepository(pool)
	tenantID, userID := seedTenantAndUser(t, pool)

	first := newLayout(tenantID, userID, "primero")
	require.NoError(t, repo.Create(context.Background(), first))
	second := newLayout(tenantID, userID, "segundo")
	require.NoError(t, repo.Create(context.Background(), second))
	require.NoError(t, repo.SoftDelete(context.Background(), tenantID, userID, first.ID))

	list, err := repo.List(context.Background(), tenantID, userID)
	require.NoError(t, err)
	require.Len(t, list, 1, "el soft-deleted no debe listarse")
	assert.Equal(t, "segundo", list[0].Name)
}
```

- [ ] **Step 3: `Update` y `SoftDelete` — feliz, not-found, no-borrar-el-ultimo**

```go
func TestUpdateHappyPathAndNotFound(t *testing.T) {
	pool := newPool(t)
	repo := dashboardlayouts.NewPostgresRepository(pool)
	tenantID, userID := seedTenantAndUser(t, pool)

	layout := newLayout(tenantID, userID, "original")
	require.NoError(t, repo.Create(context.Background(), layout))

	layout.Name = "renombrado"
	layout.Widgets = append(layout.Widgets, domain.Widget{ID: "w2", Type: "gauge", Name: "ram"})
	require.NoError(t, repo.Update(context.Background(), layout))

	got, err := repo.GetByID(context.Background(), tenantID, userID, layout.ID)
	require.NoError(t, err)
	assert.Equal(t, "renombrado", got.Name)
	assert.Len(t, got.Widgets, 2)

	ghost := newLayout(tenantID, userID, "fantasma")
	err = repo.Update(context.Background(), ghost)
	assert.ErrorIs(t, err, domain.ErrLayoutNotFound)
}

func TestSoftDeleteNoPermiteBorrarElUltimo(t *testing.T) {
	pool := newPool(t)
	repo := dashboardlayouts.NewPostgresRepository(pool)
	tenantID, userID := seedTenantAndUser(t, pool)

	only := newLayout(tenantID, userID, "unico")
	require.NoError(t, repo.Create(context.Background(), only))

	err := repo.SoftDelete(context.Background(), tenantID, userID, only.ID)
	assert.ErrorIs(t, err, domain.ErrCannotDeleteLastLayout)
}

func TestSoftDeleteNotFound(t *testing.T) {
	pool := newPool(t)
	repo := dashboardlayouts.NewPostgresRepository(pool)
	tenantID, userID := seedTenantAndUser(t, pool)

	require.NoError(t, repo.Create(context.Background(), newLayout(tenantID, userID, "a")))
	require.NoError(t, repo.Create(context.Background(), newLayout(tenantID, userID, "b")))

	err := repo.SoftDelete(context.Background(), tenantID, userID, uuid.New())
	assert.ErrorIs(t, err, domain.ErrLayoutNotFound)
}
```

- [ ] **Step 4: Correr los tests del paquete**

Run: `go test ./internal/repo/pg/dashboard_layouts/... -v`
Expected: sin `DATABASE_URL`, SKIP. Con ella, PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/repo/pg/dashboard_layouts/repository_test.go
git commit -m "test(dashboard_layouts): cubrir el repository completo, incluido el limite de 3 y no-borrar-el-ultimo"
```

---

## Task 6: `internal/repo/pg/edge_devices` — sin ningún test

**Files:**
- Create: `internal/repo/pg/edge_devices/repository_test.go`

**Interfaces:**
- Consumes: `*edge_devices.PostgresRepository` (`List`, `GetByID`, `Create`, `Update`, `SetStatus`, `UpdateHealthState`, `TouchLastSeen`, `SaveEvent`, `ListEvents`), `edge_devices.EdgeDevice`/`DeviceEvent`/`ErrDeviceNotFound`/`ErrMachineIDConflict` (`internal/domain/edge_devices`).
- Produces: n/a.

Requiere tenant real (`edge_devices_tenant_id_fkey`); `device_events.device_id` tiene FK a `edge_devices` (`ON DELETE CASCADE`), así que basta con sembrar el tenant y crear el device vía el propio repo.

- [ ] **Step 1: Crear el archivo con seed helper y CRUD feliz**

```go
package edge_devices_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domain "github.com/tu-org/embolsadora-api/internal/domain/edge_devices"
	edgedevices "github.com/tu-org/embolsadora-api/internal/repo/pg/edge_devices"
)

func newPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL no seteada; se omite el test de integracion")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	return pool
}

func seedTenant(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	id := uuid.New()
	_, err := pool.Exec(context.Background(),
		`INSERT INTO tenants (id, name, company_name, subdomain) VALUES ($1, 'edge-test', 'edge-test', $2)`,
		id, "edge-test-"+uuid.NewString()[:8])
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM tenants WHERE id = $1`, id)
	})
	return id
}

func newDevice(tenantID uuid.UUID) *domain.EdgeDevice {
	return &domain.EdgeDevice{
		ID:               uuid.New(),
		TenantID:         tenantID,
		Name:             "Edge de test",
		MachineID:        "EMB-TEST-" + uuid.NewString()[:8],
		EdgeType:         "RASPBERRY_PLC",
		RaspberryBaseURL: "http://localhost:9000",
		Status:           "ACTIVE",
	}
}

func TestCreateAndGetByID(t *testing.T) {
	pool := newPool(t)
	repo := edgedevices.NewPostgresRepository(pool)
	tenantID := seedTenant(t, pool)

	device := newDevice(tenantID)
	require.NoError(t, repo.Create(context.Background(), device))
	require.False(t, device.CreatedAt.IsZero())

	got, err := repo.GetByID(context.Background(), tenantID, device.ID)
	require.NoError(t, err)
	assert.Equal(t, device.MachineID, got.MachineID)
	assert.Equal(t, "ACTIVE", got.Status)
	assert.Equal(t, "UNKNOWN", got.LastHealthStatus, "default de la columna, no lo setea el repo")
}

func TestCreateMachineIDDuplicadoEnElMismoTenant(t *testing.T) {
	pool := newPool(t)
	repo := edgedevices.NewPostgresRepository(pool)
	tenantID := seedTenant(t, pool)

	first := newDevice(tenantID)
	require.NoError(t, repo.Create(context.Background(), first))

	dup := newDevice(tenantID)
	dup.MachineID = first.MachineID
	err := repo.Create(context.Background(), dup)
	assert.ErrorIs(t, err, domain.ErrMachineIDConflict)
}

func TestGetByIDNotFound(t *testing.T) {
	pool := newPool(t)
	repo := edgedevices.NewPostgresRepository(pool)
	tenantID := seedTenant(t, pool)

	_, err := repo.GetByID(context.Background(), tenantID, uuid.New())
	assert.ErrorIs(t, err, domain.ErrDeviceNotFound)
}

func TestListScopeaPorTenant(t *testing.T) {
	pool := newPool(t)
	repo := edgedevices.NewPostgresRepository(pool)
	tenantA := seedTenant(t, pool)
	tenantB := seedTenant(t, pool)

	require.NoError(t, repo.Create(context.Background(), newDevice(tenantA)))
	require.NoError(t, repo.Create(context.Background(), newDevice(tenantB)))

	list, err := repo.List(context.Background(), tenantA)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, tenantA, list[0].TenantID)
}
```

- [ ] **Step 2: `Update`, `SetStatus`, `UpdateHealthState`, `TouchLastSeen` — feliz + not-found**

```go
func TestUpdateHappyPathAndNotFound(t *testing.T) {
	pool := newPool(t)
	repo := edgedevices.NewPostgresRepository(pool)
	tenantID := seedTenant(t, pool)

	device := newDevice(tenantID)
	require.NoError(t, repo.Create(context.Background(), device))

	device.Name = "Edge renombrado"
	device.Status = "DISABLED"
	require.NoError(t, repo.Update(context.Background(), device))

	got, err := repo.GetByID(context.Background(), tenantID, device.ID)
	require.NoError(t, err)
	assert.Equal(t, "Edge renombrado", got.Name)
	assert.Equal(t, "DISABLED", got.Status)

	ghost := newDevice(tenantID)
	err = repo.Update(context.Background(), ghost)
	assert.ErrorIs(t, err, domain.ErrDeviceNotFound)
}

func TestSetStatus(t *testing.T) {
	pool := newPool(t)
	repo := edgedevices.NewPostgresRepository(pool)
	tenantID := seedTenant(t, pool)
	device := newDevice(tenantID)
	require.NoError(t, repo.Create(context.Background(), device))

	got, err := repo.SetStatus(context.Background(), tenantID, device.ID, "DISABLED")
	require.NoError(t, err)
	assert.Equal(t, "DISABLED", got.Status)

	_, err = repo.SetStatus(context.Background(), tenantID, uuid.New(), "ACTIVE")
	assert.ErrorIs(t, err, domain.ErrDeviceNotFound)
}

func TestUpdateHealthStateAndTouchLastSeen(t *testing.T) {
	pool := newPool(t)
	repo := edgedevices.NewPostgresRepository(pool)
	tenantID := seedTenant(t, pool)
	device := newDevice(tenantID)
	require.NoError(t, repo.Create(context.Background(), device))

	require.NoError(t, repo.UpdateHealthState(context.Background(), tenantID, device.ID, "OK", "todo bien"))
	got, err := repo.GetByID(context.Background(), tenantID, device.ID)
	require.NoError(t, err)
	assert.Equal(t, "OK", got.LastHealthStatus)
	require.NotNil(t, got.LastHealthSummary)
	assert.Equal(t, "todo bien", *got.LastHealthSummary)
	require.NotNil(t, got.LastHealthCheckAt)

	require.Nil(t, got.LastSeenAt, "todavia no se toco")
	require.NoError(t, repo.TouchLastSeen(context.Background(), tenantID, device.ID))
	got2, err := repo.GetByID(context.Background(), tenantID, device.ID)
	require.NoError(t, err)
	require.NotNil(t, got2.LastSeenAt)

	err = repo.UpdateHealthState(context.Background(), tenantID, uuid.New(), "OK", "x")
	assert.ErrorIs(t, err, domain.ErrDeviceNotFound)
	err = repo.TouchLastSeen(context.Background(), tenantID, uuid.New())
	assert.ErrorIs(t, err, domain.ErrDeviceNotFound)
}
```

- [ ] **Step 3: `SaveEvent` y `ListEvents`**

```go
func TestSaveEventAndListEventsNewestFirst(t *testing.T) {
	pool := newPool(t)
	repo := edgedevices.NewPostgresRepository(pool)
	tenantID := seedTenant(t, pool)
	device := newDevice(tenantID)
	require.NoError(t, repo.Create(context.Background(), device))

	userID := uuid.New()
	now := time.Now().UTC()
	older := &domain.DeviceEvent{
		ID: uuid.New(), DeviceID: device.ID, TenantID: tenantID,
		CheckType: "STATUS", OverallStatus: "OK", CheckedAt: now,
		UserID: userID, UserEmail: "op@test.local",
		Details: map[string]interface{}{},
	}
	require.NoError(t, repo.SaveEvent(context.Background(), older))

	newer := &domain.DeviceEvent{
		ID: uuid.New(), DeviceID: device.ID, TenantID: tenantID,
		CheckType: "HEALTH_CHECK", OverallStatus: "DEGRADED", CheckedAt: now.Add(time.Second),
		UserID: userID, UserEmail: "op@test.local",
		Details: map[string]interface{}{"cpu": 95.5},
	}
	require.NoError(t, repo.SaveEvent(context.Background(), newer))

	events, err := repo.ListEvents(context.Background(), tenantID, device.ID)
	require.NoError(t, err)
	require.Len(t, events, 2)
	assert.Equal(t, "HEALTH_CHECK", events[0].CheckType, "checked_at mas reciente va primero")
	assert.Equal(t, "STATUS", events[1].CheckType)
	assert.Equal(t, 95.5, events[0].Details["cpu"])
}
```

- [ ] **Step 4: Correr los tests del paquete**

Run: `go test ./internal/repo/pg/edge_devices/... -v`
Expected: sin `DATABASE_URL`, SKIP. Con ella, PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/repo/pg/edge_devices/repository_test.go
git commit -m "test(edge_devices): cubrir el repository completo (paquete no tenia ningun test)"
```

---

## Task 7: `internal/repo/pg/logs` — sin ningún test

**Files:**
- Create: `internal/repo/pg/logs/cursor_filters_test.go` (unit, `package logs` — necesita acceso a `decodeCursor`/`buildFilterClauses`, no exportados)
- Create: `internal/repo/pg/logs/repository_test.go` (integración, `package logs_test`)

**Interfaces:**
- Consumes (unit): `encodeCursor`/`decodeCursor`/`buildFilterClauses` (no exportados, package `logs`).
- Consumes (integración): `logs.Repository` vía `logs.New(pool)` (`Write`, `List`, `Get`, `GetContext`, `Export`, `GetRetention`, `UpsertRetention`), `domain.LogEntry`/`RetentionPolicy`/`Severity*`/`EventType*`/`ErrLogNotFound`/`ErrRetentionNotFound`/`ErrInvalidCursor`.
- Produces: n/a.

Precedente de mezclar test whitebox (`package logs`) y test de integración (`package logs_test`) en el mismo directorio: `internal/repo/mongo/metrics/decimate_test.go` (whitebox) + `repository_test.go` (integración), ambos compilando juntos sin problema. `log_entries`/`log_retention_policies` NO tienen FK a `tenants` (confirmado por grep) — no hace falta sembrar tenant, un `uuid.New()` alcanza.

- [ ] **Step 1: Unit tests de `encodeCursor`/`decodeCursor` (round-trip e inválido) y `buildFilterClauses`**

```go
package logs

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tu-org/embolsadora-api/internal/domain"
)

func TestEncodeDecodeCursorRoundTrip(t *testing.T) {
	entry := domain.LogEntry{
		ID:        uuid.New(),
		CreatedAt: time.Now().UTC().Truncate(time.Microsecond),
	}
	cursor := encodeCursor(entry)
	require.NotEmpty(t, cursor)

	decoded, err := decodeCursor(cursor)
	require.NoError(t, err)
	assert.Equal(t, entry.ID, decoded.ID)
	assert.True(t, entry.CreatedAt.Equal(decoded.CreatedAt))
}

func TestDecodeCursorInvalidoDevuelveErrInvalidCursor(t *testing.T) {
	_, err := decodeCursor("no-es-base64-valido-!!!")
	assert.ErrorIs(t, err, domain.ErrInvalidCursor)
}

func TestDecodeCursorBase64ValidoPeroJSONInvalido(t *testing.T) {
	// "aG9sYQ==" decodifica a "hola", que no es JSON valido.
	_, err := decodeCursor("aG9sYQ==")
	assert.ErrorIs(t, err, domain.ErrInvalidCursor)
}

func TestBuildFilterClausesSoloTenantPorDefecto(t *testing.T) {
	tenantID := uuid.New()
	where, args := buildFilterClauses(tenantID, "", "", nil, nil, nil, "")
	require.Len(t, where, 1)
	require.Len(t, args, 1)
	assert.Equal(t, "tenant_id = $1", where[0])
	assert.Equal(t, tenantID, args[0])
}

func TestBuildFilterClausesAcumulaTodosLosFiltrosEnOrden(t *testing.T) {
	tenantID := uuid.New()
	machineID := uuid.New()
	from := time.Now().Add(-time.Hour)
	to := time.Now()

	where, args := buildFilterClauses(tenantID, "system", "critical", &machineID, &from, &to, "falla")

	require.Len(t, where, 7)
	require.Len(t, args, 7)
	assert.Equal(t, "tenant_id = $1", where[0])
	assert.Equal(t, "event_type = $2", where[1])
	assert.Equal(t, "severity = $3", where[2])
	assert.Equal(t, "machine_id = $4", where[3])
	assert.Equal(t, "created_at >= $5", where[4])
	assert.Equal(t, "created_at <= $6", where[5])
	assert.Contains(t, where[6], "plainto_tsquery")
	assert.Equal(t, "falla", args[6])
}
```

- [ ] **Step 2: Correr solo los unit tests**

Run: `go test ./internal/repo/pg/logs/... -run 'TestEncodeDecodeCursor|TestDecodeCursor|TestBuildFilterClauses' -v`
Expected: PASS, sin necesitar `DATABASE_URL`.

- [ ] **Step 3: Crear el archivo de integración con seed helper, `Write`/`Get`/`List`**

```go
package logs_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tu-org/embolsadora-api/internal/domain"
	"github.com/tu-org/embolsadora-api/internal/repo/pg/logs"
)

func newPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL no seteada; se omite el test de integracion")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	return pool
}

// cleanupTenant borra todo lo escrito bajo un tenant de test al terminar
// (log_entries y log_retention_policies no tienen FK a tenants, asi que un
// uuid.New() sin seed alcanza — pero hay que limpiar manualmente).
func cleanupTenant(t *testing.T, pool *pgxpool.Pool, tenantID uuid.UUID) {
	t.Cleanup(func() {
		ctx := context.Background()
		_, _ = pool.Exec(ctx, `DELETE FROM log_entries WHERE tenant_id = $1`, tenantID)
		_, _ = pool.Exec(ctx, `DELETE FROM log_retention_policies WHERE tenant_id = $1`, tenantID)
	})
}

func TestWriteAndGet(t *testing.T) {
	pool := newPool(t)
	repo := logs.New(pool)
	tenantID := uuid.New()
	cleanupTenant(t, pool, tenantID)

	entry := &domain.LogEntry{
		TenantID:  tenantID,
		Severity:  domain.SeverityCritical,
		EventType: domain.EventTypeAlarmTriggered,
		Message:   "temperatura fuera de rango",
		Metadata:  map[string]any{"value": 95.5},
	}
	written, err := repo.Write(context.Background(), entry)
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, written.ID)

	got, err := repo.Get(context.Background(), tenantID, written.ID)
	require.NoError(t, err)
	assert.Equal(t, "temperatura fuera de rango", got.Message)
	assert.Equal(t, domain.SeverityCritical, got.Severity)
	assert.Equal(t, 95.5, got.Metadata["value"])
}

func TestGetNotFound(t *testing.T) {
	pool := newPool(t)
	repo := logs.New(pool)
	_, err := repo.Get(context.Background(), uuid.New(), uuid.New())
	assert.ErrorIs(t, err, domain.ErrLogNotFound)
}

func TestListFiltraPorSeverityYPaginaConCursor(t *testing.T) {
	pool := newPool(t)
	repo := logs.New(pool)
	tenantID := uuid.New()
	cleanupTenant(t, pool, tenantID)
	ctx := context.Background()

	for i, sev := range []domain.Severity{domain.SeverityInfo, domain.SeverityCritical, domain.SeverityCritical} {
		_, err := repo.Write(ctx, &domain.LogEntry{
			TenantID: tenantID, Severity: sev, EventType: domain.EventTypeSystem,
			Message: "entry", Metadata: map[string]any{"i": i},
		})
		require.NoError(t, err)
		time.Sleep(time.Millisecond) // created_at distinto para orden determinista
	}

	page1, total, err := repo.List(ctx, logs.ListParams{TenantID: tenantID, Severity: "critical", Limit: 1})
	require.NoError(t, err)
	assert.Equal(t, 2, total)
	require.Len(t, page1, 1)

	cursor := logs.EncodeCursor(page1[0])
	page2, _, err := repo.List(ctx, logs.ListParams{TenantID: tenantID, Severity: "critical", Limit: 1, Cursor: cursor})
	require.NoError(t, err)
	require.Len(t, page2, 1)
	assert.NotEqual(t, page1[0].ID, page2[0].ID, "la segunda pagina no debe repetir la fila del cursor")
}

func TestListCursorInvalidoDevuelveError(t *testing.T) {
	pool := newPool(t)
	repo := logs.New(pool)
	_, _, err := repo.List(context.Background(), logs.ListParams{TenantID: uuid.New(), Cursor: "invalido!!"})
	assert.ErrorIs(t, err, domain.ErrInvalidCursor)
}
```

- [ ] **Step 4: `GetContext`, `Export` y retención**

```go
func TestGetContextDevuelveVentanaAlrededorDelAncla(t *testing.T) {
	pool := newPool(t)
	repo := logs.New(pool)
	tenantID := uuid.New()
	cleanupTenant(t, pool, tenantID)
	ctx := context.Background()

	var ids []uuid.UUID
	for i := 0; i < 5; i++ {
		e, err := repo.Write(ctx, &domain.LogEntry{
			TenantID: tenantID, Severity: domain.SeverityInfo, EventType: domain.EventTypeSystem,
			Message: "e", Metadata: map[string]any{},
		})
		require.NoError(t, err)
		ids = append(ids, e.ID)
		time.Sleep(time.Millisecond)
	}

	before, anchor, after, err := repo.GetContext(ctx, tenantID, ids[2], 2)
	require.NoError(t, err)
	assert.Equal(t, ids[2], anchor.ID)
	require.Len(t, before, 2)
	require.Len(t, after, 2)
	assert.Equal(t, ids[0], before[0].ID, "before debe quedar en orden cronologico ascendente")
	assert.Equal(t, ids[1], before[1].ID)
	assert.Equal(t, ids[3], after[0].ID)
	assert.Equal(t, ids[4], after[1].ID)
}

func TestExportRespetaMaxRows(t *testing.T) {
	pool := newPool(t)
	repo := logs.New(pool)
	tenantID := uuid.New()
	cleanupTenant(t, pool, tenantID)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		_, err := repo.Write(ctx, &domain.LogEntry{
			TenantID: tenantID, Severity: domain.SeverityInfo, EventType: domain.EventTypeSystem,
			Message: "e", Metadata: map[string]any{},
		})
		require.NoError(t, err)
	}

	rows, total, err := repo.Export(ctx, logs.ExportParams{TenantID: tenantID, MaxRows: 2})
	require.NoError(t, err)
	assert.Equal(t, 3, total, "total cuenta todas las que matchean, sin el limite")
	assert.Len(t, rows, 2)
}

func TestRetentionUpsertAndGet(t *testing.T) {
	pool := newPool(t)
	repo := logs.New(pool)
	tenantID := uuid.New()
	cleanupTenant(t, pool, tenantID)
	ctx := context.Background()

	_, err := repo.GetRetention(ctx, tenantID)
	assert.ErrorIs(t, err, domain.ErrRetentionNotFound)

	created, err := repo.UpsertRetention(ctx, &domain.RetentionPolicy{TenantID: tenantID, RetentionDays: 30})
	require.NoError(t, err)
	assert.Equal(t, 30, created.RetentionDays)

	updated, err := repo.UpsertRetention(ctx, &domain.RetentionPolicy{TenantID: tenantID, RetentionDays: 90})
	require.NoError(t, err)
	assert.Equal(t, 90, updated.RetentionDays)

	got, err := repo.GetRetention(ctx, tenantID)
	require.NoError(t, err)
	assert.Equal(t, 90, got.RetentionDays)
}
```

- [ ] **Step 5: Correr todos los tests del paquete**

Run: `go test ./internal/repo/pg/logs/... -v`
Expected: los unit tests del Step 1 PASS siempre. Sin `DATABASE_URL`, los de integración SKIP; con ella, PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/repo/pg/logs/cursor_filters_test.go internal/repo/pg/logs/repository_test.go
git commit -m "test(logs): cubrir cursor/filtros (unit) y el repository completo (integracion) — paquete no tenia ningun test"
```

---

## Task 8: `internal/repo/pg/notifications` — sin ningún test

**Files:**
- Create: `internal/repo/pg/notifications/repository_test.go`

**Interfaces:**
- Consumes: `*notifications.PostgresRepository` (`List`, `CountUnread`, `GetByID`, `Ack`, `Close`), `domain.Notification`/`Severity*`/`StatusUnread`/`ErrNotificationNotFound`.
- Produces: n/a.

`notifications.tenant_id` NO tiene FK (confirmado por grep) — no hace falta sembrar tenant, un `uuid.New()` alcanza. La idempotencia de `Ack`/`Close` (no pisan un estado ya avanzado) es la lógica más delicada del repo.

- [ ] **Step 1: Crear el archivo con seed helper, `List`, `CountUnread`, `GetByID`**

```go
package notifications_test

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tu-org/embolsadora-api/internal/domain"
	"github.com/tu-org/embolsadora-api/internal/repo/pg/notifications"
)

func newPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL no seteada; se omite el test de integracion")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	return pool
}

func cleanupTenant(t *testing.T, pool *pgxpool.Pool, tenantID uuid.UUID) {
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM notifications WHERE tenant_id = $1`, tenantID)
	})
}

func seedNotification(t *testing.T, pool *pgxpool.Pool, tenantID uuid.UUID, severity, status string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	_, err := pool.Exec(context.Background(),
		`INSERT INTO notifications (id, tenant_id, title, message, severity, status)
		 VALUES ($1, $2, 'Alerta', 'algo paso', $3, $4)`,
		id, tenantID, severity, status)
	require.NoError(t, err)
	return id
}

func TestListFiltraPorStatusYSeverity(t *testing.T) {
	pool := newPool(t)
	repo := notifications.New(pool)
	tenantID := uuid.New()
	cleanupTenant(t, pool, tenantID)

	seedNotification(t, pool, tenantID, "critical", "unread")
	seedNotification(t, pool, tenantID, "info", "unread")
	seedNotification(t, pool, tenantID, "critical", "closed")

	status := "unread"
	severity := "critical"
	list, total, err := repo.List(context.Background(), tenantID, notifications.ListParams{
		Status: &status, Severity: &severity, Limit: 10,
	})
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	require.Len(t, list, 1)
	assert.Equal(t, "critical", string(list[0].Severity))
}

func TestCountUnread(t *testing.T) {
	pool := newPool(t)
	repo := notifications.New(pool)
	tenantID := uuid.New()
	cleanupTenant(t, pool, tenantID)

	seedNotification(t, pool, tenantID, "info", "unread")
	seedNotification(t, pool, tenantID, "info", "unread")
	seedNotification(t, pool, tenantID, "info", "closed")

	count, err := repo.CountUnread(context.Background(), tenantID)
	require.NoError(t, err)
	assert.Equal(t, 2, count)
}

func TestGetByIDNotFound(t *testing.T) {
	pool := newPool(t)
	repo := notifications.New(pool)
	_, err := repo.GetByID(context.Background(), uuid.New(), uuid.New())
	assert.ErrorIs(t, err, domain.ErrNotificationNotFound)
}
```

- [ ] **Step 2: `Ack`/`Close` — feliz e idempotencia (no pisar un estado ya avanzado)**

```go
func TestAckEsIdempotenteYNoPisaAcknowledgedAtExistente(t *testing.T) {
	pool := newPool(t)
	repo := notifications.New(pool)
	tenantID := uuid.New()
	cleanupTenant(t, pool, tenantID)
	id := seedNotification(t, pool, tenantID, "warning", "unread")

	first, err := repo.Ack(context.Background(), id, tenantID)
	require.NoError(t, err)
	assert.Equal(t, string(domain.StatusAcknowledged), string(first.Status))
	require.NotNil(t, first.AcknowledgedAt)
	firstAckAt := *first.AcknowledgedAt

	second, err := repo.Ack(context.Background(), id, tenantID)
	require.NoError(t, err)
	assert.Equal(t, string(domain.StatusAcknowledged), string(second.Status))
	require.NotNil(t, second.AcknowledgedAt)
	assert.True(t, firstAckAt.Equal(*second.AcknowledgedAt), "un segundo Ack no debe mover el timestamp original")
}

func TestCloseDesdeCualquierEstadoYEsIdempotente(t *testing.T) {
	pool := newPool(t)
	repo := notifications.New(pool)
	tenantID := uuid.New()
	cleanupTenant(t, pool, tenantID)
	id := seedNotification(t, pool, tenantID, "critical", "unread")

	closed, err := repo.Close(context.Background(), id, tenantID)
	require.NoError(t, err)
	assert.Equal(t, string(domain.StatusClosed), string(closed.Status))
	require.NotNil(t, closed.ClosedAt)
	firstClosedAt := *closed.ClosedAt

	again, err := repo.Close(context.Background(), id, tenantID)
	require.NoError(t, err)
	assert.True(t, firstClosedAt.Equal(*again.ClosedAt), "cerrar una ya cerrada no debe mover el timestamp")
}

func TestAckDeIDInexistenteEsNotFound(t *testing.T) {
	pool := newPool(t)
	repo := notifications.New(pool)
	_, err := repo.Ack(context.Background(), uuid.New(), uuid.New())
	assert.ErrorIs(t, err, domain.ErrNotificationNotFound)
}
```

- [ ] **Step 3: Correr los tests del paquete**

Run: `go test ./internal/repo/pg/notifications/... -v`
Expected: sin `DATABASE_URL`, SKIP. Con ella, PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/repo/pg/notifications/repository_test.go
git commit -m "test(notifications): cubrir el repository completo, incluida la idempotencia de Ack/Close"
```

---

## Task 9: `internal/repo/pg/permissions` — sin ningún test

**Files:**
- Create: `internal/repo/pg/permissions/repository_test.go`

**Interfaces:**
- Consumes: `*permissions.PostgresRepository` (`List`, `GetByID`, `Create`, `Update`, `Delete`), `domain.Permission`/`ErrPermissionNotFound`/`ErrPermissionIsSystem`.
- Produces: n/a.

`permissions.tenant_id` tiene FK a `tenants` SOLO cuando no es NULL (`chk_custom_perm_has_tenant` obliga a que un permiso custom tenga tenant) — los tests de permisos custom sí siembran tenant; el test de `ErrPermissionIsSystem` sembra un permiso de sistema directo por SQL (tenant_id NULL, sin FK que satisfacer).

- [ ] **Step 1: Crear el archivo con seed helpers y CRUD feliz sobre permisos custom**

```go
package permissions_test

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tu-org/embolsadora-api/internal/domain"
	"github.com/tu-org/embolsadora-api/internal/repo/pg/permissions"
)

func newPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL no seteada; se omite el test de integracion")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	return pool
}

func seedTenant(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	id := uuid.New()
	_, err := pool.Exec(context.Background(),
		`INSERT INTO tenants (id, name, company_name, subdomain) VALUES ($1, 'perms-test', 'perms-test', $2)`,
		id, "perms-test-"+uuid.NewString()[:8])
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM tenants WHERE id = $1`, id)
	})
	return id
}

func newCustomPermission(tenantID uuid.UUID) *domain.Permission {
	id := "perm_custom_" + uuid.NewString()[:8]
	return &domain.Permission{
		ID:          id,
		Name:        "Permiso de prueba",
		Section:     "testing",
		Description: "usado por repository_test.go",
		TenantID:    &tenantID,
	}
}

func TestCreateAndGetByID(t *testing.T) {
	pool := newPool(t)
	repo := permissions.NewPostgresRepository(pool)
	tenantID := seedTenant(t, pool)

	p := newCustomPermission(tenantID)
	require.NoError(t, repo.Create(context.Background(), p))

	got, err := repo.GetByID(context.Background(), p.ID, tenantID)
	require.NoError(t, err)
	assert.Equal(t, p.Name, got.Name)
	assert.False(t, got.IsSystemPermission)
	require.NotNil(t, got.TenantID)
	assert.Equal(t, tenantID, *got.TenantID)
}

func TestGetByIDDeOtroTenantEsNotFound(t *testing.T) {
	pool := newPool(t)
	repo := permissions.NewPostgresRepository(pool)
	tenantA := seedTenant(t, pool)
	tenantB := seedTenant(t, pool)

	p := newCustomPermission(tenantA)
	require.NoError(t, repo.Create(context.Background(), p))

	_, err := repo.GetByID(context.Background(), p.ID, tenantB)
	assert.ErrorIs(t, err, domain.ErrPermissionNotFound)
}

func TestListIncluyeSistemaYCustomDelTenant(t *testing.T) {
	pool := newPool(t)
	repo := permissions.NewPostgresRepository(pool)
	tenantA := seedTenant(t, pool)
	tenantB := seedTenant(t, pool)

	custom := newCustomPermission(tenantA)
	require.NoError(t, repo.Create(context.Background(), custom))
	otherCustom := newCustomPermission(tenantB)
	require.NoError(t, repo.Create(context.Background(), otherCustom))

	list, err := repo.List(context.Background(), tenantA)
	require.NoError(t, err)

	var ids []string
	for _, p := range list {
		ids = append(ids, p.ID)
	}
	assert.Contains(t, ids, custom.ID)
	assert.NotContains(t, ids, otherCustom.ID, "el permiso custom de otro tenant no debe listarse")
}
```

- [ ] **Step 2: `Update` y `Delete` — feliz, not-found, `ErrPermissionIsSystem`**

```go
func TestUpdateHappyPathAndNotFound(t *testing.T) {
	pool := newPool(t)
	repo := permissions.NewPostgresRepository(pool)
	tenantID := seedTenant(t, pool)

	p := newCustomPermission(tenantID)
	require.NoError(t, repo.Create(context.Background(), p))

	p.Name = "Nombre actualizado"
	p.Section = "otra-seccion"
	require.NoError(t, repo.Update(context.Background(), p))

	got, err := repo.GetByID(context.Background(), p.ID, tenantID)
	require.NoError(t, err)
	assert.Equal(t, "Nombre actualizado", got.Name)

	ghost := newCustomPermission(tenantID)
	err = repo.Update(context.Background(), ghost)
	assert.ErrorIs(t, err, domain.ErrPermissionNotFound)
}

func TestDeleteHappyPathAndNotFound(t *testing.T) {
	pool := newPool(t)
	repo := permissions.NewPostgresRepository(pool)
	tenantID := seedTenant(t, pool)

	p := newCustomPermission(tenantID)
	require.NoError(t, repo.Create(context.Background(), p))

	require.NoError(t, repo.Delete(context.Background(), p.ID, tenantID))

	_, err := repo.GetByID(context.Background(), p.ID, tenantID)
	assert.ErrorIs(t, err, domain.ErrPermissionNotFound)

	err = repo.Delete(context.Background(), p.ID, tenantID)
	assert.ErrorIs(t, err, domain.ErrPermissionNotFound)
}

// TestDeleteDePermisoDeSistemaFalla siembra un permiso de sistema directo por
// SQL (is_system_permission=true exige tenant_id NULL — chk_system_perm_no_tenant
// — asi que no pasa por repo.Create, pensado para permisos custom con tenant).
func TestDeleteDePermisoDeSistemaFalla(t *testing.T) {
	pool := newPool(t)
	repo := permissions.NewPostgresRepository(pool)
	tenantID := seedTenant(t, pool)

	sysID := "perm_system_test_" + uuid.NewString()[:8]
	_, err := pool.Exec(context.Background(),
		`INSERT INTO permissions (id, name, section, description, is_system_permission, tenant_id)
		 VALUES ($1, 'Sistema', 'testing', 'permiso de sistema de prueba', TRUE, NULL)`,
		sysID)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM permissions WHERE id = $1`, sysID)
	})

	err = repo.Delete(context.Background(), sysID, tenantID)
	assert.ErrorIs(t, err, domain.ErrPermissionIsSystem)
}
```

- [ ] **Step 3: Correr los tests del paquete**

Run: `go test ./internal/repo/pg/permissions/... -v`
Expected: sin `DATABASE_URL`, SKIP. Con ella, PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/repo/pg/permissions/repository_test.go
git commit -m "test(permissions): cubrir el repository completo, incluido el guard de ErrPermissionIsSystem"
```

---

## Task 10: `internal/repo/pg/invitations` — cobertura parcial (solo cloaking)

**Files:**
- Create: `internal/repo/pg/invitations/repository_test.go` (complementa `cloaking_test.go`, no lo toca — `cloaking_test.go` ya cubre el cloaking de roles globales en `ListByTenant`, `GetByID` y `GetPendingByEmailAndTenant`).

**Interfaces:**
- Consumes: `invitations.InvitationRepository` (`Create`, `ListPendingByEmail`, `UpdateStatus`, y el caso not-found no-cloaking de `GetByID`), `domain.UserInvitation`/`ErrNotFound`/`ErrInvitationAlreadyPending`.
- Produces: n/a.

`cloaking_test.go` no cubre: `Create` (happy path + el mapeo del choque contra `idx_user_invitations_pending` a `ErrInvitationAlreadyPending`), `ListPendingByEmail` (sin cloaking, cross-tenant), `UpdateStatus`, y el caso simple de `GetByID` con un id que directamente no existe (no el caso cloaking).

- [ ] **Step 1: Crear el archivo con seed helpers y `Create`**

```go
package invitations_test

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tu-org/embolsadora-api/internal/domain"
	invRepo "github.com/tu-org/embolsadora-api/internal/repo/pg/invitations"
)

const platformTenant = "11b36b85-033d-4bb3-9e31-4c92161887c0"

func newPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL no seteada; se omite el test de integracion")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	return pool
}

// seedInviter crea un usuario descartable para usar como invited_by (FK real).
func seedInviter(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	id := uuid.New().String()
	_, err := pool.Exec(context.Background(),
		`INSERT INTO users (id, email, name, status) VALUES ($1, $2, 'Inviter', 'active')`,
		id, id+"@inv.local")
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, id)
	})
	return id
}

func TestCreateHappyPath(t *testing.T) {
	pool := newPool(t)
	repo := invRepo.NewInvitationRepository(pool)
	inviterID := seedInviter(t, pool)
	email := uuid.NewString() + "@create.local"

	created, err := repo.Create(context.Background(), &domain.UserInvitation{
		TenantID: platformTenant, Email: email, RoleID: "admin", InvitedBy: inviterID,
	})
	require.NoError(t, err)
	require.NotEmpty(t, created.ID)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM user_invitations WHERE id = $1`, created.ID)
	})

	assert.Equal(t, "pending", string(created.Status))
	assert.Equal(t, email, created.Email)
}

func TestCreateDuplicadoPendingDevuelveErrInvitationAlreadyPending(t *testing.T) {
	pool := newPool(t)
	repo := invRepo.NewInvitationRepository(pool)
	inviterID := seedInviter(t, pool)
	email := uuid.NewString() + "@dup.local"

	first, err := repo.Create(context.Background(), &domain.UserInvitation{
		TenantID: platformTenant, Email: email, RoleID: "admin", InvitedBy: inviterID,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM user_invitations WHERE id = $1`, first.ID)
	})

	_, err = repo.Create(context.Background(), &domain.UserInvitation{
		TenantID: platformTenant, Email: email, RoleID: "operario", InvitedBy: inviterID,
	})
	assert.ErrorIs(t, err, domain.ErrInvitationAlreadyPending)
}
```

- [ ] **Step 2: `ListPendingByEmail`, `UpdateStatus`, `GetByID` not-found simple**

```go
func TestListPendingByEmailCrossTenantSinCloaking(t *testing.T) {
	pool := newPool(t)
	repo := invRepo.NewInvitationRepository(pool)
	inviterID := seedInviter(t, pool)
	email := uuid.NewString() + "@pending.local"

	inv, err := repo.Create(context.Background(), &domain.UserInvitation{
		TenantID: platformTenant, Email: email, RoleID: "super_admin", InvitedBy: inviterID,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM user_invitations WHERE id = $1`, inv.ID)
	})

	list, err := repo.ListPendingByEmail(context.Background(), email)
	require.NoError(t, err)
	require.Len(t, list, 1, "ListPendingByEmail es self-action: debe ver incluso invitaciones a rol global")
	assert.Equal(t, "super_admin", list[0].RoleID)
}

func TestUpdateStatus(t *testing.T) {
	pool := newPool(t)
	repo := invRepo.NewInvitationRepository(pool)
	inviterID := seedInviter(t, pool)
	email := uuid.NewString() + "@status.local"

	inv, err := repo.Create(context.Background(), &domain.UserInvitation{
		TenantID: platformTenant, Email: email, RoleID: "admin", InvitedBy: inviterID,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM user_invitations WHERE id = $1`, inv.ID)
	})

	require.NoError(t, repo.UpdateStatus(context.Background(), inv.ID, domain.InvitationStatusAccepted))

	got, err := repo.GetByID(context.Background(), inv.ID, platformTenant, true)
	require.NoError(t, err)
	assert.Equal(t, domain.InvitationStatusAccepted, got.Status)
}

func TestGetByIDInexistenteEsNotFound(t *testing.T) {
	pool := newPool(t)
	repo := invRepo.NewInvitationRepository(pool)

	_, err := repo.GetByID(context.Background(), uuid.NewString(), platformTenant, true)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}
```

- [ ] **Step 3: Correr los tests del paquete completo (nuevo + `cloaking_test.go` existente)**

Run: `go test ./internal/repo/pg/invitations/... -v`
Expected: sin `DATABASE_URL`, todos SKIP. Con ella, PASS — incluidos los tests ya existentes de `cloaking_test.go`, que no se tocan.

- [ ] **Step 4: Commit**

```bash
git add internal/repo/pg/invitations/repository_test.go
git commit -m "test(invitations): cubrir Create, ListPendingByEmail, UpdateStatus y GetByID not-found simple (complementa cloaking_test.go)"
```

---

## Verificación final (después de todas las tareas)

- [ ] **`go build ./...`** — debe compilar sin errores.
- [ ] **`go test ./...`** sin `DATABASE_URL`/`MONGO_URI` — todo lo nuevo debe hacer SKIP limpio, nada debe FAIL.
- [ ] **`go test ./...`** con `DATABASE_URL`/`MONGO_URI`/`REDIS_URL` seteadas (`docker compose up -d db redis mongo`) — todo debe pasar en verde, incluidos los tests preexistentes.
- [ ] Si el repo tiene `golangci-lint` configurado (ver PR #87), correr `golangci-lint run ./...` sobre los paquetes tocados antes de abrir el PR.
