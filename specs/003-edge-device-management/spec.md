# Feature Specification: Edge Device Management API

**Feature Branch**: `003-edge-device-management`
**Created**: 2026-03-09
**Status**: Draft
**Input**: User description: "Pact contract para edge-device-service-api — gestión completa de edge devices por tenant"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - List Edge Devices for a Tenant (Priority: P1)

An administrator views the complete list of edge devices registered for their tenant to get an overview of all physical hardware connected to the platform.

**Why this priority**: Core operation for situational awareness. Without the ability to list devices, no other device operation can be discovered or initiated. All other stories depend on first knowing what devices exist.

**Independent Test**: Can be fully tested by authenticating with a valid token and requesting the device list for a tenant, verifying that all registered devices are returned with their current operational status.

**Acceptance Scenarios**:

1. **Given** a tenant has one or more edge devices registered, **When** an authenticated user requests the device list for that tenant, **Then** the system returns all devices with their full profile (id, name, description, machineId, edgeType, connectivity addresses, status, health info, and timestamps).
2. **Given** a tenant has no edge devices registered, **When** the device list is requested, **Then** the system returns an empty list.
3. **Given** an unauthenticated request (no Authorization token), **When** the device list is requested, **Then** the system returns 401 Unauthorized.

---

### User Story 2 - Register a New Edge Device (Priority: P1)

An administrator registers a new physical edge device (Raspberry Pi + PLC) into the platform for a given tenant, enabling the system to track and communicate with it.

**Why this priority**: Without registering devices, the platform has no hardware to monitor. This is the entry point for all physical asset management.

**Independent Test**: Can be tested by submitting a new device registration and verifying the created device is returned with ACTIVE status, server-assigned ID, and unknown health state (since no check has been performed yet).

**Acceptance Scenarios**:

1. **Given** no device exists with a given machineId in the tenant, **When** an authenticated user registers a new device with required fields (name, machineId, edgeType, raspberryBaseUrl), **Then** the system creates the device, sets initial status to ACTIVE and health to UNKNOWN, and returns the full device profile with a server-generated ID and timestamps.
2. **Given** a device with the same machineId already exists in the same tenant, **When** a registration is attempted with that machineId, **Then** the system returns 409 Conflict.
3. **Given** the same machineId exists in a different tenant, **When** the device is registered in the current tenant, **Then** registration succeeds (machineId uniqueness is scoped per tenant).
4. **Given** required fields are missing (e.g., name or machineId), **When** a registration is submitted, **Then** the system returns 400 Bad Request.

---

### User Story 3 - View a Specific Edge Device (Priority: P2)

An administrator retrieves the full profile of a specific edge device by its unique ID to inspect its configuration, connectivity details, and last known health state.

**Why this priority**: Enables targeted device inspection before editing, enabling/disabling, or performing checks — supports operational decision-making.

**Independent Test**: Can be tested by requesting a known device by ID and verifying all profile fields are returned, including connectivity addresses and last health information.

**Acceptance Scenarios**:

1. **Given** an edge device with a known ID exists in the tenant, **When** an authenticated user requests that device by ID, **Then** the system returns the full device profile.
2. **Given** a device ID that does not exist, **When** a request is made for that device, **Then** the system returns 404 Not Found.

---

### User Story 4 - Update Edge Device Configuration (Priority: P2)

An administrator updates a device's descriptive fields (name, description) to reflect changes in naming conventions or operational notes, without changing physical identifiers.

**Why this priority**: Supports operational maintenance of the device registry as physical deployments evolve.

**Independent Test**: Can be tested by submitting updated name/description and verifying the response reflects the new values with a refreshed last-updated timestamp, while all other fields remain unchanged.

**Acceptance Scenarios**:

1. **Given** an edge device exists, **When** an authenticated user submits updated name or description fields, **Then** the system returns the device with those fields updated and the last-modified timestamp refreshed.
2. **Given** an update is submitted for a non-existent device ID, **When** the update request is processed, **Then** the system returns 404 Not Found.

---

### User Story 5 - Enable or Disable an Edge Device (Priority: P2)

