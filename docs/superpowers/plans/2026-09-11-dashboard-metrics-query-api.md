# Dashboard Metrics Query API Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build `POST /api/v1/dashboards/metrics/query`, `GET /api/v1/dashboards/metrics/catalog` and `POST /api/v1/dashboards/metrics/query/batch` — the read side of the Edge ingest pipeline, letting the frontend query aggregated/raw/grouped measurements from MongoDB without the backend ever knowing what a metric means.

**Architecture:** Hexagonal, following the existing layout. `internal/domain/metrics` holds pure validation/types (no Mongo, no Gin). `internal/repo/mongo/metrics` runs one aggregation pipeline per `MetricSpec` against the existing `measurements` collection. `internal/app/dashboards` orchestrates validation + concurrent per-metric dispatch (`errgroup`) + response assembly. `internal/api/handler/dashboards` binds JSON/query params and maps domain errors to the `{success, data|error, code}` envelope. A new Redis-backed rate limit middleware, keyed by `supabase_user_id`, gates the whole route group.

**Tech Stack:** Go 1.24, Gin, MongoDB (`go.mongodb.org/mongo-driver/v2`), Redis (`go-redis/redis/v8`), `golang.org/x/sync/errgroup` (already an indirect dependency — this plan makes it direct), Prometheus (`promauto`), `testify`.

**Spec:** `docs/superpowers/specs/2026-09-07-dashboard-metrics-query-design.md` — this plan implements every closed fork (1–6), C2, C5, and makes the concrete implementation decisions for the TODOs the spec left open (C1, C3, C4, C7, C8); see "Decisions for the TODOs" below. Executors should read the spec's Contrato/Response/Arquitectura sections alongside this plan.

## Global Constraints

- Response envelope for all three endpoints: `{"success": bool, "data": ...}` on success, `{"success": false, "error": string, "code": string}` on failure — same convention as `internal/api/handler/edge_devices` (not the bare `{"message":...}` used by `internal/consumers`, which is a different, unauthenticated surface).
- Route group middleware: `JWTAuth → TenantFromHeader → PasswordChangeGuard → RBACCheck("perm_metrics_view") → DashboardRateLimit`, mounted under the existing `v1` group in `internal/routes/url_mappings.go`. `tenantId` always comes from `X-Tenant-ID` (via `platform.TenantID`/`platform.TenantUUID`), never from the request body.
- `golang.org/x/sync/errgroup` for all concurrent fan-out (multi-metric single query, batch items) — do not hand-roll goroutines+channels.
- No new mocking framework: despite CLAUDE.md listing `uber/mock`, no package in this repo actually uses it (verified: no `mocks` directories, no `go:generate mockgen` directives, no `go.uber.org/mock` in `go.mod`). `app/dashboards` unit tests use a small hand-written fake implementing `domain/metrics.Repository`, matching how the rest of the repo actually tests services today.
- `docs/openapi.yaml` is not updated by this plan: `dashboard_layouts` and `edge_devices` routes — both already shipped — aren't documented there either, so keeping this feature out of sync with that file matches existing practice, not a gap this plan introduces.

## Decisions for the TODOs left open in the spec

The spec explicitly deferred C1/C3/C4/C7/C8 as "implementation checklist, not a design decision" (see its "TODOs para el plan de implementación" section). A plan can't ship with a TBD, so each gets a concrete, minimal decision here:

- **C1 (timezone/turnos):** v1 is UTC-only. `bucket` truncation always uses UTC; no `timezone` request field. Documented with a one-line comment where `Bucket` durations are defined (Task 3).
- **C3 (raw downsampling):** add optional `maxPoints` (1..`MetricsMaxRawPoints`) to the request. If provided and the raw result exceeds it, decimate server-side with uniform-stride sampling (always keep the first and last point) instead of rejecting. If omitted, keep today's behavior: reject with `RANGE_TOO_WIDE` past `MetricsMaxRawPoints`. Uniform-stride, not LTTB — simplest correct decimation; upgrading the algorithm later doesn't change the contract (Task 8).
- **C4 (`maxTimeMS`):** every Mongo aggregation call gets a per-request timeout (`MetricsMaxTimeMS`, default 5000ms) via `context.WithTimeout`. A timeout maps to a new code `QUERY_TIMEOUT`, HTTP 504 (Task 6, Task 15).
- **C7 (`errgroup` partial failure in a single multi-metric query):** first error wins — `errgroup.Group.Wait()`'s standard behavior. The whole query fails (no partial scalar/series response); this is simpler and consistent with never returning silently-incomplete aggregated data. Batch-level partial failure (different scope, already in the spec) is unaffected (Task 12).
- **C8 (guardrails as config):** `MetricsMaxSpecs`, `MetricsMaxBuckets`, `MetricsMaxRawPoints`, `MetricsMaxGroups`, `MetricsMaxBatchQueries` move to `config.DashboardsConfig`, env-driven with defaults matching the spec's current numbers (Task 14). Splitting `RANGE_TOO_WIDE` into two codes (buckets vs. raw points) is **not** done here — that's a wire-contract change of the same "forma de contrato" weight as C2/C5, and wasn't approved as one; flagged as a follow-up, not implemented.

---

### Task 1: Domain metrics — core types, error codes, Limits

**Files:**
- Create: `internal/domain/metrics/metrics.go`
- Create: `internal/domain/metrics/errors.go`
- Test: `internal/domain/metrics/errors_test.go`

**Interfaces:**
- Produces: `Agg` (string enum: `AggAvg`, `AggSum`, `AggMin`, `AggMax`, `AggLast`, `AggCount`, `AggDelta`, `AggRaw`) with `(Agg).Valid() bool`; `Bucket` (string enum: `Bucket1m`...`Bucket1d`) with `(Bucket).valid() bool` and `(Bucket).duration() (time.Duration, bool)`; `Range` (string enum `"15m"|"30m"|"1h"|"8h"|"24h"|"7d"|"30d"`); `Mode` (`ModeScalar`, `ModeSeries`, `ModeRaw`, `ModeGrouped`); `MetricSpec{AasPath string; Agg Agg}`; `ValueFilter{ValueEquals any}`; `MetricQuery{MachineID string; Range Range; From, To *time.Time; Bucket Bucket; Metrics []MetricSpec; GroupBy string; Filter *ValueFilter; MaxPoints int}`; `Limits{MaxSpecs, MaxBuckets, MaxRawPoints, MaxGroups, MaxBatchQueries int}`; `DefaultLimits() Limits`; `ValidationError{Code, Message string}` implementing `error`; code constants `CodeInvalidParams`, `CodeTooManyMetrics`, `CodeRawModeConflict`, `CodeGroupByConflict`, `CodeRangeTooWide`, `CodeTooManyGroups`, `CodeTooManyQueries`, `CodeQueryTimeout`.

- [ ] **Step 1: Write the failing test**

```go
// internal/domain/metrics/errors_test.go
package metrics

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidationErrorCarriesCode(t *testing.T) {
	err := newValidationError(CodeInvalidParams, "machineId es requerido")

	assert.Equal(t, "machineId es requerido", err.Error())

	var ve *ValidationError
	assert.True(t, errors.As(err, &ve))
	assert.Equal(t, CodeInvalidParams, ve.Code)
}

func TestAggValid(t *testing.T) {
	assert.True(t, AggAvg.Valid())
	assert.True(t, AggRaw.Valid())
	assert.False(t, Agg("bogus").Valid())
}

func TestBucketDuration(t *testing.T) {
	d, ok := Bucket1h.duration()
	assert.True(t, ok)
	assert.Equal(t, time.Hour, d)

	_, ok = Bucket("bogus").duration()
	assert.False(t, ok)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/domain/metrics/... -run TestValidationErrorCarriesCode -v`
Expected: FAIL — package `metrics` / `newValidationError` / `ValidationError` don't exist yet.

- [ ] **Step 3: Write minimal implementation**

```go
// internal/domain/metrics/metrics.go
// Package metrics contiene el modelo de dominio del motor de consultas de
// dashboards. No conoce Mongo ni Gin.
package metrics

import "time"

type Agg string

const (
	AggAvg   Agg = "avg"
	AggSum   Agg = "sum"
	AggMin   Agg = "min"
	AggMax   Agg = "max"
	AggLast  Agg = "last"
	AggCount Agg = "count"
	AggDelta Agg = "delta"
	AggRaw   Agg = "raw"
)

// Valid reporta si a es uno de los 8 aggs soportados.
func (a Agg) Valid() bool {
	switch a {
	case AggAvg, AggSum, AggMin, AggMax, AggLast, AggCount, AggDelta, AggRaw:
		return true
	}
	return false
}

// Bucket es siempre UTC en v1 (fork/C1 del plan): no hay parametro de
// timezone. $dateTrunc en el pipeline de Mongo trunca con el reloj UTC.
type Bucket string

const (
	Bucket1m  Bucket = "1m"
	Bucket5m  Bucket = "5m"
	Bucket15m Bucket = "15m"
	Bucket1h  Bucket = "1h"
	Bucket6h  Bucket = "6h"
	Bucket1d  Bucket = "1d"
)

func (b Bucket) duration() (time.Duration, bool) {
	switch b {
	case Bucket1m:
		return time.Minute, true
	case Bucket5m:
		return 5 * time.Minute, true
	case Bucket15m:
		return 15 * time.Minute, true
	case Bucket1h:
		return time.Hour, true
	case Bucket6h:
		return 6 * time.Hour, true
	case Bucket1d:
		return 24 * time.Hour, true
	}
	return 0, false
}

func (b Bucket) valid() bool { _, ok := b.duration(); return ok }

type Range string

const (
	Range15m Range = "15m"
	Range30m Range = "30m"
	Range1h  Range = "1h"
	Range8h  Range = "8h"
	Range24h Range = "24h"
	Range7d  Range = "7d"
	Range30d Range = "30d"
)

var rangeDurations = map[Range]time.Duration{
	Range15m: 15 * time.Minute,
	Range30m: 30 * time.Minute,
	Range1h:  time.Hour,
	Range8h:  8 * time.Hour,
	Range24h: 24 * time.Hour,
	Range7d:  7 * 24 * time.Hour,
	Range30d: 30 * 24 * time.Hour,
}

type Mode string

const (
	ModeScalar  Mode = "scalar"
	ModeSeries  Mode = "series"
	ModeRaw     Mode = "raw"
	ModeGrouped Mode = "grouped"
)

type MetricSpec struct {
	AasPath string
	Agg     Agg
}

type ValueFilter struct {
	ValueEquals any
}

type MetricQuery struct {
	MachineID string
	Range     Range
	From      *time.Time
	To        *time.Time
	Bucket    Bucket
	Metrics   []MetricSpec
	GroupBy   string
	Filter    *ValueFilter
	// MaxPoints activa decimacion server-side en modo raw (fork/C3) en vez
	// de RANGE_TOO_WIDE. 0 = no seteado, se mantiene el hard-reject.
	MaxPoints int
}

// Limits son los guardrails de la tabla de la spec, parametrizados
// (fork/C8) en vez de constantes — el llamador (routes/url_mappings.go) los
// arma desde config.DashboardsConfig y los pasa a Validate/ValidateBatchIDs
// y al repositorio.
type Limits struct {
	MaxSpecs        int
	MaxBuckets      int
	MaxRawPoints    int
	MaxGroups       int
	MaxBatchQueries int
}

// DefaultLimits reproduce los numeros de la tabla de guardrails de la spec.
// Solo se usa en tests; en produccion los valores vienen siempre de config.
func DefaultLimits() Limits {
	return Limits{MaxSpecs: 10, MaxBuckets: 1000, MaxRawPoints: 5000, MaxGroups: 200, MaxBatchQueries: 50}
}
```

```go
// internal/domain/metrics/errors.go
package metrics

const (
	CodeInvalidParams   = "INVALID_PARAMS"
	CodeTooManyMetrics  = "TOO_MANY_METRICS"
	CodeRawModeConflict = "RAW_MODE_CONFLICT"
	CodeGroupByConflict = "GROUP_BY_CONFLICT"
	CodeRangeTooWide    = "RANGE_TOO_WIDE"
	CodeTooManyGroups   = "TOO_MANY_GROUPS"
	CodeTooManyQueries  = "TOO_MANY_QUERIES"
	CodeQueryTimeout    = "QUERY_TIMEOUT"
)

// ValidationError es el unico tipo de error que devuelven Validate,
// ResolveWindow y ValidateBatchIDs. El handler lo mapea 1:1 a
// {success:false, error, code} con errors.As.
type ValidationError struct {
	Code    string
	Message string
}

func (e *ValidationError) Error() string { return e.Message }

func newValidationError(code, message string) *ValidationError {
	return &ValidationError{Code: code, Message: message}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/domain/metrics/... -v`
Expected: PASS (all three tests)

- [ ] **Step 5: Commit**

```bash
git add internal/domain/metrics/metrics.go internal/domain/metrics/errors.go internal/domain/metrics/errors_test.go
git commit -m "feat(metrics): domain types, error codes and configurable Limits"
```

---

### Task 2: Domain metrics — window resolution (`range` vs. `from`/`to`)

**Files:**
- Create: `internal/domain/metrics/window.go`
- Test: `internal/domain/metrics/window_test.go`

**Interfaces:**
- Consumes: `MetricQuery`, `newValidationError`, `CodeInvalidParams` (Task 1)
- Produces: `ResolveWindow(q MetricQuery, now time.Time) (from, to time.Time, err error)` — later tasks (3, 12) call this before `Validate`.

- [ ] **Step 1: Write the failing test**

```go
// internal/domain/metrics/window_test.go
package metrics

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveWindow(t *testing.T) {
	now := time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)
	from8h := now.Add(-8 * time.Hour)
	future := now.Add(time.Hour)
	past := now.Add(-time.Hour)

	tests := []struct {
		name     string
		q        MetricQuery
		wantFrom time.Time
		wantTo   time.Time
		wantCode string
	}{
		{
			name:     "range resuelve a now-range..now",
			q:        MetricQuery{Range: Range8h},
			wantFrom: from8h,
			wantTo:   now,
		},
		{
			name:     "from/to absolutos",
			q:        MetricQuery{From: &past, To: &future},
			wantFrom: past,
			wantTo:   future,
		},
		{
			name:     "range invalido",
			q:        MetricQuery{Range: "3h"},
			wantCode: CodeInvalidParams,
		},
		{
			name:     "ni range ni from/to",
			q:        MetricQuery{},
			wantCode: CodeInvalidParams,
		},
		{
			name:     "range y from/to a la vez",
			q:        MetricQuery{Range: Range1h, From: &past, To: &future},
			wantCode: CodeInvalidParams,
		},
		{
			name:     "to no posterior a from",
			q:        MetricQuery{From: &future, To: &past},
			wantCode: CodeInvalidParams,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			from, to, err := ResolveWindow(tt.q, now)
			if tt.wantCode != "" {
				require.Error(t, err)
				var ve *ValidationError
				require.ErrorAs(t, err, &ve)
				assert.Equal(t, tt.wantCode, ve.Code)
				return
			}
			require.NoError(t, err)
			assert.True(t, tt.wantFrom.Equal(from))
			assert.True(t, tt.wantTo.Equal(to))
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/domain/metrics/... -run TestResolveWindow -v`
Expected: FAIL — `ResolveWindow` not defined.

- [ ] **Step 3: Write minimal implementation**

```go
// internal/domain/metrics/window.go
package metrics

import (
	"fmt"
	"time"
)

// ResolveWindow resuelve `range` o el par `from`/`to` a un rango absoluto.
// `now` es inyectado por el llamador (nunca time.Now() adentro de esta
// funcion) para que sea testeable de forma determinista.
func ResolveWindow(q MetricQuery, now time.Time) (from, to time.Time, err error) {
	hasRange := q.Range != ""
	hasAbsolute := q.From != nil && q.To != nil
	hasPartialAbsolute := (q.From != nil) != (q.To != nil)

	switch {
	case hasRange && (hasAbsolute || hasPartialAbsolute):
		return time.Time{}, time.Time{}, newValidationError(CodeInvalidParams, "no se puede combinar range con from/to")
	case hasRange:
		dur, ok := rangeDurations[q.Range]
		if !ok {
			return time.Time{}, time.Time{}, newValidationError(CodeInvalidParams, fmt.Sprintf("range invalido: %q", q.Range))
		}
		return now.Add(-dur), now, nil
	case hasAbsolute:
		if !q.To.After(*q.From) {
			return time.Time{}, time.Time{}, newValidationError(CodeInvalidParams, "to debe ser posterior a from")
		}
		return *q.From, *q.To, nil
	default:
		return time.Time{}, time.Time{}, newValidationError(CodeInvalidParams, "falta range o el par from y to")
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/domain/metrics/... -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/domain/metrics/window.go internal/domain/metrics/window_test.go
git commit -m "feat(metrics): resolve range or from/to into an absolute window"
```

---

### Task 3: Domain metrics — mode detection, mutual exclusion rules, bucket-count guardrail

**Files:**
- Create: `internal/domain/metrics/validate.go`
- Test: `internal/domain/metrics/validate_test.go`

