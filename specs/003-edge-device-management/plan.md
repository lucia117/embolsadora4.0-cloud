# Implementation Plan: Edge Device Management API

**Branch**: `003-edge-device-management` | **Date**: 2026-03-09 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/003-edge-device-management/spec.md`

---

## Summary

Implement a full CRUD + lifecycle management API for edge devices (Raspberry Pi + PLC units) deployed at industrial sites. The API honors the consumer-driven pact contract from `embolsadora-frontend`, exposing endpoints under `/api/tenants/{tenantId}/...` with JWT Bearer authentication. Beyond CRUD, the API acts as a proxy for on-demand device checks (status, health) and live telemetry retrieval, and persists check results as immutable audit events.

---

## Technical Context

**Language/Version**: Go 1.24+
**Primary Dependencies**: Gin (HTTP), pgx/v5 (PostgreSQL), Zap (logging), Prometheus (metrics), net/http stdlib (edge client), testify + uber/mock (testing)
**Storage**: PostgreSQL — two new tables: `edge_devices`, `device_events`
**Testing**: `testing` package + testify + uber/mock; Docker containers for integration tests
**Target Platform**: Linux server (Docker/Cloud Run)
**Project Type**: Web service — monolith modular (hexagonal)
**Performance Goals**: List < 500ms p95; single device retrieval < 200ms; device checks < 5s (device-bound)
**Constraints**: Multi-tenant isolation mandatory; JWT auth required on all endpoints; no cross-tenant data leakage
**Scale/Scope**: Up to 1,000 devices per tenant; tens of check events per device per day

---

## Constitution Check

*GATE: Pre-design verification against Constitution v1.1.0*

| Principle | Status | Notes |
|-----------|--------|-------|
| I — Hexagonal Architecture | ✅ Pass | New layers follow `transport → app → domain ← repo` pattern; `platform/edgeclient` package for outbound HTTP |
| II — Security / Tenant Isolation | ✅ Pass | JWT Bearer on all routes; tenantId resolved from path (subdomain); all DB queries include tenant_id; RBAC for write ops |
| III — Observability | ✅ Pass | Zap logging + Prometheus counters/histograms required in each phase |
| IV — Integration Testing | ✅ Pass | Migration tests + contract tests (pact) required |
| V — Semantic Versioning | ✅ Pass | New endpoints (MINOR); no breaking changes to existing contracts |

**Gate Result**: PASS — no violations. No Complexity Tracking required.

---

## Project Structure

### Documentation (this feature)

```text
specs/003-edge-device-management/
├── plan.md              # This file
├── research.md          # Phase 0 — design decisions
├── spec.md              # Feature specification
├── plan/
│   ├── data-model.md    # DB schema and entity definitions
│   └── quickstart.md    # Local dev guide
├── contracts/
│   └── edge-device-service-api.openapi.yaml
└── checklists/
    └── requirements.md
```

### Source Code Layout

```text
internal/
├── api/
│   ├── handler/
│   │   └── edge_devices/
│   │       ├── dto/
│   │       │   └── dto.go                    # Request/response types
│   │       ├── list_devices.go
│   │       ├── create_device.go
│   │       ├── get_device.go
│   │       ├── update_device.go
│   │       ├── enable_device.go
│   │       ├── disable_device.go
│   │       ├── status_check.go
│   │       ├── health_check.go
│   │       ├── get_telemetry.go
│   │       └── list_events.go
│   └── usecases/
│       └── edge_devices/
│           └── [one usecase per operation]   # Thin orchestration
├── app/
│   └── edge_devices/
│       └── service.go                        # Application service layer
├── domain/
│   └── edge_devices/
│       ├── edge_device.go                    # Domain aggregate + validation
│       ├── commands.go                       # CreateDeviceCommand, UpdateDeviceCommand
│       └── errors.go                         # Domain error types
├── repo/
│   └── pg/
│       └── edge_devices/
│           └── repository.go                 # PostgreSQL implementation
├── platform/
│   └── edgeclient/
│       ├── client.go                         # EdgeDeviceClient interface
│       └── http_client.go                    # net/http implementation
└── routes/
    └── url_mappings.go                       # New /api/tenants group registration

