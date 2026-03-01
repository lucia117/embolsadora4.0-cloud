# Research: User Role Assignment Management

**Branch**: `001-user-role-assignments` | **Date**: 2026-02-27

No NEEDS CLARIFICATION markers were identified during spec writing. All decisions were resolved either from the Pact contract, the codebase exploration, or team discussion prior to planning. This document consolidates those decisions as a reference for implementation.

---

## Decision 1: Unique Active Role Constraint Strategy

**Decision**: Enforce "one active role per user+tenant" using a PostgreSQL partial unique index at the DB level, supplemented by application-level error handling for 409 responses.

**Rationale**:
- A DB-level constraint is the most reliable guard against race conditions and bypasses. Application-level checks alone (check-then-insert) are vulnerable to TOCTOU races under concurrent load.
- A **partial index** (`WHERE status = 'active'`) allows unlimited `revoked` and `pending` records for the same user+tenant pair, which is required for audit trail preservation.
- pgx/v5 returns a `*pgconn.PgError` with `Code == "23505"` (unique_violation) on conflict, which the use case can intercept and convert to a domain error for the handler to map to HTTP 409.

**Alternatives considered**:
- Advisory locks + application check: more complex, still requires DB fallback; rejected in favor of DB constraint.
- Unique index on all statuses: prevents revoked history for same user+tenant; incompatible with FR-005 (soft delete).

---

## Decision 2: Soft Delete (Revoke) Semantics

**Decision**: DELETE endpoint does not remove the row. It sets `status = 'revoked'` and updates `updated_at`. The row is permanently preserved.

**Rationale**: FR-005 and SC-003 explicitly require audit trail preservation. The Pact contract DELETE response returns `{id, status: "revoked"}`, confirming the record persists post-revocation.

**Alternatives considered**:
- Hard delete + audit log table: higher implementation complexity, two writes; rejected.
- Soft delete with `deleted_at` timestamp: less idiomatic for this domain where status is already the semantic signal; rejected in favor of status field.

---

## Decision 3: Bulk Assign — All-or-Nothing Transaction

**Decision**: `POST /user-roles/bulk` executes inside a single PostgreSQL transaction. If any of the target users already has an active role in the tenant, the transaction is rolled back entirely and a 409 is returned.

**Rationale**: Team decision during planning (confirmed). Ensures data consistency and predictable behavior for clients. The Pact response format `{assigned: N, failed: 0, assignments: [...]}` remains compatible — on success, failed is always 0.

**Alternatives considered**:
- Partial success (assign some, fail others): rejected by team in favor of all-or-nothing.
- Pre-check then bulk insert (two-phase): still vulnerable to race conditions; rejected in favor of single transaction with constraint enforcement.

**Implementation note**: Use `pgxpool.Pool.Begin(ctx)` to get a transaction, iterate inserts, catch unique constraint violation (PgError 23505), rollback and return `ErrUserAlreadyHasActiveRole`. Commit only if all succeed.

---

## Decision 4: Roles as a Catalog Table

**Decision**: Create a `roles` table with predefined rows (id, name). Role IDs are short, human-readable strings (`admin`, `operario`, `cliente_admin`, `cliente_operario`).

**Rationale**: The Pact `GET /users/:userId/roles` response includes `roleName` (e.g., "Admin", "Operario"), which requires a JOIN source. Deriving the name from the ID string (title-casing) would be fragile and non-extensible. A catalog table provides a proper JOIN target and allows future role additions without code changes.

**Alternatives considered**:
- Derive roleName from roleId string: fragile, not extensible, breaks if names diverge from IDs; rejected.
- Roles as an enum in PostgreSQL: harder to extend without migrations; rejected.

**Predefined roles**:

| id | name |
|----|------|
| `admin` | Admin |
| `operario` | Operario |
| `cliente_admin` | Cliente Admin |
| `cliente_operario` | Cliente Operario |

---

## Decision 5: Response Envelope Format

**Decision**: All user-role endpoints use the Pact-defined response envelope: `{"success": true, "data": {...}}` for success, `{"success": false, "error": "..."}` for errors.

**Rationale**: This is mandated by the Pact contract. The existing tenant endpoints use a different format (raw objects/arrays), but since user-roles is a new feature with its own Pact contract, it must adhere to its own contract. Mixing is acceptable in a monolith where each feature has independent contracts.

**Alternatives considered**:
- Align with tenant response format: would violate the Pact contract; rejected.
- Unified response envelope across all features: scope creep for this feature; deferred to a future refactor.

---

## Decision 6: assignedBy Source

**Decision**: `assigned_by` is populated from the authenticated user's identity extracted from the JWT claims via `c.Request.Context()` (the same context used by `apimw.TenantFromJWT()`).

**Rationale**: The middleware already extracts tenant and user identity from the JWT. For now (since RBAC is a TODO in the project), the `assignedBy` field is set from the JWT subject claim. If the JWT doesn't carry a user ID, the field is stored as NULL.

**Implementation note**: Add a `platform.UserID(ctx)` helper (or use existing context keys) to retrieve the authenticated user's ID within use cases.

---

## Decision 7: UUID vs. Custom ID Format for UTR records

**Decision**: Use standard UUIDs (`gen_random_uuid()`) for UTR record IDs, not the `utr_` prefix format shown in the Pact examples.

**Rationale**: The Pact `matchingRules` for `$.body.data.id` use `"match": "type"` (not regex), meaning any string value satisfies the contract. The project consistently uses UUIDs across all entities. Consistency takes precedence over matching the example data literally.

---

## Decision 8: GET /user-roles tenantId Parameter

**Decision**: Accept `tenantId` as a UUID query parameter. Validate format (must be parseable as UUID) and return 400 on invalid format.

**Rationale**: The Pact example uses `tenantId=acme` (a string), but the project uses UUIDs as tenant identifiers throughout. The Pact `matchingRules` do not constrain the tenantId format. UUID consistency is maintained across the system.

---

## No Open Questions

All NEEDS CLARIFICATION items have been resolved. Implementation may proceed directly to data modeling and contract definition.
