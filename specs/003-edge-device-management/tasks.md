# Tasks: Edge Device Management API

**Input**: Design documents from `/specs/003-edge-device-management/`
**Prerequisites**: plan.md ✅ | spec.md ✅ | research.md ✅ | data-model.md ✅ | contracts/ ✅

**Tests**: Not explicitly requested in spec — no test tasks generated.

**Organization**: Tasks grouped by user story to enable independent implementation and testing.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies on incomplete tasks)
- **[Story]**: User story this task belongs to (US1–US9)
- Exact file paths included in every description

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Create directory structure and database migration.

- [ ] T001 Create Go package directories: `internal/domain/edge_devices/`, `internal/repo/pg/edge_devices/`, `internal/app/edge_devices/`, `internal/api/handler/edge_devices/dto/`, `internal/platform/edgeclient/`, `tests/integration/edge_devices/`
- [ ] T002 Write migration up file `migrations/0005_create_edge_devices_tables.up.sql` with `edge_devices` table (UUID PK, tenant_id FK, name, description, machine_id, edge_type, raspberry_base_url, plc_address, status CHECK ACTIVE/DISABLED, last_seen_at, last_health_check_at, last_health_status, last_health_summary, created_at, updated_at), UNIQUE(tenant_id, machine_id), indexes, and auto-update trigger for updated_at
- [ ] T003 [P] Write migration down file `migrations/0005_create_edge_devices_tables.down.sql` with DROP TABLE device_events, DROP TABLE edge_devices, DROP FUNCTION update_edge_devices_updated_at
- [ ] T004 [P] Write migration up additions to `migrations/0005_create_edge_devices_tables.up.sql` for `device_events` table (UUID PK, device_id FK → edge_devices, tenant_id, check_type CHECK STATUS/HEALTH_CHECK, checked_at, overall_status, summary, details JSONB, user_id, user_email), indexes on (device_id), (tenant_id), (device_id, checked_at DESC)

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core domain, repository, edge client, middleware, and service constructor — MUST complete before any user story.

**⚠️ CRITICAL**: No user story handler can be wired until this phase is complete.

