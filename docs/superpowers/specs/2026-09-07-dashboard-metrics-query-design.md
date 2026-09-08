# Dashboard Metrics Query API

**Date**: 2026-09-07
**Status**: Approved, pending implementation plan

## Context

`POST /api/v1/consumers/events` ya persiste mediciones del Edge en MongoDB
(`internal/repo/mongo/measurements`), pero no existe ninguna forma de leerlas de
vuelta. El frontend necesita alimentar los widgets de `specs/005-dashboard-layouts`
con datos reales: valores agregados (ej. "gramos promedio por bolsa en las
últimas 8hs", "cantidad de bolsas procesadas"), series de tiempo para gráficos
de barra/timeline, puntos crudos sin totalizar, y distribuciones categóricas
para gráficos de torta.

El esquema de `Measurement` (`internal/domain/ingest/measurement.go`) es
deliberadamente genérico: `payload map[string]any` no se valida nunca (D-8,
`docs/superpowers/plans/2026-08-05-cloud-ingest-endpoint.md`), y el contenido
real —qué `aasPath` existen, qué significan, cómo se relacionan entre sí— lo
decide el Edge/PLC y **todavía no está cerrado ni versionado**. Cualquier
diseño que asuma nombres de métrica o relaciones entre eventos específicas
quedaría obsoleto en cuanto el Edge cambie de idea.

## Decisión: REST con un "query object", no GraphQL

Se evaluó GraphQL explícitamente. Se descarta porque:

- El frontend pide **una llamada por widget**, nunca batching de múltiples
  entidades heterogéneas en un solo round-trip — la ventaja central de GraphQL
  no aplica.
- La respuesta es siempre "un valor, una serie o una lista de puntos", nunca un
  grafo de entidades relacionadas donde el cliente elige qué campos anidados
  traer.
- Introducir GraphQL sumaría una dependencia nueva (gqlgen o similar), un
  transporte paralelo al de Gin, resolvers para reinyectar tenant/RBAC en el
  contexto, y superficie nueva que asegurar (introspection, playground) — costo
  real sin beneficio para este caso.

En cambio, se define un **endpoint único con un cuerpo JSON estructurado**
("query object") que el backend valida contra un esquema cerrado y traduce a
un pipeline de agregación de Mongo. Esto da la flexibilidad que se buscaba en
GraphQL (el cliente arma la consulta que necesita) sin la maquinaria: es el
mismo patrón que usan APIs de búsqueda de solo lectura con cuerpos complejos
(Elasticsearch `_search`).

**Principio de diseño que gobierna todo lo demás**: la API nunca conoce el
*significado* de una métrica. Solo trabaja con nombres de campos como strings
opacos (`aasPath`, rutas dentro de `payload`). Cualquier métrica nueva que el
Edge decida emitir mañana queda consultable el mismo día, sin tocar este
backend — es la única forma de cumplir "ser abiertos a que el Edge cambie sin
tener que tocar esta API".

## No-objetivos

- **Correlación exacta por bolsa** (ej. "el peso Y la temperatura de sellado de
  la bolsa N, emparejadas"). Requeriría que el Edge marque eventos relacionados
  con un identificador de bolsa/ciclo compartido — campo que no existe hoy en
  `Measurement` y que no está definido. Diseñar contra un campo que no existe
  sería apostar a una forma de esquema que puede no llegar así. Si el Edge lo
  define más adelante, se puede sumar como un filtro adicional sin romper este
  contrato (es un campo nuevo, no un reemplazo).
- **Agregación cross-máquina** (`machineId` opcional, agregando todas las
  máquinas de un tenant). En el MVP cada tenant tiene una sola máquina, y el
  índice `ix_tenant_machine_path_ts` ya existente está pensado para consultas
  con `machineId` fijo. `machineId` es requerido a propósito: aflojarlo a
  opcional el día que haga falta es un cambio no-breaking; exigirlo recién en
  ese momento sí lo sería.
- **`groupBy` combinado con `bucket`** (categoría × tiempo simultáneamente).
  Cada uno resuelve un tipo de gráfico distinto (torta vs. barra/línea);
  combinarlos es una extensión aditiva para cuando haga falta, no un requisito
  de v1.
- Batching de múltiples widgets en un solo request — descartado explícitamente
  en las conversaciones previas a este spec.

## Contrato

```
POST /api/v1/dashboards/metrics/query
```

Mismo grupo de rutas que el resto de `/api/v1`: `JWTAuth → TenantFromHeader →
PasswordChangeGuard → RBACCheck(perm_metrics_view)`. `tenantId` sale siempre
del header `X-Tenant-ID` (nunca del body) y se inyecta server-side en el
`$match` inicial de cada pipeline — el body no puede pedir datos de otro
tenant pase lo que pase en su contenido. Es un endpoint de solo lectura sin
efectos secundarios; usa POST porque el cuerpo es una estructura anidada
(lista de métricas, filtro) que no entra prolijo en una query string.

### Request

```go
type MetricQueryRequest struct {
    MachineID string        `json:"machineId"`         // requerido
    From      time.Time     `json:"from"`               // requerido, RFC3339
    To        time.Time     `json:"to"`                 // requerido, > From
    Bucket    string        `json:"bucket,omitempty"`   // "1m"|"5m"|"15m"|"1h"|"6h"|"1d"
    Metrics   []MetricSpec  `json:"metrics"`             // requerido, 1..10
    GroupBy   string        `json:"groupBy,omitempty"`  // "payload.<campo>"
    Filter    *ValueFilter  `json:"filter,omitempty"`
}

type MetricSpec struct {
    AasPath string `json:"aasPath"` // requerido
    Agg     string `json:"agg"`     // avg|sum|min|max|last|count|delta|raw
}

type ValueFilter struct {
    ValueEquals any `json:"valueEquals,omitempty"` // bool|number|string
}
```

**Reglas de exclusión mutua** (determinan cuál de los 4 modos de respuesta se
usa):

1. `agg:"raw"` solo es válido si es la única entrada de `metrics` y no hay
   `bucket` ni `groupBy`.
2. `groupBy` solo es válido con exactamente 1 métrica y sin `bucket`.
3. Cualquier otro caso: modo multi-métrica — escalar si no hay `bucket`, serie
   si lo hay.

**Guardrails** (400 con `code`, protegen a Mongo de una consulta cara o mal
formada):

| Code | Motivo |
|---|---|
| `INVALID_PARAMS` | falta `machineId`/`from`/`to`/`metrics`, `from >= to`, `aasPath`/`agg`/`bucket` inválido |
| `TOO_MANY_METRICS` | `len(metrics) > 10` |
| `RAW_MODE_CONFLICT` | `agg:"raw"` combinado con `bucket`, `groupBy` o más de 1 métrica |
| `GROUP_BY_CONFLICT` | `groupBy` combinado con `bucket` o más de 1 métrica |
| `RANGE_TOO_WIDE` | más de 1000 buckets esperados, o más de 5000 puntos en modo `raw` — se detecta pidiendo `limit+1` y rechazando si se excede, nunca devolviendo datos truncados sin avisar |
| `TOO_MANY_GROUPS` | `groupBy` produce más de 200 grupos distintos |

Errores 401/403 los maneja la cadena de middleware existente, sin código
propio.

### Response — 4 formas según el modo

**Escalar** (multi-métrica, sin `bucket`):
```json
{"success": true, "data": {
  "machineId": "EMB-DEV-001", "from": "...", "to": "...",
  "results": [
    {"aasPath": "Operativos/Pesada/peso", "agg": "avg", "value": 1.023, "sampleCount": 1450}
  ]
}}
```

**Serie bucketizada** (multi-métrica, con `bucket`):
```json
{"success": true, "data": {
  "machineId": "EMB-DEV-001", "from": "...", "to": "...", "bucket": "1h",
  "series": [
    {"ts": "2026-09-07T02:00:00Z", "results": [
      {"aasPath": "Operativos/Pesada/peso", "agg": "avg", "value": 1.02, "sampleCount": 34},
      {"aasPath": "Operativos/Pesada/peso", "agg": "count", "value": 34, "sampleCount": 34}
    ]}
  ]
}}
```
Un bucket sin datos para una métrica simplemente no la incluye en su
`results` — nunca se rellena con `0`: un cero fabricado se leería como
"midió cero", que es un hecho distinto de "no hubo datos".

**Cruda** (`agg:"raw"`):
```json
{"success": true, "data": {
  "machineId": "EMB-DEV-001", "from": "...", "to": "...",
  "aasPath": "Operativos/Sellado/temperatura",
  "points": [{"ts": "2026-09-07T09:40:03Z", "value": 82.5}]
}}
```

**Agrupada** (`groupBy`):
```json
{"success": true, "data": {
  "machineId": "EMB-DEV-001", "from": "...", "to": "...",
  "aasPath": "Alarmas/tipo", "agg": "count", "groupBy": "payload.tipo",
  "groups": [{"key": "sellado_defectuoso", "value": 12}]
}}
```

### Ejemplos de uso (del frontend)

- **Cantidad de bolsas procesadas en 8hs** (asumiendo un evento
  `Operativos/Pesada/peso` por bolsa): `metrics:[{aasPath:"Operativos/Pesada/peso", agg:"count"}]`,
  `from`/`to` cubriendo las 8hs, sin `bucket` → modo escalar, `results[0].value`.
- **Promedio de kg por bolsa en la última producción**: mismo `aasPath`,
  `agg:"avg"`.
- **Temperatura de cierre por cada bolsa, últimos 20 min**: `metrics:[{aasPath:"Operativos/Sellado/temperatura", agg:"raw"}]`
  → modo crudo, un punto por evento.
- **Peso promedio y cantidad de bolsas por hora, mismo gráfico**:
  `bucket:"1h"`, `metrics:[{aasPath:"...peso", agg:"avg"}, {aasPath:"...peso", agg:"count"}]`
  → modo serie, dos series alineadas por `ts`.

## Arquitectura interna

Sigue el layout hexagonal existente (`domain → app → transport`, `repo` aparte):

- **`internal/domain/metrics/`** — tipos puros: `MetricQuery`, `MetricSpec`,
  enums `Agg`/`Bucket` y toda la validación de arriba (reglas de exclusión
  mutua incluidas). No conoce Mongo ni Gin. Interfaz:
  ```go
  type Repository interface {
      Query(ctx context.Context, tenantID string, q MetricQuery) (QueryResult, error)
  }
  ```
- **`internal/repo/mongo/metrics/`** — única pieza que arma pipelines de
  agregación contra la colección `measurements` (la misma que usa
  `internal/repo/mongo/measurements`). Corre **un pipeline por `MetricSpec`**,
  no uno combinado: cada `aasPath` es un stream de datos independiente, así
  que separarlos mantiene cada pipeline simple y testeable en aislamiento, al
  costo de N round-trips en vez de 1 — aceptable con el tope de 10 métricas y
  sin requisito de latencia sub-ms. Se corren concurrentes (`errgroup`) cuando
  hay más de una métrica.
- **`internal/app/dashboards/`** — usecase que valida vía `domain/metrics`,
  dispara las sub-consultas, arma la forma de respuesta (una de las 4) según
  el modo detectado.
- **`internal/api/handler/dashboards/query_metrics.go`** — handler Gin: bind
  del JSON, mapea errores de dominio a `{success:false, error, code}`.
- **`internal/routes/url_mappings.go`** — registra la ruta dentro del grupo
  `/api/v1` ya existente, con `RBACCheck(perm_metrics_view)`.
- **Migración nueva** — permiso `perm_metrics_view`, siguiendo el patrón
  `_view`/`_manage` de `perm_edge_devices_view` (migración `000011`), seedeado
  a los roles que hoy tienen `perm_dashboard`/`perm_analytics`.

### Traducción a pipeline de Mongo, por `agg`

Todos arrancan con `$match: {tenantId, machineId, "payload.aasPath": aasPath,
ts: {$gte: from, $lte: to}}` — el mismo shape que ya cubre el índice
`ix_tenant_machine_path_ts`.

- `avg`/`sum`/`min`/`max`: `$match` adicional con `$isNumber` sobre
  `payload.value` (payload no se valida en la ingesta, D-8, así que un valor
  no numérico se descarta en vez de romper la consulta) → `$group` con el
  acumulador correspondiente + `sampleCount: {$sum: 1}`.
- `last`: `$sort ts:1` → `$group` con `$last`.
- `delta`: `$sort ts:1` → `$group` con `$first` y `$last`; la resta
  (`last - first`) se hace en el usecase, no en el pipeline.
- `count`: `$match` (+ igualdad sobre `payload.value` si viene
  `filter.valueEquals`) → `$count`, o `$group` por bucket con `$sum: 1`.
- `raw`: `$sort ts:1` → `$limit(5001)` para poder distinguir "hay exactamente
  5000" de "hay más y hay que rechazar con `RANGE_TOO_WIDE`".
- `groupBy`: `$group` por el campo indicado, usando el mismo acumulador que le
  corresponde al `agg` pedido en el único `MetricSpec` del request (no siempre
  `count` — "promedio de temperatura por turno" es tan válido como "cantidad
  de alarmas por tipo"), `$sort` descendente por ese valor, `$limit(201)` con
  la misma lógica de detección de exceso que `raw`.

**Nota de seguridad**: `groupBy` es el único punto donde un string que manda
el cliente se convierte en una referencia de campo dentro del pipeline
(`"$" + groupBy`). Se valida contra un patrón estricto
(`^payload\.[a-zA-Z0-9_]+(\.[a-zA-Z0-9_]+)*$`) antes de interpolarlo — sin
esto, un valor malicioso podría intentar leerse como un operador de Mongo en
vez de un nombre de campo.

## Testing

- **`domain/metrics`**: unit tests table-driven (testify) para cada regla de
  validación y de exclusión mutua, y cada guardrail de la tabla de arriba.
- **`repo/mongo/metrics`**: integration tests detrás de `MONGO_URI` (mismo
  patrón que `internal/repo/mongo/measurements/repository_test.go`), con
  fixtures sembradas cubriendo cada `agg`. El caso más importante: verificar
  que el `$match` de `tenantId` nunca deja leer datos de otro tenant, ni
  siquiera con un `groupBy` o `aasPath` que coincida por casualidad.
- **`app/dashboards`**: unit tests con `Repository` mockeado (`uber/mock`,
  mismo patrón que el resto del repo) para el armado de las 4 formas de
  respuesta y la orquestación concurrente multi-métrica.
- **`handler/dashboards`**: unit tests de bind/validación y mapeo de errores a
  código HTTP, mismo patrón que `internal/consumers/events_handler_test.go`.
