# Research: Edge Device Management API

**Feature**: 003-edge-device-management
**Date**: 2026-03-09
**Status**: Complete

---

## Decision 1: Route Prefix — `/api/tenants/` vs `/api/v1/`

**Decision**: Honor the pact contract exactly. Register routes under `/api/tenants/{tenantId}/edge-devices`.

**Rationale**: The pact is a consumer-driven contract; the frontend was built and tested against `/api/tenants/...`. Changing the path would break the consumer. A new Gin group `/api/tenants` is registered alongside the existing `/api/v1` and `/api/auth` groups, sharing the same JWT + CORS middleware stack. This extends the ABM surface without violating it — both groups use JWT Bearer auth.

**Alternatives Considered**:
- `/api/v1/tenants/{tenantId}/edge-devices` — Aligns with ABM prefix convention but breaks the existing pact contract. Rejected.
- `/api/v1/edge-devices` with `X-Tenant-ID` header — Consistent with existing user management pattern but contradicts the pact interface design. Rejected.

---

## Decision 2: `tenantId` Path Parameter Format — Subdomain Slug vs UUID

**Decision**: The `tenantId` path parameter is the tenant's `subdomain` field (string slug, e.g., `"acme"`), not the UUID primary key.

**Rationale**: The pact uses `"acme"` as the tenantId value in all interactions. The `tenants` table has a `subdomain VARCHAR(100) UNIQUE NOT NULL` column with an index (`idx_tenants_subdomain`). This allows lookups by slug without exposing internal UUIDs in URLs. The middleware must resolve the subdomain to the internal tenant UUID for subsequent DB queries.

**Alternatives Considered**:
- UUID in path — More directly maps to DB primary key but exposes internal IDs and doesn't match the pact value format. Rejected.

---

## Decision 3: Outbound HTTP Client for Device Checks (Status / Health / Telemetry)

**Decision**: Introduce a `platform/edgeclient` package with an `EdgeDeviceClient` interface that wraps `net/http` for outbound calls to the Raspberry Pi `raspberryBaseUrl`. The client is injected into the service layer.

**Rationale**: Status check, health check, and telemetry endpoints require the backend to act as a proxy — it receives an authenticated request from the frontend and makes an outbound HTTP call to the physical edge device at its registered `raspberryBaseUrl`. This is a new I/O pattern in the codebase (all existing I/O is DB/Redis). Using an interface enables mocking in unit tests without network calls.

**Implementation Pattern**:
- `EdgeDeviceClient` interface: `StatusCheck(ctx, baseURL) (*CheckResult, error)`, `HealthCheck(ctx, baseURL) (*CheckResult, error)`, `GetTelemetry(ctx, baseURL) (*TelemetrySnapshot, error)`
- Timeout: 10s (configurable) — device calls are inherently slower than DB queries
- Failure handling: Network errors → `overallStatus: "ERROR"` with summary, not HTTP 500 on API surface

**Alternatives Considered**:
- Direct `http.Get` in handler — No abstraction, untestable. Rejected.
- Message queue / async polling — Architecturally correct for IoT at scale, but overkill for MVP; adds Redis/queue dependency. Rejected for now (ADR recommended for v2).

---

## Decision 4: Event History Persistence

**Decision**: Create a `device_events` table to persist every check result (status check and health check) as an immutable audit record. Telemetry is NOT persisted (it is a live proxy call only).

**Rationale**: The pact defines a `GET .../events` endpoint returning historical check records with `userId` and `userEmail`. This requires storing check results. Telemetry is excluded from history because it is a real-time snapshot, not a triggered diagnostic event.

**Alternatives Considered**:
- JSONB in `edge_devices.last_check_details` only — No history, can't satisfy the events endpoint. Rejected.
- Separate events service — Premature microservice split; monolith handles this cleanly. Rejected.

---

## Decision 5: Enable/Disable Idempotency

**Decision**: Enable and disable operations are idempotent. Calling `POST .../enable` on an already ACTIVE device returns 200 with the current state unchanged. Same for `disable` on DISABLED.

**Rationale**: Simplifies frontend retry logic and avoids 409 errors for repeated state transitions that have no side effects. The pact does not test this edge case, so we choose the safer behavior.

---

## Decision 6: Authentication for `/api/tenants` Route Group

**Decision**: Use the existing JWT Bearer middleware (`apimw.JWTAuth()`) and tenant resolution middleware, same as the ABM surface.

**Rationale**: Edge device management is an administrative capability requiring the same JWT authentication model. No new auth surface is needed.

**Note**: The RBAC check for write operations (POST, PUT) will enforce `admin` role. Read operations (GET) are available to any authenticated user of the tenant.

---

## Decision 7: No New External Dependencies

All required packages are already in `go.mod`:
- `net/http` (stdlib) — outbound HTTP client for edge device calls
- `github.com/gin-gonic/gin` — routing
- `github.com/jackc/pgx/v5` — PostgreSQL
- `go.uber.org/zap` — logging
- `github.com/prometheus/client_golang` — metrics
- `github.com/stretchr/testify` + `go.uber.org/mock` — testing

---

## Decision 8: Migration Numbering

Next migration: `0005_create_edge_devices_tables` (after `0004_create_users_table`).

Two tables created in a single migration:
- `edge_devices` — device registry
- `device_events` — immutable check event log

---

## Open Items (Post-MVP)

- **ADR recommended**: Evaluate async/event-driven telemetry ingestion at scale (vs. synchronous proxy calls)
- **Cursor-based pagination** for events endpoint when history grows large
- **Circuit breaker** for outbound edge device calls (prevent cascading failures when many devices are unreachable)
- **Webhook support** for push-based health alerts from edge devices
