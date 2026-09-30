---
id: 024
title: "Dashboard Metrics Query API"
tier: feature
status: done
veredicto: A
owner: Lucia Scharff
date: 2026-09-07
repos: [embolsadora4.0-cloud, embolsadora-frontend]
origin: superpowers
issues: [97]
prs: [78]
adrs: [CLOUD-ADR-017]
traducido: 2026-09-29
last_reviewed: 2026-09-29
---

# 024 — Dashboard Metrics Query API

> **Procedencia.** `origen: original` significa que el requisito está escrito en
> [`design.md`](design.md) (2026-09-07, con cierres del 2026-09-10 y 2026-09-11) o en la
> sección "Decisions for the TODOs" de [`plan.md`](plan.md) (2026-09-11), que cerró los
> puntos C1, C3, C4, C7 y C8 que el diseño había dejado abiertos. `origen: derivado`
> significa que se reconstruyó del código. **Cada requisito se verificó contra `develop`**
> el 2026-09-29.

## Contexto y problema

La ingesta ([022](../022-cloud-ingest-endpoint/spec.md)) guarda las mediciones del Edge en
MongoDB, pero no había forma de leerlas. El frontend necesitaba alimentar los widgets de
los dashboards ([005a](../005a-dashboard-layouts/spec.md)) con datos reales:

- valores agregados ("gramos promedio por bolsa en las últimas 8 h");
- series de tiempo para gráficos de barras o líneas;
- puntos crudos;
- distribuciones por categoría para gráficos de torta.

La dificultad es que el contenido de las mediciones **no está cerrado**: qué `aasPath`
existen y qué significan lo decide el Edge, y `payload` no se valida en la ingesta. Un
diseño que asuma nombres de métricas queda obsoleto cuando el Edge cambie. El principio
que gobierna el diseño es que **la API nunca conoce el significado de una métrica**:
trabaja con `aasPath` y rutas de `payload` como strings opacos. Así, una métrica nueva
del Edge es consultable el mismo día, sin tocar el backend.

## Alcance

**Entra:**

- Consulta estructurada (`POST …/query`) con cuatro modos de respuesta.
- Catálogo de `aasPath` observados (`GET …/catalog`).
- Consulta en lote para resolver un dashboard entero en un request (`POST …/query/batch`).
- Permiso `perm_metrics_view` y rate limit por usuario.

**No entra:**

- Correlación exacta por bolsa: no existe un identificador de ciclo compartido entre
  eventos.
- Agregación entre máquinas: `machineId` es obligatorio.
- `groupBy` combinado con `bucket`.
- Batching heterogéneo al estilo GraphQL.
- Polling automático por widget en v1: el refresco es manual o al enfocar la ventana.
- Caché, rollups, colecciones de series de tiempo y TTL.
- Zona horaria: v1 trabaja solo en UTC.

## Requisitos

### Contrato y seguridad

- **RF-001** `origen: original` (design, Contrato) — El sistema DEBE exponer
  `POST /api/v1/dashboards/metrics/query`, `GET /api/v1/dashboards/metrics/catalog` y
  `POST /api/v1/dashboards/metrics/query/batch` dentro del grupo `/api/v1`
  (`JWTAuth → TenantFromHeader → PasswordChangeGuard`), protegidos por
  `RBACCheck("perm_metrics_view")`.
  → `internal/api/handler/dashboards/routes.go`, `internal/routes/url_mappings.go`,
  `migrations/000015_metrics_view_permission.up.sql`.
- **RF-002** `origen: original` (design, Contrato) — El tenant DEBE salir del header
  `X-Tenant-ID`, nunca del body, y aplicarse server-side en el `$match` inicial de cada
  pipeline, de modo que ningún contenido del body pueda leer datos de otro tenant.
  → `baseMatch` en `internal/repo/mongo/metrics/repository.go`; test de aislamiento en
  `repository_test.go`.