- [ ] T005 Implement EdgeDevice domain aggregate in `internal/domain/edge_devices/edge_device.go`: struct with all fields (ID, TenantID, Name, Description, MachineID, EdgeType, RaspberryBaseURL, PLCAddress, Status, LastSeenAt, LastHealthCheckAt, LastHealthStatus, LastHealthSummary, CreatedAt, UpdatedAt); DeviceEvent struct (ID, DeviceID, TenantID, CheckType, CheckedAt, OverallStatus, Summary, Details map, UserID, UserEmail); CheckResult struct (CheckType, CheckedAt, OverallStatus, Summary, Details map); TelemetrySnapshot struct (CapturedAt, CPU, RAM, Disk, TemperatureCelsius, UptimeSeconds, PLC)
- [ ] T006 [P] Implement domain commands in `internal/domain/edge_devices/commands.go`: CreateDeviceCommand (Name, MachineID, EdgeType, RaspberryBaseURL, Description, PLCAddress), UpdateDeviceCommand (Name *string, Description *string — both optional pointer fields)
- [ ] T007 [P] Implement domain errors in `internal/domain/edge_devices/errors.go`: ErrDeviceNotFound (maps → 404), ErrMachineIDConflict (maps → 409 with message "CONFLICT: machineId ya existe en el tenant"), ErrDeviceDisabled (maps → 400 with code "EDGE_DEVICE_DISABLED"), ErrDeviceValidation (maps → 400)
- [ ] T008 Define Repository interface in `internal/domain/edge_devices/repository.go`: List(ctx, tenantID) ([]*EdgeDevice, error), GetByID(ctx, tenantID, deviceID) (*EdgeDevice, error), Create(ctx, *EdgeDevice) error, Update(ctx, *EdgeDevice) error, SetStatus(ctx, tenantID, deviceID, status string) (*EdgeDevice, error), UpdateHealthState(ctx, tenantID, deviceID, status, summary string) error, SaveEvent(ctx, *DeviceEvent) error, ListEvents(ctx, tenantID, deviceID) ([]*DeviceEvent, error)
- [ ] T009 Implement PostgreSQL repository in `internal/repo/pg/edge_devices/repository.go` implementing the Repository interface: all 8 methods using pgx/v5 pool, parameterized queries with tenant_id scoping on every query; List uses SELECT all columns WHERE tenant_id=$1; Create uses INSERT ... RETURNING; SetStatus uses UPDATE status WHERE tenant_id AND id RETURNING; SaveEvent uses INSERT INTO device_events; ListEvents uses SELECT ... WHERE device_id=$1 AND tenant_id=$2 ORDER BY checked_at DESC
- [ ] T010 [P] Define EdgeDeviceClient interface in `internal/platform/edgeclient/client.go`: StatusCheck(ctx context.Context, baseURL string) (*domain.CheckResult, error), HealthCheck(ctx context.Context, baseURL string) (*domain.CheckResult, error), GetTelemetry(ctx context.Context, baseURL string) (*domain.TelemetrySnapshot, error)
- [ ] T011 Implement HTTP EdgeDeviceClient in `internal/platform/edgeclient/http_client.go`: HTTPClient struct with http.Client (timeout 10s, configurable via EDGE_CLIENT_TIMEOUT env), NewHTTPClient(timeout time.Duration) *HTTPClient; StatusCheck calls GET {baseURL}/status, parses JSON into CheckResult; HealthCheck calls GET {baseURL}/health, parses JSON; GetTelemetry calls GET {baseURL}/telemetry, parses JSON; all network errors return CheckResult{OverallStatus: "ERROR", Summary: err.Error()} instead of propagating error to caller
- [ ] T012 [P] Define response DTOs in `internal/api/handler/edge_devices/dto/dto.go`: EdgeDeviceResponse (camelCase JSON tags matching pact: id, tenantId, name, description, machineId, edgeType, raspberryBaseUrl, plcAddress, status, lastSeenAt, lastHealthCheckAt, lastHealthStatus, lastHealthSummary, createdAt, updatedAt), CreateDeviceRequest, UpdateDeviceRequest, CheckResultResponse, TelemetryResponse, DeviceEventResponse, SuccessResponse[T], ErrorResponse; mapping functions EdgeDeviceToResponse(*domain.EdgeDevice) EdgeDeviceResponse, etc.
- [ ] T013 Implement ResolveTenantFromPath middleware in `internal/api/middleware/resolve_tenant_path.go`: extract `:tenantId` from Gin path params, query `tenants` table by subdomain column, if not found abort with 404, set resolved UUID into context via `platform.TenantID`, call c.Next(); inject *pgxpool.Pool dependency via closure
- [ ] T014 Implement Service struct and constructor in `internal/app/edge_devices/service.go`: Service struct with fields repo domain.Repository, client edgeclient.EdgeDeviceClient, logger *zap.Logger; NewService(repo, client, logger) *Service; stub all 10 method signatures returning nil/zero values (methods filled in per user story phase)
- [ ] T015 Register `/api/tenants` Gin route group in `internal/routes/url_mappings.go`: instantiate edgeclient.NewHTTPClient, edge_devices postgres repo, edge_devices service; create group with JWTAuth + CORS + RequestID + Logger + ResolveTenantFromPath middleware; register route group — routes added per user story phase

**Checkpoint**: Foundation ready — user story phases can begin.

---

## Phase 3: User Story 1 — List Edge Devices (Priority: P1) 🎯 MVP Start

**Goal**: Authenticated users can retrieve the full list of edge devices for their tenant.

**Independent Test**: `GET /api/tenants/acme/edge-devices` with valid JWT returns array of devices (or empty array); without JWT returns 401.

- [ ] T016 [US1] Implement `ListDevices(ctx context.Context, tenantID uuid.UUID) ([]*domain.EdgeDevice, error)` method body in `internal/app/edge_devices/service.go`: call repo.List(ctx, tenantID), return results; log info with tenant_id field
- [ ] T017 [US1] Implement ListDevices handler in `internal/api/handler/edge_devices/list_devices.go`: extract tenantID from context (platform.TenantID), call service.ListDevices, map results via dto.EdgeDeviceToResponse slice, return `{"success": true, "data": [...]}` with 200; map ErrDeviceNotFound → 404
- [ ] T018 [US1] Register route `GET /tenants/:tenantId/edge-devices` → ListDevices handler in route registration (called from T015 setup in url_mappings.go or in a dedicated `internal/api/handler/edge_devices/routes.go`)

