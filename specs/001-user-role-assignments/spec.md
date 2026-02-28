# Feature Specification: User Role Assignment Management

**Feature Branch**: `001-user-role-assignments`
**Created**: 2026-02-27
**Status**: Draft
**Input**: User description: "quiero implementar los cambios que estuvimos relevando"

## User Scenarios & Testing *(mandatory)*

<!--
  IMPORTANT: User stories should be PRIORITIZED as user journeys ordered by importance.
  Each user story/journey must be INDEPENDENTLY TESTABLE - meaning if you implement just ONE of them,
  you should still have a viable MVP (Minimum Viable Product) that delivers value.

  Assign priorities (P1, P2, P3, etc.) to each story, where P1 is the most critical.
  Think of each story as a standalone slice of functionality that can be:
  - Developed independently
  - Tested independently
  - Deployed independently
  - Demonstrated to users independently
-->

### User Story 1 - Assign a Role to a User (Priority: P1)

As an administrator, I need to assign a specific role to a user within my organization's tenant so that the user gains the appropriate level of access to the platform.

**Why this priority**: Role assignment is the foundational operation. Without it, no role management is possible. Every other story depends on assignments existing first.

**Independent Test**: An administrator can assign a role to a user, and immediately the system reflects that the user holds that role in that tenant.

**Acceptance Scenarios**:

1. **Given** an administrator is authenticated and a user exists in the system without a role in this tenant, **When** the administrator assigns a role (e.g., "Operario") to that user, **Then** the system confirms the assignment and the user's status is immediately "active".
2. **Given** a user already has an active role assignment in this tenant, **When** an administrator attempts to assign another role to the same user, **Then** the system rejects the request with a clear message indicating the user already has an active role and directing the administrator to update instead.
3. **Given** an administrator provides invalid or incomplete data (missing user ID, tenant ID, or role ID), **When** the assignment is attempted, **Then** the system rejects it with an informative validation error.

---

### User Story 2 - View Role Assignments for a Tenant (Priority: P1)

As an administrator, I need to see all user-role assignments within my organization's tenant, optionally filtered by status, so that I can audit and manage who has access and at what level.

**Why this priority**: Visibility into existing assignments is critical for oversight and management. Administrators cannot effectively manage roles without being able to list them.

**Independent Test**: An administrator can retrieve the full list of role assignments for their tenant and can filter the list by status (e.g., only pending, only active, only revoked).

**Acceptance Scenarios**:

1. **Given** a tenant has multiple user-role assignments in various states, **When** an administrator requests the full list, **Then** all assignments are returned with their complete details (user identifier, role, status, who assigned, when).
2. **Given** a tenant has a mix of active, pending, and revoked assignments, **When** the administrator filters by status "pending", **Then** only pending assignments are returned.
3. **Given** a tenant has no assignments yet, **When** the administrator requests the list, **Then** an empty list is returned without error.

---

### User Story 3 - Revoke a Role Assignment (Priority: P2)

As an administrator, I need to revoke a user's role assignment within my tenant so that the user loses access when they leave the organization or change responsibilities, while preserving an audit trail of the history.

**Why this priority**: Access revocation is critical for security. Historical record preservation ensures accountability and auditability.

**Independent Test**: An administrator can revoke an existing assignment, and the system marks it as revoked (not deleted), so the audit history remains intact.

**Acceptance Scenarios**:

1. **Given** a user has an active role assignment, **When** an administrator revokes it, **Then** the assignment's status changes to "revoked", the original data is preserved, and the user no longer appears as actively assigned.
2. **Given** an administrator attempts to revoke a non-existent assignment, **Then** the system returns a clear "not found" error.
3. **Given** an assignment has already been revoked, **When** the administrator attempts to revoke it again, **Then** the system handles this gracefully without error or data corruption.

---

### User Story 4 - Update a Role Assignment (Priority: P2)

As an administrator, I need to change a user's role within my tenant (e.g., promote from Operario to Admin) without needing to revoke and reassign, so that role transitions are seamless and the assignment record is preserved.

**Why this priority**: Role transitions are a common operational need. Forcing a revoke + reassign workflow would lose continuity and create unnecessary friction.

**Independent Test**: An administrator can update the role on an existing assignment, and the change is immediately reflected without creating duplicate records.

**Acceptance Scenarios**:

1. **Given** a user has an active assignment with role "Operario", **When** the administrator changes the role to "Admin", **Then** the same assignment record is updated with the new role and the updated timestamp.
2. **Given** an administrator attempts to update a non-existent assignment, **Then** the system returns a clear "not found" error.

---

### User Story 5 - Bulk Assign Roles to Multiple Users (Priority: P3)

As an administrator, I need to assign the same role to multiple users at once (e.g., when onboarding a team), so that I don't have to perform individual assignments for each user separately.

