# Feature Specification: Dashboard Layouts API

**Feature Branch**: `005-dashboard-layouts`
**Created**: 2026-03-24
**Status**: Draft
**Input**: User description: "Implement Dashboard Layouts API: tenants can create, list, update and delete dashboard layout configurations with widget positions stored as JSONB. Max 3 layouts per tenant. Cannot delete the last layout."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - List Dashboard Layouts (Priority: P1)

A tenant user views all saved dashboard layouts to select which one to display on their monitoring screen.

**Why this priority**: The list is the entry point for all other layout operations. Without it, users cannot discover or navigate to existing layouts.

**Independent Test**: Can be fully tested by authenticating with a valid token and requesting the layout list for a tenant, verifying that all registered layouts are returned with their widget configurations and metadata.

**Acceptance Scenarios**:

1. **Given** a tenant has one or more layouts saved, **When** an authenticated user requests the layout list, **Then** the system returns all layouts with their id, name, widget list, createdAt, updatedAt, and a meta object with total count and the per-tenant limit.
2. **Given** a tenant has no layouts, **When** the list is requested, **Then** the system returns an empty data array with meta.total = 0.
3. **Given** an unauthenticated request, **When** the list is requested, **Then** the system returns 401 Unauthorized.

---

### User Story 2 - Create a Dashboard Layout (Priority: P1)

A tenant administrator creates a new named layout with an initial set of widgets to customize their monitoring view.

**Why this priority**: Creating a layout is the prerequisite for any personalized monitoring experience. Without at least one layout, the dashboard cannot display anything.

**Independent Test**: Can be tested by submitting a new layout with a unique name and widget list and verifying that the response returns the created layout with a server-assigned id and timestamps.

**Acceptance Scenarios**:

1. **Given** a tenant has fewer than 3 layouts and no layout named "Layout Produccion", **When** an authenticated user creates a layout with that name and a widget list, **Then** the system returns 200 with the created layout including a server-generated id, the provided name, widgets, and createdAt/updatedAt timestamps.
2. **Given** a tenant already has a layout named "Dashboard Principal", **When** a new layout with that same name is submitted, **Then** the system returns 409 Conflict with error code `DUPLICATE_NAME`.
3. **Given** a tenant already has 3 layouts, **When** a new layout creation is attempted, **Then** the system returns 403 Forbidden with error code `LIMIT_REACHED`.

---

### User Story 3 - Get a Specific Dashboard Layout (Priority: P2)

A tenant user retrieves a single layout by its ID to load its widget configuration into the dashboard renderer.

**Why this priority**: Needed for rendering a specific layout. Lower priority than list/create since it's a supporting operation to the main workflow.

**Independent Test**: Can be tested by requesting a known layout by ID and verifying that the full widget configuration is returned correctly.

**Acceptance Scenarios**:

1. **Given** a layout with a known ID exists for the tenant, **When** an authenticated user requests it by ID, **Then** the system returns 200 with the full layout profile: id, name, widgets, createdAt, updatedAt.
2. **Given** a layout ID that does not exist, **When** requested, **Then** the system returns 404 Not Found.

---

### User Story 4 - Update a Dashboard Layout (Priority: P2)

A tenant administrator renames a layout or rearranges its widget positions to reflect an updated monitoring configuration.

**Why this priority**: Supports the iterative customization of monitoring views without needing to delete and recreate layouts.

**Independent Test**: Can be tested by submitting an update with a new name and/or new widget list and verifying the response reflects the changes with a refreshed updatedAt timestamp.

**Acceptance Scenarios**:

1. **Given** a layout exists, **When** an authenticated user submits a new name and/or widget list, **Then** the system returns 200 with the updated layout and a refreshed updatedAt timestamp.
2. **Given** a tenant has layouts A and B, **When** the user tries to rename layout A to match the name of layout B, **Then** the system returns 409 Conflict with error code `DUPLICATE_NAME`.
3. **Given** a layout ID that does not exist, **When** an update is submitted, **Then** the system returns 404 Not Found with error code `NOT_FOUND`.

---

### User Story 5 - Delete a Dashboard Layout (Priority: P2)

A tenant administrator removes an unused layout to keep the layout list clean and under the 3-layout limit.

**Why this priority**: Allows tenants to stay under the limit and maintain an organized set of dashboards. Includes the safety rule of preserving at least one layout.

**Independent Test**: Can be tested by deleting one layout from a tenant with multiple layouts and verifying it is removed. Deleting the last layout must be rejected.

**Acceptance Scenarios**:

1. **Given** a tenant has two or more layouts, **When** an authenticated user deletes one by ID, **Then** the system returns 200 with a success message confirming deletion.
2. **Given** a tenant has exactly one layout, **When** the user attempts to delete it, **Then** the system returns 400 Bad Request — tenants must retain at least one layout at all times.
3. **Given** a layout ID that does not exist, **When** deletion is attempted, **Then** the system returns 404 Not Found.

---

### Edge Cases