**Checkpoint**: `GET /api/tenants/acme/edge-devices` returns 200 with array; 401 without token.

---

## Phase 4: User Story 2 — Register a New Edge Device (Priority: P1)

**Goal**: Administrators can register a new edge device with initial ACTIVE status and UNKNOWN health.

**Independent Test**: `POST /api/tenants/acme/edge-devices` with valid body returns 201 with new device; duplicate machineId returns 409.

- [ ] T019 [US2] Implement `CreateDevice(ctx, tenantID, cmd CreateDeviceCommand) (*domain.EdgeDevice, error)` in `internal/app/edge_devices/service.go`: build EdgeDevice struct from command + tenantID, call repo.Create; on pgx unique constraint violation return ErrMachineIDConflict; log info with tenant_id, machine_id fields
- [ ] T020 [US2] Implement CreateDevice handler in `internal/api/handler/edge_devices/create_device.go`: parse and validate CreateDeviceRequest body (name and machineId required, edgeType must be RASPBERRY_PLC, raspberryBaseUrl required), build CreateDeviceCommand, call service.CreateDevice, return `{"success": true, "data": {...}}` 201; map ErrMachineIDConflict → 409 `{"success": false, "error": "CONFLICT: machineId ya existe en el tenant"}`; map validation errors → 400
- [ ] T021 [US2] Register route `POST /tenants/:tenantId/edge-devices` → CreateDevice handler

**Checkpoint**: Full CRUD list+create works. Pact interactions 1, 2, 3 should pass.

---

## Phase 5: User Story 6 — Status Check on Active Device (Priority: P1)

**Goal**: Operators can trigger an on-demand connectivity+version check on an ACTIVE device; DISABLED devices are rejected.

**Independent Test**: `POST /api/tenants/acme/edge-devices/{id}/status` on ACTIVE device returns 200 with checkType STATUS; on DISABLED device returns 400 EDGE_DEVICE_DISABLED.

- [ ] T022 [US6] Implement `StatusCheck(ctx, tenantID, deviceID uuid.UUID, userID uuid.UUID, userEmail string) (*domain.CheckResult, error)` in `internal/app/edge_devices/service.go`: get device by ID, if not found return ErrDeviceNotFound, if DISABLED return ErrDeviceDisabled; call client.StatusCheck(ctx, device.RaspberryBaseURL); build DeviceEvent with checkType=STATUS, checkedAt=now, overallStatus from result, details from result; call repo.SaveEvent; call repo.UpdateHealthState to update device's last_health_* fields; return CheckResult; log info with device_id, check_type, overall_status
- [ ] T023 [US6] Implement StatusCheck handler in `internal/api/handler/edge_devices/status_check.go`: extract deviceID from path, extract userID+userEmail from JWT context, call service.StatusCheck; return `{"success": true, "data": {checkType, checkedAt, overallStatus, summary, details}}` 200; map ErrDeviceDisabled → 400 `{"success": false, "error": "EDGE_DEVICE_DISABLED"}`; map ErrDeviceNotFound → 404
- [ ] T024 [US6] Register route `POST /tenants/:tenantId/edge-devices/:deviceId/status` → StatusCheck handler

**Checkpoint**: Status check flow works end-to-end. Pact interactions 9, 10 should pass.

---

## Phase 6: User Story 3 — View a Specific Edge Device (Priority: P2)

**Goal**: Users can retrieve the full profile of a single device by ID.

**Independent Test**: `GET /api/tenants/acme/edge-devices/{id}` returns 200 with full profile; non-existent ID returns 404.

- [ ] T025 [US3] Implement `GetDevice(ctx, tenantID, deviceID uuid.UUID) (*domain.EdgeDevice, error)` in `internal/app/edge_devices/service.go`: call repo.GetByID(ctx, tenantID, deviceID); if not found return ErrDeviceNotFound; log info
- [ ] T026 [US3] Implement GetDevice handler in `internal/api/handler/edge_devices/get_device.go`: extract deviceID from path, call service.GetDevice, return 200 with device response; map ErrDeviceNotFound → 404 `{"success": false, "error": "Not found"}`
- [ ] T027 [US3] Register route `GET /tenants/:tenantId/edge-devices/:deviceId` → GetDevice handler