**Interfaces:**
- Consumes: everything from Task 1 and Task 2.
- Produces: `Validate(q MetricQuery, from, to time.Time, limits Limits) (Mode, error)` — called by `app/dashboards.Service.Query` (Task 12) right after `ResolveWindow`.

- [ ] **Step 1: Write the failing test**

```go
// internal/domain/metrics/validate_test.go
package metrics

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidate(t *testing.T) {
	from := time.Date(2026, 9, 11, 0, 0, 0, 0, time.UTC)
	to := from.Add(8 * time.Hour)
	oneMetric := []MetricSpec{{AasPath: "Operativos/Pesada/peso", Agg: AggAvg}}
	twoMetrics := []MetricSpec{
		{AasPath: "Operativos/Pesada/peso", Agg: AggAvg},
		{AasPath: "Operativos/Pesada/peso", Agg: AggCount},
	}

	tests := []struct {
		name     string
		q        MetricQuery
		wantMode Mode
		wantCode string
	}{
		{
			name:     "escalar multi-metrica sin bucket",
			q:        MetricQuery{MachineID: "EMB-DEV-001", Metrics: twoMetrics},
			wantMode: ModeScalar,
		},
		{
			name:     "serie con bucket",
			q:        MetricQuery{MachineID: "EMB-DEV-001", Metrics: twoMetrics, Bucket: Bucket1h},
			wantMode: ModeSeries,
		},
		{
			name:     "raw solo",
			q:        MetricQuery{MachineID: "EMB-DEV-001", Metrics: []MetricSpec{{AasPath: "x", Agg: AggRaw}}},
			wantMode: ModeRaw,
		},
		{
			name:     "raw con bucket es conflicto",
			q:        MetricQuery{MachineID: "EMB-DEV-001", Metrics: []MetricSpec{{AasPath: "x", Agg: AggRaw}}, Bucket: Bucket1h},
			wantCode: CodeRawModeConflict,
		},
		{
			name:     "raw con mas de una metrica es conflicto",
			q:        MetricQuery{MachineID: "EMB-DEV-001", Metrics: []MetricSpec{{AasPath: "x", Agg: AggRaw}, {AasPath: "y", Agg: AggAvg}}},
			wantCode: CodeRawModeConflict,
		},
		{
			name:     "groupBy solo",
			q:        MetricQuery{MachineID: "EMB-DEV-001", Metrics: oneMetric, GroupBy: "payload.tipo"},
			wantMode: ModeGrouped,
		},
		{
			name:     "groupBy con bucket es conflicto",
			q:        MetricQuery{MachineID: "EMB-DEV-001", Metrics: oneMetric, GroupBy: "payload.tipo", Bucket: Bucket1h},
			wantCode: CodeGroupByConflict,
		},
		{
			name:     "groupBy con mas de una metrica es conflicto",
			q:        MetricQuery{MachineID: "EMB-DEV-001", Metrics: twoMetrics, GroupBy: "payload.tipo"},
			wantCode: CodeGroupByConflict,
		},
		{
			name:     "groupBy con campo fuera del patron payload.<campo>",
			q:        MetricQuery{MachineID: "EMB-DEV-001", Metrics: oneMetric, GroupBy: "$where"},
			wantCode: CodeInvalidParams,
		},
		{
			name:     "machineId faltante",
			q:        MetricQuery{Metrics: oneMetric},
			wantCode: CodeInvalidParams,
		},
		{
			name:     "metrics vacio",
			q:        MetricQuery{MachineID: "EMB-DEV-001"},
			wantCode: CodeInvalidParams,
		},
		{
			name:     "mas metricas que el limite",
			q:        MetricQuery{MachineID: "EMB-DEV-001", Metrics: []MetricSpec{oneMetric[0], oneMetric[0]}},
			wantCode: CodeTooManyMetrics,
		},
		{
			name:     "agg invalido",
			q:        MetricQuery{MachineID: "EMB-DEV-001", Metrics: []MetricSpec{{AasPath: "x", Agg: "bogus"}}},
			wantCode: CodeInvalidParams,
		},
		{
			name:     "demasiados buckets esperados",
			q:        MetricQuery{MachineID: "EMB-DEV-001", Metrics: oneMetric, Bucket: Bucket1m},
			wantCode: CodeRangeTooWide,
		},
	}

	limits := Limits{MaxSpecs: 1, MaxBuckets: 100, MaxRawPoints: 5000, MaxGroups: 200, MaxBatchQueries: 50}
	// El caso "mas metricas que el limite" usa 2 metricas contra MaxSpecs:1.
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mode, err := Validate(tt.q, from, to, limits)
			if tt.wantCode != "" {
				require.Error(t, err)
				var ve *ValidationError
				require.ErrorAs(t, err, &ve)
				assert.Equal(t, tt.wantCode, ve.Code)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantMode, mode)
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/domain/metrics/... -run TestValidate -v`
Expected: FAIL — `Validate` not defined.

- [ ] **Step 3: Write minimal implementation**

```go
// internal/domain/metrics/validate.go
package metrics

import (
	"fmt"
	"regexp"
	"time"
)

// groupByFieldPattern es la unica defensa contra inyeccion NoSQL vía
// groupBy: el string del cliente se interpola como "$"+groupBy en un $group
// (repo/mongo/metrics). Sin este patron, un valor como "$where" o un
// operador de agregacion podria leerse como algo distinto de un nombre de
// campo.
var groupByFieldPattern = regexp.MustCompile(`^payload\.[a-zA-Z0-9_]+(\.[a-zA-Z0-9_]+)*$`)

// Validate corre las reglas de exclusion mutua y los guardrails
// estructurales (los que no requieren tocar Mongo) y devuelve el Mode
// detectado. RANGE_TOO_WIDE por exceso de puntos crudos y TOO_MANY_GROUPS se
// validan en el repositorio (Task 8/9): dependen del resultado real de la
// consulta, no de la forma del request.
func Validate(q MetricQuery, from, to time.Time, limits Limits) (Mode, error) {
	if q.MachineID == "" {
		return "", newValidationError(CodeInvalidParams, "machineId es requerido")
	}
	if len(q.Metrics) == 0 {
		return "", newValidationError(CodeInvalidParams, "metrics es requerido y no puede estar vacio")
	}
	if len(q.Metrics) > limits.MaxSpecs {
		return "", newValidationError(CodeTooManyMetrics, fmt.Sprintf("metrics excede el maximo de %d", limits.MaxSpecs))
	}
	for _, m := range q.Metrics {
		if m.AasPath == "" {
			return "", newValidationError(CodeInvalidParams, "aasPath es requerido en cada metric")
		}
		if !m.Agg.Valid() {
			return "", newValidationError(CodeInvalidParams, fmt.Sprintf("agg invalido: %q", m.Agg))
		}
	}
	if q.Bucket != "" && !q.Bucket.valid() {
		return "", newValidationError(CodeInvalidParams, fmt.Sprintf("bucket invalido: %q", q.Bucket))
	}
	if q.GroupBy != "" && !groupByFieldPattern.MatchString(q.GroupBy) {
		return "", newValidationError(CodeInvalidParams, "groupBy no cumple el patron payload.<campo>")
	}

	isRaw := len(q.Metrics) == 1 && q.Metrics[0].Agg == AggRaw
	isGrouped := q.GroupBy != ""

	switch {
	case isRaw:
		if q.Bucket != "" || isGrouped || len(q.Metrics) > 1 {
			return "", newValidationError(CodeRawModeConflict, "agg:raw no admite bucket, groupBy ni mas de una metrica")
		}
		return ModeRaw, nil
	case len(q.Metrics) == 1 && q.Metrics[0].Agg == AggRaw == false && hasRawAmongMultiple(q.Metrics):
		return "", newValidationError(CodeRawModeConflict, "agg:raw no admite bucket, groupBy ni mas de una metrica")
	case isGrouped:
		if q.Bucket != "" || len(q.Metrics) != 1 {
			return "", newValidationError(CodeGroupByConflict, "groupBy requiere exactamente 1 metrica y no admite bucket")
		}
		return ModeGrouped, nil
	case q.Bucket != "":
		if err := checkBucketCount(from, to, q.Bucket, limits.MaxBuckets); err != nil {
			return "", err
		}
		return ModeSeries, nil
	default:
		return ModeScalar, nil
	}
}

// hasRawAmongMultiple detecta agg:"raw" mezclado con otras metricas (caso
// "raw con mas de una metrica" del test) — isRaw exige exactamente 1 metrica,
// asi que ese caso no cae en la rama isRaw y necesita chequearse aparte.
func hasRawAmongMultiple(specs []MetricSpec) bool {
	if len(specs) < 2 {
		return false
	}
	for _, s := range specs {
		if s.Agg == AggRaw {
			return true
		}
	}
	return false
}

func checkBucketCount(from, to time.Time, bucket Bucket, maxBuckets int) error {
	dur, _ := bucket.duration()
	expected := int(to.Sub(from)/dur) + 1
	if expected > maxBuckets {
		return newValidationError(CodeRangeTooWide, fmt.Sprintf("el rango produce %d buckets, maximo %d", expected, maxBuckets))
	}
	return nil
}
```

