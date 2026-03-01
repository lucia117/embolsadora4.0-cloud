# Feature Specification: User Management API

**Feature Branch**: `002-user-management`
**Created**: 2026-03-01
**Status**: Draft
**Input**: User description: "Pact contract para user-service — CRUD completo de usuarios multi-tenant"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - List Users for a Tenant (Priority: P1)

An administrator accesses the list of all users belonging to their tenant to review who has access to the platform. Users are fetched with pagination (limit + offset) to handle large user bases efficiently.

**Why this priority**: Core operation that enables tenant user governance; all other user operations depend on being able to see existing users.

**Independent Test**: Can be fully tested by requesting the user list for a given tenant (via X-Tenant-ID header) and verifying paginated results include all expected users with their roles.

**Acceptance Scenarios**:

1. **Given** users exist for a tenant, **When** an authorized user requests the user list with X-Tenant-ID header and optional pagination params (?limit=20&offset=0), **Then** the system returns all users belonging to that tenant (filtered by soft-deleted flag) with full profile (id, firstName, lastName, email, role, tenantId, timestamps), limited to the specified page.
2. **Given** no users exist for a tenant, **When** the user list is requested, **Then** the system returns an empty list with pagination metadata.
3. **Given** pagination params are missing, **When** the user list is requested, **Then** the system returns a default page (limit=20, offset=0).
4. **Given** an unauthorized request (no valid auth token), **When** the user list is requested, **Then** the system returns an authentication error.

---

### User Story 2 - View a Specific User's Profile (Priority: P2)

An administrator retrieves the full profile of a specific user by their unique ID to review their details.

**Why this priority**: Enables targeted user inspection before editing or taking action on a single user.

**Independent Test**: Can be tested by requesting a user by a known ID and verifying all profile fields are returned correctly, including the optional avatar image URL.

**Acceptance Scenarios**:

1. **Given** a user with a known ID exists, **When** a request is made for that user by ID, **Then** the system returns the full user profile (id, firstName, lastName, email, role, tenantId, image, createdAt, updatedAt).
2. **Given** a user ID that does not exist, **When** a request is made for that user, **Then** the system returns a not-found error.

---

### User Story 3 - Create a New User (Priority: P1)

An administrator creates a new user account and assigns them a role within the tenant. The system ensures email uniqueness is scoped per tenant (two different tenants can have users with the same email).

**Why this priority**: Essential for onboarding new personnel to the platform.

**Independent Test**: Can be tested by submitting a new user profile with X-Tenant-ID header and verifying the created user record is returned with a server-assigned ID and timestamps.

**Acceptance Scenarios**:

1. **Given** no user exists with a given email in the tenant, **When** a complete user profile is submitted via POST (firstName, lastName, email, role with X-Tenant-ID header), **Then** the system creates the user and returns the full profile (201) with a generated ID, timestamps, and deleted_at=null.
2. **Given** a user with the same email already exists within the same tenant, **When** a creation request is submitted with that email, **Then** the system returns a 409 Conflict error.
3. **Given** the same email exists in a different tenant, **When** the user is created in the current tenant, **Then** the system allows creation (email is unique per tenant, not globally).
4. **Given** required fields are missing (e.g., email or role), **When** a creation request is submitted, **Then** the system returns a 400 Bad Request with validation errors.
5. **Given** an optional avatar image URL is provided, **When** the user is created, **Then** the image URL is persisted and returned in the profile.

---

### User Story 4 - Update an Existing User's Profile (Priority: P2)

An administrator partially updates a user's profile (e.g., name or role) without replacing the entire record.

**Why this priority**: Supports role reassignment and profile corrections without destructive full-record replacement.

**Independent Test**: Can be tested by submitting a partial update and verifying only the specified fields changed while others remain intact.

**Acceptance Scenarios**:

1. **Given** a user exists, **When** a partial update is submitted with changed fields (e.g., firstName, role), **Then** the system returns the updated full profile with only those fields changed and the updatedAt timestamp refreshed.
2. **Given** a user ID that does not exist, **When** an update is submitted, **Then** the system returns a not-found error.
3. **Given** an update with an invalid role value, **When** the update is submitted, **Then** the system returns a validation error.

---

### User Story 5 - Remove a User (Priority: P3)

An administrator removes a user from the platform, revoking their access. Deletion is soft (logical) — the user record is marked as deleted but retained in the database for audit and compliance purposes.

**Why this priority**: Necessary for offboarding but lower priority since existing users can still operate without this capability in the short term.

**Independent Test**: Can be tested by deleting a known user with X-Tenant-ID header and verifying the operation succeeds (204 No Content) and the user no longer appears in list queries.

**Acceptance Scenarios**:

1. **Given** a user exists, **When** a DELETE request is submitted with X-Tenant-ID header, **Then** the system soft-deletes the user (sets deleted_at timestamp) and returns 204 No Content.
2. **Given** a soft-deleted user exists, **When** the user list is requested, **Then** the user does not appear in results (filtered by deleted_at IS NULL).
3. **Given** a soft-deleted user's ID is requested directly, **When** a GET request is made, **Then** the system returns 404 Not Found (treating soft-deleted as non-existent for query purposes).
4. **Given** a user ID that does not exist, **When** a deletion request is submitted, **Then** the system returns 404 Not Found.

