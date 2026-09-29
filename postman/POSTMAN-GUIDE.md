# Embolsadora API - Postman Complete Guide

Master collection for testing all Embolsadora 4.0 Cloud API endpoints across all features.

## 📦 Collection Files

### Primary Collection (USE THIS)
- **`Embolsadora-API-Complete.postman_collection.json`** — Master collection with 17 folders covering every route registered in `internal/routes/url_mappings.go` and `internal/api/router.go`.

### Environment Files
- **`env-local.postman_environment.json`** — Variables for a local `go run cmd/api/main.go` server (`baseUrl=http://localhost:8080`).

## 🚀 Quick Start

### Step 1: Import Master Collection

```
Postman → File → Import → Select "Embolsadora-API-Complete.postman_collection.json"
```

### Step 2: Import the Environment

```
Postman → File → Import → Select "env-local.postman_environment.json"
```

Set `token` (a Supabase JWT — see Authentication below), `tenant_id` / `tenant_uuid` (a tenant UUID), and `tenantSubdomain` (that same tenant's subdomain, used by the Edge Devices and Public folders, which take the tenant from the URL instead of a header).

### Step 3: Start API Server

```bash
go run cmd/api/main.go
# Output: Listening on :8080
```

### Step 4: Execute Requests

Folders, in collection order:

1. **Auth** — Login, `GET /me`, `PATCH /users/me` (self-service), change password
2. **Public** — Unauthenticated tenant lookup (invitation/reset callback links)
3. **Invitations** — Create/list/resend/revoke
4. **Users (Auth)** — Force password change (admin-triggered)
5. **Tenants** — Tenant CRUD (admin)
6. **Users** — User CRUD, status, pending list, roles-across-tenants
7. **User Roles** — Assign/list/update/revoke/bulk-assign
8. **Dashboard Layouts** — Per-user dashboard layout CRUD
9. **Dashboard Metrics** — Query/catalog/batch-query time-series metrics
10. **Edge Devices** — Device CRUD, enable/disable, status/health checks, telemetry, events, Edge Device API Keys (create/list/revoke)
11. **Consumers (Edge Pi ingest)** — `POST /events` (real, frozen contract), `POST /heartbeat` (still a 501 stub)
12. **Roles** — Role CRUD (Pact-style contract tests)
13. **Alarm Rules** — Alarm rule CRUD
14. **Notifications** — List/count/ack/close
15. **Logs** — List/filter/export/stream, retention policy
16. **Permissions** — Permission catalog CRUD

---

## 🔐 Authentication

### Getting a JWT Token

#### Option 1: Generate Mock Token
Use any JWT from jwt.io with claims like:
```json
{
  "sub": "admin-user-id",
  "name": "Admin User",
  "role": "admin",
  "iat": 1677000000
}
```

#### Option 2: Use the Actual Login Endpoint
```bash
POST http://localhost:8080/api/v1/auth/login
Body: {"email": "admin@example.com", "password": "..."}
# Copy "access_token" from response → {{token}}
```

---

## 🧪 Common Testing Workflows

### Workflow 1: Complete User Management

```
1. SET Variables
   - baseUrl, token, tenant_id, tenantSubdomain

2. Users/List Users → 200 OK
3. Users/Create User → 201 Created (copies id to user_id)
4. Users/Get User by ID → 200 OK (verify creation)
5. Users/Update User → 200 OK (change role)
6. Users/Delete User → 204 No Content
7. Users/List Users → Verify user is soft-deleted (doesn't appear)
```

### Workflow 2: Role Assignment Workflow

```
1. SET Variables (same as above + userId)

2. User Roles/POST Assign Role → 201 Created (auto-extracts utrId)
3. User Roles/GET List Assignments → 200 OK
4. User Roles/PUT Update Role → 200 OK
5. User Roles/DELETE Revoke Role → 200 OK (soft revoke)
```

### Workflow 3: Edge Device Monitoring + Ingest Credentials

```
1. SET Variables
   - tenantSubdomain, device_id (from create response)

2. Edge Devices/US2 - Create Edge Device → 201 Created
3. Edge Devices/US1 - List Edge Devices → 200 OK
4. Edge Devices/US6 - Status Check → 200 OK
5. Edge Devices/API Keys - Create Device API Key → 201 Created
   (auto-extracts edge_api_key_id; the "key" field in the response is the
   ONLY time the plaintext secret is ever shown — copy it now if you plan
   to actually authenticate a Pi with it)
6. Edge Devices/API Keys - List Device API Keys → 200 OK (no secret in the list)
7. Edge Devices/API Keys - Revoke Device API Key → 204 No Content
```

### Workflow 4: Ingest a Metric, Then Query It

```
1. SET Variables
   - consumer_api_key (an active edge device API key's plaintext "key")
   - machine_id (the machineId of an existing, ACTIVE edge device)

2. Consumers (Edge Pi ingest)/POST Ingest Events → 200 OK, accepted=1
3. Dashboard Metrics/US2 - Catalog Metrics → 200 OK, confirm the aasPath appears
4. Dashboard Metrics/US1 - Query Metrics (single) → 200 OK
```

### Workflow 5: Public Tenant Lookup (no auth)

```
1. SET Variables
   - tenantSubdomain (or a tenant UUID)

2. Public/GET Public Tenant → 200 OK, no Authorization header sent
   404 if the tenant is inactive or doesn't exist
```

---

## ⚠️ Error Handling

| Status | Error Code | Cause | Resolution |
|--------|-----------|-------|-----------|
| 400 | `MISSING_HEADER` | Missing X-Tenant-ID | Add header in collection |
| 400 | `VALIDATION_ERROR` | Invalid JSON or required fields missing | Check request body |
| 400 | `INVALID_PARAMS` | Dashboard Metrics guardrail (bad range/bucket/agg) | Check `internal/domain/metrics/validate.go` |
| 400 | `EDGE_DEVICE_DISABLED` | Device status is DISABLED | Device must be ACTIVE |
| 400 | `IMMUTABLE_FIELD` | Attempted to change email/tenantId | Only update mutable fields |
| 401 | `UNAUTHORIZED` | Invalid or missing JWT | Get fresh token |
| 403 | `INSUFFICIENT_PERMISSIONS` | Missing the RBAC permission for that route | Check the request's `description` for the required `perm_*` |
| 404 | `NOT_FOUND` | Resource doesn't exist | Verify ID and tenant |
| 409 | `CONFLICT` | `machineId` already exists, or user already has an active role | Use PUT/PATCH to update instead |
| 504 | `QUERY_TIMEOUT` | Dashboard Metrics query took too long | Narrow the range/bucket |
| 500 | Internal error | Server error | Check API logs |

Notes:
- **`POST /api/v1/consumers/events` es el endpoint real**, not a stub — it always returns `200` with `{data: {accepted, rejected, errors}}`, even on partial rejection. `400` only for a malformed/oversized batch. `POST /api/v1/consumers/heartbeat` is the one still returning `501`.
- **`PATCH /api/v1/users/me`** has no RBAC check by design — any authenticated user can update their own name.
- **`GET /api/v1/public/tenants/:idOrSubdomain`** sends no `Authorization`/`X-Tenant-ID` at all.

---

## 📊 Multi-Tenant Isolation

**CRITICAL**: Almost every operation is tenant-scoped, but the mechanism differs by surface:

- **`/api/v1/*` under the main `v1` group** (Users, Invitations, Roles, Dashboard Layouts, Dashboard Metrics, Alarm Rules, Notifications, Logs, Permissions, User Roles): `X-Tenant-ID` header (UUID).
- **`/api/v1/tenants/:tenantId/edge-devices/*`** (Edge Devices, incl. API keys): tenant comes from the **URL path segment**, no header.
- **`/api/v1/consumers/*`**: no tenant header or path segment at all — the tenant is resolved server-side from the API key.
- **`/api/v1/public/*`**: no tenant scoping, no auth — deliberately safe to call before any session exists.

---

## 🛠️ Troubleshooting

### "Unknown Variable {{tenant_id}}" / "{{tenantSubdomain}}"
**Fix**: Click collection → environment → set the variable to an actual UUID/subdomain.

### "401 Unauthorized"
**Fix**: Get a fresh token from `POST /api/v1/auth/login` or jwt.io.

### "403 INSUFFICIENT_PERMISSIONS"
**Fix**: Check the request's `description` field for the exact `perm_*` id it requires, then confirm your test user's role has it (`GET /api/v1/roles` or `GET /api/v1/permissions`).

### "409 Conflict - machineId already exists"
**Fix**: Use a unique `machineId` or a different tenant.

---

## 📚 Additional Resources

- **Ingest contract**: `docs/superpowers/plans/2026-08-05-cloud-ingest-endpoint.md`
- **Edge Device Management spec**: `specs/003-edge-device-management/spec.md`
- **Dashboard Layout spec**: `specs/005-dashboard-layouts` (see `specs/`)
- **OpenAPI Contract**: `docs/openapi.yaml`

---

## 📝 Version History

- **2026-09-19**: Synced against the actual routes in `internal/routes/url_mappings.go` / `internal/api/router.go` — added Dashboard Metrics (query/catalog/batch), Public tenant lookup, `PATCH /users/me`, edge device API keys; corrected the Consumers folder (events is a real endpoint, not a 501 stub).
- **2026-03-11**: Master collection created — consolidated all 4 separate collections
- **2026-03-02**: User Management API collection + docs
- **2026-03-11**: Edge Device Management collection + README
- **2026-02-28**: User Role Assignments collection
- **2026-02-27**: Tenants collection

---

**Status**: Production-Ready
**Collection Version**: 3.0 (Synced 2026-09-19)
**Last Updated**: 2026-09-19