migrations/
├── 0005_create_edge_devices_tables.up.sql
└── 0005_create_edge_devices_tables.down.sql

tests/
└── integration/
    └── edge_devices/
        └── [integration tests]
```

**Structure Decision**: Single-project Go monolith following existing hexagonal pattern. New `platform/edgeclient` package introduced for outbound HTTP calls to Raspberry Pi devices. No new top-level projects added.

---

## Key Design Decisions

| Decision | Rationale | Trade-offs |
|----------|-----------|-----------|
| **Route prefix `/api/tenants/`** | Honor pact contract exactly — frontend built against this path | New Gin group alongside `/api/v1`; not a breaking change |
| **tenantId = subdomain slug** | Pact uses "acme" (string slug); tenants table has unique `subdomain` column | Requires one DB lookup per request to resolve UUID; minimal overhead |
| **EdgeDeviceClient interface** | Testable abstraction over outbound HTTP to Raspberry Pi | Extra interface; justified by testability and potential future circuit-breaker |
| **Status/health checks are synchronous** | MVP simplicity; direct proxy to device and return | Blocks request thread for up to 10s; async pattern recommended for v2 |
| **Telemetry not persisted** | Real-time snapshot; not an audit event | History only shows triggered checks, not telemetry readings |
| **Enable/disable idempotent** | Safer for frontend retry logic | No error on repeated same-state transitions |
| **machineId immutable** | Physical identifier should not change post-registration | UPDATE endpoint only allows name/description |

---

## I. Architecture & Design

### Architectural Alignment

- **Surface**: New Gin group `/api/tenants` — extends ABM surface auth model (JWT Bearer + CORS)
- **Authentication**: JWT Bearer; RBAC enforced at handler level (admin role for writes)
- **Tenant Isolation**: Subdomain slug in path → resolved to UUID via middleware; all DB queries include `tenant_id`
- **Layer Pattern**: `transport → app → domain ← repo | platform/edgeclient`
- **New I/O Pattern**: Outbound HTTP calls to edge devices via `platform/edgeclient.EdgeDeviceClient`

### Route Registration

```go
// internal/routes/url_mappings.go — new group
tenants := r.Group(
    "/api/tenants",
    apimw.RequestID(),
    apimw.Logger(),
    apimw.CORS(),
    apimw.JWTAuth(),
    apimw.ResolveTenantFromPath(),  // resolves :tenantId subdomain → UUID in context
)
edgeDevices.RegisterRoutes(tenants, edgeDevicesDeps)
```

---

## II. Implementation Phases

### Phase 1 — Database Migration

**Goal**: Create `edge_devices` and `device_events` tables.

**Files**:
- `migrations/0005_create_edge_devices_tables.up.sql`
- `migrations/0005_create_edge_devices_tables.down.sql`

**Key elements**:
- `edge_devices`: UUID PK, composite UNIQUE (tenant_id, machine_id), status CHECK constraint, auto-updated `updated_at` trigger
- `device_events`: UUID PK, FK → edge_devices, JSONB `details` column, insert-only
- Indexes for tenant-scoped queries and event history ordering

**Constitution gates**: Migration tests required against Docker Postgres.

---

### Phase 2 — Domain Layer

**Goal**: Define the EdgeDevice aggregate, validation rules, commands, and domain errors.

**Files**:
- `internal/domain/edge_devices/edge_device.go`
- `internal/domain/edge_devices/commands.go`
- `internal/domain/edge_devices/errors.go`

**Key elements**:
- `EdgeDevice` struct with all fields; `Validate()` method
- `CreateDeviceCommand` (name, machineId, edgeType, raspberryBaseUrl, description, plcAddress)
- `UpdateDeviceCommand` (name, description — both optional)
- Errors: `ErrDeviceNotFound`, `ErrMachineIDConflict`, `ErrDeviceDisabled`
- State transition: `Enable()`, `Disable()` methods

---

### Phase 3 — Repository Layer

**Goal**: PostgreSQL implementation of device and event persistence.

**Files**:
- `internal/repo/pg/edge_devices/repository.go`

**Interface** (in `internal/domain/edge_devices/`):
```go
type Repository interface {
    List(ctx context.Context, tenantID uuid.UUID) ([]*EdgeDevice, error)
    GetByID(ctx context.Context, tenantID, deviceID uuid.UUID) (*EdgeDevice, error)
    Create(ctx context.Context, device *EdgeDevice) error
    Update(ctx context.Context, device *EdgeDevice) error
    SetStatus(ctx context.Context, tenantID, deviceID uuid.UUID, status string) (*EdgeDevice, error)
    UpdateHealthState(ctx context.Context, tenantID, deviceID uuid.UUID, status, summary string) error
    SaveEvent(ctx context.Context, event *DeviceEvent) error
    ListEvents(ctx context.Context, tenantID, deviceID uuid.UUID) ([]*DeviceEvent, error)
}
```

---

### Phase 4 — Edge Device Client (platform layer)

**Goal**: HTTP client that proxies calls to physical edge devices.

**Files**:
- `internal/platform/edgeclient/client.go` (interface)
- `internal/platform/edgeclient/http_client.go` (implementation)

**Interface**:
```go
type EdgeDeviceClient interface {
    StatusCheck(ctx context.Context, baseURL string) (*CheckResult, error)
    HealthCheck(ctx context.Context, baseURL string) (*CheckResult, error)
    GetTelemetry(ctx context.Context, baseURL string) (*TelemetrySnapshot, error)
}
```

**Behavior**:
- Default timeout: 10s (configurable via env `EDGE_CLIENT_TIMEOUT`)
- Network errors → return `CheckResult{OverallStatus: "ERROR", Summary: err.Error()}`
- Non-2xx from device → return `CheckResult{OverallStatus: "ERROR"}`

---

### Phase 5 — Application Service Layer

**Goal**: Orchestrate domain + repo + edge client.

**Files**:
- `internal/app/edge_devices/service.go`

**Operations**:
1. `ListDevices(ctx, tenantID)` → `[]*EdgeDevice`
2. `GetDevice(ctx, tenantID, deviceID)` → `*EdgeDevice`
3. `CreateDevice(ctx, tenantID, cmd)` → `*EdgeDevice`
4. `UpdateDevice(ctx, tenantID, deviceID, cmd)` → `*EdgeDevice`
5. `EnableDevice(ctx, tenantID, deviceID)` → `*EdgeDevice`
6. `DisableDevice(ctx, tenantID, deviceID)` → `*EdgeDevice`
7. `StatusCheck(ctx, tenantID, deviceID, userID, userEmail)` → `*CheckResult`
   - Validates device ACTIVE, calls client, persists DeviceEvent, updates device health state
8. `HealthCheck(ctx, tenantID, deviceID, userID, userEmail)` → `*CheckResult`
   - Same as StatusCheck with HEALTH_CHECK type
9. `GetTelemetry(ctx, tenantID, deviceID)` → `*TelemetrySnapshot`
   - Validates device ACTIVE, calls client — does NOT persist event
10. `ListEvents(ctx, tenantID, deviceID)` → `[]*DeviceEvent`

---

### Phase 6 — HTTP Handlers

**Goal**: Transport layer — parse requests, call service, map to JSON response.

**Files**:
- `internal/api/handler/edge_devices/dto/dto.go`
- One handler file per operation (10 total)

**Response envelope** (matching pact):
```json
{ "success": true, "data": { ... } }
{ "success": false, "error": "..." }
```

**Error mapping**:
| Domain Error | HTTP Status |
|-------------|-------------|
| `ErrDeviceNotFound` | 404 |
| `ErrMachineIDConflict` | 409 with `"CONFLICT: machineId ya existe en el tenant"` |
| `ErrDeviceDisabled` | 400 with `"EDGE_DEVICE_DISABLED"` |
| Validation error | 400 |
| Unauthenticated | 401 with `"No autorizado"` |

---

### Phase 7 — Middleware: Tenant Resolution from Path

**Goal**: Resolve `:tenantId` subdomain slug to internal UUID and inject into context.

**File**: `internal/api/middleware/resolve_tenant_path.go`

**Logic**:
1. Extract `:tenantId` from Gin path param
2. Query `tenants` table by `subdomain`
3. If not found → 404
4. Set `platform.TenantID` in context
5. Call `c.Next()`

This is a new middleware (existing `ExtractTenantID` reads `X-Tenant-ID` header).

---

### Phase 8 — Router Integration

**Goal**: Wire dependency injection and register routes.

**File**: `internal/routes/url_mappings.go`

**Changes**:
- Add new `/api/tenants` Gin group with JWT + CORS middleware
- Instantiate `edgeclient.NewHTTPClient(cfg.EdgeClientTimeout)`
- Instantiate `edge_devices.NewPostgresRepository(db)`
- Instantiate `edge_devices.NewService(repo, client, logger)`
- Call `edge_devices.RegisterRoutes(tenantsGroup, service)`

---

### Phase 9 — Observability

**Goal**: Logging and metrics on all operations.

**Logging** (Zap, structured):
- All service methods log at `Info` level with fields: `tenant_id`, `device_id`, `operation`
- Errors logged at `Error` level with cause
- Device check outbound calls logged with `base_url` and duration

**Prometheus metrics**:
- `edge_device_requests_total{operation, status}` — counter per operation and result
- `edge_device_check_duration_seconds{check_type}` — histogram for device check latency
- `edge_device_client_errors_total{check_type}` — counter for outbound connectivity failures

---

### Phase 10 — Tests

**Goal**: Unit + integration coverage.

**Unit tests** (uber/mock):
- `app/edge_devices/service_test.go` — mock repo + mock edgeclient
- All 10 operations; happy path + error paths

**Integration tests** (Docker Postgres):
- `tests/integration/edge_devices/` — real DB, mock edge client
- Covers: create with duplicate machineId, cross-tenant isolation, state transitions, event persistence

**Pact verification**:
- Provider verification against `embolsadora-frontend` pact file
- All 14 pact interactions must pass

---

## III. Constitution Compliance (Post-Design)

| Principle | Compliance |
|-----------|-----------|
| I — Hexagonal | ✅ `transport → app → domain ← repo | edgeclient` |
| II — Security | ✅ JWT on all routes; subdomain → UUID resolution; tenant_id in all queries |
| III — Observability | ✅ Zap logging + Prometheus counters + histograms in Phase 9 |
| IV — Integration Tests | ✅ DB integration tests + pact provider verification in Phase 10 |
| V — Versioning | ✅ New endpoints = MINOR bump; OpenAPI contract updated |

**ADR Recommended**: Document the `/api/tenants/` route group as a pact-driven addition to the ABM surface, and note the async outbound call pattern for future consideration.

---

## IV. Dependencies & Risks

| Risk | Likelihood | Mitigation |
|------|-----------|-----------|
| Edge device unreachable during check | High (physical devices) | Return ERROR status in CheckResult, not HTTP 500 |
| Pact path doesn't match `/api/v1` convention | Known | Document in ADR; honor pact as source of truth |
| Check calls block for up to 10s | Medium | Configurable timeout; async approach in v2 |
| Tenant subdomain lookup adds latency | Low | Indexed column; sub-ms lookup |

---

## V. Artifacts Summary

| Artifact | Path |
|---------|------|
| Feature spec | `specs/003-edge-device-management/spec.md` |
| Research | `specs/003-edge-device-management/research.md` |
| Data model | `specs/003-edge-device-management/plan/data-model.md` |
| OpenAPI contract | `specs/003-edge-device-management/contracts/edge-device-service-api.openapi.yaml` |
| Quickstart | `specs/003-edge-device-management/plan/quickstart.md` |
| Tasks (next) | `specs/003-edge-device-management/tasks.md` — generated by `/speckit.tasks` |