---

### Edge Cases

- What happens when the X-Tenant-ID header is missing from a request? (Return 400 Bad Request)
- What happens when a user from tenant A attempts to list users from tenant B? (Return 403 Forbidden)
- What happens if pagination params are invalid (limit < 1, negative offset)? (Return 400 Bad Request with validation message)
- What happens if an update (PATCH) attempts to change the email or tenantId? (Return 400 Bad Request — these fields are immutable)
- How does the system behave when concurrent requests try to create users with the same email in the same tenant? (Database constraint ensures exactly one succeeds, other gets 409 Conflict)
- What happens when a soft-deleted user is restored? (Out of scope for MVP; not supported in initial version)

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST accept X-Tenant-ID header in all requests and use it to scope user operations to the identified tenant.
- **FR-002**: The system MUST return 400 Bad Request if X-Tenant-ID header is missing.
- **FR-003**: The system MUST return 403 Forbidden if a user attempts to access data from a tenant they don't belong to (validated via JWT claims).
- **FR-004**: The system MUST retrieve a paginated list of all users belonging to a specific tenant with query parameters: limit (default 20) and offset (default 0).
- **FR-005**: The system MUST return pagination metadata in list responses: total count, returned count, limit, offset.
- **FR-006**: The system MUST allow authorized users to retrieve the full profile of a single user identified by their unique ID (soft-deleted users return 404).
- **FR-007**: The system MUST allow authorized administrators to create a new user within a tenant, requiring: firstName, lastName, email, role, with optional image URL.
- **FR-008**: The system MUST prevent creation of duplicate users with the same email address **within the same tenant** (return 409 Conflict).
- **FR-009**: The system MUST allow duplicate emails across different tenants (email uniqueness is scoped per tenant).
- **FR-010**: The system MUST allow authorized administrators to partially update a user's profile (firstName, lastName, role) without replacing the entire record.
- **FR-011**: The system MUST prevent updates to immutable fields (email, tenantId) and return 400 Bad Request if attempted.
- **FR-012**: The system MUST perform soft deletes: set deleted_at timestamp instead of removing the row, return 204 No Content.
- **FR-013**: The system MUST exclude soft-deleted users from all list queries and return 404 when accessing soft-deleted users by ID.
- **FR-014**: The system MUST assign a server-generated unique identifier, creation timestamp (createdAt), last-updated timestamp (updatedAt), and soft-delete timestamp (deleted_at) for each user.
- **FR-015**: The system MUST support an optional avatar image URL as part of a user's profile.
- **FR-016**: The system MUST enforce access control: only authorized administrators (role=admin) may perform write operations (create, update, delete).

### Key Entities

- **User**: Represents a platform user. Key attributes:
  - Unique ID (server-generated, scoped per tenant)
  - First name, last name, email (unique per tenant, immutable)
  - Assigned role (`admin`, `user`, or other defined roles)
  - Tenant affiliation (immutable, cannot be changed)
  - Optional avatar image URL
  - Creation timestamp (createdAt)
  - Last-updated timestamp (updatedAt)
  - Soft-delete timestamp (deleted_at) — null if active, set to ISO 8601 timestamp when deleted

- **Tenant**: Organizational unit that groups users. Referenced by X-Tenant-ID header in all user operations for data isolation. Each user belongs to exactly one tenant.

- **Role**: Permission level assigned to a user (e.g., `admin`, `user`). Determines what actions the user may perform on the platform. Only `admin` role can perform create, update, delete operations.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: List operation (GET /api/users) returns paginated results in under 500ms for tenants with up to 10,000 users, with default limit=20.
- **SC-002**: Single user retrieval (GET /api/users/:id) returns in under 100ms.
- **SC-003**: User creation (POST /api/users) completes in under 1 second, with duplicate email detection returning 409 Conflict within 100ms.
- **SC-004**: User update (PATCH /api/users/:id) and soft delete (DELETE /api/users/:id) complete in under 500ms.
- **SC-005**: The system correctly isolates user data per tenant — no operation exposes users from a different tenant; attempting cross-tenant access returns 403 Forbidden.
- **SC-006**: All invalid requests (missing X-Tenant-ID header, missing required fields, invalid pagination params, attempting to modify immutable fields) receive a descriptive error response (4xx status with error code and message).
- **SC-007**: 100% of user creation requests result in a user record with a unique ID, accurate timestamps (createdAt, updatedAt), and deleted_at=null.
- **SC-008**: Soft-deleted users do not appear in list queries and return 404 when accessed by ID.
- **SC-009**: Role-based access control is enforced — only users with `admin` role can execute create, update, delete operations; non-admin attempts return 403 Forbidden.
- **SC-010**: Email uniqueness is correctly scoped per tenant — two different tenants can have users with identical email addresses without conflict.