An administrator changes the operational state of an edge device — activating it to resume monitoring or deactivating it to pause communication — without deleting the device from the registry.

**Why this priority**: Provides operational flexibility. Disabling a device allows temporary suspension (e.g., maintenance) without losing registration data or history.

**Independent Test**: Can be tested by enabling a DISABLED device and verifying it transitions to ACTIVE. Disabling an ACTIVE device should set status to DISABLED while preserving last health data.

**Acceptance Scenarios**:

1. **Given** a DISABLED edge device, **When** an authenticated user enables the device, **Then** the system transitions it to ACTIVE status and returns the updated profile.
2. **Given** an ACTIVE edge device, **When** an authenticated user disables the device, **Then** the system transitions it to DISABLED status and returns the updated profile.
3. **Given** a non-existent device ID, **When** an enable or disable request is submitted, **Then** the system returns 404 Not Found.

---

### User Story 6 - Perform a Status Check on an Edge Device (Priority: P1)

An operator triggers an on-demand connectivity check on an active edge device to verify that the device is reachable and confirm its software version.

**Why this priority**: Enables real-time operational validation. Knowing whether a physical device is online and responsive is critical for monitoring reliability.

**Independent Test**: Can be tested by triggering a status check on an ACTIVE reachable device and verifying the response includes check type STATUS, timestamp, OK status, and version detail. Testing failure path requires a DISABLED device to confirm the operation is rejected.

**Acceptance Scenarios**:

1. **Given** an ACTIVE edge device that is reachable, **When** an authenticated user triggers a status check, **Then** the system returns a check result with checkType STATUS, timestamp, overall status OK, summary, and version detail.
2. **Given** a DISABLED edge device, **When** a status check is triggered, **Then** the system returns 400 Bad Request with error code EDGE_DEVICE_DISABLED.
3. **Given** an ACTIVE device that is unreachable at the network level, **When** a status check is triggered, **Then** the system returns a check result with a non-OK overall status reflecting the connectivity failure.

---

### User Story 7 - Perform a Full Health Check on an Edge Device (Priority: P2)

An operator triggers a comprehensive hardware diagnostic on an active edge device to assess its resource utilization (CPU, RAM, disk, temperature, uptime).

**Why this priority**: Provides deeper diagnostics beyond simple connectivity — enables proactive identification of hardware degradation before failures occur.

**Independent Test**: Can be tested by triggering a health check on an ACTIVE reachable device and verifying the response includes all hardware metrics with the HEALTH_CHECK type identifier.

**Acceptance Scenarios**:

1. **Given** an ACTIVE edge device that is reachable, **When** an authenticated user triggers a health check, **Then** the system returns a check result with checkType HEALTH_CHECK, timestamp, overall status, summary, and hardware metrics (cpu usagePercent, ram usedPercent, disk usedPercent, temperatureCelsius, uptimeSeconds).
2. **Given** a DISABLED edge device, **When** a health check is triggered, **Then** the system returns 400 Bad Request with error code EDGE_DEVICE_DISABLED.

---

### User Story 8 - View Real-Time Telemetry from an Edge Device (Priority: P2)

An operator views the current telemetry snapshot from an active edge device to see live hardware resource consumption and PLC connectivity state.

**Why this priority**: Provides real-time operational visibility — the core monitoring use case of the Embolsadora 4.0 platform.

**Independent Test**: Can be tested by requesting telemetry from an ACTIVE device and verifying the response includes all hardware metrics plus PLC connectivity information (reachable flag, latency, last heartbeat).

**Acceptance Scenarios**:

1. **Given** an ACTIVE edge device, **When** an authenticated user requests telemetry, **Then** the system returns the latest captured metrics: capturedAt timestamp, CPU usage percent, RAM (percent, used MB, total MB), disk (percent, used GB, total GB), temperature in Celsius, uptime in seconds, and PLC status (reachable, latency ms, last heartbeat timestamp).
2. **Given** a DISABLED edge device, **When** telemetry is requested, **Then** the system returns 400 Bad Request indicating the device is not operational.

---