- **RF-003** `origen: original` (design, Fork 4) — El sistema DEBE limitar la tasa por
  usuario (`supabase_user_id`), no por tenant: 15 req/min con ráfaga de 5
  (`DASHBOARDS_METRICS_RATE_LIMIT_RPM`, `…_BURST`). Sin Redis, el límite falla abierto.
  → `internal/api/middleware/dashboard_ratelimit.go`.

### Consulta

- **RF-004** `origen: original` (design, Request) — El request DEBE llevar `machineId`
  (obligatorio) y `metrics` (1 a `DASHBOARDS_METRICS_MAX_SPECS`, 10 por defecto), cada uno
  con `aasPath` y `agg` en `{avg, sum, min, max, last, count, delta, raw}`. Opcionales:
  `bucket` en `{1m, 5m, 15m, 1h, 6h, 1d}`, `groupBy`, `filter.valueEquals` y `maxPoints`.
  → `internal/domain/metrics/metrics.go`, `validate.go`.
- **RF-005** `origen: original` (design, C2) — La ventana temporal DEBE pedirse con
  **exactamente uno** de: `range` en `{15m, 30m, 1h, 8h, 24h, 7d, 30d}`, resuelto con el
  reloj del servidor al ejecutar; o `from`/`to` absolutos RFC3339 con `from < to`.
  → `internal/domain/metrics/window.go`.
- **RF-006** `origen: original` (design, reglas de exclusión mutua) — El modo DEBE
  decidirse así:
  - `agg: raw` solo si es la única métrica y no hay `bucket` ni `groupBy`;
  - `groupBy` solo con exactamente una métrica y sin `bucket`;
  - en cualquier otro caso, `scalar` sin `bucket` o `series` con `bucket`.
  → `internal/domain/metrics/validate.go`.
- **RF-007** `origen: original` (design, nota de seguridad) — `groupBy` DEBE validarse
  contra `^payload\.[a-zA-Z0-9_]+(\.[a-zA-Z0-9_]+)*$` antes de interpolarse en el
  pipeline, porque es el único string del cliente que se convierte en referencia de campo.
  → `groupByFieldPattern` en `validate.go`.
- **RF-008** `origen: original` (design, Traducción a pipeline) — Las agregaciones
  numéricas (`avg`, `sum`, `min`, `max`, `delta`) DEBEN descartar los valores no numéricos
  de `payload.value` sin fallar la consulta, y contar cada descarte en
  `dashboard_metrics_non_numeric_discarded_total` (Fork 2). `delta` es `last − first` y
  solo tiene sentido para métricas monotónicas (Fork 5).
  → `internal/repo/mongo/metrics/repository.go` (`reportNonNumericDiscards`).
- **RF-009** `origen: original` (design, Arquitectura) — Una consulta multi-métrica DEBE
  resolverse con un pipeline por `MetricSpec`, en paralelo.
  → `errgroup` en `internal/app/dashboards/service.go`.
- **RF-010** `origen: original` (plan, C7) — Si falla cualquier sub-consulta de una
  consulta multi-métrica, DEBE fallar la consulta entera: no se devuelven datos agregados
  incompletos.
  → `g.Wait()` en `Service.queryScalar` y `Service.querySeries`.
- **RF-011** `origen: original` (plan, C1) — Los buckets DEBEN truncarse en UTC; no hay
  parámetro de zona horaria en v1.
  → comentario de `Bucket` en `metrics.go`.

### Respuesta

- **RF-012** `origen: original` (design, Response y C5) — Las cuatro formas de respuesta
  DEBEN llevar el discriminador `mode` (`scalar`, `series`, `raw` o `grouped`), `dataAsOf`
  (el `ts` máximo de los documentos que matchearon, o `null`) y `from`/`to` como
  timestamps absolutos efectivamente resueltos.
  → `ModeScalar…ModeGrouped` en `metrics.go`, `maxDataAsOf` en `service.go`,
  `internal/api/handler/dashboards/dto/`.
- **RF-013** `origen: original` (design, Response) — En el modo `series`, un bucket sin
  datos para una métrica NO DEBE rellenarse con 0: la métrica simplemente no aparece en
  ese bucket.