- What happens when no Authorization token is provided? (Return 401 Unauthorized)
- What happens when the name field is empty or missing on create/update? (Return 400 Bad Request)
- What happens when the widgets array is omitted? (Treat as empty widget list — still valid)
- What happens if two concurrent requests create layouts with the same name in the same tenant? (Exactly one succeeds; the other receives 409 Conflict)
- What happens if an update changes only the widget positions without changing the name? (Operation succeeds; name uniqueness check applies only against other layouts)
- What happens when a tenant already has 3 layouts and tries to update one? (Update is allowed — only creation is restricted by the limit)

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST require a valid Bearer JWT token in the Authorization header for every endpoint; requests without a valid token MUST return 401 with `{ success: false, error: "No autorizado" }`.
- **FR-002**: The system MUST scope all layout operations to the tenant identified by the `X-Tenant-ID` request header (UUID), enforcing strict data isolation between tenants.
- **FR-003**: The system MUST allow authenticated users to retrieve the list of all dashboard layouts for their tenant, returning each layout's id, name, widget list, createdAt, updatedAt, and a meta object with total and limit.
- **FR-004**: The system MUST allow authenticated users to create a new dashboard layout providing a name and an optional widgets array.
- **FR-005**: The system MUST enforce a maximum of 3 layouts per tenant; creation attempts when the limit is reached MUST return 403 with error code `LIMIT_REACHED`.
- **FR-006**: The system MUST enforce name uniqueness per tenant; creation or update with a name that already belongs to another layout in the same tenant MUST return 409 with error code `DUPLICATE_NAME`.
- **FR-007**: The system MUST assign a server-generated unique ID, and server-assigned createdAt and updatedAt timestamps to each layout at creation time.
- **FR-008**: The system MUST allow authenticated users to retrieve a single layout by its ID within the tenant, returning 404 when not found.
- **FR-009**: The system MUST allow authenticated users to update a layout's name and/or widget list by ID; updatedAt MUST be refreshed on every successful update.
- **FR-010**: The system MUST allow authenticated users to delete a layout by ID, but MUST return 400 if the target layout is the tenant's only remaining layout.
- **FR-011**: Each widget MUST carry: id, type, name, title, description, category, icon, and a position object with x, y, w, h, and i fields.
- **FR-012**: The system MUST store the widgets array as a structured document; the full widget list is replaced on every update.
- **FR-013**: The system MUST return `{ success: true, data: ... }` on successful responses and `{ success: false, error: "..." }` on all error responses.

### Key Entities

- **DashboardLayout**: A named configuration of widgets saved by a tenant for their monitoring view. Key attributes:
  - Unique ID (server-generated)
  - Tenant affiliation (scoped by `X-Tenant-ID` header UUID, immutable after creation)
  - Name (mutable, unique per tenant)
  - Widgets array (mutable, replaced in full on each update)
  - Creation timestamp (immutable)
  - Last-updated timestamp (refreshed on every modification)

- **Widget**: A visual component within a layout, defined entirely within the layout document. Key attributes:
  - Unique ID within the layout
  - Type (e.g., `machine-status`, `bag-counter`)
  - Display fields: name, title, description, category, icon
  - Position: grid coordinates (x, y) and dimensions (w, h) plus grid key (i)

- **Tenant**: Organizational unit that scopes all layout operations. Identified by the `X-Tenant-ID` request header (UUID).

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Users can retrieve the full layout list for their tenant in under 300ms for tenants with up to 3 layouts.
- **SC-002**: Layout creation completes in under 500ms; limit and name uniqueness violations return their error responses in under 200ms.
- **SC-003**: Single layout retrieval returns in under 200ms.
- **SC-004**: Layout update completes in under 500ms and the refreshed updatedAt is immediately visible in subsequent queries.
- **SC-005**: Layout deletion completes in under 300ms; rejection of last-layout deletion returns in under 200ms.
- **SC-006**: The system correctly isolates layout data per tenant — no operation can read or modify layouts from a different tenant.
- **SC-007**: 100% of requests without a valid token receive 401 Unauthorized.
- **SC-008**: The 3-layout limit is enforced with zero false positives — existing layouts are never blocked from being updated or read due to the limit.
- **SC-009**: Name uniqueness is enforced per tenant — the same layout name can coexist across different tenants without conflict.
- **SC-010**: A tenant always retains at least one layout; deletion of the last layout is rejected 100% of the time.

## Assumptions

- Widget IDs are client-generated and stored as-is; the server does not validate widget ID uniqueness within a layout.
- The widgets array is a full replacement on update — partial widget patching is not supported.
- Authentication is handled by Supabase JWT (consistent with existing API surface).
- The tenant is identified via the `X-Tenant-ID` request header (UUID). The user is identified from the JWT Bearer token.
- No pagination is required for the list endpoint given the hard limit of 3 layouts per tenant.
- All authenticated users within a tenant can perform all layout operations (no sub-role restrictions beyond JWT auth).
