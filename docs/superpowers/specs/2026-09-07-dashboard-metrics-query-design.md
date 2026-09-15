# Dashboard Metrics Query API

**Date**: 2026-09-07
**Status**: Approved, pending implementation plan
**Actualizado**: 2026-09-10 — cierre de 6 forks abiertos (ver sección
"Forks cerrados" al final): catálogo de aasPath sumado a v1, rate limit
por usuario, y notas de riesgo/observabilidad para `payload.value` no
escalar y `delta` sobre contadores.
**Actualizado (2)**: 2026-09-11 — replanteo del Fork 4 a partir de review
de PR: v1 no hace polling automático por widget (refresh manual +
on-focus), se agrega `dataAsOf` a las 4 formas de respuesta y un endpoint
batch (`POST .../query/batch`) para que un dashboard completo resuelva en
un solo request.
**Actualizado (3)**: 2026-09-11 — cierre de C2 y C5 (ver "Ajustes de
contrato cerrados"): `range` como alternativa a `from`/`to`, y discriminador
`mode` explícito en las 4 formas de respuesta.

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

- El frontend pide **una consulta por widget**, nunca un grafo de entidades
  heterogéneas relacionadas en un solo round-trip — la ventaja central de
  GraphQL no aplica. (El endpoint batch agregado más abajo agrupa N veces la
  misma forma de query homogénea para resolver un dashboard completo; no es
  el batching heterogéneo que GraphQL resuelve, así que no reabre este
  argumento.)
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
- **Batching heterogéneo estilo GraphQL** (el cliente arma un grafo arbitrario
  de entidades distintas en un solo round-trip) — descartado explícitamente
  en las conversaciones previas a este spec, y es la razón central para no
  usar GraphQL (ver más arriba). *Actualizado 2026-09-11*: esto no incluye el
  endpoint batch agregado en Contrato — ese agrupa N veces la misma forma de
  `MetricQueryRequest` para resolver un dashboard completo en un request, no
  un grafo heterogéneo; se agregó para que el modelo "sin polling en v1" no
  dependa de que el frontend dispare un burst de hasta 50 requests al cargar
  la pantalla.

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

**v1 — sin polling automático por widget.** El dashboard resuelve sus
widgets al montar la pantalla, más un botón "Actualizar" manual y refresh
on-focus — mismo criterio que ya usa `specs/007` para el layout ("sin
auto-guardado, solo guardado manual explícito"). No hay `setInterval` por
widget refetcheando cada 10-60s. Esto es lo que hace que el rate limit de
15 req/min por usuario (ver "Forks cerrados", fork 4) y la ausencia de cache
alcancen: la carga por usuario es un puñado de requests al abrir el
dashboard o al pedir refresh, no un flujo continuo. **Revisar en v2**: con
datos reales de producción (cadencia real del Edge por `aasPath`), decidir
un modelo de refresh vivo (poll a nivel dashboard cada 60-120s, o SSE desde
el cloud en cada ingest) — aditivo sobre este contrato, no breaking.

### Request

```go
type MetricQueryRequest struct {
    MachineID string        `json:"machineId"`         // requerido
    Range     string        `json:"range,omitempty"`    // "15m"|"30m"|"1h"|"8h"|"24h"|"7d"|"30d" — mutuamente excluyente con from/to
    From      *time.Time    `json:"from,omitempty"`     // RFC3339 — requerido si no viene `range`
    To        *time.Time    `json:"to,omitempty"`       // RFC3339, > From — requerido si no viene `range`
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

**Selección de ventana temporal — `range` vs. `from`/`to`.** Exactamente uno
de los dos debe venir: `range` (un enum de ventanas relativas comunes) o el
par `from`+`to` (rango absoluto arbitrario, para selectores de fecha custom
del frontend). Cuando viene `range`, el backend resuelve `to = now()` (reloj
del servidor) y `from = to - range` **en el momento de ejecutar la
consulta** — el cliente nunca calcula ni envía timestamps para sus casos más
comunes ("últimas 8hs", "últimos 20 min"), lo que elimina el desajuste de
reloj cliente/servidor y hace que la misma consulta lógica ("últimas 8hs de
peso") tenga siempre la misma forma de request, sea cual sea el momento en
que se dispare — precondición para que una cache futura (ver Fork 4) pueda
tener cache keys estables sin que el equipo tenga que rediseñar el contrato
en ese momento. `from`/`to` absolutos siguen existiendo para rangos que no
encajan en el enum (ej. "el turno del martes pasado").

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
| `INVALID_PARAMS` | falta `machineId`/`metrics`; falta tanto `range` como `from`+`to`, o vienen los dos a la vez; `range` fuera del enum permitido; `from >= to`; `aasPath`/`agg`/`bucket` inválido |
| `TOO_MANY_METRICS` | `len(metrics) > 10` |
| `RAW_MODE_CONFLICT` | `agg:"raw"` combinado con `bucket`, `groupBy` o más de 1 métrica |
| `GROUP_BY_CONFLICT` | `groupBy` combinado con `bucket` o más de 1 métrica |
| `RANGE_TOO_WIDE` | más de 1000 buckets esperados, o más de 5000 puntos en modo `raw` — se detecta pidiendo `limit+1` y rechazando si se excede, nunca devolviendo datos truncados sin avisar |
| `TOO_MANY_GROUPS` | `groupBy` produce más de 200 grupos distintos |

Errores 401/403 los maneja la cadena de middleware existente, sin código
propio.

### Response — 4 formas según el modo

Las 4 formas comparten dos campos: `mode` (discriminador explícito —
`"scalar"|"series"|"raw"|"grouped"`, uno por cada modo detectado en las
reglas de exclusión mutua de arriba) y `dataAsOf` (el `$max: "$ts"` del set
de documentos que matchearon el filtro, no `time.Now()` del servidor). El
frontend no necesita inferir el modo mirando qué claves están presentes
(`results` vs. `series` vs. `points` vs. `groups`) — arma un discriminated
union sobre `mode` directamente, y el tipo generado desde `docs/openapi.yaml`
queda limpio en vez de "4 shapes que se solapan". `dataAsOf` es casi gratis
— cada pipeline ya ordena u opera sobre `ts` — y es el gancho para que v2
pueda hacer refresh condicional ("¿avanzó `dataAsOf` desde la última carga?
recién ahí reconsulto") sin cambiar el contrato. Si el rango no matcheó
ningún documento, `dataAsOf` es `null`. `from`/`to` en la respuesta son
siempre el rango absoluto efectivamente resuelto por el servidor —si el
request vino con `range`, acá están los timestamps concretos a los que se
resolvió, no el string original— para que el cliente nunca tenga que
recalcular `range` por su cuenta.

**Escalar** (multi-métrica, sin `bucket`):
```json
{"success": true, "data": {
  "mode": "scalar",
  "machineId": "EMB-DEV-001", "from": "...", "to": "...", "dataAsOf": "2026-09-07T09:59:47Z",
  "results": [
    {"aasPath": "Operativos/Pesada/peso", "agg": "avg", "value": 1.023, "sampleCount": 1450}
  ]
}}
```

**Serie bucketizada** (multi-métrica, con `bucket`):
```json
{"success": true, "data": {
  "mode": "series",
  "machineId": "EMB-DEV-001", "from": "...", "to": "...", "bucket": "1h",
  "dataAsOf": "2026-09-07T09:59:47Z",
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
  "mode": "raw",
  "machineId": "EMB-DEV-001", "from": "...", "to": "...", "dataAsOf": "2026-09-07T09:59:47Z",
  "aasPath": "Operativos/Sellado/temperatura",
  "points": [{"ts": "2026-09-07T09:40:03Z", "value": 82.5}]
}}
```

**Agrupada** (`groupBy`):
```json
{"success": true, "data": {
  "mode": "grouped",
  "machineId": "EMB-DEV-001", "from": "...", "to": "...", "dataAsOf": "2026-09-07T09:59:47Z",
  "aasPath": "Alarmas/tipo", "agg": "count", "groupBy": "payload.tipo",
  "groups": [{"key": "sellado_defectuoso", "value": 12}]
}}
```

### Endpoint de catálogo (`aasPath` observados)

```
GET /api/v1/dashboards/metrics/catalog?machineId=EMB-DEV-001
```

Mismo middleware chain y permiso que el endpoint de query
(`perm_metrics_view`). El Edge todavía no cierra ni versiona los nombres de
`aasPath` (ver Contexto), así que el frontend no puede hardcodearlos: este
endpoint devuelve los `aasPath` que **efectivamente aparecieron** para ese
`machineId` dentro del tenant — un `distinct`/`$group` sobre
`payload.aasPath` en `measurements`, no un registro declarado a mano. Si el
Edge nunca emitió un path, no aparece en el catálogo, sin importar si "va a"
emitirlo en el futuro.

```json
{"success": true, "data": {
  "machineId": "EMB-DEV-001",
  "aasPaths": ["Operativos/Pesada/peso", "Operativos/Sellado/temperatura"]
}}
```

Sin filtro de rango temporal en v1 — devuelve todo lo histórico observado
para esa máquina. Si el volumen de `aasPath` distintos crece lo suficiente
como para que la respuesta sea pesada, se puede sumar `from`/`to` opcionales
sin romper el contrato (son parámetros aditivos). `machineId` ausente o vacío
→ 400 `INVALID_PARAMS`, mismo código que usa el endpoint de query.

### Endpoint batch (un dashboard = un request)

```
POST /api/v1/dashboards/metrics/query/batch
```

Mismo middleware chain, permiso y rate limit que el endpoint single. Un
dashboard puede tener hasta 50 widgets (`dashboard-layout.ts` en el
frontend), cada uno con su propio `MetricQueryRequest` — sin batch, cargar
la pantalla es un burst de hasta 50 requests HTTP simultáneos que reventaría
cualquier rate limit razonable. El batch agrupa **N veces la misma forma de
query** (no un grafo de entidades heterogéneas, ver nota en la sección
GraphQL más arriba), así que no reabre ese argumento.

```go
type BatchMetricQueryRequest struct {
    Queries []BatchMetricQueryItem `json:"queries"` // requerido, 1..50
}

type BatchMetricQueryItem struct {
    ID string `json:"id"` // requerido, opaco para el backend — el frontend lo usa para
                           // correlacionar la respuesta con el widget que la pidió
    MetricQueryRequest      // mismo shape que el endpoint single (inline)
}
```

Cada item se valida y ejecuta **independientemente** — un `aasPath` mal
armado en el widget 7 no tira abajo los otros 49. La respuesta del batch
siempre es `success:true` a nivel envelope (el batch en sí se procesó);
el resultado por item lleva su propio flag:

```json
{"success": true, "data": {
  "results": [
    {"id": "widget-bag-counter", "success": true, "data": {
      "mode": "scalar",
      "machineId": "EMB-DEV-001", "from": "...", "to": "...", "dataAsOf": "2026-09-07T09:59:47Z",
      "results": [{"aasPath": "Operativos/Pesada/peso", "agg": "count", "value": 812, "sampleCount": 812}]
    }},
    {"id": "widget-temp-raw", "success": false, "error": "rango produce más de 5000 puntos", "code": "RANGE_TOO_WIDE"}
  ]
}}
```
El `data` de cada item exitoso es exactamente una de las 4 formas de
respuesta del endpoint single (según el modo que ese item detectó), `dataAsOf`
incluido.

**Guardrail nuevo**: `TOO_MANY_QUERIES` (400) si `len(queries) > 50` — mismo
tope que el máximo de widgets por layout, para no aceptar un batch que ya
sabemos que ningún dashboard real puede generar. `id` duplicado entre items
→ `INVALID_PARAMS`.

Los items se resuelven concurrentes (`errgroup`, mismo patrón que las
sub-consultas multi-métrica de un query single) — el tope de 50 acota el
fan-out total de pipelines de Mongo por request de la misma forma que el
tope de 10 métricas acota el query single.

### Ejemplos de uso (del frontend)

- **Cantidad de bolsas procesadas en 8hs** (asumiendo un evento
  `Operativos/Pesada/peso` por bolsa): `range:"8h"`,
  `metrics:[{aasPath:"Operativos/Pesada/peso", agg:"count"}]`, sin `bucket`
  → modo escalar, `results[0].value`.
- **Promedio de kg por bolsa en la última producción**: mismo `aasPath`,
  `agg:"avg"`, mismo `range`.
- **Temperatura de cierre por cada bolsa, últimos 20 min**: `range:"15m"` (o
  el valor del enum más cercano a lo que pida el widget),
  `metrics:[{aasPath:"Operativos/Sellado/temperatura", agg:"raw"}]` → modo
  crudo, un punto por evento.
- **Peso promedio y cantidad de bolsas por hora, mismo gráfico**:
  `range:"24h"`, `bucket:"1h"`,
  `metrics:[{aasPath:"...peso", agg:"avg"}, {aasPath:"...peso", agg:"count"}]`
  → modo serie, dos series alineadas por `ts`.
- **Rango custom elegido a mano en un selector de fecha** (ej. "el turno del
  martes pasado"): `from`/`to` absolutos en vez de `range`.

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
- **`internal/repo/mongo/metrics/`** — arma pipelines de agregación contra la
  colección `measurements` (la misma que usa
  `internal/repo/mongo/measurements`). Corre **un pipeline por `MetricSpec`**,
  no uno combinado: cada `aasPath` es un stream de datos independiente, así
  que separarlos mantiene cada pipeline simple y testeable en aislamiento, al
  costo de N round-trips en vez de 1 — aceptable con el tope de 10 métricas y
  sin requisito de latencia sub-ms. Se corren concurrentes (`errgroup`) cuando
  hay más de una métrica. También expone el `distinct` de `payload.aasPath`
  que usa el endpoint de catálogo.
- **`internal/app/dashboards/`** — usecase que valida vía `domain/metrics`,
  dispara las sub-consultas, arma la forma de respuesta (una de las 4) según
  el modo detectado; incluye el usecase (más simple) del catálogo, y el
  usecase de batch, que reusa el usecase de query single por cada item
  (`errgroup`) y agrega el resultado con su `id`.
- **`internal/api/handler/dashboards/query_metrics.go`**,
  **`.../catalog_metrics.go`** y **`.../batch_query_metrics.go`** —
  handlers Gin: bind del JSON/query params, mapean errores de dominio a
  `{success:false, error, code}` (o al item correspondiente, en el caso del
  batch).
- **`internal/api/middleware/`** — nuevo middleware de rate limit Redis-backed
  para el grupo de rutas `dashboards/metrics`, mismo patrón que
  `internal/consumers/ratelimit.go` pero con clave `supabase_user_id` (sale
  del contexto que deja `JWTAuth`) en vez de API key. Ver "Forks cerrados" al
  final para el porqué y el umbral.
- **`internal/routes/url_mappings.go`** — registra ambas rutas dentro del
  grupo `/api/v1` ya existente, con `RBACCheck(perm_metrics_view)` y el nuevo
  rate limit.
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
  acumulador correspondiente + `sampleCount: {$sum: 1}`. El descarte nunca
  falla la consulta ni se reporta al cliente — ver "Forks cerrados" (fork 2)
  para cómo se observa esto internamente.
- `last`: `$sort ts:1` → `$group` con `$last`.
- `delta`: `$sort ts:1` → `$group` con `$first` y `$last`; la resta
  (`last - first`) se hace en el usecase, no en el pipeline. Solo tiene
  sentido para métricas monotónicas — ver "Forks cerrados" (fork 5) para la
  advertencia sobre contadores que resetean.
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
  Incluye los handlers de catálogo y batch.
- **batch**: unit tests de `app/dashboards` cubriendo fallo parcial (un item
  falla, los demás no), `TOO_MANY_QUERIES`, `id` duplicado, y que el orden de
  `results` no depende del orden de finalización de los goroutines
  (correlacionar por `id`, no por índice).
- **rate limit**: unit test del middleware nuevo (umbral, clave por
  `supabase_user_id`, fail-open sin Redis) más un integration test que
  verifica el 429 al superar 15 req/min, mismo patrón que
  `internal/consumers/middleware/middleware_test.go`.

## Forks cerrados (2026-09-10)

Seis decisiones abiertas al momento de escribir el spec original, cerradas
en sesión de brainstorming con datos de dominio aportados por el equipo.

**1. Endpoint de catálogo de `aasPath`: entra en v1.**
Elegido: `GET /api/v1/dashboards/metrics/catalog` (ver Contrato), devolviendo
`aasPath` observados vía `distinct` sobre `measurements`, no un registro
declarado a mano. Descartado: posponerlo a después de v1 — se descarta porque
el Edge todavía no cerró los nombres de `aasPath` (no es solo variación entre
máquinas), así que el frontend no puede hardcodearlos sin quedar
desincronizado. Revisar cuando: el Edge publique un esquema/registro
declarativo de métricas — ahí el catálogo podría enriquecerse con metadata
(unidad, tipo) sin romper el contrato de descubrimiento en sí.

**2. Dependencia de `payload.value` escalar: se declara y se instrumenta, no
se fuerza.**
Elegido: documentar la asunción como riesgo conocido (payload no se valida en
ingesta, D-8) e instrumentar con métrica Prometheus
`dashboard_metrics_non_numeric_discarded_total{tenant,aasPath,agg}`
incrementada cuando `$isNumber` descarta un valor en avg/sum/min/max/delta.
Descartado: validar/normalizar `payload.value` en la ingesta — fuera de
alcance de este spec (tocaría el contrato frozen de
`docs/superpowers/plans/2026-08-05-cloud-ingest-endpoint.md`) y no hay
evidencia hoy de que el Edge mande valores no escalares. Revisar cuando: la
métrica muestre volumen sostenido no-cero.

**3. Modelo de consistencia: append-only, sin necesidad de modelo especial.**
Elegido: consistencia eventual simple — no hace falta versionado ni
invalidación de cache por mutación, porque los measurements son insert-only
en la práctica (dedup por `eventId`, sin updates posteriores). Un evento
tardío por reintento de red es orden de llegada, no mutación, y ya lo
resuelve el `$sort ts:1` de cada pipeline. Descartado: diseñar contra
snapshot reads o versionado de documentos — no hay mutación real que
justifique esa complejidad. Revisar cuando: exista un flujo real de
corrección/reproceso retroactivo sobre measurements ya insertados.

**4. Cache + rate limit + pipelines.**
Elegido (cache): ninguno dedicado en v1 — Mongo con `ix_tenant_machine_path_ts`
responde directo sin necesidad de otra capa de estado que mantener
consistente.
Elegido (rate limit): sí se suma — middleware Redis-backed, mismo patrón que
`internal/consumers/ratelimit.go`, con clave `supabase_user_id` (no por
tenant, para que un usuario individual no consuma el balde de todo el
tenant) y umbral de **15 req/min por usuario**, fail-open sin Redis (mismo
nil-safety que el resto de la app). Descartado: rate limit por tenant — un
usuario individual podría agotarlo y afectar a sus compañeros de tenant sin
haber hecho nada mal. Elegido (pipelines): se confirma "N pipelines" (uno
por `MetricSpec`, concurrentes vía `errgroup`) tal como estaba en el spec
original — el volumen bajo no justifica combinar en un único pipeline con
`$facet`.

*Replanteo (2026-09-11, tras review de PR)*: el cierre original justificaba
"sin cache" con "consulta on-demand (no polling)", pero el frontend real
**sí** pollea por widget (10-60s, hasta 50 widgets/dashboard) — con eso, "1
request por widget" + "15 req/min" + "sin cache" no cerraban entre sí (un
dashboard de 20 widgets a 60s ya supera el umbral, y el first-paint de un
dashboard grande dispara un burst que lo revienta de entrada). La resolución
no fue subir el umbral ni agregar cache, sino sacar el polling automático de
v1 (ver "v1 — sin polling automático" en Contrato): con carga manual + un
endpoint batch por dashboard (ver Contrato), la carga real por usuario vuelve
a ser un puñado de requests, y **15 req/min sigue siendo el número correcto**
— solo cambió el motivo por el que alcanza. `dataAsOf` queda como el gancho
para que v2 decida un modelo de refresh vivo con datos reales de producción.
Revisar cuando: se mida en prod la cadencia real del Edge por `aasPath` y se
decida el modelo de refresh de v2 (poll a nivel dashboard o SSE) — ahí sí
recalcular el umbral contra el nuevo patrón de tráfico.

**5. Fragilidad semántica de `count`/`delta` para contadores: riesgo
documentado, sin mitigación activa.**
Elegido: dejar `delta` documentado como válido solo para métricas
monotónicas conocidas (ver nota en "Traducción a pipeline de Mongo"), sin
implementar detección de reset. Descartado: agregar lógica de detección de
reset (valores decrecientes) o exigir un id de "epoch" del contador — no hay
ningún `aasPath` contador acumulativo hoy, así que sería resolver un
problema que no existe todavía. Revisar cuando: el Edge emita un `aasPath`
tipo contador que pueda resetear (reinicio de máquina, cambio de turno,
etc.) dentro de una ventana consultable.

**6. Request-time vs. rollup vs. time-series collection + retención.**
Elegido: 100% request-time sobre la colección `measurements` existente, sin
rollups ni colección de series de tiempo separada, y sin TTL index en v1.
Retención de measurements crudos: semanas/meses según necesidad operativa,
sin política de borrado automático todavía. Descartado: introducir rollups
pre-agregados o una colección de series de tiempo desde v1 — volumen bajo
(decenas/cientos de eventos por hora por `aasPath`) y ventanas de consulta
cortas (horas/días) hacen que sea complejidad sin beneficio medible hoy.
Revisar cuando: el volumen por máquina o el horizonte de consulta pedido por
el frontend crezcan lo suficiente como para que un `$group` directo empiece
a ser lento.

## Ajustes de contrato cerrados (C2, C5) — 2026-09-11

El review de PR marcó varios ajustes de contrato como "van directo al
plan" (C1, C3, C4, C7, C8 — checklist de implementación, no decisiones de
diseño), pero dos los marcó como **forma de contrato**: caros de cambiar
después de v1 porque no son aditivos limpios una vez que el frontend ya
integró. Se cierran acá, junto con los forks.

**C2 — Rangos relativos (`range`) además de `from`/`to` absolutos.**
Elegido: sumar `range` (enum `"15m"|"30m"|"1h"|"8h"|"24h"|"7d"|"30d"`) como
alternativa a `from`/`to`, resuelto a timestamps absolutos con el reloj del
servidor en el momento de ejecutar la consulta (ver "Selección de ventana
temporal" en Contrato). Descartado: dejar `from`/`to` como única forma de
pedir ventana temporal — con eso el cliente recalcula timestamps en cada
carga/refresh, lo que ata el desajuste de reloj cliente/servidor al
resultado y hace que la misma consulta lógica ("últimas 8hs") nunca tenga
la misma forma de request dos veces — precisamente lo que impediría que una
cache futura (Fork 4) dedupe por cache key. Descartado también: reemplazar
`from`/`to` por completo — un selector de fecha custom en el frontend
necesita rango absoluto arbitrario, que no entra en un enum finito.
Revisar cuando: el frontend necesite una ventana relativa que no está en el
enum — sumar valores es aditivo, no requiere este brainstorming de nuevo.

**C5 — Discriminador `mode` explícito en la respuesta.**
Elegido: agregar `mode: "scalar"|"series"|"raw"|"grouped"` a las 4 formas de
respuesta (ver Response). Descartado: dejar que el frontend infiera el modo
mirando qué claves están presentes (`results` vs. `series` vs. `points` vs.
`groups`) — funciona, pero da un tipo generado desde OpenAPI sucio (4 shapes
que se solapan en vez de un discriminated union limpio) y ata al frontend a
inspeccionar la forma del payload en vez de leer un campo. El costo de
agregarlo ahora es una línea por response; agregarlo después de que el
frontend ya escribió lógica de detección estructural sería un cambio
breaking de facto en la práctica, aunque el campo en sí sea aditivo.

## TODOs para el plan de implementación (no bloquean)

Del mismo review de PR: ítems de checklist, no decisiones de diseño — no
necesitan brainstorming, pero tienen que quedar resueltos (código o
justificación explícita de "no ahora") antes de cerrar la implementación.

- **C1 — Timezone/turnos.** Buckets `1d`/`6h` alineados a UTC parten el
  turno noche de la planta (UTC−3). Decidir: sumar parámetro `timezone`
  (`$dateTrunc` lo soporta) o declarar explícitamente "v1 es UTC-only".
  Además, el ejemplo de `groupBy` "promedio de temperatura por turno" en
  este spec no se puede expresar hoy — `turno` no es un campo de `payload`.
  Sacar el ejemplo o explicar cómo se derivaría.
- **C3 — `raw`: downsampling en vez de hard-reject.** El tope de 5000 puntos
  rechaza, por ejemplo, 20 minutos a >4Hz. Evaluar `maxPoints` con
  decimación server-side (LTTB / min-max) en vez de 400 `RANGE_TOO_WIDE`
  directo.
- **C4 — `maxTimeMS` en la agregación.** `RANGE_TOO_WIDE` es una estimación
  pre-flight (buckets/puntos esperados); no frena un `$match` que escanea
  millones de documentos antes de llegar a agrupar. Sumar un techo duro de
  tiempo de ejecución en el pipeline de Mongo.
- **C7 — Fallo parcial en el `errgroup` del query single.** Si de 10
  `MetricSpec` en un mismo `MetricQueryRequest` fallan 3, especificar: ¿500
  total o respuesta parcial con los 7 que sí resolvieron? (Distinto del
  fallo parcial del endpoint batch, que ya quedó definido arriba.)
- **C8 — Guardrails como config, no constantes.** Los topes de 1000 buckets
  / 5000 puntos / 200 grupos / 50 queries del batch están hardcodeados en el
  spec sin un presupuesto de latencia que los justifique. Moverlos a config
  (env o tabla) y separar `RANGE_TOO_WIDE` en dos códigos según la causa
  (demasiados buckets vs. demasiados puntos crudos), en vez de un único code
  mezclando ambas.