### User Story 9 - View Edge Device Event History (Priority: P3)

An administrator reviews the historical log of all check events performed on an edge device to audit activity, identify recurring issues, and trace who performed each check.

**Why this priority**: Supports audit and compliance needs. Lower priority because the device still functions without history visibility; event logs are important for governance but not day-to-day operations.

**Independent Test**: Can be tested by requesting events for a device with prior check history and verifying that each event record includes the check type, result status, timestamp, and the user who triggered it.

**Acceptance Scenarios**:

1. **Given** an edge device with check history, **When** an authenticated user requests its event log, **Then** the system returns a list of past events each containing: id, checkType (STATUS or HEALTH_CHECK), checkedAt, overallStatus, summary, userId, and userEmail of the triggering user.
2. **Given** an edge device with no check history, **When** events are requested, **Then** the system returns an empty list.
3. **Given** a non-existent device ID, **When** events are requested, **Then** the system returns 404 Not Found.

---

### Edge Cases

- What happens when a request includes no Authorization token? (Return 401 Unauthorized)
- What happens when an authenticated user attempts to access devices from a tenant they do not belong to? (Return 403 Forbidden)
- What happens when an ACTIVE device is unreachable during a status or health check? (The check is attempted, the failure is captured, and a check result with non-OK status is returned — the API call itself succeeds with 200)
- What happens when concurrent registrations attempt to create devices with the same machineId in the same tenant? (Exactly one succeeds; the other receives 409 Conflict)
- What happens if the PLC is unreachable even when the Raspberry Pi edge device is online? (Telemetry returns plc.reachable = false with no latency value)
- What happens if required registration fields (name, machineId, edgeType, raspberryBaseUrl) are missing from the request body? (Return 400 Bad Request with validation details)
- What happens when enable is called on an already ACTIVE device, or disable on an already DISABLED device? (Operation is idempotent — return 200 with current state unchanged)

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST require a valid Bearer JWT token in the Authorization header for every endpoint; requests without a valid token MUST return 401 Unauthorized.
- **FR-002**: The system MUST scope all edge device operations to the tenant identified by the `tenantId` path parameter.
- **FR-003**: The system MUST return 403 Forbidden when an authenticated user attempts to access a tenant they are not authorized for.
- **FR-004**: The system MUST allow authorized users to retrieve the list of all edge devices registered for a given tenant.
- **FR-005**: The system MUST allow authorized administrators to register a new edge device for a tenant, requiring: name, machineId, edgeType, raspberryBaseUrl; with optional: description, plcAddress.
- **FR-006**: The system MUST enforce machineId uniqueness within a tenant; duplicate machineId registrations within the same tenant MUST return 409 Conflict.
- **FR-007**: The system MUST allow machineId duplication across different tenants (uniqueness is scoped per tenant, not globally).
- **FR-008**: Upon successful registration, the system MUST assign initial status ACTIVE, initial health status UNKNOWN, and null values for lastSeenAt and lastHealthCheckAt.
- **FR-009**: The system MUST allow authorized users to retrieve the full profile of a single edge device by its unique ID within the tenant.
- **FR-010**: The system MUST return 404 Not Found when a requested device ID does not exist within the specified tenant.
- **FR-011**: The system MUST allow authorized administrators to update the mutable fields of an edge device (name, description) and refresh the last-modified timestamp on successful update.
- **FR-012**: The system MUST allow authorized administrators to transition an edge device between ACTIVE and DISABLED states via dedicated enable and disable actions.
- **FR-013**: The system MUST reject status check and health check requests on DISABLED devices with 400 Bad Request and error code EDGE_DEVICE_DISABLED.
- **FR-014**: The system MUST allow authorized users to trigger an on-demand status check on an ACTIVE edge device, returning: checkType STATUS, checkedAt timestamp, overallStatus, summary, and device version detail.
- **FR-015**: The system MUST allow authorized users to trigger a full health check on an ACTIVE edge device, returning: checkType HEALTH_CHECK, checkedAt, overallStatus, summary, and hardware metrics (CPU usage percent, RAM usage percent, disk usage percent, temperature in Celsius, uptime in seconds).
- **FR-016**: The system MUST allow authorized users to retrieve the latest telemetry snapshot from an ACTIVE edge device, including: capturedAt timestamp, CPU metrics, RAM metrics (percent + MB), disk metrics (percent + GB), temperature, uptime, and PLC connectivity info (reachable flag, latency ms, last heartbeat timestamp).
- **FR-017**: The system MUST allow authorized users to retrieve the event history for an edge device, with each event containing: id, checkType, checkedAt, overallStatus, summary, userId, and userEmail of the triggering user.
- **FR-018**: The system MUST assign a server-generated UUID as the device identifier at registration time.
- **FR-019**: The system MUST persist createdAt and updatedAt timestamps for each device, updating updatedAt on every modification.