**Checkpoint**: Pact interactions 4, 5 should pass.

---

## Phase 7: User Story 4 — Update Edge Device Configuration (Priority: P2)

**Goal**: Administrators can update name and/or description of a device; updatedAt is refreshed.

**Independent Test**: `PUT /api/tenants/acme/edge-devices/{id}` with updated name returns 200 with updated device and new updatedAt.

- [ ] T028 [US4] Implement `UpdateDevice(ctx, tenantID, deviceID uuid.UUID, cmd UpdateDeviceCommand) (*domain.EdgeDevice, error)` in `internal/app/edge_devices/service.go`: get device by ID (ErrDeviceNotFound if absent); apply non-nil fields from cmd to device struct; call repo.Update(ctx, device); return updated device; log info with updated fields
- [ ] T029 [US4] Implement UpdateDevice handler in `internal/api/handler/edge_devices/update_device.go`: parse UpdateDeviceRequest (name and description both optional), build UpdateDeviceCommand, call service.UpdateDevice, return 200 with updated device response; map ErrDeviceNotFound → 404
- [ ] T030 [US4] Register route `PUT /tenants/:tenantId/edge-devices/:deviceId` → UpdateDevice handler

**Checkpoint**: Pact interaction 6 should pass.

---

## Phase 8: User Story 5 — Enable / Disable an Edge Device (Priority: P2)

**Goal**: Administrators can transition device between ACTIVE and DISABLED states; operations are idempotent.

**Independent Test**: `POST .../enable` on DISABLED device returns 200 with status=ACTIVE; `POST .../disable` on ACTIVE device returns 200 with status=DISABLED; repeated calls return 200 unchanged.

- [ ] T031 [US5] Implement `EnableDevice(ctx, tenantID, deviceID uuid.UUID) (*domain.EdgeDevice, error)` and `DisableDevice(ctx, tenantID, deviceID) (*domain.EdgeDevice, error)` in `internal/app/edge_devices/service.go`: both call repo.SetStatus with target status string; if repo.GetByID returns not found, return ErrDeviceNotFound; if device already has target status, return device unchanged (idempotent); log info with device_id, new_status
- [ ] T032 [P] [US5] Implement EnableDevice handler in `internal/api/handler/edge_devices/enable_device.go`: extract deviceID from path, call service.EnableDevice, return 200 with device response; map ErrDeviceNotFound → 404
- [ ] T033 [P] [US5] Implement DisableDevice handler in `internal/api/handler/edge_devices/disable_device.go`: extract deviceID from path, call service.DisableDevice, return 200 with device response; map ErrDeviceNotFound → 404
- [ ] T034 [US5] Register routes `POST /tenants/:tenantId/edge-devices/:deviceId/enable` → EnableDevice and `POST /tenants/:tenantId/edge-devices/:deviceId/disable` → DisableDevice

**Checkpoint**: Pact interactions 7, 8 should pass.

---

## Phase 9: User Story 7 — Full Health Check (Priority: P2)

**Goal**: Operators can trigger a full hardware diagnostic (CPU, RAM, disk, temperature, uptime) on an ACTIVE device.

**Independent Test**: `POST .../health-check` on ACTIVE reachable device returns 200 with checkType HEALTH_CHECK and hardware metrics; DISABLED device returns 400.

- [ ] T035 [US7] Implement `HealthCheck(ctx, tenantID, deviceID uuid.UUID, userID uuid.UUID, userEmail string) (*domain.CheckResult, error)` in `internal/app/edge_devices/service.go`: same pattern as StatusCheck (T022) but uses client.HealthCheck and checkType=HEALTH_CHECK; persists DeviceEvent; updates device health state; log info with device_id, check_type, overall_status
- [ ] T036 [US7] Implement HealthCheck handler in `internal/api/handler/edge_devices/health_check.go`: same structure as StatusCheck handler (T023) calling service.HealthCheck; details response includes cpu, ram, disk, temperatureCelsius, uptimeSeconds
- [ ] T037 [US7] Register route `POST /tenants/:tenantId/edge-devices/:deviceId/health-check` → HealthCheck handler

**Checkpoint**: Pact interaction 11 should pass.

---

## Phase 10: User Story 8 — Real-Time Telemetry (Priority: P2)

**Goal**: Operators can retrieve a live hardware + PLC connectivity snapshot from an ACTIVE device.