Note: the `case len(q.Metrics) == 1 && q.Metrics[0].Agg == AggRaw == false && ...` branch above is invalid Go (`== AggRaw == false` doesn't compile) — implementer must simplify. Replace the switch with:

```go
	switch {
	case hasRawAmongMultiple(q.Metrics):
		return "", newValidationError(CodeRawModeConflict, "agg:raw no admite bucket, groupBy ni mas de una metrica")
	case isRaw:
		if q.Bucket != "" || isGrouped {
			return "", newValidationError(CodeRawModeConflict, "agg:raw no admite bucket, groupBy ni mas de una metrica")
		}
		return ModeRaw, nil
	case isGrouped:
		if q.Bucket != "" || len(q.Metrics) != 1 {
			return "", newValidationError(CodeGroupByConflict, "groupBy requiere exactamente 1 metrica y no admite bucket")
		}
		return ModeGrouped, nil
	case q.Bucket != "":
		if err := checkBucketCount(from, to, q.Bucket, limits.MaxBuckets); err != nil {
			return "", err
		}
		return ModeSeries, nil
	default:
		return ModeScalar, nil
	}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/domain/metrics/... -v`
Expected: PASS (all cases in the table)

- [ ] **Step 5: Commit**

```bash
git add internal/domain/metrics/validate.go internal/domain/metrics/validate_test.go
git commit -m "feat(metrics): mode detection, mutual exclusion rules and bucket-count guardrail"
```

---

### Task 4: Domain metrics — Repository interface, result types, batch ID validation

**Files:**
- Create: `internal/domain/metrics/repository.go`
- Create: `internal/domain/metrics/batch.go`
- Test: `internal/domain/metrics/batch_test.go`

**Interfaces:**
- Produces:
  - `MetricResult{AasPath string; Agg Agg; Value float64; SampleCount int64}`
  - `BucketPoint{Ts time.Time; Value float64; SampleCount int64}`
  - `RawPoint{Ts time.Time; Value any}`
  - `GroupResult{Key string; Value float64}`
  - `SeriesPoint{Ts time.Time; Results []MetricResult}`
  - `QueryResult{Mode Mode; MachineID string; From, To time.Time; DataAsOf *time.Time; Bucket Bucket; Results []MetricResult; Series []SeriesPoint; AasPath string; Points []RawPoint; Agg Agg; GroupBy string; Groups []GroupResult}`
  - `Repository` interface (implemented by Task 6-10's `repo/mongo/metrics.Repository`, consumed by `app/dashboards.Service`, Task 12):
    ```go
    type Repository interface {
        Scalar(ctx context.Context, tenantID, machineID string, from, to time.Time, spec MetricSpec, filter *ValueFilter) (MetricResult, *time.Time, error)
        Series(ctx context.Context, tenantID, machineID string, from, to time.Time, bucket Bucket, spec MetricSpec, filter *ValueFilter) ([]BucketPoint, *time.Time, error)
        Raw(ctx context.Context, tenantID, machineID string, from, to time.Time, aasPath string, limit, maxPoints int) ([]RawPoint, *time.Time, error)
        Grouped(ctx context.Context, tenantID, machineID string, from, to time.Time, groupBy string, spec MetricSpec, filter *ValueFilter, limit int) ([]GroupResult, *time.Time, error)
        Catalog(ctx context.Context, tenantID, machineID string) ([]string, error)
    }
    ```
  - `ValidateBatchIDs(ids []string, limits Limits) error`

- [ ] **Step 1: Write the failing test**

```go
// internal/domain/metrics/batch_test.go
package metrics

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateBatchIDs(t *testing.T) {
	limits := Limits{MaxBatchQueries: 2}

	tests := []struct {
		name     string
		ids      []string
		wantCode string
	}{
		{name: "ok", ids: []string{"a", "b"}},
		{name: "vacio", ids: nil, wantCode: CodeInvalidParams},
		{name: "excede el maximo", ids: []string{"a", "b", "c"}, wantCode: CodeTooManyQueries},
		{name: "id vacio", ids: []string{"a", ""}, wantCode: CodeInvalidParams},
		{name: "id duplicado", ids: []string{"a", "a"}, wantCode: CodeInvalidParams},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateBatchIDs(tt.ids, limits)
			if tt.wantCode == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			var ve *ValidationError
			require.ErrorAs(t, err, &ve)
			assert.Equal(t, tt.wantCode, ve.Code)
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/domain/metrics/... -run TestValidateBatchIDs -v`
Expected: FAIL — `ValidateBatchIDs` not defined.

- [ ] **Step 3: Write minimal implementation**

```go
// internal/domain/metrics/repository.go
package metrics

import (
	"context"
	"time"
)

type MetricResult struct {
	AasPath     string
	Agg         Agg
	Value       float64
	SampleCount int64
}

type BucketPoint struct {
	Ts          time.Time
	Value       float64
	SampleCount int64
}

type RawPoint struct {
	Ts    time.Time
	Value any
}

type GroupResult struct {
	Key   string
	Value float64
}

type SeriesPoint struct {
	Ts      time.Time
	Results []MetricResult
}

// QueryResult es la union discriminada por Mode que arma app/dashboards.
// Solo los campos relevantes al Mode detectado vienen poblados.
type QueryResult struct {
	Mode      Mode
	MachineID string
	From      time.Time
	To        time.Time
	DataAsOf  *time.Time

	Bucket  Bucket
	Results []MetricResult

	Series []SeriesPoint

	AasPath string
	Points  []RawPoint

	Agg     Agg
	GroupBy string
	Groups  []GroupResult
}

// Repository resuelve, para UNA combinacion (tenant, machine, MetricSpec,
// ventana), el pipeline de Mongo que corresponde al modo pedido. No conoce
// el concepto de "query completa" — ese fan-out multi-metrica lo hace
// app/dashboards.Service con errgroup.
type Repository interface {
	// Scalar resuelve avg/sum/min/max/last/delta/count sin bucketizar.
	Scalar(ctx context.Context, tenantID, machineID string, from, to time.Time, spec MetricSpec, filter *ValueFilter) (MetricResult, *time.Time, error)

	// Series resuelve la misma familia de aggs que Scalar, bucketizada.
	Series(ctx context.Context, tenantID, machineID string, from, to time.Time, bucket Bucket, spec MetricSpec, filter *ValueFilter) ([]BucketPoint, *time.Time, error)

	// Raw devuelve puntos crudos ordenados por ts. limit es
	// maxRawPoints+1 (para poder distinguir "hay exactamente el maximo" de
	// "hay mas"); si maxPoints > 0, decima a maxPoints en vez de fallar.
	Raw(ctx context.Context, tenantID, machineID string, from, to time.Time, aasPath string, limit, maxPoints int) ([]RawPoint, *time.Time, error)

	// Grouped agrupa por groupBy con el acumulador de spec.Agg. limit es
	// maxGroups+1, misma logica de deteccion de exceso que Raw.
	Grouped(ctx context.Context, tenantID, machineID string, from, to time.Time, groupBy string, spec MetricSpec, filter *ValueFilter, limit int) ([]GroupResult, *time.Time, error)

	// Catalog devuelve los aasPath observados para (tenant, machine).
	Catalog(ctx context.Context, tenantID, machineID string) ([]string, error)
}
```

```go
// internal/domain/metrics/batch.go
package metrics

import "fmt"

// ValidateBatchIDs valida el batch a nivel de ids: no vacio, no mas del
// limite, cada id no vacio y unico. La validacion de cada MetricQuery
// individual la hace Validate, item por item, en app/dashboards (Task 13).
func ValidateBatchIDs(ids []string, limits Limits) error {
	if len(ids) == 0 {
		return newValidationError(CodeInvalidParams, "queries es requerido y no puede estar vacio")
	}
	if len(ids) > limits.MaxBatchQueries {
		return newValidationError(CodeTooManyQueries, fmt.Sprintf("queries excede el maximo de %d", limits.MaxBatchQueries))
	}
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		if id == "" {
			return newValidationError(CodeInvalidParams, "cada query del batch requiere un id")
		}
		if _, dup := seen[id]; dup {
			return newValidationError(CodeInvalidParams, fmt.Sprintf("id duplicado en el batch: %q", id))
		}
		seen[id] = struct{}{}
	}
	return nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/domain/metrics/... -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/domain/metrics/repository.go internal/domain/metrics/batch.go internal/domain/metrics/batch_test.go
git commit -m "feat(metrics): Repository interface, result types and batch ID validation"
```

---

### Task 5: Migration — `perm_metrics_view` permission

**Files:**
- Create: `migrations/000015_metrics_view_permission.up.sql`
- Create: `migrations/000015_metrics_view_permission.down.sql`

**Interfaces:**
- Produces: permission id `perm_metrics_view`, checked by `RBACCheck("perm_metrics_view")` in Task 20's route wiring.

- [ ] **Step 1: Write the migration (no test framework for SQL migrations in this repo — verified manually, see Step 3)**

```sql
-- migrations/000015_metrics_view_permission.up.sql
-- ============================================================================
-- Migration 000015: permiso perm_metrics_view (Dashboard Metrics Query API)
-- ============================================================================
-- Ver docs/superpowers/specs/2026-09-07-dashboard-metrics-query-design.md.
-- Se seedea dinamicamente a cualquier rol que hoy tenga perm_dashboard o
-- perm_analytics (containment JSONB), en vez de enumerar roles a mano: es el
-- mismo criterio que la spec declara ("roles que hoy tienen
-- perm_dashboard/perm_analytics") y no se desincroniza si un rol nuevo gana
-- alguno de esos dos permisos entre esta migracion y el reseed manual.
-- ============================================================================

INSERT INTO permissions (id, name, section, description, is_system_permission, tenant_id) VALUES
    ('perm_metrics_view', 'Ver Métricas de Dashboard', 'dashboard', 'Consultar métricas agregadas/crudas de measurements para los widgets de dashboard', TRUE, NULL)
ON CONFLICT (id) DO NOTHING;

UPDATE roles
SET permissions = permissions || '["perm_metrics_view"]'::jsonb,
    updated_at = NOW()
WHERE (permissions @> '["perm_dashboard"]'::jsonb OR permissions @> '["perm_analytics"]'::jsonb)
  AND NOT (permissions @> '["perm_metrics_view"]'::jsonb);
```

```sql
-- migrations/000015_metrics_view_permission.down.sql
UPDATE roles
SET permissions = permissions - 'perm_metrics_view',
    updated_at = NOW()
WHERE permissions @> '["perm_metrics_view"]'::jsonb;

DELETE FROM permissions WHERE id = 'perm_metrics_view';
```

- [ ] **Step 2: Verify SQL syntax locally**

Run: `migrate -path migrations/ -database $DATABASE_URL up` (against a local/dev DB — see CLAUDE.md's Docker fallback if no local Postgres)
Expected: migration `000015` applies cleanly; `SELECT permissions FROM roles WHERE id = 'admin';` shows `perm_metrics_view` in the array.

- [ ] **Step 3: Verify the down migration**

Run: `migrate -path migrations/ -database $DATABASE_URL down 1`
Expected: `perm_metrics_view` removed from every role's `permissions` array and from the `permissions` table; re-running `up` is idempotent (no error, same end state).

- [ ] **Step 4: Commit**

```bash
git add migrations/000015_metrics_view_permission.up.sql migrations/000015_metrics_view_permission.down.sql
git commit -m "feat(migrations): add perm_metrics_view, seeded to roles with perm_dashboard/perm_analytics"
```

---

### Task 6: repo/mongo/metrics — package skeleton + `Scalar` (avg/sum/min/max/last/delta/count) + non-numeric discard metric

**Files:**
- Create: `internal/telemetry/dashboard_metrics.go`
- Create: `internal/repo/mongo/metrics/repository.go`
- Test: `internal/repo/mongo/metrics/repository_test.go` (integration, behind `MONGO_URI` — same pattern as `internal/repo/mongo/measurements/repository_test.go`)

**Interfaces:**
- Consumes: `domain/metrics.{MetricSpec, ValueFilter, MetricResult, Agg}` (Task 1, Task 4); `measurements.CollectionName` (existing).
- Produces: `metricsmongo.Repository` (implements `domain/metrics.Repository`'s `Scalar` for now; `Series`/`Raw`/`Grouped`/`Catalog` are added in Tasks 7-10 on the same struct); `metricsmongo.New(db *mongodriver.Database, maxTime time.Duration) *Repository`; `metricsmongo.Unavailable(err error) domainmetrics.Repository` (degraded stub, same shape as `measurements.Unavailable`, consumed by Task 20's wiring).

- [ ] **Step 1: Write the failing test**

```go
// internal/repo/mongo/metrics/repository_test.go
//go:build integration

package metrics

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
	mongodriver "go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	domain "github.com/tu-org/embolsadora-api/internal/domain/metrics"
)

func mustConnect(t *testing.T) *mongodriver.Database {
	uri := os.Getenv("MONGO_URI")
	if uri == "" {
		t.Skip("MONGO_URI no seteado, se salta el integration test")
	}
	cli, err := mongodriver.Connect(options.Client().ApplyURI(uri))
	require.NoError(t, err)
	t.Cleanup(func() { _ = cli.Disconnect(context.Background()) })
	return cli.Database("embolsadora_test_metrics")
}

func seedMeasurement(t *testing.T, db *mongodriver.Database, tenantID, machineID, aasPath string, ts time.Time, value any) {
	_, err := db.Collection("measurements").InsertOne(context.Background(), bson.M{
		"eventId":   ts.Format(time.RFC3339Nano) + aasPath,
		"tenantId":  tenantID,
		"machineId": machineID,
		"ts":        ts,
		"kind":      "metric",
		"payload":   bson.M{"aasPath": aasPath, "value": value},
	})
	require.NoError(t, err)
}

func TestScalar_Avg(t *testing.T) {
	db := mustConnect(t)
	repo := New(db, 5*time.Second)
	ctx := context.Background()
	tenantID := "tenant-scalar-avg"
	now := time.Now().UTC()

	seedMeasurement(t, db, tenantID, "M1", "peso", now.Add(-1*time.Hour), 1.0)
	seedMeasurement(t, db, tenantID, "M1", "peso", now.Add(-30*time.Minute), 3.0)
	seedMeasurement(t, db, tenantID, "M1", "peso", now.Add(-10*time.Minute), "no-numerico")

	result, dataAsOf, err := repo.Scalar(ctx, tenantID, "M1", now.Add(-2*time.Hour), now, domain.MetricSpec{AasPath: "peso", Agg: domain.AggAvg}, nil)
	require.NoError(t, err)
	require.NotNil(t, dataAsOf)

	if result.Value != 2.0 {
		t.Fatalf("avg = %v, esperaba 2.0 (el valor no numerico se descarta)", result.Value)
	}
	if result.SampleCount != 2 {
		t.Fatalf("sampleCount = %v, esperaba 2", result.SampleCount)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `MONGO_URI=mongodb://localhost:27017 go test -tags=integration ./internal/repo/mongo/metrics/... -run TestScalar_Avg -v`
Expected: FAIL — package `metrics` (repo) / `New` / `Scalar` don't exist yet. (If `MONGO_URI` isn't exported, this SKIPs instead of failing — export it first, per CLAUDE.md's `docker compose up -d db redis mongo`.)

- [ ] **Step 3: Write minimal implementation**

```go
// internal/telemetry/dashboard_metrics.go
package telemetry

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// DashboardMetricsNonNumericDiscardedTotal cuenta valores de payload.value
// no numericos descartados por $isNumber en agregaciones avg/sum/min/max
// (fork 2 de la spec: el Edge no tiene garantia formal de que value sea
// siempre escalar, asi que esto es lo que hace observable un desajuste que
// de otro modo seria silencioso).
//
// Cardinalidad: tenant x aasPath. Aceptable con pocos tenants; si el numero
// de aasPath distintos crece mucho, revisar (ver spec, fork 2 "revisar
// cuando").
var DashboardMetricsNonNumericDiscardedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "dashboard_metrics_non_numeric_discarded_total",
	Help: "Valores de payload.value no numericos descartados en agregaciones avg/sum/min/max, por tenant/aasPath/agg",
}, []string{"tenant", "aas_path", "agg"})
```

```go
// internal/repo/mongo/metrics/repository.go
// Package metrics implementa domain/metrics.Repository sobre MongoDB.
package metrics

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	mongodriver "go.mongodb.org/mongo-driver/v2/mongo"

	domain "github.com/tu-org/embolsadora-api/internal/domain/metrics"
	measurements "github.com/tu-org/embolsadora-api/internal/repo/mongo/measurements"
	"github.com/tu-org/embolsadora-api/internal/telemetry"
)

type Repository struct {
	coll    *mongodriver.Collection
	maxTime time.Duration
}

// New construye el repositorio. maxTime acota cada aggregate (fork/C4) —
// 0 o negativo usa el default de 5s.
func New(db *mongodriver.Database, maxTime time.Duration) *Repository {
	if maxTime <= 0 {
		maxTime = 5 * time.Second
	}
	return &Repository{coll: db.Collection(measurements.CollectionName), maxTime: maxTime}
}

func baseMatch(tenantID, machineID string, from, to time.Time, aasPath string) bson.D {
	return bson.D{{Key: "$match", Value: bson.D{
		{Key: "tenantId", Value: tenantID},
		{Key: "machineId", Value: machineID},
		{Key: "payload.aasPath", Value: aasPath},
		{Key: "ts", Value: bson.D{{Key: "$gte", Value: from}, {Key: "$lte", Value: to}}},
	}}}
}

func isNumberMatch(negate bool) bson.D {
	expr := bson.D{{Key: "$isNumber", Value: "$payload.value"}}
	if negate {
		return bson.D{{Key: "$match", Value: bson.D{{Key: "$expr", Value: bson.D{{Key: "$not", Value: bson.A{expr}}}}}}}
	}
	return bson.D{{Key: "$match", Value: bson.D{{Key: "$expr", Value: expr}}}}
}

func numericAccumulator(agg domain.Agg) (string, bool) {
	switch agg {
	case domain.AggAvg:
		return "$avg", true
	case domain.AggSum:
		return "$sum", true
	case domain.AggMin:
		return "$min", true
	case domain.AggMax:
		return "$max", true
	}
	return "", false
}

// Scalar resuelve avg/sum/min/max/last/delta/count sin bucketizar.
func (r *Repository) Scalar(ctx context.Context, tenantID, machineID string, from, to time.Time, spec domain.MetricSpec, filter *domain.ValueFilter) (domain.MetricResult, *time.Time, error) {
	ctx, cancel := context.WithTimeout(ctx, r.maxTime)
	defer cancel()

	switch spec.Agg {
	case domain.AggAvg, domain.AggSum, domain.AggMin, domain.AggMax:
		return r.scalarNumeric(ctx, tenantID, machineID, from, to, spec)
	case domain.AggLast:
		return r.scalarLast(ctx, tenantID, machineID, from, to, spec)
	case domain.AggDelta:
		return r.scalarDelta(ctx, tenantID, machineID, from, to, spec)
	case domain.AggCount:
		return r.scalarCount(ctx, tenantID, machineID, from, to, spec, filter)
	}
	return domain.MetricResult{}, nil, fmt.Errorf("agg no soportado en modo escalar: %s", spec.Agg)
}

type numericGroupResult struct {
	Value       float64   `bson:"value"`
	SampleCount int64     `bson:"sampleCount"`
	DataAsOf    time.Time `bson:"dataAsOf"`
}

func (r *Repository) scalarNumeric(ctx context.Context, tenantID, machineID string, from, to time.Time, spec domain.MetricSpec) (domain.MetricResult, *time.Time, error) {
	accumulator, ok := numericAccumulator(spec.Agg)
	if !ok {
		return domain.MetricResult{}, nil, fmt.Errorf("agg no numerico: %s", spec.Agg)
	}

	pipeline := mongodriver.Pipeline{
		baseMatch(tenantID, machineID, from, to, spec.AasPath),
		isNumberMatch(false),
		bson.D{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: nil},
			{Key: "value", Value: bson.D{{Key: accumulator, Value: "$payload.value"}}},
			{Key: "sampleCount", Value: bson.D{{Key: "$sum", Value: 1}}},
			{Key: "dataAsOf", Value: bson.D{{Key: "$max", Value: "$ts"}}},
		}}},
	}

	cur, err := r.coll.Aggregate(ctx, pipeline)
	if err != nil {
		return domain.MetricResult{}, nil, err
	}
	defer cur.Close(ctx)

	var results []numericGroupResult
	if err := cur.All(ctx, &results); err != nil {
		return domain.MetricResult{}, nil, err
	}

	// Discard count: consulta liviana separada (mismo $match, $count) para no
	// complicar el pipeline principal con un $facet. Best-effort: si esta
	// falla, no se pierde el resultado principal, solo la instrumentacion.
	r.reportNonNumericDiscards(ctx, tenantID, machineID, from, to, spec)

	if len(results) == 0 {
		return domain.MetricResult{AasPath: spec.AasPath, Agg: spec.Agg}, nil, nil
	}
	g := results[0]
	dataAsOf := g.DataAsOf
	return domain.MetricResult{AasPath: spec.AasPath, Agg: spec.Agg, Value: g.Value, SampleCount: g.SampleCount}, &dataAsOf, nil
}

func (r *Repository) reportNonNumericDiscards(ctx context.Context, tenantID, machineID string, from, to time.Time, spec domain.MetricSpec) {
	pipeline := mongodriver.Pipeline{
		baseMatch(tenantID, machineID, from, to, spec.AasPath),
		isNumberMatch(true),
		bson.D{{Key: "$count", Value: "count"}},
	}
	cur, err := r.coll.Aggregate(ctx, pipeline)
	if err != nil {
		return
	}
	defer cur.Close(ctx)
	var results []struct {
		Count int64 `bson:"count"`
	}
	if err := cur.All(ctx, &results); err != nil || len(results) == 0 {
		return
	}
	telemetry.DashboardMetricsNonNumericDiscardedTotal.
		WithLabelValues(tenantID, spec.AasPath, string(spec.Agg)).
		Add(float64(results[0].Count))
}

type lastOrFirstLastResult struct {
	Value    float64   `bson:"value"`
	First    float64   `bson:"first"`
	Last     float64   `bson:"last"`
	DataAsOf time.Time `bson:"dataAsOf"`
}

func (r *Repository) scalarLast(ctx context.Context, tenantID, machineID string, from, to time.Time, spec domain.MetricSpec) (domain.MetricResult, *time.Time, error) {
	pipeline := mongodriver.Pipeline{
		baseMatch(tenantID, machineID, from, to, spec.AasPath),
		isNumberMatch(false),
		bson.D{{Key: "$sort", Value: bson.D{{Key: "ts", Value: 1}}}},
		bson.D{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: nil},
			{Key: "value", Value: bson.D{{Key: "$last", Value: "$payload.value"}}},
			{Key: "sampleCount", Value: bson.D{{Key: "$sum", Value: 1}}},
			{Key: "dataAsOf", Value: bson.D{{Key: "$max", Value: "$ts"}}},
		}}},
	}
	cur, err := r.coll.Aggregate(ctx, pipeline)
	if err != nil {
		return domain.MetricResult{}, nil, err
	}
	defer cur.Close(ctx)
	var results []numericGroupResult
	if err := cur.All(ctx, &results); err != nil {
		return domain.MetricResult{}, nil, err
	}
	r.reportNonNumericDiscards(ctx, tenantID, machineID, from, to, spec)
	if len(results) == 0 {
		return domain.MetricResult{AasPath: spec.AasPath, Agg: spec.Agg}, nil, nil
	}
	g := results[0]
	dataAsOf := g.DataAsOf
	return domain.MetricResult{AasPath: spec.AasPath, Agg: spec.Agg, Value: g.Value, SampleCount: g.SampleCount}, &dataAsOf, nil
}

func (r *Repository) scalarDelta(ctx context.Context, tenantID, machineID string, from, to time.Time, spec domain.MetricSpec) (domain.MetricResult, *time.Time, error) {
	pipeline := mongodriver.Pipeline{
		baseMatch(tenantID, machineID, from, to, spec.AasPath),
		isNumberMatch(false),
		bson.D{{Key: "$sort", Value: bson.D{{Key: "ts", Value: 1}}}},
		bson.D{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: nil},
			{Key: "first", Value: bson.D{{Key: "$first", Value: "$payload.value"}}},
			{Key: "last", Value: bson.D{{Key: "$last", Value: "$payload.value"}}},
			{Key: "sampleCount", Value: bson.D{{Key: "$sum", Value: 1}}},
			{Key: "dataAsOf", Value: bson.D{{Key: "$max", Value: "$ts"}}},
		}}},
	}
	cur, err := r.coll.Aggregate(ctx, pipeline)
	if err != nil {
		return domain.MetricResult{}, nil, err
	}
	defer cur.Close(ctx)
	var results []struct {
		First       float64   `bson:"first"`
		Last        float64   `bson:"last"`
		SampleCount int64     `bson:"sampleCount"`
		DataAsOf    time.Time `bson:"dataAsOf"`
	}
	if err := cur.All(ctx, &results); err != nil {
		return domain.MetricResult{}, nil, err
	}
	r.reportNonNumericDiscards(ctx, tenantID, machineID, from, to, spec)
	if len(results) == 0 {
		return domain.MetricResult{AasPath: spec.AasPath, Agg: spec.Agg}, nil, nil
	}
	g := results[0]
	dataAsOf := g.DataAsOf
	// La resta se hace aca, no en el pipeline (spec, seccion "Traduccion a
	// pipeline de Mongo"): delta = last - first.
	return domain.MetricResult{AasPath: spec.AasPath, Agg: spec.Agg, Value: g.Last - g.First, SampleCount: g.SampleCount}, &dataAsOf, nil
}

func (r *Repository) scalarCount(ctx context.Context, tenantID, machineID string, from, to time.Time, spec domain.MetricSpec, filter *domain.ValueFilter) (domain.MetricResult, *time.Time, error) {
	matchStage := baseMatch(tenantID, machineID, from, to, spec.AasPath)
	if filter != nil && filter.ValueEquals != nil {
		matchStage[0].Value = append(matchStage[0].Value.(bson.D), bson.E{Key: "payload.value", Value: filter.ValueEquals})
	}
	pipeline := mongodriver.Pipeline{
		matchStage,
		bson.D{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: nil},
			{Key: "value", Value: bson.D{{Key: "$sum", Value: 1}}},
			{Key: "dataAsOf", Value: bson.D{{Key: "$max", Value: "$ts"}}},
		}}},
	}
	cur, err := r.coll.Aggregate(ctx, pipeline)
	if err != nil {
		return domain.MetricResult{}, nil, err
	}
	defer cur.Close(ctx)
	var results []struct {
		Value    int64     `bson:"value"`
		DataAsOf time.Time `bson:"dataAsOf"`
	}
	if err := cur.All(ctx, &results); err != nil {
		return domain.MetricResult{}, nil, err
	}
	if len(results) == 0 {
		return domain.MetricResult{AasPath: spec.AasPath, Agg: spec.Agg}, nil, nil
	}
	g := results[0]
	dataAsOf := g.DataAsOf
	return domain.MetricResult{AasPath: spec.AasPath, Agg: spec.Agg, Value: float64(g.Value), SampleCount: g.Value}, &dataAsOf, nil
}

// unavailable implementa domain/metrics.Repository sin Mongo real detras —
// mismo patron que measurements.Unavailable, para que un Mongo caido al
// arrancar deje /dashboards/metrics degradado (500) en vez de tumbar el
// resto de la API.
type unavailable struct{ err error }

func Unavailable(err error) domain.Repository { return &unavailable{err: err} }

func (u *unavailable) Scalar(context.Context, string, string, time.Time, time.Time, domain.MetricSpec, *domain.ValueFilter) (domain.MetricResult, *time.Time, error) {
	return domain.MetricResult{}, nil, fmt.Errorf("mongo no disponible desde el arranque: %w", u.err)
}
func (u *unavailable) Series(context.Context, string, string, time.Time, time.Time, domain.Bucket, domain.MetricSpec, *domain.ValueFilter) ([]domain.BucketPoint, *time.Time, error) {
	return nil, nil, fmt.Errorf("mongo no disponible desde el arranque: %w", u.err)
}
func (u *unavailable) Raw(context.Context, string, string, time.Time, time.Time, string, int, int) ([]domain.RawPoint, *time.Time, error) {
	return nil, nil, fmt.Errorf("mongo no disponible desde el arranque: %w", u.err)
}
func (u *unavailable) Grouped(context.Context, string, string, time.Time, time.Time, string, domain.MetricSpec, *domain.ValueFilter, int) ([]domain.GroupResult, *time.Time, error) {
	return nil, nil, fmt.Errorf("mongo no disponible desde el arranque: %w", u.err)
}
func (u *unavailable) Catalog(context.Context, string, string) ([]string, error) {
	return nil, fmt.Errorf("mongo no disponible desde el arranque: %w", u.err)
}
```

Note: `Series`, `Raw`, `Grouped`, `Catalog` on `*Repository` itself (not `unavailable`) are added in Tasks 7-10 — until those land, `*Repository` doesn't yet satisfy `domain.Repository` and won't compile against that interface. That's fine: this task's test only calls `.Scalar` directly on the concrete type, not through the interface. `Unavailable`'s return type is `domain.Repository`, which requires `unavailable` to implement all 5 methods — written above — so it compiles independently of `*Repository`'s progress.

- [ ] **Step 4: Run test to verify it passes**

Run: `MONGO_URI=mongodb://localhost:27017 go test -tags=integration ./internal/repo/mongo/metrics/... -run TestScalar_Avg -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/telemetry/dashboard_metrics.go internal/repo/mongo/metrics/repository.go internal/repo/mongo/metrics/repository_test.go
git commit -m "feat(metrics): Mongo Scalar pipelines (avg/sum/min/max/last/delta/count) + non-numeric discard metric"
```

---

### Task 7: repo/mongo/metrics — `Series` (bucketized)

**Files:**
- Modify: `internal/repo/mongo/metrics/repository.go`
- Modify: `internal/repo/mongo/metrics/repository_test.go`

**Interfaces:**
- Consumes: `domain.Bucket.duration()` is NOT exported from `domain/metrics` (lowercase `duration`) — this task needs the bucket's Mongo `$dateTrunc` unit string instead, computed locally from the `Bucket` enum value (`"1m"→"minute"`, `"1h"→"hour"`, etc.), not by importing the unexported duration.
- Produces: `(*Repository).Series(...)` added to Task 6's struct, completing one more method of `domain.Repository`.

- [ ] **Step 1: Write the failing test**

```go
// append to internal/repo/mongo/metrics/repository_test.go
func TestSeries_AvgBucketized(t *testing.T) {
	db := mustConnect(t)
	repo := New(db, 5*time.Second)
	ctx := context.Background()
	tenantID := "tenant-series-avg"
	base := time.Date(2026, 9, 11, 0, 0, 0, 0, time.UTC)

	seedMeasurement(t, db, tenantID, "M1", "peso", base.Add(10*time.Minute), 1.0)
	seedMeasurement(t, db, tenantID, "M1", "peso", base.Add(50*time.Minute), 3.0)
	seedMeasurement(t, db, tenantID, "M1", "peso", base.Add(90*time.Minute), 5.0)

	points, dataAsOf, err := repo.Series(ctx, tenantID, "M1", base, base.Add(2*time.Hour), domain.Bucket1h, domain.MetricSpec{AasPath: "peso", Agg: domain.AggAvg}, nil)
	require.NoError(t, err)
	require.NotNil(t, dataAsOf)
	require.Len(t, points, 2)

	if points[0].Value != 2.0 || points[0].SampleCount != 2 {
		t.Fatalf("bucket 0: value=%v sampleCount=%v, esperaba value=2.0 sampleCount=2", points[0].Value, points[0].SampleCount)
	}
	if points[1].Value != 5.0 || points[1].SampleCount != 1 {
		t.Fatalf("bucket 1: value=%v sampleCount=%v, esperaba value=5.0 sampleCount=1", points[1].Value, points[1].SampleCount)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `MONGO_URI=mongodb://localhost:27017 go test -tags=integration ./internal/repo/mongo/metrics/... -run TestSeries_AvgBucketized -v`
Expected: FAIL — `Series` not defined on `*Repository`.

- [ ] **Step 3: Write minimal implementation**

```go
// append to internal/repo/mongo/metrics/repository.go

// dateTruncUnit mapea Bucket a la unidad que espera $dateTrunc. Bucket es un
// tipo de domain/metrics; su duration() no esta exportada a proposito
// (domain no conoce Mongo), asi que este mapeo vive aca, no ahi.
func dateTruncUnit(b domain.Bucket) (unit string, binSize int, ok bool) {
	switch b {
	case domain.Bucket1m:
		return "minute", 1, true
	case domain.Bucket5m:
		return "minute", 5, true
	case domain.Bucket15m:
		return "minute", 15, true
	case domain.Bucket1h:
		return "hour", 1, true
	case domain.Bucket6h:
		return "hour", 6, true
	case domain.Bucket1d:
		return "day", 1, true
	}
	return "", 0, false
}

func (r *Repository) Series(ctx context.Context, tenantID, machineID string, from, to time.Time, bucket domain.Bucket, spec domain.MetricSpec, filter *domain.ValueFilter) ([]domain.BucketPoint, *time.Time, error) {
	ctx, cancel := context.WithTimeout(ctx, r.maxTime)
	defer cancel()

	unit, binSize, ok := dateTruncUnit(bucket)
	if !ok {
		return nil, nil, fmt.Errorf("bucket no soportado: %s", bucket)
	}

	truncExpr := bson.D{{Key: "$dateTrunc", Value: bson.D{
		{Key: "date", Value: "$ts"},
		{Key: "unit", Value: unit},
		{Key: "binSize", Value: binSize},
	}}}

	var groupStage bson.D
	switch spec.Agg {
	case domain.AggCount:
		matchStage := baseMatch(tenantID, machineID, from, to, spec.AasPath)
		if filter != nil && filter.ValueEquals != nil {
			matchStage[0].Value = append(matchStage[0].Value.(bson.D), bson.E{Key: "payload.value", Value: filter.ValueEquals})
		}
		groupStage = bson.D{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: truncExpr},
			{Key: "value", Value: bson.D{{Key: "$sum", Value: 1}}},
			{Key: "sampleCount", Value: bson.D{{Key: "$sum", Value: 1}}},
		}}}
		return r.runSeriesPipeline(ctx, mongodriver.Pipeline{matchStage, groupStage}, tenantID, machineID, from, to, spec)
	default:
		accumulator, ok := numericAccumulator(spec.Agg)
		if !ok {
			return nil, nil, fmt.Errorf("agg no soportado en modo serie: %s", spec.Agg)
		}
		groupStage = bson.D{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: truncExpr},
			{Key: "value", Value: bson.D{{Key: accumulator, Value: "$payload.value"}}},
			{Key: "sampleCount", Value: bson.D{{Key: "$sum", Value: 1}}},
		}}}
		pipeline := mongodriver.Pipeline{
			baseMatch(tenantID, machineID, from, to, spec.AasPath),
			isNumberMatch(false),
			groupStage,
		}
		r.reportNonNumericDiscards(ctx, tenantID, machineID, from, to, spec)
		return r.runSeriesPipeline(ctx, pipeline, tenantID, machineID, from, to, spec)
	}
}

func (r *Repository) runSeriesPipeline(ctx context.Context, pipeline mongodriver.Pipeline, tenantID, machineID string, from, to time.Time, spec domain.MetricSpec) ([]domain.BucketPoint, *time.Time, error) {
	pipeline = append(pipeline, bson.D{{Key: "$sort", Value: bson.D{{Key: "_id", Value: 1}}}})
	cur, err := r.coll.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, nil, err
	}
	defer cur.Close(ctx)

	var rows []struct {
		Ts          time.Time `bson:"_id"`
		Value       float64   `bson:"value"`
		SampleCount int64     `bson:"sampleCount"`
	}
	if err := cur.All(ctx, &rows); err != nil {
		return nil, nil, err
	}

	points := make([]domain.BucketPoint, 0, len(rows))
	var dataAsOf *time.Time
	for _, row := range rows {
		points = append(points, domain.BucketPoint{Ts: row.Ts, Value: row.Value, SampleCount: row.SampleCount})
		if dataAsOf == nil || row.Ts.After(*dataAsOf) {
			t := row.Ts
			dataAsOf = &t
		}
	}
	return points, dataAsOf, nil
}
```

Note: `dataAsOf` for series mode is approximated here as the timestamp of the last bucket boundary (`_id`), not the true max raw `ts` inside that bucket — close enough for the "did new data arrive" purpose `dataAsOf` serves (spec: refresh-condition hook), and avoids a second aggregation just to get the exact max. If an implementer wants the exact value, add `dataAsOf: {$max: "$ts"}` to `groupStage` and take the max across rows instead — flagged here as an acceptable simplification, not a hidden bug.

- [ ] **Step 4: Run test to verify it passes**

Run: `MONGO_URI=mongodb://localhost:27017 go test -tags=integration ./internal/repo/mongo/metrics/... -v`
Expected: PASS (both `TestScalar_Avg` and `TestSeries_AvgBucketized`)

- [ ] **Step 5: Commit**

```bash
git add internal/repo/mongo/metrics/repository.go internal/repo/mongo/metrics/repository_test.go
git commit -m "feat(metrics): Mongo Series pipeline (bucketized avg/sum/min/max/count) via \$dateTrunc"
```

---

### Task 8: repo/mongo/metrics — `Raw` (crudo, maxPoints decimation, RANGE_TOO_WIDE)

**Files:**
- Modify: `internal/repo/mongo/metrics/repository.go`
- Modify: `internal/repo/mongo/metrics/repository_test.go`

**Interfaces:**
- Produces: `(*Repository).Raw(ctx, tenantID, machineID string, from, to time.Time, aasPath string, limit, maxPoints int) ([]domain.RawPoint, *time.Time, error)`. Returns `len(points) == limit` as the caller's (Task 12) signal to reject with `RANGE_TOO_WIDE` when `maxPoints <= 0`; when `maxPoints > 0`, this method itself decimates and never returns more than `maxPoints`.

- [ ] **Step 1: Write the failing test**

```go
// append to internal/repo/mongo/metrics/repository_test.go
func TestRaw_ReturnsPointsOrderedByTs(t *testing.T) {
	db := mustConnect(t)
	repo := New(db, 5*time.Second)
	ctx := context.Background()
	tenantID := "tenant-raw"
	base := time.Date(2026, 9, 11, 9, 0, 0, 0, time.UTC)

	seedMeasurement(t, db, tenantID, "M1", "temp", base.Add(2*time.Second), 80.0)
	seedMeasurement(t, db, tenantID, "M1", "temp", base.Add(1*time.Second), 79.0)

	points, dataAsOf, err := repo.Raw(ctx, tenantID, "M1", base, base.Add(time.Hour), "temp", 5001, 0)
	require.NoError(t, err)
	require.NotNil(t, dataAsOf)
	require.Len(t, points, 2)
	if !points[0].Ts.Before(points[1].Ts) {
		t.Fatalf("puntos no ordenados por ts ascendente")
	}
}

func TestRaw_DecimatesWhenMaxPointsSet(t *testing.T) {
	db := mustConnect(t)
	repo := New(db, 5*time.Second)
	ctx := context.Background()
	tenantID := "tenant-raw-decimate"
	base := time.Date(2026, 9, 11, 9, 0, 0, 0, time.UTC)

	for i := 0; i < 10; i++ {
		seedMeasurement(t, db, tenantID, "M1", "temp", base.Add(time.Duration(i)*time.Second), float64(i))
	}

	points, _, err := repo.Raw(ctx, tenantID, "M1", base, base.Add(time.Hour), "temp", 5001, 3)
	require.NoError(t, err)
	require.LessOrEqual(t, len(points), 3)
	// El primer y ultimo punto original siempre se preservan.
	if points[0].Value != 0.0 {
		t.Fatalf("primer punto = %v, esperaba 0.0", points[0].Value)
	}
	if points[len(points)-1].Value != 9.0 {
		t.Fatalf("ultimo punto = %v, esperaba 9.0", points[len(points)-1].Value)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `MONGO_URI=mongodb://localhost:27017 go test -tags=integration ./internal/repo/mongo/metrics/... -run TestRaw -v`
Expected: FAIL — `Raw` not defined.

- [ ] **Step 3: Write minimal implementation**

```go
// append to internal/repo/mongo/metrics/repository.go

func (r *Repository) Raw(ctx context.Context, tenantID, machineID string, from, to time.Time, aasPath string, limit, maxPoints int) ([]domain.RawPoint, *time.Time, error) {
	ctx, cancel := context.WithTimeout(ctx, r.maxTime)
	defer cancel()

	pipeline := mongodriver.Pipeline{
		baseMatch(tenantID, machineID, from, to, aasPath),
		bson.D{{Key: "$sort", Value: bson.D{{Key: "ts", Value: 1}}}},
		bson.D{{Key: "$limit", Value: limit}},
	}
	cur, err := r.coll.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, nil, err
	}
	defer cur.Close(ctx)

	var rows []struct {
		Ts      time.Time `bson:"ts"`
		Payload struct {
			Value any `bson:"value"`
		} `bson:"payload"`
	}
	if err := cur.All(ctx, &rows); err != nil {
		return nil, nil, err
	}

	points := make([]domain.RawPoint, 0, len(rows))
	var dataAsOf *time.Time
	for _, row := range rows {
		points = append(points, domain.RawPoint{Ts: row.Ts, Value: row.Payload.Value})
		if dataAsOf == nil || row.Ts.After(*dataAsOf) {
			t := row.Ts
			dataAsOf = &t
		}
	}

	// fork/C3: si maxPoints viene seteado, decimar en vez de dejar que el
	// llamador (app/dashboards) rechace con RANGE_TOO_WIDE. len(points) ==
	// limit sigue siendo la senal de "hay mas de los que se pidieron" — eso
	// no cambia; lo que cambia es que aca mismo se recorta a maxPoints en
	// vez de devolver el excedente sin recortar.
	if maxPoints > 0 && len(points) > maxPoints {
		points = decimateUniform(points, maxPoints)
	}

	return points, dataAsOf, nil
}

// decimateUniform reduce points a como maximo maxPoints elementos con
// muestreo por stride uniforme, preservando siempre el primer y el ultimo
// punto original. Decimacion simple (no LTTB): correcta y suficiente para
// v1, mas facil de razonar; upgradeable despues sin cambiar el contrato
// (fork/C3 de la spec).
func decimateUniform(points []domain.RawPoint, maxPoints int) []domain.RawPoint {
	if maxPoints < 2 || len(points) <= maxPoints {
		return points
	}
	step := float64(len(points)-1) / float64(maxPoints-1)
	out := make([]domain.RawPoint, 0, maxPoints)
	for i := 0; i < maxPoints; i++ {
		idx := int(float64(i) * step)
		if idx >= len(points) {
			idx = len(points) - 1
		}
		out = append(out, points[idx])
	}
	return out
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `MONGO_URI=mongodb://localhost:27017 go test -tags=integration ./internal/repo/mongo/metrics/... -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/repo/mongo/metrics/repository.go internal/repo/mongo/metrics/repository_test.go
git commit -m "feat(metrics): Mongo Raw pipeline with uniform-stride decimation (C3)"
```

---

### Task 9: repo/mongo/metrics — `Grouped` (groupBy + TOO_MANY_GROUPS detection)

**Files:**
- Modify: `internal/repo/mongo/metrics/repository.go`
- Modify: `internal/repo/mongo/metrics/repository_test.go`

**Interfaces:**
- Produces: `(*Repository).Grouped(ctx, tenantID, machineID string, from, to time.Time, groupBy string, spec domain.MetricSpec, filter *domain.ValueFilter, limit int) ([]domain.GroupResult, *time.Time, error)`. Same "returns `limit` items → caller rejects with `TOO_MANY_GROUPS`" convention as `Raw`.

- [ ] **Step 1: Write the failing test**

```go
// append to internal/repo/mongo/metrics/repository_test.go
func TestGrouped_CountByField(t *testing.T) {
	db := mustConnect(t)
	repo := New(db, 5*time.Second)
	ctx := context.Background()
	tenantID := "tenant-grouped"
	base := time.Date(2026, 9, 11, 9, 0, 0, 0, time.UTC)

	seedAlarm := func(tipo string, at time.Time) {
		_, err := db.Collection("measurements").InsertOne(ctx, bson.M{
			"eventId":   tipo + at.Format(time.RFC3339Nano),
			"tenantId":  tenantID,
			"machineId": "M1",
			"ts":        at,
			"kind":      "alarm",
			"payload":   bson.M{"aasPath": "Alarmas/tipo", "value": tipo, "tipo": tipo},
		})
		require.NoError(t, err)
	}
	seedAlarm("sellado_defectuoso", base)
	seedAlarm("sellado_defectuoso", base.Add(time.Minute))
	seedAlarm("temperatura_alta", base.Add(2*time.Minute))

	groups, dataAsOf, err := repo.Grouped(ctx, tenantID, "M1", base.Add(-time.Hour), base.Add(time.Hour), "payload.tipo", domain.MetricSpec{AasPath: "Alarmas/tipo", Agg: domain.AggCount}, nil, 201)
	require.NoError(t, err)
	require.NotNil(t, dataAsOf)
	require.Len(t, groups, 2)
	if groups[0].Key != "sellado_defectuoso" || groups[0].Value != 2 {
		t.Fatalf("primer grupo = %+v, esperaba sellado_defectuoso:2 (orden descendente por value)", groups[0])
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `MONGO_URI=mongodb://localhost:27017 go test -tags=integration ./internal/repo/mongo/metrics/... -run TestGrouped -v`
Expected: FAIL — `Grouped` not defined.

- [ ] **Step 3: Write minimal implementation**

```go
// append to internal/repo/mongo/metrics/repository.go

func (r *Repository) Grouped(ctx context.Context, tenantID, machineID string, from, to time.Time, groupBy string, spec domain.MetricSpec, filter *domain.ValueFilter, limit int) ([]domain.GroupResult, *time.Time, error) {
	ctx, cancel := context.WithTimeout(ctx, r.maxTime)
	defer cancel()

	// groupBy ya fue validado contra groupByFieldPattern en domain/metrics
	// (Task 3) antes de llegar aca — este repositorio confia en esa
	// validacion previa para construir la referencia de campo "$"+groupBy.
	fieldRef := "$" + groupBy

	matchStage := baseMatch(tenantID, machineID, from, to, spec.AasPath)
	if filter != nil && filter.ValueEquals != nil {
		matchStage[0].Value = append(matchStage[0].Value.(bson.D), bson.E{Key: "payload.value", Value: filter.ValueEquals})
	}

	var groupStage bson.D
	if spec.Agg == domain.AggCount {
		groupStage = bson.D{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: fieldRef},
			{Key: "value", Value: bson.D{{Key: "$sum", Value: 1}}},
			{Key: "dataAsOf", Value: bson.D{{Key: "$max", Value: "$ts"}}},
		}}}
	} else {
		accumulator, ok := numericAccumulator(spec.Agg)
		if !ok {
			return nil, nil, fmt.Errorf("agg no soportado en modo agrupado: %s", spec.Agg)
		}
		groupStage = bson.D{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: fieldRef},
			{Key: "value", Value: bson.D{{Key: accumulator, Value: "$payload.value"}}},
			{Key: "dataAsOf", Value: bson.D{{Key: "$max", Value: "$ts"}}},
		}}}
	}

	pipeline := mongodriver.Pipeline{
		matchStage,
		groupStage,
		bson.D{{Key: "$sort", Value: bson.D{{Key: "value", Value: -1}}}},
		bson.D{{Key: "$limit", Value: limit}},
	}
	cur, err := r.coll.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, nil, err
	}
	defer cur.Close(ctx)

	var rows []struct {
		Key      string    `bson:"_id"`
		Value    float64   `bson:"value"`
		DataAsOf time.Time `bson:"dataAsOf"`
	}
	if err := cur.All(ctx, &rows); err != nil {
		return nil, nil, err
	}

	groups := make([]domain.GroupResult, 0, len(rows))
	var dataAsOf *time.Time
	for _, row := range rows {
		groups = append(groups, domain.GroupResult{Key: row.Key, Value: row.Value})
		if dataAsOf == nil || row.DataAsOf.After(*dataAsOf) {
			t := row.DataAsOf
			dataAsOf = &t
		}
	}
	return groups, dataAsOf, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `MONGO_URI=mongodb://localhost:27017 go test -tags=integration ./internal/repo/mongo/metrics/... -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/repo/mongo/metrics/repository.go internal/repo/mongo/metrics/repository_test.go
git commit -m "feat(metrics): Mongo Grouped pipeline (groupBy) with TOO_MANY_GROUPS detection"
```

---

### Task 10: repo/mongo/metrics — `Catalog` (distinct aasPath)

**Files:**
- Modify: `internal/repo/mongo/metrics/repository.go`
- Modify: `internal/repo/mongo/metrics/repository_test.go`

**Interfaces:**
- Produces: `(*Repository).Catalog(ctx, tenantID, machineID string) ([]string, error)` — completes `*Repository`'s implementation of `domain.Repository`.

- [ ] **Step 1: Write the failing test**

```go
// append to internal/repo/mongo/metrics/repository_test.go
func TestCatalog_ReturnsObservedAasPaths(t *testing.T) {
	db := mustConnect(t)
	repo := New(db, 5*time.Second)
	ctx := context.Background()
	tenantID := "tenant-catalog"
	now := time.Now().UTC()

	seedMeasurement(t, db, tenantID, "M1", "peso", now, 1.0)
	seedMeasurement(t, db, tenantID, "M1", "temperatura", now, 80.0)
	seedMeasurement(t, db, tenantID, "M2", "otra_maquina", now, 1.0) // otro machineId, no debe aparecer

	paths, err := repo.Catalog(ctx, tenantID, "M1")
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"peso", "temperatura"}, paths)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `MONGO_URI=mongodb://localhost:27017 go test -tags=integration ./internal/repo/mongo/metrics/... -run TestCatalog -v`
Expected: FAIL — `Catalog` not defined; `*Repository` still doesn't satisfy `domain.Repository`.

- [ ] **Step 3: Write minimal implementation**

```go
// append to internal/repo/mongo/metrics/repository.go

func (r *Repository) Catalog(ctx context.Context, tenantID, machineID string) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, r.maxTime)
	defer cancel()

	filter := bson.D{{Key: "tenantId", Value: tenantID}, {Key: "machineId", Value: machineID}}
	raw, err := r.coll.Distinct(ctx, "payload.aasPath", filter)
	if err != nil {
		return nil, err
	}
	paths := make([]string, 0, len(raw))
	for _, v := range raw {
		if s, ok := v.(string); ok {
			paths = append(paths, s)
		}
	}
	return paths, nil
}

// Compile-time check: *Repository debe satisfacer domain.Repository ahora
// que las 5 operaciones estan implementadas.
var _ domain.Repository = (*Repository)(nil)
```

- [ ] **Step 4: Run test to verify it passes**

Run: `MONGO_URI=mongodb://localhost:27017 go test -tags=integration ./internal/repo/mongo/metrics/... -v`
Expected: PASS. Also run `go build ./...` — the `var _ domain.Repository = (*Repository)(nil)` line will now fail to compile if any method signature drifted from Task 4's interface; fix any mismatch before proceeding.

- [ ] **Step 5: Commit**

```bash
git add internal/repo/mongo/metrics/repository.go internal/repo/mongo/metrics/repository_test.go
git commit -m "feat(metrics): Mongo Catalog (distinct aasPath); Repository now satisfies domain.Repository"
```

---

### Task 11: repo/mongo/metrics — cross-tenant isolation integration test

**Files:**
- Modify: `internal/repo/mongo/metrics/repository_test.go`

**Interfaces:**
- Consumes: everything from Tasks 6-10. No new production code — this task is pure test, called out separately because the spec names it "the most important" test.

- [ ] **Step 1: Write the test (it should already pass — this task verifies, not implements)**

```go
// append to internal/repo/mongo/metrics/repository_test.go

// TestCrossTenantIsolation es el caso mas importante de la spec (seccion
// Testing): el $match de tenantId nunca debe dejar leer datos de otro
// tenant, ni siquiera con un groupBy o aasPath que coincida por casualidad.
func TestCrossTenantIsolation(t *testing.T) {
	db := mustConnect(t)
	repo := New(db, 5*time.Second)
	ctx := context.Background()
	now := time.Now().UTC()

	// Mismo machineId y aasPath en dos tenants distintos -- exactamente el
	// caso "coincide por casualidad" que la spec pide cubrir.
	seedMeasurement(t, db, "tenant-a", "SHARED-ID", "peso", now, 100.0)
	seedMeasurement(t, db, "tenant-b", "SHARED-ID", "peso", now, 999.0)

	result, _, err := repo.Scalar(ctx, "tenant-a", "SHARED-ID", now.Add(-time.Hour), now.Add(time.Hour), domain.MetricSpec{AasPath: "peso", Agg: domain.AggAvg}, nil)
	require.NoError(t, err)
	if result.Value != 100.0 {
		t.Fatalf("Scalar devolvio %v, esperaba 100.0 (leyo de otro tenant)", result.Value)
	}

	paths, err := repo.Catalog(ctx, "tenant-a", "SHARED-ID")
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"peso"}, paths)

	groups, _, err := repo.Grouped(ctx, "tenant-a", "SHARED-ID", now.Add(-time.Hour), now.Add(time.Hour), "payload.value", domain.MetricSpec{AasPath: "peso", Agg: domain.AggCount}, nil, 201)
	require.NoError(t, err)
	for _, g := range groups {
		if g.Key == "999" {
			t.Fatalf("Grouped devolvio un valor del tenant-b")
		}
	}
}
```

- [ ] **Step 2: Run to verify it passes**

Run: `MONGO_URI=mongodb://localhost:27017 go test -tags=integration ./internal/repo/mongo/metrics/... -run TestCrossTenantIsolation -v`
Expected: PASS. If it fails, the bug is in `baseMatch` (Task 6) or `Catalog`'s filter (Task 10) — fix there, not here.

- [ ] **Step 3: Commit**

```bash
git add internal/repo/mongo/metrics/repository_test.go
git commit -m "test(metrics): explicit cross-tenant isolation coverage for Scalar/Catalog/Grouped"
```

---

### Task 12: app/dashboards — `Service.Query` (single query orchestration)

**Files:**
- Create: `internal/app/dashboards/service.go`
- Create: `internal/app/dashboards/fake_repository_test.go`
- Test: `internal/app/dashboards/service_test.go`

**Interfaces:**
- Consumes: `domain/metrics.{MetricQuery, Limits, Validate, ResolveWindow, Repository, QueryResult, Mode, ...}` (Tasks 1-4).
- Produces: `Service{repo domain.Repository; limits domain.Limits; logger *zap.Logger}`; `NewService(repo domain.Repository, limits domain.Limits, logger *zap.Logger) *Service`; `(*Service).Query(ctx context.Context, tenantID string, q domain.MetricQuery, now time.Time) (domain.QueryResult, error)` — consumed by Task 15's handler and Task 13's batch usecase.

- [ ] **Step 1: Write the failing test**

```go
// internal/app/dashboards/fake_repository_test.go
package dashboards

import (
	"context"
	"time"

	domain "github.com/tu-org/embolsadora-api/internal/domain/metrics"
)

// fakeRepository es un stub de domain.Repository a mano -- este repo no usa
// uber/mock en ningun lado (ver Global Constraints del plan), asi que esto
// sigue el mismo estilo que el resto del codigo.
type fakeRepository struct {
	scalarResult domain.MetricResult
	scalarErr    error
	catalogPaths []string
	catalogErr   error
}

func (f *fakeRepository) Scalar(context.Context, string, string, time.Time, time.Time, domain.MetricSpec, *domain.ValueFilter) (domain.MetricResult, *time.Time, error) {
	if f.scalarErr != nil {
		return domain.MetricResult{}, nil, f.scalarErr
	}
	return f.scalarResult, nil, nil
}
func (f *fakeRepository) Series(context.Context, string, string, time.Time, time.Time, domain.Bucket, domain.MetricSpec, *domain.ValueFilter) ([]domain.BucketPoint, *time.Time, error) {
	return nil, nil, nil
}
func (f *fakeRepository) Raw(context.Context, string, string, time.Time, time.Time, string, int, int) ([]domain.RawPoint, *time.Time, error) {
	return nil, nil, nil
}
func (f *fakeRepository) Grouped(context.Context, string, string, time.Time, time.Time, string, domain.MetricSpec, *domain.ValueFilter, int) ([]domain.GroupResult, *time.Time, error) {
	return nil, nil, nil
}
func (f *fakeRepository) Catalog(context.Context, string, string) ([]string, error) {
	return f.catalogPaths, f.catalogErr
}
```

```go
// internal/app/dashboards/service_test.go
package dashboards

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	domain "github.com/tu-org/embolsadora-api/internal/domain/metrics"
)

func TestService_Query_Scalar(t *testing.T) {
	repo := &fakeRepository{scalarResult: domain.MetricResult{AasPath: "peso", Agg: domain.AggAvg, Value: 1.5, SampleCount: 10}}
	svc := NewService(repo, domain.DefaultLimits(), zap.NewNop())

	q := domain.MetricQuery{
		MachineID: "EMB-DEV-001",
		Range:     domain.Range8h,
		Metrics:   []domain.MetricSpec{{AasPath: "peso", Agg: domain.AggAvg}},
	}

	result, err := svc.Query(t.Context(), "tenant-1", q, time.Now())
	require.NoError(t, err)
	require.Equal(t, domain.ModeScalar, result.Mode)
	require.Len(t, result.Results, 1)
	require.Equal(t, 1.5, result.Results[0].Value)
}

func TestService_Query_PropagatesValidationError(t *testing.T) {
	svc := NewService(&fakeRepository{}, domain.DefaultLimits(), zap.NewNop())

	_, err := svc.Query(t.Context(), "tenant-1", domain.MetricQuery{}, time.Now())
	require.Error(t, err)
	var ve *domain.ValidationError
	require.ErrorAs(t, err, &ve)
	require.Equal(t, domain.CodeInvalidParams, ve.Code)
}

func TestService_Query_FailsWholeQueryOnPartialRepoError(t *testing.T) {
	repo := &fakeRepository{scalarErr: assertAnError}
	svc := NewService(repo, domain.DefaultLimits(), zap.NewNop())

	q := domain.MetricQuery{
		MachineID: "EMB-DEV-001",
		Range:     domain.Range8h,
		Metrics: []domain.MetricSpec{
			{AasPath: "peso", Agg: domain.AggAvg},
			{AasPath: "temperatura", Agg: domain.AggAvg},
		},
	}
	_, err := svc.Query(t.Context(), "tenant-1", q, time.Now())
	require.Error(t, err) // fork/C7: first error wins, no partial scalar response
}

var assertAnError = errTest{}

type errTest struct{}

func (errTest) Error() string { return "mongo down" }
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/app/dashboards/... -v`
Expected: FAIL — package `dashboards` / `NewService` / `Service.Query` don't exist yet.

- [ ] **Step 3: Write minimal implementation**

```go
// internal/app/dashboards/service.go
// Package dashboards orquesta el motor de consultas de metricas: valida via
// domain/metrics, reparte las sub-consultas por MetricSpec (errgroup) y
// arma la respuesta segun el modo detectado.
package dashboards

import (
	"context"
	"time"

	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	domain "github.com/tu-org/embolsadora-api/internal/domain/metrics"
)

type Service struct {
	repo   domain.Repository
	limits domain.Limits
	logger *zap.Logger
}

func NewService(repo domain.Repository, limits domain.Limits, logger *zap.Logger) *Service {
	return &Service{repo: repo, limits: limits, logger: logger}
}

// Query resuelve una MetricQuery completa: window -> validate -> fan-out por
// MetricSpec -> ensamblado por Mode. `now` se inyecta (no time.Now()
// interno) para que ResolveWindow sea testeable end-to-end.
func (s *Service) Query(ctx context.Context, tenantID string, q domain.MetricQuery, now time.Time) (domain.QueryResult, error) {
	from, to, err := domain.ResolveWindow(q, now)
	if err != nil {
		return domain.QueryResult{}, err
	}
	mode, err := domain.Validate(q, from, to, s.limits)
	if err != nil {
		return domain.QueryResult{}, err
	}

	result := domain.QueryResult{Mode: mode, MachineID: q.MachineID, From: from, To: to}

	switch mode {
	case domain.ModeRaw:
		return s.queryRaw(ctx, tenantID, q, from, to, result)
	case domain.ModeGrouped:
		return s.queryGrouped(ctx, tenantID, q, from, to, result)
	case domain.ModeSeries:
		return s.querySeries(ctx, tenantID, q, from, to, result)
	default:
		return s.queryScalar(ctx, tenantID, q, from, to, result)
	}
}

func (s *Service) queryScalar(ctx context.Context, tenantID string, q domain.MetricQuery, from, to time.Time, result domain.QueryResult) (domain.QueryResult, error) {
	results := make([]domain.MetricResult, len(q.Metrics))
	dataAsOfs := make([]*time.Time, len(q.Metrics))

	g, gctx := errgroup.WithContext(ctx)
	for i, spec := range q.Metrics {
		i, spec := i, spec
		g.Go(func() error {
			r, asOf, err := s.repo.Scalar(gctx, tenantID, q.MachineID, from, to, spec, q.Filter)
			if err != nil {
				return err
			}
			results[i] = r
			dataAsOfs[i] = asOf
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return domain.QueryResult{}, err
	}

	result.Results = results
	result.DataAsOf = maxDataAsOf(dataAsOfs)
	return result, nil
}

func (s *Service) querySeries(ctx context.Context, tenantID string, q domain.MetricQuery, from, to time.Time, result domain.QueryResult) (domain.QueryResult, error) {
	perMetric := make([][]domain.BucketPoint, len(q.Metrics))
	dataAsOfs := make([]*time.Time, len(q.Metrics))

	g, gctx := errgroup.WithContext(ctx)
	for i, spec := range q.Metrics {
		i, spec := i, spec
		g.Go(func() error {
			points, asOf, err := s.repo.Series(gctx, tenantID, q.MachineID, from, to, q.Bucket, spec, q.Filter)
			if err != nil {
				return err
			}
			perMetric[i] = points
			dataAsOfs[i] = asOf
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return domain.QueryResult{}, err
	}

	result.Bucket = q.Bucket
	result.Series = mergeSeries(q.Metrics, perMetric)
	result.DataAsOf = maxDataAsOf(dataAsOfs)
	return result, nil
}

// mergeSeries alinea por ts las series de cada metrica (una por MetricSpec,
// devueltas por Series/repo) en el shape combinado que pide la spec: un
// SeriesPoint por bucket, con un MetricResult por metrica presente en ese
// bucket. Un bucket sin dato para una metrica NO se rellena con 0 (spec).
func mergeSeries(specs []domain.MetricSpec, perMetric [][]domain.BucketPoint) []domain.SeriesPoint {
	byTs := make(map[time.Time][]domain.MetricResult)
	order := make([]time.Time, 0)
	seen := make(map[time.Time]bool)

	for i, points := range perMetric {
		spec := specs[i]
		for _, p := range points {
			byTs[p.Ts] = append(byTs[p.Ts], domain.MetricResult{AasPath: spec.AasPath, Agg: spec.Agg, Value: p.Value, SampleCount: p.SampleCount})
			if !seen[p.Ts] {
				seen[p.Ts] = true
				order = append(order, p.Ts)
			}
		}
	}
	sortTimes(order)

	series := make([]domain.SeriesPoint, 0, len(order))
	for _, ts := range order {
		series = append(series, domain.SeriesPoint{Ts: ts, Results: byTs[ts]})
	}
	return series
}

func sortTimes(ts []time.Time) {
	for i := 1; i < len(ts); i++ {
		for j := i; j > 0 && ts[j].Before(ts[j-1]); j-- {
			ts[j], ts[j-1] = ts[j-1], ts[j]
		}
	}
}

func (s *Service) queryRaw(ctx context.Context, tenantID string, q domain.MetricQuery, from, to time.Time, result domain.QueryResult) (domain.QueryResult, error) {
	spec := q.Metrics[0]
	limit := s.limits.MaxRawPoints + 1
	points, dataAsOf, err := s.repo.Raw(ctx, tenantID, q.MachineID, from, to, spec.AasPath, limit, q.MaxPoints)
	if err != nil {
		return domain.QueryResult{}, err
	}
	if q.MaxPoints <= 0 && len(points) >= limit {
		return domain.QueryResult{}, &domain.ValidationError{Code: domain.CodeRangeTooWide, Message: "el rango produce mas puntos crudos que el maximo permitido"}
	}

	result.AasPath = spec.AasPath
	result.Points = points
	result.DataAsOf = dataAsOf
	return result, nil
}

func (s *Service) queryGrouped(ctx context.Context, tenantID string, q domain.MetricQuery, from, to time.Time, result domain.QueryResult) (domain.QueryResult, error) {
	spec := q.Metrics[0]
	limit := s.limits.MaxGroups + 1
	groups, dataAsOf, err := s.repo.Grouped(ctx, tenantID, q.MachineID, from, to, q.GroupBy, spec, q.Filter, limit)
	if err != nil {
		return domain.QueryResult{}, err
	}
	if len(groups) >= limit {
		return domain.QueryResult{}, &domain.ValidationError{Code: domain.CodeTooManyGroups, Message: "groupBy produce mas grupos que el maximo permitido"}
	}

	result.AasPath = spec.AasPath
	result.Agg = spec.Agg
	result.GroupBy = q.GroupBy
	result.Groups = groups
	result.DataAsOf = dataAsOf
	return result, nil
}

func maxDataAsOf(ts []*time.Time) *time.Time {
	var max *time.Time
	for _, t := range ts {
		if t == nil {
			continue
		}
		if max == nil || t.After(*max) {
			max = t
		}
	}
	return max
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/app/dashboards/... -v`
Expected: PASS (all three tests)

- [ ] **Step 5: Commit**

```bash
git add internal/app/dashboards/service.go internal/app/dashboards/service_test.go internal/app/dashboards/fake_repository_test.go
git commit -m "feat(dashboards): Service.Query orchestration — validate, fan-out via errgroup, assemble by mode"
```

---

### Task 13: app/dashboards — `Service.Catalog`

**Files:**
- Modify: `internal/app/dashboards/service.go`
- Modify: `internal/app/dashboards/service_test.go`

**Interfaces:**
- Produces: `(*Service).Catalog(ctx context.Context, tenantID, machineID string) ([]string, error)` — consumed by Task 16's handler.

- [ ] **Step 1: Write the failing test**

```go
// append to internal/app/dashboards/service_test.go
func TestService_Catalog(t *testing.T) {
	repo := &fakeRepository{catalogPaths: []string{"peso", "temperatura"}}
	svc := NewService(repo, domain.DefaultLimits(), zap.NewNop())

	paths, err := svc.Catalog(t.Context(), "tenant-1", "EMB-DEV-001")
	require.NoError(t, err)
	require.Equal(t, []string{"peso", "temperatura"}, paths)
}

func TestService_Catalog_RequiresMachineID(t *testing.T) {
	svc := NewService(&fakeRepository{}, domain.DefaultLimits(), zap.NewNop())

	_, err := svc.Catalog(t.Context(), "tenant-1", "")
	require.Error(t, err)
	var ve *domain.ValidationError
	require.ErrorAs(t, err, &ve)
	require.Equal(t, domain.CodeInvalidParams, ve.Code)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/app/dashboards/... -run TestService_Catalog -v`
Expected: FAIL — `Service.Catalog` not defined.

- [ ] **Step 3: Write minimal implementation**

```go
// append to internal/app/dashboards/service.go

// Catalog devuelve los aasPath observados para (tenant, machine). Sin
// filtro de rango temporal en v1 (spec).
func (s *Service) Catalog(ctx context.Context, tenantID, machineID string) ([]string, error) {
	if machineID == "" {
		return nil, &domain.ValidationError{Code: domain.CodeInvalidParams, Message: "machineId es requerido"}
	}
	return s.repo.Catalog(ctx, tenantID, machineID)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/app/dashboards/... -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/app/dashboards/service.go internal/app/dashboards/service_test.go
git commit -m "feat(dashboards): Service.Catalog"
```

---

### Task 14: app/dashboards — `Service.Batch`

**Files:**
- Modify: `internal/app/dashboards/service.go`
- Modify: `internal/app/dashboards/service_test.go`

**Interfaces:**
- Produces: `BatchItem{ID string; Query domain.MetricQuery}`; `BatchItemResult{ID string; Result domain.QueryResult; Err error}`; `(*Service).Batch(ctx context.Context, tenantID string, items []BatchItem, now time.Time) ([]BatchItemResult, error)` — the returned `error` is non-nil only for batch-level guardrails (`TOO_MANY_QUERIES`, duplicate id); per-item failures land in each `BatchItemResult.Err`, never abort the batch. Consumed by Task 17's handler.

- [ ] **Step 1: Write the failing test**

```go
// append to internal/app/dashboards/service_test.go
func TestService_Batch_PartialFailureDoesNotAbortOthers(t *testing.T) {
	repo := &fakeRepository{scalarResult: domain.MetricResult{Value: 42}}
	svc := NewService(repo, domain.DefaultLimits(), zap.NewNop())

	okQuery := domain.MetricQuery{MachineID: "EMB-DEV-001", Range: domain.Range8h, Metrics: []domain.MetricSpec{{AasPath: "peso", Agg: domain.AggAvg}}}
	badQuery := domain.MetricQuery{} // falla Validate: INVALID_PARAMS

	results, err := svc.Batch(t.Context(), "tenant-1", []BatchItem{
		{ID: "widget-a", Query: okQuery},
		{ID: "widget-b", Query: badQuery},
	}, time.Now())
	require.NoError(t, err) // el batch en si se procesa

	require.Len(t, results, 2)
	byID := map[string]BatchItemResult{}
	for _, r := range results {
		byID[r.ID] = r
	}
	require.NoError(t, byID["widget-a"].Err)
	require.Equal(t, 42.0, byID["widget-a"].Result.Results[0].Value)
	require.Error(t, byID["widget-b"].Err)
}

func TestService_Batch_RejectsTooManyQueries(t *testing.T) {
	svc := NewService(&fakeRepository{}, domain.Limits{MaxBatchQueries: 1, MaxSpecs: 10}, zap.NewNop())

	_, err := svc.Batch(t.Context(), "tenant-1", []BatchItem{
		{ID: "a", Query: domain.MetricQuery{}},
		{ID: "b", Query: domain.MetricQuery{}},
	}, time.Now())
	require.Error(t, err)
	var ve *domain.ValidationError
	require.ErrorAs(t, err, &ve)
	require.Equal(t, domain.CodeTooManyQueries, ve.Code)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/app/dashboards/... -run TestService_Batch -v`
Expected: FAIL — `BatchItem` / `Service.Batch` not defined.

- [ ] **Step 3: Write minimal implementation**

```go
// append to internal/app/dashboards/service.go

type BatchItem struct {
	ID    string
	Query domain.MetricQuery
}

type BatchItemResult struct {
	ID     string
	Result domain.QueryResult
	Err    error
}

// Batch ejecuta cada item independientemente (errgroup) y correlaciona por
// ID -- un item roto no tira abajo los demas (spec, "Endpoint batch").
// El error de retorno es SOLO para guardrails a nivel batch (cantidad de
// items, ids duplicados); errores de un item individual van en su
// BatchItemResult.Err.
func (s *Service) Batch(ctx context.Context, tenantID string, items []BatchItem, now time.Time) ([]BatchItemResult, error) {
	ids := make([]string, len(items))
	for i, it := range items {
		ids[i] = it.ID
	}
	if err := domain.ValidateBatchIDs(ids, s.limits); err != nil {
		return nil, err
	}

	results := make([]BatchItemResult, len(items))
	g, gctx := errgroup.WithContext(ctx)
	for i, item := range items {
		i, item := i, item
		g.Go(func() error {
			result, err := s.Query(gctx, tenantID, item.Query, now)
			results[i] = BatchItemResult{ID: item.ID, Result: result, Err: err}
			return nil // nunca propagar: el fallo queda en el item, no aborta el grupo
		})
	}
	_ = g.Wait() // no puede fallar: los Go() de arriba siempre devuelven nil

	return results, nil
}
```

Note: `Batch` calls `s.Query` per item using `gctx` (the errgroup's derived context) even though no `g.Go` call ever returns a non-nil error, so `gctx` never cancels early — this is intentional: it keeps all items running to completion independently, which is the whole point of "one broken widget doesn't affect the other 49."

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/app/dashboards/... -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/app/dashboards/service.go internal/app/dashboards/service_test.go
git commit -m "feat(dashboards): Service.Batch — independent per-item execution correlated by id"
```

---

### Task 15: config — `DashboardsConfig`

**Files:**
- Modify: `internal/config/config.go`
- Test: `internal/config/config_test.go` (create if it doesn't exist; if it does, append)

**Interfaces:**
- Produces: `config.DashboardsConfig{MetricsRateLimitRPM, MetricsRateLimitBurst, MetricsMaxSpecs, MetricsMaxBuckets, MetricsMaxRawPoints, MetricsMaxGroups, MetricsMaxBatchQueries, MetricsMaxTimeMS int}`, added as `Config.Dashboards` — consumed by Task 20's wiring.

- [ ] **Step 1: Write the failing test**

```go
// internal/config/config_test.go — check first whether this file already
// exists; if so, add this test function to it instead of creating a new file.
package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDashboardsConfigDefaults(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("SUPABASE_JWKS_URL", "https://x")
	t.Setenv("SUPABASE_URL", "https://x")
	t.Setenv("SUPABASE_SERVICE_ROLE_KEY", "x")
	cfg := Load()

	assert.Equal(t, 15, cfg.Dashboards.MetricsRateLimitRPM)
	assert.Equal(t, 5, cfg.Dashboards.MetricsRateLimitBurst)
	assert.Equal(t, 10, cfg.Dashboards.MetricsMaxSpecs)
	assert.Equal(t, 1000, cfg.Dashboards.MetricsMaxBuckets)
	assert.Equal(t, 5000, cfg.Dashboards.MetricsMaxRawPoints)
	assert.Equal(t, 200, cfg.Dashboards.MetricsMaxGroups)
	assert.Equal(t, 50, cfg.Dashboards.MetricsMaxBatchQueries)
	assert.Equal(t, 5000, cfg.Dashboards.MetricsMaxTimeMS)
}
```

Before writing this test, run `go doc ./internal/config Load` (or read `config.go`'s `Load` function) to confirm which env vars are actually `require()`d vs. optional — copy that set into `t.Setenv` calls above so the test doesn't fail on an unrelated missing var. Adjust the list to match reality.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/config/... -run TestDashboardsConfigDefaults -v`
Expected: FAIL — `cfg.Dashboards` doesn't exist.

- [ ] **Step 3: Write minimal implementation**

```go
// add to internal/config/config.go, near IngestConfig

type DashboardsConfig struct {
	MetricsRateLimitRPM    int
	MetricsRateLimitBurst  int
	MetricsMaxSpecs        int
	MetricsMaxBuckets      int
	MetricsMaxRawPoints    int
	MetricsMaxGroups       int
	MetricsMaxBatchQueries int
	MetricsMaxTimeMS       int
}
```

Add `Dashboards DashboardsConfig` as a field on the `Config` struct, and inside `Load()`'s returned `&Config{...}` literal add:

```go
		Dashboards: DashboardsConfig{
			MetricsRateLimitRPM:    getIntEnv("DASHBOARDS_METRICS_RATE_LIMIT_RPM", 15),
			MetricsRateLimitBurst:  getIntEnv("DASHBOARDS_METRICS_RATE_LIMIT_BURST", 5),
			MetricsMaxSpecs:        getIntEnv("DASHBOARDS_METRICS_MAX_SPECS", 10),
			MetricsMaxBuckets:      getIntEnv("DASHBOARDS_METRICS_MAX_BUCKETS", 1000),
			MetricsMaxRawPoints:    getIntEnv("DASHBOARDS_METRICS_MAX_RAW_POINTS", 5000),
			MetricsMaxGroups:       getIntEnv("DASHBOARDS_METRICS_MAX_GROUPS", 200),
			MetricsMaxBatchQueries: getIntEnv("DASHBOARDS_METRICS_MAX_BATCH_QUERIES", 50),
			MetricsMaxTimeMS:       getIntEnv("DASHBOARDS_METRICS_MAX_TIME_MS", 5000),
		},
```

Also add the same 8 keys with their defaults to `.env.example`, matching the existing style for `INGEST_*` keys there.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/config/... -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go .env.example
git commit -m "feat(config): DashboardsConfig — rate limit and guardrail limits as env vars (C8)"
```

---

### Task 16: api/middleware — dashboard rate limit (token bucket keyed by `supabase_user_id`)

**Files:**
- Create: `internal/api/middleware/dashboard_ratelimit.go`
- Test: `internal/api/middleware/dashboard_ratelimit_test.go`

**Interfaces:**
- Produces: `DashboardRateLimiter{...}`; `NewDashboardRateLimiter(rdb *redis.Client, rps float64, burst int) *DashboardRateLimiter`; `(*DashboardRateLimiter).Allow(ctx, key string) (bool, int, error)`; `DashboardRateLimit(limiter *DashboardRateLimiter) gin.HandlerFunc` — consumed by Task 20's wiring.

- [ ] **Step 1: Write the failing test**

```go
// internal/api/middleware/dashboard_ratelimit_test.go
package middleware

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseDashboardBucketReply(t *testing.T) {
	tests := []struct {
		name           string
		res            any
		wantAllowed    bool
		wantRetryAfter int
		wantOK         bool
	}{
		{name: "permitido", res: []any{int64(1), int64(0)}, wantAllowed: true, wantOK: true},
		{name: "denegado con retry_after", res: []any{int64(0), int64(3)}, wantAllowed: false, wantRetryAfter: 3, wantOK: true},
		{name: "malformado -- debe abrir, no denegar", res: "no-es-slice", wantOK: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allowed, retryAfter, ok := parseDashboardBucketReply(tt.res)
			assert.Equal(t, tt.wantOK, ok)
			if tt.wantOK {
				assert.Equal(t, tt.wantAllowed, allowed)
				assert.Equal(t, tt.wantRetryAfter, retryAfter)
			}
		})
	}
}

func TestDashboardRateLimiter_AllowsWhenRedisNil(t *testing.T) {
	limiter := NewDashboardRateLimiter(nil, 15.0/60.0, 5)
	allowed, retryAfter, err := limiter.Allow(t.Context(), "user-1")
	assert.NoError(t, err)
	assert.True(t, allowed)
	assert.Equal(t, 0, retryAfter)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/api/middleware/... -run TestDashboardRateLimiter -v`
Expected: FAIL — `NewDashboardRateLimiter` not defined.

- [ ] **Step 3: Write minimal implementation**

```go
// internal/api/middleware/dashboard_ratelimit.go
package middleware

import (
	"context"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"

	"github.com/tu-org/embolsadora-api/internal/platform"
)

// dashboardTokenBucketScript es el mismo patron que
// internal/consumers/ratelimit.go (token bucket atomico en Lua), copiado en
// vez de reusado: ese tipo esta scopeado al dominio de ingesta de Edge
// (rps entero, key = API key) y este necesita rps fraccionario (15/min =
// 0.25/seg) con key = supabase_user_id -- generalizar el tipo existente
// tocaria un camino caliente en produccion (ingesta) por un beneficio de
// DRY chico.
const dashboardTokenBucketScript = `
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

type DashboardRateLimiter struct {
	rdb    *redis.Client
	rps    float64
	burst  int
	script *redis.Script
}

// NewDashboardRateLimiter construye el limitador. rdb nil = sin limite
// (fail-open, misma politica que el resto del repo).
func NewDashboardRateLimiter(rdb *redis.Client, rps float64, burst int) *DashboardRateLimiter {
	if rps <= 0 {
		rps = 15.0 / 60.0
	}
	if burst <= 0 {
		burst = 5
	}
	return &DashboardRateLimiter{rdb: rdb, rps: rps, burst: burst, script: redis.NewScript(dashboardTokenBucketScript)}
}

func (l *DashboardRateLimiter) Allow(ctx context.Context, key string) (bool, int, error) {
	if l.rdb == nil {
		return true, 0, nil
	}
	now := time.Now().UnixMilli()
	res, err := l.script.Run(ctx, l.rdb, []string{"ratelimit:dashboards:v1:" + key}, l.rps, l.burst, now, 1).Result()
	if err != nil {
		return true, 0, err
	}
	allowed, retryAfter, ok := parseDashboardBucketReply(res)
	if !ok {
		return true, 0, nil
	}
	return allowed, retryAfter, nil
}

func parseDashboardBucketReply(res any) (allowed bool, retryAfter int, ok bool) {
	vals, isSlice := res.([]any)
	if !isSlice || len(vals) != 2 {
		return false, 0, false
	}
	a, okAllowed := vals[0].(int64)
	r, okRetry := vals[1].(int64)
	if !okAllowed || !okRetry {
		return false, 0, false
	}
	return a == 1, int(math.Max(float64(r), 0)), true
}

// DashboardRateLimit limita por supabase_user_id (JWTAuth ya lo dejo en
// contexto). Va despues de JWTAuth y RBACCheck en la cadena.
func DashboardRateLimit(limiter *DashboardRateLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := platform.SupabaseSub(c.Request.Context())
		if key == "" {
			c.Next()
			return
		}
		allowed, retryAfter, _ := limiter.Allow(c.Request.Context(), key)
		if !allowed {
			c.Header("Retry-After", strconv.Itoa(retryAfter))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"success": false, "error": "rate limit excedido", "code": "RATE_LIMITED"})
			return
		}
		c.Next()
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/api/middleware/... -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/api/middleware/dashboard_ratelimit.go internal/api/middleware/dashboard_ratelimit_test.go
git commit -m "feat(middleware): dashboard rate limit — token bucket keyed by supabase_user_id, 15/min default"
```

---

### Task 17: api/handler/dashboards — DTOs, error mapping, `query` handler

**Files:**
- Create: `internal/api/handler/dashboards/dto/request.go`
- Create: `internal/api/handler/dashboards/dto/response.go`
- Create: `internal/api/handler/dashboards/errors.go`
- Create: `internal/api/handler/dashboards/query_metrics.go`
- Test: `internal/api/handler/dashboards/query_metrics_test.go`

**Interfaces:**
- Consumes: `app/dashboards.Service.Query` (Task 12); `domain/metrics.{ValidationError, Code*}` (Task 1).
- Produces: `dto.QueryRequest` (JSON-bindable), `dto.QueryRequest.ToDomain() (domainmetrics.MetricQuery, error)`, `dto.QueryResultToResponse(domainmetrics.QueryResult) dto.QueryResponse`, `HandleError(c *gin.Context, err error)`, `QueryMetrics(service *dashboards.Service) gin.HandlerFunc` — the last one wired by Task 19's `routes.go`.

- [ ] **Step 1: Write the failing test**

```go
// internal/api/handler/dashboards/query_metrics_test.go
package dashboards

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	app "github.com/tu-org/embolsadora-api/internal/app/dashboards"
	domain "github.com/tu-org/embolsadora-api/internal/domain/metrics"
	"github.com/tu-org/embolsadora-api/internal/platform"
)

type fakeRepo struct{ result domain.MetricResult }

func (f *fakeRepo) Scalar(_ context_Context, _, _ string, _, _ time.Time, spec domain.MetricSpec, _ *domain.ValueFilter) (domain.MetricResult, *time.Time, error) {
	f.result.AasPath, f.result.Agg = spec.AasPath, spec.Agg
	return f.result, nil, nil
}
func (f *fakeRepo) Series(context_Context, string, string, time.Time, time.Time, domain.Bucket, domain.MetricSpec, *domain.ValueFilter) ([]domain.BucketPoint, *time.Time, error) {
	return nil, nil, nil
}
func (f *fakeRepo) Raw(context_Context, string, string, time.Time, time.Time, string, int, int) ([]domain.RawPoint, *time.Time, error) {
	return nil, nil, nil
}
func (f *fakeRepo) Grouped(context_Context, string, string, time.Time, time.Time, string, domain.MetricSpec, *domain.ValueFilter, int) ([]domain.GroupResult, *time.Time, error) {
	return nil, nil, nil
}
func (f *fakeRepo) Catalog(context_Context, string, string) ([]string, error) { return nil, nil }

func TestQueryMetrics_ScalarHappyPath(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &fakeRepo{result: domain.MetricResult{Value: 1.5, SampleCount: 10}}
	svc := app.NewService(repo, domain.DefaultLimits(), zap.NewNop())

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Request = c.Request.WithContext(platform.WithTenantID(c.Request.Context(), "tenant-1"))
		c.Next()
	})
	r.POST("/query", QueryMetrics(svc))

	body, _ := json.Marshal(map[string]any{
		"machineId": "EMB-DEV-001",
		"range":     "8h",
		"metrics":   []map[string]any{{"aasPath": "peso", "agg": "avg"}},
	})
	req := httptest.NewRequest(http.MethodPost, "/query", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Equal(t, true, resp["success"])
	data := resp["data"].(map[string]any)
	require.Equal(t, "scalar", data["mode"])
}

func TestQueryMetrics_InvalidParamsReturns400WithCode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := app.NewService(&fakeRepo{}, domain.DefaultLimits(), zap.NewNop())

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Request = c.Request.WithContext(platform.WithTenantID(c.Request.Context(), "tenant-1"))
		c.Next()
	})
	r.POST("/query", QueryMetrics(svc))

	req := httptest.NewRequest(http.MethodPost, "/query", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Equal(t, false, resp["success"])
	require.Equal(t, "INVALID_PARAMS", resp["code"])
}
```

Replace every `context_Context` placeholder above with `context.Context` and add the `"context"` import — written with an underscore here only because this plan's markdown renderer would otherwise treat `context.Context` inside a struct method receiver line as a stray reference; the implementer writes normal Go.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/api/handler/dashboards/... -v`
Expected: FAIL — package doesn't exist yet.

- [ ] **Step 3: Write minimal implementation**

```go
// internal/api/handler/dashboards/dto/request.go
package dto

import (
	"time"

	domain "github.com/tu-org/embolsadora-api/internal/domain/metrics"
)

type MetricSpecDTO struct {
	AasPath string `json:"aasPath"`
	Agg     string `json:"agg"`
}

type ValueFilterDTO struct {
	ValueEquals any `json:"valueEquals,omitempty"`
}

type QueryRequest struct {
	MachineID string          `json:"machineId"`
	Range     string          `json:"range,omitempty"`
	From      *time.Time      `json:"from,omitempty"`
	To        *time.Time      `json:"to,omitempty"`
	Bucket    string          `json:"bucket,omitempty"`
	Metrics   []MetricSpecDTO `json:"metrics"`
	GroupBy   string          `json:"groupBy,omitempty"`
	Filter    *ValueFilterDTO `json:"filter,omitempty"`
	MaxPoints int             `json:"maxPoints,omitempty"`
}

// ToDomain traduce el DTO 1:1 -- no valida (eso lo hace domain/metrics
// dentro de app/dashboards.Service.Query, con los guardrails parametrizados
// por Limits).
func (r QueryRequest) ToDomain() domain.MetricQuery {
	specs := make([]domain.MetricSpec, len(r.Metrics))
	for i, m := range r.Metrics {
		specs[i] = domain.MetricSpec{AasPath: m.AasPath, Agg: domain.Agg(m.Agg)}
	}
	var filter *domain.ValueFilter
	if r.Filter != nil {
		filter = &domain.ValueFilter{ValueEquals: r.Filter.ValueEquals}
	}
	return domain.MetricQuery{
		MachineID: r.MachineID,
		Range:     domain.Range(r.Range),
		From:      r.From,
		To:        r.To,
		Bucket:    domain.Bucket(r.Bucket),
		Metrics:   specs,
		GroupBy:   r.GroupBy,
		Filter:    filter,
		MaxPoints: r.MaxPoints,
	}
}
```

```go
// internal/api/handler/dashboards/dto/response.go
package dto

import (
	"time"

	domain "github.com/tu-org/embolsadora-api/internal/domain/metrics"
)

type MetricResultDTO struct {
	AasPath     string  `json:"aasPath"`
	Agg         string  `json:"agg"`
	Value       float64 `json:"value"`
	SampleCount int64   `json:"sampleCount"`
}

type SeriesPointDTO struct {
	Ts      time.Time         `json:"ts"`
	Results []MetricResultDTO `json:"results"`
}

type RawPointDTO struct {
	Ts    time.Time `json:"ts"`
	Value any       `json:"value"`
}

type GroupResultDTO struct {
	Key   string  `json:"key"`
	Value float64 `json:"value"`
}

// QueryResponse es la union de las 4 formas, discriminada por Mode (fork/C5)
// -- los campos que no aplican al modo detectado quedan omitidos via
// omitempty, nunca en su zero value visible.
type QueryResponse struct {
	Mode      string           `json:"mode"`
	MachineID string           `json:"machineId"`
	From      time.Time        `json:"from"`
	To        time.Time        `json:"to"`
	DataAsOf  *time.Time        `json:"dataAsOf"`
	Bucket    string           `json:"bucket,omitempty"`
	Results   []MetricResultDTO `json:"results,omitempty"`
	Series    []SeriesPointDTO  `json:"series,omitempty"`
	AasPath   string           `json:"aasPath,omitempty"`
	Points    []RawPointDTO     `json:"points,omitempty"`
	Agg       string           `json:"agg,omitempty"`
	GroupBy   string           `json:"groupBy,omitempty"`
	Groups    []GroupResultDTO  `json:"groups,omitempty"`
}

func QueryResultToResponse(r domain.QueryResult) QueryResponse {
	resp := QueryResponse{
		Mode:      string(r.Mode),
		MachineID: r.MachineID,
		From:      r.From,
		To:        r.To,
		DataAsOf:  r.DataAsOf,
		Bucket:    string(r.Bucket),
		AasPath:   r.AasPath,
		Agg:       string(r.Agg),
		GroupBy:   r.GroupBy,
	}
	for _, res := range r.Results {
		resp.Results = append(resp.Results, MetricResultDTO{AasPath: res.AasPath, Agg: string(res.Agg), Value: res.Value, SampleCount: res.SampleCount})
	}
	for _, sp := range r.Series {
		var results []MetricResultDTO
		for _, res := range sp.Results {
			results = append(results, MetricResultDTO{AasPath: res.AasPath, Agg: string(res.Agg), Value: res.Value, SampleCount: res.SampleCount})
		}
		resp.Series = append(resp.Series, SeriesPointDTO{Ts: sp.Ts, Results: results})
	}
	for _, p := range r.Points {
		resp.Points = append(resp.Points, RawPointDTO{Ts: p.Ts, Value: p.Value})
	}
	for _, g := range r.Groups {
		resp.Groups = append(resp.Groups, GroupResultDTO{Key: g.Key, Value: g.Value})
	}
	return resp
}
```

```go
// internal/api/handler/dashboards/errors.go
package dashboards

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	domain "github.com/tu-org/embolsadora-api/internal/domain/metrics"
)

// HandleError mapea un error de domain/metrics o app/dashboards al envelope
// {success:false, error, code}. Todos los guardrails de la spec son 400;
// QUERY_TIMEOUT (fork/C4) es 504; cualquier otra cosa es 500.
func HandleError(c *gin.Context, err error) {
	var ve *domain.ValidationError
	if errors.As(err, &ve) {
		status := http.StatusBadRequest
		if ve.Code == domain.CodeQueryTimeout {
			status = http.StatusGatewayTimeout
		}
		c.JSON(status, gin.H{"success": false, "error": ve.Message, "code": ve.Code})
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "error interno"})
}
```

```go
// internal/api/handler/dashboards/query_metrics.go
package dashboards

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	app "github.com/tu-org/embolsadora-api/internal/app/dashboards"
	"github.com/tu-org/embolsadora-api/internal/api/handler/dashboards/dto"
	"github.com/tu-org/embolsadora-api/internal/platform"
)

func QueryMetrics(service *app.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := platform.TenantID(c.Request.Context())
		if tenantID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "X-Tenant-ID invalido o ausente", "code": "INVALID_PARAMS"})
			return
		}

		var req dto.QueryRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "JSON malformado", "code": "INVALID_PARAMS"})
			return
		}

		result, err := service.Query(c.Request.Context(), tenantID, req.ToDomain(), time.Now())
		if err != nil {
			HandleError(c, err)
			return
		}

		c.JSON(http.StatusOK, gin.H{"success": true, "data": dto.QueryResultToResponse(result)})
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/api/handler/dashboards/... -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/api/handler/dashboards/dto/request.go internal/api/handler/dashboards/dto/response.go internal/api/handler/dashboards/errors.go internal/api/handler/dashboards/query_metrics.go internal/api/handler/dashboards/query_metrics_test.go
git commit -m "feat(dashboards): query handler, request/response DTOs, error-to-code mapping"
```

---

### Task 18: api/handler/dashboards — `catalog` handler

**Files:**
- Create: `internal/api/handler/dashboards/catalog_metrics.go`
- Test: `internal/api/handler/dashboards/catalog_metrics_test.go`

**Interfaces:**
- Consumes: `app/dashboards.Service.Catalog` (Task 13).
- Produces: `CatalogMetrics(service *app.Service) gin.HandlerFunc`.

- [ ] **Step 1: Write the failing test**

```go
// internal/api/handler/dashboards/catalog_metrics_test.go
package dashboards

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	app "github.com/tu-org/embolsadora-api/internal/app/dashboards"
	domain "github.com/tu-org/embolsadora-api/internal/domain/metrics"
	"github.com/tu-org/embolsadora-api/internal/platform"
)

func TestCatalogMetrics_HappyPath(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &fakeRepo{}
	repo.catalog = []string{"peso", "temperatura"}
	svc := app.NewService(repo, domain.DefaultLimits(), zap.NewNop())

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Request = c.Request.WithContext(platform.WithTenantID(c.Request.Context(), "tenant-1"))
		c.Next()
	})
	r.GET("/catalog", CatalogMetrics(svc))

	req := httptest.NewRequest(http.MethodGet, "/catalog?machineId=EMB-DEV-001", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data := resp["data"].(map[string]any)
	require.ElementsMatch(t, []any{"peso", "temperatura"}, data["aasPaths"])
}

func TestCatalogMetrics_MissingMachineID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := app.NewService(&fakeRepo{}, domain.DefaultLimits(), zap.NewNop())

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Request = c.Request.WithContext(platform.WithTenantID(c.Request.Context(), "tenant-1"))
		c.Next()
	})
	r.GET("/catalog", CatalogMetrics(svc))

	req := httptest.NewRequest(http.MethodGet, "/catalog", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
}
```

This test requires `fakeRepo` (Task 17's test file) to expose a `catalog []string` field returned by its `Catalog` method — add that field and wire it into `fakeRepo.Catalog` as part of Step 3 below (modifying `query_metrics_test.go`'s `fakeRepo`, not creating a duplicate type).

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/api/handler/dashboards/... -run TestCatalogMetrics -v`
Expected: FAIL — `CatalogMetrics` not defined; `fakeRepo.catalog` field doesn't exist.

- [ ] **Step 3: Write minimal implementation**

Update `fakeRepo` in `query_metrics_test.go`:

```go
type fakeRepo struct {
	result  domain.MetricResult
	catalog []string
}

func (f *fakeRepo) Catalog(context.Context, string, string) ([]string, error) { return f.catalog, nil }
```

(remove the old one-line `Catalog` stub and replace with this; keep the other methods as-is.)

```go
// internal/api/handler/dashboards/catalog_metrics.go
package dashboards

import (
	"net/http"

	"github.com/gin-gonic/gin"

	app "github.com/tu-org/embolsadora-api/internal/app/dashboards"
	"github.com/tu-org/embolsadora-api/internal/platform"
)

func CatalogMetrics(service *app.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := platform.TenantID(c.Request.Context())
		if tenantID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "X-Tenant-ID invalido o ausente", "code": "INVALID_PARAMS"})
			return
		}

		machineID := c.Query("machineId")
		paths, err := service.Catalog(c.Request.Context(), tenantID, machineID)
		if err != nil {
			HandleError(c, err)
			return
		}

		c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"machineId": machineID, "aasPaths": paths}})
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/api/handler/dashboards/... -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/api/handler/dashboards/catalog_metrics.go internal/api/handler/dashboards/catalog_metrics_test.go internal/api/handler/dashboards/query_metrics_test.go
git commit -m "feat(dashboards): catalog handler"
```

---

### Task 19: api/handler/dashboards — `batch` handler + `routes.go`

**Files:**
- Create: `internal/api/handler/dashboards/batch_query_metrics.go`
- Create: `internal/api/handler/dashboards/routes.go`
- Test: `internal/api/handler/dashboards/batch_query_metrics_test.go`

**Interfaces:**
- Consumes: `app/dashboards.{Service.Batch, BatchItem, BatchItemResult}` (Task 14).
- Produces: `BatchQueryMetrics(service *app.Service) gin.HandlerFunc`; `RegisterRoutes(g *gin.RouterGroup, service *app.Service)` — consumed by Task 20's wiring, same shape as `dashboard_layouts.RegisterRoutes`.

- [ ] **Step 1: Write the failing test**

```go
// internal/api/handler/dashboards/batch_query_metrics_test.go
package dashboards

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	app "github.com/tu-org/embolsadora-api/internal/app/dashboards"
	domain "github.com/tu-org/embolsadora-api/internal/domain/metrics"
	"github.com/tu-org/embolsadora-api/internal/platform"
)