**Why this priority**: Bulk operations improve administrative efficiency for large teams but are not critical for initial functionality; individual assignment covers the core use case.

**Independent Test**: An administrator can select multiple users and a role, submit a bulk assignment, and all users receive that role in a single operation. If any conflict exists, the entire operation is rejected cleanly.

**Acceptance Scenarios**:

1. **Given** multiple users exist without active roles in a tenant, **When** an administrator bulk-assigns a role to all of them, **Then** all assignments are created and the response confirms the total count of successful assignments.
2. **Given** one of the selected users already has an active role in the tenant, **When** the bulk assignment is attempted, **Then** the entire operation is rejected with a clear conflict message and no partial changes are made.
3. **Given** an empty list of users is provided, **When** the bulk assignment is attempted, **Then** the system rejects it with a validation error.

---

### User Story 6 - View a User's Roles Across All Tenants (Priority: P3)

As a platform administrator (MRG Admin), I need to see all the roles a specific user holds across every tenant in the system, so that I can have a complete picture of that user's access for platform-wide governance.

**Why this priority**: Cross-tenant visibility is a power-admin feature for the MRG Admin role. Standard tenant admins do not require this; it is a platform governance capability.

**Independent Test**: A platform administrator can look up any user by their identifier and receive a consolidated list of all their role assignments across all tenants, including tenant names and role names.

**Acceptance Scenarios**:

1. **Given** a user has active assignments in two different tenants, **When** a platform administrator requests that user's roles, **Then** both assignments are returned with tenant name, role name, and current status.
2. **Given** a user has no assignments in any tenant, **When** their roles are requested, **Then** an empty list is returned without error.

---

### Edge Cases

- What happens when a `userId` or `tenantId` references an entity that does not exist in the system?
- How does the system handle concurrent assignment requests for the same user+tenant combination (race condition)?
- What happens when the bulk assignment list contains duplicate user IDs?
- Are revoked assignments included in the default list response, or only when explicitly filtered?
- What happens when an administrator attempts to update the role on an already-revoked assignment?
- What happens when the role ID provided does not correspond to a valid role in the system?

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST allow an authorized administrator to assign exactly one role to a user within a given tenant; if the user already has an active role in that tenant, the system MUST reject the request with a conflict status.
- **FR-002**: The system MUST allow an authorized administrator to list all user-role assignments for a tenant, returning all relevant details (user identifier, role, status, who assigned, and timestamps).
- **FR-003**: The system MUST allow administrators to filter the list of assignments by status (active, pending, or revoked).
- **FR-004**: The system MUST allow an authorized administrator to update the role on an existing assignment without creating a new record.
- **FR-005**: The system MUST allow an authorized administrator to revoke a role assignment; the assignment record MUST be preserved with status "revoked" (the record is never physically deleted).
- **FR-006**: The system MUST allow an authorized administrator to assign the same role to multiple users in a single operation (bulk assignment); if any of the selected users already has an active role in the target tenant, the entire operation MUST be rejected without making any partial changes.
- **FR-007**: The system MUST allow a platform administrator to retrieve all role assignments for a specific user across all tenants, including the human-readable name of each tenant and role.
- **FR-008**: All role assignment operations MUST automatically record the identity of the authenticated actor who performed the action.
- **FR-009**: All write operations MUST require authentication; unauthenticated requests MUST be rejected.
- **FR-010**: The system MUST validate all required inputs and return clear, descriptive error messages for any invalid or missing data.

### Key Entities *(include if feature involves data)*

- **Role**: A named permission level within the platform (e.g., Admin, Operario, Cliente Admin, Cliente Operario). Each role has a unique identifier and a human-readable name. Roles are predefined by the platform.
- **User-Tenant-Role Assignment (UTR)**: Represents the tripartite relationship between a user, a tenant, and a role. Tracks the current status of the assignment (active, pending, or revoked), the identity of who performed the assignment, and when the assignment was created, activated, and last modified.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: An administrator can complete a single role assignment in under 30 seconds from initiating the request to receiving confirmation.
- **SC-002**: The unique-active-role constraint is enforced in 100% of assignment attempts — zero duplicate active assignments can exist for the same user+tenant combination.
- **SC-003**: Revoked assignments are always preserved — the rate of data loss on revocation is 0%.
- **SC-004**: A bulk assignment of up to 100 users completes within 5 seconds under normal operating conditions.
- **SC-005**: All unauthorized or unauthenticated requests are rejected — 0% of unprotected access succeeds.
- **SC-006**: Status filters return exclusively matching results — 100% filter accuracy with no cross-status leakage.
- **SC-007**: The system returns a meaningful error message for 100% of invalid or malformed requests, with no silent failures or unhandled errors.