**Independent Test**: `GET .../telemetry` on ACTIVE device returns 200 with capturedAt, all hardware metrics, and plc object including reachable flag.

- [ ] T038 [US8] Implement `GetTelemetry(ctx, tenantID, deviceID uuid.UUID) (*domain.TelemetrySnapshot, error)` in `internal/app/edge_devices/service.go`: get device by ID (ErrDeviceNotFound if absent); if DISABLED return ErrDeviceDisabled; call client.GetTelemetry(ctx, device.RaspberryBaseURL); return snapshot (NOT persisted as event); log info with device_id, plc_reachable
- [ ] T039 [US8] Implement GetTelemetry handler in `internal/api/handler/edge_devices/get_telemetry.go`: extract deviceID from path, call service.GetTelemetry, return 200 with telemetry response (capturedAt, cpu, ram, disk, temperatureCelsius, uptimeSeconds, plc); map ErrDeviceNotFound → 404; map ErrDeviceDisabled → 400 EDGE_DEVICE_DISABLED
- [ ] T040 [US8] Register route `GET /tenants/:tenantId/edge-devices/:deviceId/telemetry` → GetTelemetry handler

**Checkpoint**: Pact interaction 12 should pass.

---

## Phase 11: User Story 9 — Event History (Priority: P3)

**Goal**: Administrators can view the full historical log of triggered checks for a device, ordered newest-first, with user attribution.

**Independent Test**: `GET .../events` on device with check history returns 200 with array of events each containing checkType, checkedAt, overallStatus, userId, userEmail; device with no history returns empty array.

- [ ] T041 [US9] Implement `ListEvents(ctx, tenantID, deviceID uuid.UUID) ([]*domain.DeviceEvent, error)` in `internal/app/edge_devices/service.go`: verify device exists via repo.GetByID (ErrDeviceNotFound if absent); call repo.ListEvents(ctx, tenantID, deviceID); return results; log info with device_id, event_count
- [ ] T042 [US9] Implement ListEvents handler in `internal/api/handler/edge_devices/list_events.go`: extract deviceID from path, call service.ListEvents, map results to DeviceEventResponse slice (id, checkType, checkedAt, overallStatus, summary, userId, userEmail), return `{"success": true, "data": [...]}` 200; map ErrDeviceNotFound → 404
- [ ] T043 [US9] Register route `GET /tenants/:tenantId/edge-devices/:deviceId/events` → ListEvents handler

**Checkpoint**: All 9 user stories complete. Pact interaction 13 should pass.

---

## Phase 12: Polish & Cross-Cutting Concerns

**Purpose**: Observability, pact validation, and code review pass.

- [ ] T044 [P] Add Prometheus metrics to `internal/app/edge_devices/service.go`: register counter `edge_device_requests_total{operation, tenant_id, status}` incremented on every service method call; register histogram `edge_device_check_duration_seconds{check_type}` observed on StatusCheck and HealthCheck; register counter `edge_device_client_errors_total{check_type}` incremented when overall_status is ERROR
- [ ] T045 [P] Add structured Zap logging to all service methods in `internal/app/edge_devices/service.go`: each method logs zap.Info at entry with tenant_id, device_id (where applicable), operation name; log zap.Error on domain errors with error field; check and health methods log zap.Info with overall_status and duration fields
- [ ] T046 Validate all 14 pact interactions from `pacts/edge-device-service-api.json` pass against the running local API: run server, execute pact verification tool or manual curl requests matching each interaction's request/response, confirm all pass; document any mismatches found

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 (Setup)**: No dependencies — start immediately
- **Phase 2 (Foundational)**: Depends on Phase 1 — BLOCKS all user story phases
- **Phase 3–11 (User Stories)**: All depend on Phase 2 completion
  - P1 stories (Phase 3, 4, 5) should be delivered first
  - P2 stories (Phase 6–10) can follow in any order
  - P3 story (Phase 11) last
- **Phase 12 (Polish)**: Depends on all desired user stories complete

### User Story Dependencies