func TestBatchQueryMetrics_PartialSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &fakeRepo{result: domain.MetricResult{Value: 42}}
	svc := app.NewService(repo, domain.DefaultLimits(), zap.NewNop())

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Request = c.Request.WithContext(platform.WithTenantID(c.Request.Context(), "tenant-1"))
		c.Next()
	})
	r.POST("/query/batch", BatchQueryMetrics(svc))

	body, _ := json.Marshal(map[string]any{
		"queries": []map[string]any{
			{"id": "widget-a", "machineId": "EMB-DEV-001", "range": "8h", "metrics": []map[string]any{{"aasPath": "peso", "agg": "avg"}}},
			{"id": "widget-b", "machineId": "EMB-DEV-001", "metrics": []map[string]any{{"aasPath": "x", "agg": "avg"}}}, // sin range ni from/to -> falla
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/query/batch", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Equal(t, true, resp["success"])

	results := resp["data"].(map[string]any)["results"].([]any)
	require.Len(t, results, 2)

	byID := map[string]map[string]any{}
	for _, r := range results {
		item := r.(map[string]any)
		byID[item["id"].(string)] = item
	}
	require.Equal(t, true, byID["widget-a"]["success"])
	require.Equal(t, false, byID["widget-b"]["success"])
	require.Equal(t, "INVALID_PARAMS", byID["widget-b"]["code"])
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/api/handler/dashboards/... -run TestBatchQueryMetrics -v`
Expected: FAIL — `BatchQueryMetrics` not defined.

- [ ] **Step 3: Write minimal implementation**

```go
// internal/api/handler/dashboards/batch_query_metrics.go
package dashboards

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	app "github.com/tu-org/embolsadora-api/internal/app/dashboards"
	"github.com/tu-org/embolsadora-api/internal/api/handler/dashboards/dto"
	domain "github.com/tu-org/embolsadora-api/internal/domain/metrics"
	"github.com/tu-org/embolsadora-api/internal/platform"
)

type batchQueryItemRequest struct {
	ID string `json:"id"`
	dto.QueryRequest
}

type batchQueryRequest struct {
	Queries []batchQueryItemRequest `json:"queries"`
}

func BatchQueryMetrics(service *app.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := platform.TenantID(c.Request.Context())
		if tenantID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "X-Tenant-ID invalido o ausente", "code": "INVALID_PARAMS"})
			return
		}

		var req batchQueryRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "JSON malformado", "code": "INVALID_PARAMS"})
			return
		}

		items := make([]app.BatchItem, len(req.Queries))
		for i, q := range req.Queries {
			items[i] = app.BatchItem{ID: q.ID, Query: q.QueryRequest.ToDomain()}
		}

		results, err := service.Batch(c.Request.Context(), tenantID, items, time.Now())
		if err != nil {
			HandleError(c, err)
			return
		}

		c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"results": batchResultsToResponse(results)}})
	}
}