- **RF-014** `origen: derivado` — Las respuestas usan el envelope
  `{"success":true,"data":…}` y los errores `{"success":false,"error","code"}`. Es el
  formato que fijó el diseño; queda anotado porque contradice la estandarización
  [016](../016-api-standardization/spec.md), que eliminó ese envelope en otros módulos.
  → `internal/api/handler/dashboards/query_metrics.go`, `errors.go`.

### Guardrails

- **RF-015** `origen: original` (design, tabla de guardrails; plan, C8) — El sistema DEBE
  rechazar con 400 y un `code`:
  - `INVALID_PARAMS`, `TOO_MANY_METRICS`, `RAW_MODE_CONFLICT` y `GROUP_BY_CONFLICT`;
  - `RANGE_TOO_WIDE` si se esperan más de `DASHBOARDS_METRICS_MAX_BUCKETS` buckets (1000)
    o más de `DASHBOARDS_METRICS_MAX_RAW_POINTS` puntos crudos (5000);
  - `TOO_MANY_GROUPS` si se superan `DASHBOARDS_METRICS_MAX_GROUPS` grupos (200).

  Los excesos se detectan pidiendo `límite + 1`: nunca se devuelven datos truncados sin
  avisar. Todos los topes son configuración, no constantes.
  → `internal/domain/metrics/errors.go`, `internal/config/config.go` (`DashboardsConfig`).
- **RF-016** `origen: original` (plan, C3) — En modo `raw`, si viene `maxPoints`, el
  sistema DEBE decimar del lado del servidor (muestreo por paso uniforme que conserva el
  primer y el último punto) en vez de rechazar. `RANGE_TOO_WIDE` se dispara igual si la
  lectura alcanzó el tope, pida o no `maxPoints`.
  → `Repository.Raw`, `decimate_test.go`, `Service.queryRaw`.
- **RF-017** `origen: original` (plan, C4) — Cada agregación DEBE estar acotada por
  `DASHBOARDS_METRICS_MAX_TIME_MS` (5000 ms por defecto). Un timeout responde **504** con
  `code: QUERY_TIMEOUT`.
  → `internal/api/handler/dashboards/errors.go`.
- **RF-018** `origen: derivado` — Si Mongo no estaba disponible al arrancar, esta
  superficie responde error en cada request y no tumba el resto de la API.
  → `metricsmongo.Unavailable` en `internal/routes/url_mappings.go`.

### Catálogo y lote

- **RF-019** `origen: original` (design, Fork 1) — `GET …/catalog?machineId=` DEBE
  devolver los `aasPath` **efectivamente observados** para esa máquina en el tenant (un
  `distinct` sobre `measurements`, no un registro declarado), sin filtro temporal.
  `machineId` ausente responde 400 `INVALID_PARAMS`.
  → `Service.Catalog`, `catalog_metrics.go`.
- **RF-020** `origen: original` (design, endpoint batch) — `POST …/query/batch` DEBE
  aceptar de 1 a `DASHBOARDS_METRICS_MAX_BATCH_QUERIES` (50) items con `id` único.
  - Cada item se valida y ejecuta de forma independiente, en paralelo, y la respuesta se
    correlaciona por `id`.
  - Un item que falla no afecta a los demás.
  - Más de 50 items responde `TOO_MANY_QUERIES`; un `id` duplicado, `INVALID_PARAMS`.
  → `internal/domain/metrics/batch.go`, `Service.Batch`, `batch_query_metrics.go`.

### No funcionales

- **RNF-001** `origen: original` (design, Fork 3 y Fork 6) — Consistencia eventual
  simple sobre una colección de solo inserción; 100 % request-time, sin rollups ni caché
  en v1.
- **RNF-002** `origen: original` (design, Traducción a pipeline) — Todos los pipelines
  arrancan con el `$match` que cubre el índice `ix_tenant_machine_path_ts` de la ingesta.

## Criterios de éxito