| Story | Depends On | Can Start After |
|-------|-----------|----------------|
| US1 – List (P1) | Phase 2 | T015 complete |
| US2 – Register (P1) | Phase 2 | T015 complete |
| US6 – Status Check (P1) | Phase 2 + US2 (device must exist to check) | T021 complete |
| US3 – View (P2) | Phase 2 | T015 complete |
| US4 – Update (P2) | Phase 2 + US3 | T027 complete |
| US5 – Enable/Disable (P2) | Phase 2 | T015 complete |
| US7 – Health Check (P2) | Phase 2 + US6 (shares event persistence) | T024 complete |
| US8 – Telemetry (P2) | Phase 2 | T015 complete |
| US9 – Events (P3) | Phase 2 + US6 or US7 (events must be generated) | T037 complete |

### Within Each User Story

- Service method → handler → route registration (sequential within story)
- Handlers across different stories (e.g., T032 and T033 for enable/disable) can run in parallel [P]

---

## Parallel Execution Examples

### Parallel within Phase 2 (Foundational)

```
T006 (commands.go) ─────────────────── parallel ───┐
T007 (errors.go) ──────────────────── parallel ───┤ → T008 (repository.go)
T010 (edgeclient interface) ────────── parallel ───┤
T012 (DTOs) ───────────────────────── parallel ───┘
```

### Parallel across P1 User Stories (after Phase 2)

```
Phase 2 complete
     ├── Phase 3 (US1 – List)     → T016, T017, T018
     └── Phase 4 (US2 – Register) → T019, T020, T021
          └── Phase 5 (US6 – Status Check) depends on US2
```

### Parallel within Phase 8 (US5)

```
T032 (enable handler) ── parallel ──┐
T033 (disable handler) ─ parallel ──┘ → T034 (register both routes)
```

---

## Implementation Strategy

### MVP First (P1 Stories: US1 + US2 + US6)

1. Complete Phase 1: Setup (migration files)
2. Complete Phase 2: Foundational (domain, repo, client, middleware, service constructor)
3. Complete Phase 3: US1 — List devices
4. Complete Phase 4: US2 — Register device
5. Complete Phase 5: US6 — Status check
6. **STOP and VALIDATE**: Test all 3 P1 stories; verify pact interactions 1–3, 9, 10 pass
7. Demo to stakeholders — MVP delivers: device registry + connectivity monitoring

### Incremental Delivery

1. Setup + Foundational → Stack wired ✓
2. US1 + US2 → Device registry operational ✓
3. US6 → Status monitoring ✓ (MVP)
4. US3 + US4 + US5 → Full CRUD + lifecycle management ✓
5. US7 + US8 → Deep diagnostics + real-time telemetry ✓
6. US9 → Audit trail ✓
7. Polish → All 14 pact interactions verified ✓

### Parallel Team Strategy

With 2 developers after Phase 2 completes:
- **Dev A**: US1 → US2 → US6 → US7 (monitoring track)
- **Dev B**: US3 → US4 → US5 → US8 → US9 (CRUD + telemetry track)

---

## Summary

| Metric | Value |
|--------|-------|
| Total tasks | 46 |
| Setup tasks | 4 (T001–T004) |
| Foundational tasks | 11 (T005–T015) |
| US1 tasks | 3 (T016–T018) |
| US2 tasks | 3 (T019–T021) |
| US6 tasks | 3 (T022–T024) |
| US3 tasks | 3 (T025–T027) |
| US4 tasks | 3 (T028–T030) |
| US5 tasks | 4 (T031–T034) |
| US7 tasks | 3 (T035–T037) |
| US8 tasks | 3 (T038–T040) |
| US9 tasks | 3 (T041–T043) |
| Polish tasks | 3 (T044–T046) |
| Tasks with [P] | 12 |
| Parallel opportunities | Phase 2 (4 parallel), Phase 8 (2 parallel), cross-story |
| Pact interactions covered | 14/14 |
| MVP scope | Phase 1 + 2 + US1 + US2 + US6 (20 tasks) |

## Notes

- [P] tasks operate on different files — safe to run simultaneously
- Each user story phase delivers a testable increment
- The service.go file is modified incrementally — one method added per story phase (sequential within story)
- Pact interaction mapping: interactions 1–3 ↔ US1+US2, 4–5 ↔ US3, 6 ↔ US4, 7–8 ↔ US5, 9–10 ↔ US6, 11 ↔ US7, 12 ↔ US8, 13 ↔ US9, 14 ↔ Foundational JWT middleware
- Commit after each checkpoint for clean git history