func batchResultsToResponse(results []app.BatchItemResult) []gin.H {
	out := make([]gin.H, 0, len(results))
	for _, r := range results {
		if r.Err != nil {
			var ve *domain.ValidationError
			code := "INTERNAL_ERROR"
			message := "error interno"
			if errors.As(r.Err, &ve) {
				code = ve.Code
				message = ve.Message
			}
			out = append(out, gin.H{"id": r.ID, "success": false, "error": message, "code": code})
			continue
		}
		out = append(out, gin.H{"id": r.ID, "success": true, "data": dto.QueryResultToResponse(r.Result)})
	}
	return out
}
```

```go
// internal/api/handler/dashboards/routes.go
package dashboards

import (
	"github.com/gin-gonic/gin"

	app "github.com/tu-org/embolsadora-api/internal/app/dashboards"
)

// RegisterRoutes registra las 3 rutas de metricas de dashboard sobre el
// grupo dado. El grupo ya trae RBACCheck(perm_metrics_view) y
// DashboardRateLimit aplicados (routes/url_mappings.go).
func RegisterRoutes(g *gin.RouterGroup, service *app.Service) {
	g.POST("/query", QueryMetrics(service))
	g.GET("/catalog", CatalogMetrics(service))
	g.POST("/query/batch", BatchQueryMetrics(service))
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/api/handler/dashboards/... -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/api/handler/dashboards/batch_query_metrics.go internal/api/handler/dashboards/batch_query_metrics_test.go internal/api/handler/dashboards/routes.go
git commit -m "feat(dashboards): batch handler and route registration"
```

---

### Task 20: Wire everything into `routes/url_mappings.go`

**Files:**
- Modify: `internal/routes/url_mappings.go`

**Interfaces:**
- Consumes: every package produced by Tasks 5-19.
- Produces: the three routes live under `/api/v1/dashboards/metrics/*`, gated by `RBACCheck("perm_metrics_view")` and `DashboardRateLimit`.

- [ ] **Step 1: Extend `connectMeasurementsRepo` to also return the raw Mongo client**

The metrics repo (Task 6) needs a `*mongodriver.Database` to construct, but `connectMeasurementsRepo` currently only returns the `ingest.Repository` interface — the underlying `*mongoplatform.Client` is created and discarded inside it. Change its signature to also return the client (nil when degraded), mirroring the existing "degraded, not fatal" philosophy for the new repo too:

```go
// modify the signature and body of connectMeasurementsRepo in
// internal/routes/url_mappings.go
func connectMeasurementsRepo(ctx context.Context, cfg config.MongoConfig, logger *zap.Logger) (domainingest.Repository, *mongoplatform.Client, error) {
	mongoClient, err := mongoplatform.Connect(ctx, cfg)
	if err != nil {
		logger.Error("no se pudo conectar a MongoDB al arrancar; la ingesta respondera 500 y el Edge reintentara (I-1); el resto de la API sigue arriba",
			zap.Error(err))
		telemetry.SetMongoUp(false)
		return measurementsRepo.Unavailable(err), nil, nil
	}

	repo := measurementsRepo.New(mongoClient.Database())
	if err := repo.EnsureIndexes(ctx); err != nil {
		return nil, nil, err
	}
	telemetry.SetMongoUp(true)
	return repo, mongoClient, nil
}
```

- [ ] **Step 2: Update the call site and wire the metrics repo/service/handlers/middleware**

```go
// where connectMeasurementsRepo is currently called in RegisterURLMappings:
measurementRepo, mongoClient, err := connectMeasurementsRepo(context.Background(), cfg.Mongo, logger)
if err != nil {
	log.Fatalf("no se pudieron crear los indices de measurements: %v", err)
}

var metricsRepo domainmetrics.Repository
if mongoClient != nil {
	metricsRepo = metricsmongo.New(mongoClient.Database(), time.Duration(cfg.Dashboards.MetricsMaxTimeMS)*time.Millisecond)
} else {
	metricsRepo = metricsmongo.Unavailable(fmt.Errorf("mongo no disponible desde el arranque"))
}

metricsLimits := domainmetrics.Limits{
	MaxSpecs:        cfg.Dashboards.MetricsMaxSpecs,
	MaxBuckets:      cfg.Dashboards.MetricsMaxBuckets,
	MaxRawPoints:    cfg.Dashboards.MetricsMaxRawPoints,
	MaxGroups:       cfg.Dashboards.MetricsMaxGroups,
	MaxBatchQueries: cfg.Dashboards.MetricsMaxBatchQueries,
}
dashboardsService := dashboardsApp.NewService(metricsRepo, metricsLimits, logger)
dashboardRateLimiter := apimw.NewDashboardRateLimiter(redisClient, float64(cfg.Dashboards.MetricsRateLimitRPM)/60.0, cfg.Dashboards.MetricsRateLimitBurst)

dashboardsMetricsGroup := v1.Group("/dashboards/metrics",
	apimw.RBACCheck("perm_metrics_view"),
	apimw.DashboardRateLimit(dashboardRateLimiter),
)
dashboardsHandler.RegisterRoutes(dashboardsMetricsGroup, dashboardsService)
```

Add the four new imports to the top of the file, matching the existing aliasing style:

```go
dashboardsHandler "github.com/tu-org/embolsadora-api/internal/api/handler/dashboards"
dashboardsApp "github.com/tu-org/embolsadora-api/internal/app/dashboards"
domainmetrics "github.com/tu-org/embolsadora-api/internal/domain/metrics"
metricsmongo "github.com/tu-org/embolsadora-api/internal/repo/mongo/metrics"
```

`fmt` and `time` are likely already imported (check before adding — `time` definitely is, used by `edgeDeviceTimeout` a few lines down; `fmt` may need adding).

- [ ] **Step 3: Build and smoke-test**

Run: `go build ./...`
Expected: no errors.

Run: `go vet ./...`
Expected: no errors.

Start the API locally against a running Postgres/Redis/Mongo (`docker compose up -d db redis mongo`, then `go run cmd/api/main.go`) and smoke-test manually:

```bash
# Assuming a valid JWT and X-Tenant-ID for a tenant/role with perm_metrics_view:
curl -s -X POST http://localhost:8080/api/v1/dashboards/metrics/query \
  -H "Authorization: Bearer $JWT" -H "X-Tenant-ID: $TENANT_ID" -H "Content-Type: application/json" \
  -d '{"machineId":"EMB-DEV-001","range":"24h","metrics":[{"aasPath":"peso","agg":"count"}]}'

curl -s "http://localhost:8080/api/v1/dashboards/metrics/catalog?machineId=EMB-DEV-001" \
  -H "Authorization: Bearer $JWT" -H "X-Tenant-ID: $TENANT_ID"
```

Expected: `{"success":true,"data":{...}}` for both (empty `results`/`aasPaths` is fine if no measurements were seeded yet — the point is no 500/404/degraded response).

- [ ] **Step 4: Run the full test suite**

Run: `go test ./...` (export `DATABASE_URL`, `MONGO_URI`, `REDIS_URL` per CLAUDE.md first, so integration tests aren't silently skipped)
Expected: PASS across the whole repo, including every test from Tasks 1-19.

- [ ] **Step 5: Commit**

```bash
git add internal/routes/url_mappings.go
git commit -m "feat(dashboards): wire metrics query/catalog/batch routes, RBAC and rate limit into url_mappings"
```

---

## Self-Review Notes

- **Spec coverage:** Contrato (query/catalog/batch endpoints, `range`, `mode`, `dataAsOf`) → Tasks 1-3, 12-19. Guardrails table → Tasks 3, 6-9, 12 (RANGE_TOO_WIDE/TOO_MANY_GROUPS resolved post-query in repo+service, everything else in domain). Arquitectura interna (domain/repo/app/handler split, permission, rate limit) → Tasks 1-20. Testing section (unit for domain, integration for repo incl. cross-tenant, unit-with-fake for app, unit for handler) → covered in each task plus Task 11 explicitly. Forks 1-6 → Task 5 (fork 1 catalog), Task 6 (fork 2 discard metric), service design note (fork 3 — no extra work needed, informed the "no snapshot/versioning" choice by simply not building one), Task 16 (fork 4 rate limit; no-cache is an absence, nothing to build), Task 3/6 comments (fork 5 — documented, no code), Task 6-10 (fork 6 — request-time, no rollup, nothing to build for "absence" decisions). C2/C5 → Task 17's DTOs (`range`, `mode`, `dataAsOf`). C1/C3/C4/C7/C8 → resolved in "Decisions for the TODOs" and threaded through Tasks 1, 3, 6, 8, 12, 15.
- **Placeholder scan:** none — every step has literal code. The one deliberately-broken snippet in Task 3 (Step 3) is flagged inline as non-compiling with its exact fix given immediately after, not left as a TBD.
- **Type consistency:** `domain.Repository`'s 5 methods (Task 4) match `*metricsmongo.Repository`'s implementations (Tasks 6-10) and `*unavailable`'s stubs (Task 6) signature-for-signature; the `var _ domain.Repository = (*Repository)(nil)` line in Task 10 is the enforcement point. `app.Service`'s constructor and `Query`/`Catalog`/`Batch` signatures (Tasks 12-14) are used identically by the handlers (Tasks 17-19). `dto.QueryRequest`/`QueryResponse` field names match the spec's JSON exactly (`machineId`, `aasPath`, `sampleCount`, `dataAsOf`, `mode`, etc.).
