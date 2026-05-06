# Research: Dashboard Layouts API

**Feature**: 005-dashboard-layouts
**Date**: 2026-03-24

---

## Decision 1: Widget storage strategy

**Decision**: Store widgets as a JSONB column in the `dashboard_layouts` table (single-document model). No separate `widgets` table.

**Rationale**: Widgets are always read/written as a complete unit — there is no use case for querying individual widgets across layouts. A JSONB column avoids join overhead and perfectly matches the Pact contract which treats widgets as an embedded array. Widget schema is flexible (different types have different fields), which JSONB handles naturally.

**Alternatives considered**:
- Separate `widgets` table with FK to `dashboard_layouts` — rejected because it adds join complexity for zero query benefit; widget queries are always scoped to one layout.
- Text/JSON string column — rejected in favor of JSONB for native indexing support if needed in future.

---

## Decision 2: Layout ID format

**Decision**: Use PostgreSQL `UUID` as the primary key, server-generated via `gen_random_uuid()`.

**Rationale**: Consistent with all other entities in the project (users, tenants, edge devices all use UUID PKs). The Pact contract uses string IDs like `"acme-1708000000000"` but the matching rule is `{ "match": "type" }` — any string is accepted, so a UUID string satisfies the contract.

**Alternatives considered**:
- Timestamp-based slugs (e.g., `acme-1708000000000`) — rejected; not idiomatic, collision-prone, leaks timestamp info.
- Sequential integer IDs — rejected; UUID is already the project standard.

---

## Decision 3: Tenant isolation strategy

**Decision**: Reuse the existing `/api/tenants/:tenantId` Gin group with `apimw.ResolveTenantFromPath(db)` middleware. Dashboard layout routes are registered into this same group alongside edge device routes.

**Rationale**: The middleware already resolves the `:tenantId` subdomain slug to a UUID and injects it into context via `platform.TenantID`. Reusing it means zero new middleware code and consistent tenant isolation behavior.

**Alternatives considered**:
- New separate Gin group — rejected; duplicates middleware wiring for identical behavior.
- `X-Tenant-ID` header (existing v1 pattern) — rejected; Pact contract uses path parameter, not header.

---

## Decision 4: Name uniqueness enforcement

**Decision**: Enforce via a PostgreSQL `UNIQUE` constraint on `(tenant_id, name)` where `deleted_at IS NULL`, implemented as a partial unique index.

**Rationale**: Database-level enforcement is the only safe guarantee against race conditions (two concurrent creates with the same name). The partial index excludes soft-deleted rows, so a deleted layout name can be reused.

**Alternatives considered**:
- Application-level check (SELECT before INSERT) — rejected; susceptible to TOCTOU race conditions under concurrent requests.
- Full UNIQUE constraint without soft-delete awareness — rejected; would block name reuse after deletion.

---

## Decision 5: Max-3-layouts enforcement

**Decision**: Enforce at the application service layer with a `COUNT` query before insert, wrapped in a database transaction.

**Rationale**: There is no clean PostgreSQL constraint for "max rows per tenant group". Application-level enforcement inside a transaction is the standard approach. The transaction prevents race conditions where two concurrent creates both pass the count check.

**Alternatives considered**:
- PostgreSQL trigger — possible but adds DDL complexity; application-layer is more visible and testable.
- Advisory locks — overkill for this use case.

---

## Decision 6: Migration numbering

**Decision**: Use `0006_create_dashboard_layouts_table` (4-digit prefix, matching the newer migration naming convention: `0004_`, `0005_`).

**Rationale**: The repo has two naming conventions (6-digit `000001_` and 4-digit `0004_`). The newer features (user management, edge devices) use 4-digit. Continuing that pattern for consistency within the new feature set.

---

## Decision 7: No separate app/ service package

**Decision**: Follow the pattern from `003-edge-device-management` — implement the service in `internal/app/dashboard_layouts/service.go`.

**Rationale**: Consistent with existing architecture. The service layer orchestrates domain + repository and is the boundary for business rules (limit check, uniqueness).

---

## Decision 8: Soft delete

**Decision**: Include a `deleted_at` column for soft delete, consistent with the users table pattern.

**Rationale**: Soft delete allows name reuse after deletion and preserves audit history. The partial unique index on `(tenant_id, name) WHERE deleted_at IS NULL` handles this correctly.