### Key Entities

- **EdgeDevice**: Represents a physical edge computing unit deployed at an industrial site. Key attributes:
  - Unique ID (server-generated, UUID)
  - Tenant affiliation (immutable, set at registration, scoped by tenantId path param)
  - Name and optional description (mutable via update)
  - Machine identifier (`machineId`) — unique per tenant, immutable after registration
  - Edge type (e.g., `RASPBERRY_PLC`) — defines the hardware profile
  - Connectivity addresses: Raspberry Pi base URL and optional PLC IP address
  - Operational status: `ACTIVE` or `DISABLED`
  - Last seen timestamp, last health check timestamp, last health status (`OK`, `UNKNOWN`), last health summary
  - Creation and last-updated timestamps

- **CheckResult**: Represents the outcome of an on-demand check performed against a device. Key attributes:
  - Check type: `STATUS` (connectivity + version) or `HEALTH_CHECK` (full hardware diagnostics)
  - Timestamp of when the check was performed
  - Overall status: `OK`, `DEGRADED`, `ERROR`, or `UNKNOWN`
  - Human-readable summary
  - Details: type-specific payload (software version for STATUS; hardware resource metrics for HEALTH_CHECK)

- **TelemetrySnapshot**: Represents a point-in-time hardware metrics reading from a device. Key attributes:
  - Capture timestamp
  - CPU usage (percent)
  - RAM usage (percent, used MB, total MB)
  - Disk usage (percent, used GB, total GB)
  - Temperature in Celsius
  - Uptime in seconds
  - PLC connectivity state: reachable flag, round-trip latency in ms, last heartbeat timestamp

- **DeviceEvent**: An immutable entry in the historical log of checks performed on a device. Key attributes:
  - Unique ID
  - Check type and result (checkType, overallStatus, summary)
  - Timestamp of the check (checkedAt)
  - User who triggered the check (userId + userEmail)

- **Tenant**: Organizational unit that groups edge devices. Identified by the tenantId path parameter; used to enforce data isolation across all operations.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Administrators can view the full list of registered edge devices for their tenant in under 500ms for tenants with up to 1,000 devices.
- **SC-002**: Single device retrieval returns in under 200ms for any existing device.
- **SC-003**: Device registration completes in under 1 second; duplicate machineId detection returns 409 Conflict within 200ms.
- **SC-004**: Enable and disable state transitions complete in under 500ms and are immediately reflected in subsequent device queries.
- **SC-005**: Status checks and health checks return results in under 5 seconds (inclusive of physical device response time); the platform's own processing adds no more than 200ms of overhead.
- **SC-006**: Telemetry retrieval returns the latest captured snapshot in under 500ms.
- **SC-007**: The system correctly isolates edge device data per tenant — no operation exposes devices from a different tenant; attempting cross-tenant access returns 403 Forbidden.
- **SC-008**: 100% of invalid requests (missing or invalid auth, missing required fields, operations on DISABLED devices) receive a descriptive error response with an appropriate HTTP status code and a machine-readable error code.
- **SC-009**: 100% of device registrations result in a record with a unique UUID, ACTIVE status, UNKNOWN health state, and accurate creation and update timestamps.
- **SC-010**: machineId uniqueness is correctly enforced per tenant — the same machineId can coexist across different tenants without conflict, but within the same tenant it is guaranteed unique.