El diseño no numeró criterios de aceptación. Los siguientes se derivan de su sección
"Testing" y se verifican con los tests existentes. Los de `repo/mongo/metrics` necesitan
`MONGO_URI`.

| CE | Criterio | Evidencia |
|---|---|---|
| **CE-001** | Cada regla de validación, exclusión mutua y guardrail rechaza con el `code` correcto | `internal/domain/metrics/validate_test.go`, `window_test.go`, `batch_test.go`, `errors_test.go` |
| **CE-002** | El `$match` de tenant nunca deja leer datos de otro tenant, ni con `groupBy` ni con un `aasPath` que coincida | `internal/repo/mongo/metrics/repository_test.go` |
| **CE-003** | Cada `agg` devuelve el valor correcto sobre fixtures sembradas | `repository_test.go` |
| **CE-004** | Las cuatro formas de respuesta se arman según el modo, con `mode` y `dataAsOf` | `internal/app/dashboards/service_test.go` |
| **CE-005** | En el lote, un item que falla no afecta a los demás, y el orden de `results` no depende del orden de finalización | `service_test.go`, `batch_query_metrics_test.go` |
| **CE-006** | Superar el rate limit por usuario devuelve 429, y sin Redis falla abierto | `internal/api/middleware/dashboard_ratelimit_test.go` |
| **CE-007** | Con `maxPoints`, el modo `raw` decima y conserva el primer y el último punto | `internal/repo/mongo/metrics/decimate_test.go` |

## Decisiones

| Decisión | Alternativas descartadas | Por qué | Fuente |
|---|---|---|---|
| REST con un "query object" | GraphQL | Una consulta por widget, sin grafo de entidades; GraphQL suma dependencia, transporte paralelo y superficie a asegurar sin beneficio | design |
| Catálogo observado, no declarado | Registro a mano; posponerlo | El Edge no cerró los nombres de `aasPath` | Fork 1 |
| Rate limit por usuario, 15 req/min | Por tenant | Un usuario no debe agotar el cupo de sus compañeros | Fork 4 |
| Sin polling en v1, con un endpoint de lote | Polling por widget con caché | El polling rompía el límite; el lote resuelve un dashboard en un request | Fork 4 (replanteo 2026-09-11) |
| `range` relativo además de `from`/`to` | Solo `from`/`to` | Evita el desajuste de reloj y da cache keys estables | C2 |
| Discriminador `mode` | Inferir el modo por las claves presentes | Un tipo generado limpio; agregarlo después sería breaking de facto | C5 |
| Decimación por paso uniforme | LTTB | Es la más simple correcta; cambiar el algoritmo no cambia el contrato | C3 (plan) |
| Fallo total si falla una sub-consulta | Respuesta parcial | Nunca devolver agregados incompletos en silencio | C7 (plan) |

La decisión de guardar las mediciones en MongoDB está en
[CLOUD-ADR-017](../../docs/adr/CLOUD-ADR-017-mediciones-en-mongodb.md).

## Riesgos y preguntas abiertas

- **`RANGE_TOO_WIDE` mezcla dos causas** (demasiados buckets o demasiados puntos). El plan
  la dejó como seguimiento (C8), sin implementar, porque separarla es un cambio de
  contrato.
- **Zona horaria.** Los buckets de `1d` y `6h` en UTC parten el turno noche de la planta
  (UTC−3) (C1).
- **Valores no escalares** en `payload.value`: se descartan y se cuentan, pero no se
  corrigen en origen (Fork 2).
- **`delta` sobre contadores que se reinician** da resultados erróneos; no hay detección
  de reinicios (Fork 5).
- **Modelo de refresco de v2** (polling a nivel dashboard o SSE): pendiente de medir la
  cadencia real del Edge en producción. `dataAsOf` es el gancho previsto.
- **Dos convenciones de respuesta en la API** (RF-014 frente a 016).
- **Referencia a casos de uso del hub** (`UC-xx`): pendiente hasta que exista
  `embolsadora-docs`.
