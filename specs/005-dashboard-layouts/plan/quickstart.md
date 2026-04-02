# Quickstart: Dashboard Layouts API

**Feature**: 005-dashboard-layouts
**Date**: 2026-04-02

## Prerequisites

- Docker running
- `DATABASE_URL` env var pointing to a local Postgres instance
- A valid JWT token from Supabase (use existing dev token)
- A tenant UUID in the `tenants` table (e.g. `550e8400-e29b-41d4-a716-446655440001`)
- The authenticated user must have an active role in `user_tenant_roles` for that tenant

---

## 1. Apply the migrations

```bash
migrate -path migrations/ -database $DATABASE_URL up
```

Verify:
```bash
psql $DATABASE_URL -c "\d dashboard_layouts"
```

---

## 2. Start the API

```bash
docker compose up
```

API available at `http://localhost:8080`

---

## 3. Smoke test with curl

Replace `<TOKEN>` with your JWT and `<TENANT_UUID>` with the tenant UUID.

### List layouts (empty)
```bash
curl -s \
  -H "Authorization: Bearer <TOKEN>" \
  -H "X-Tenant-ID: <TENANT_UUID>" \
  http://localhost:8080/api/v1/dashboard-layouts | jq .
```
Expected: `{ "success": true, "data": [], "meta": { "total": 0, "limit": 3 } }`

### Create a layout
```bash
curl -s -X POST \
  -H "Authorization: Bearer <TOKEN>" \
  -H "X-Tenant-ID: <TENANT_UUID>" \
  -H "Content-Type: application/json" \
  -d '{"name":"Dashboard Principal","widgets":[{"id":"w1","type":"machine-status","name":"Estado","title":"Estado de Maquinas","description":"Estado general","category":"overview","icon":"Activity","position":{"x":0,"y":0,"w":6,"h":3,"i":"w1"}}]}' \
  http://localhost:8080/api/v1/dashboard-layouts | jq .
```
Expected: `{ "success": true, "data": { "id": "...", "name": "Dashboard Principal", ... } }`

### Get by ID
```bash
LAYOUT_ID="<id-from-create-response>"
curl -s \
  -H "Authorization: Bearer <TOKEN>" \
  -H "X-Tenant-ID: <TENANT_UUID>" \
  http://localhost:8080/api/v1/dashboard-layouts/$LAYOUT_ID | jq .
```

### Update
```bash
curl -s -X PUT \
  -H "Authorization: Bearer <TOKEN>" \
  -H "X-Tenant-ID: <TENANT_UUID>" \
  -H "Content-Type: application/json" \
  -d '{"name":"Dashboard Principal v2","widgets":[]}' \
  http://localhost:8080/api/v1/dashboard-layouts/$LAYOUT_ID | jq .
```

### Delete
```bash
curl -s -X DELETE \
  -H "Authorization: Bearer <TOKEN>" \
  -H "X-Tenant-ID: <TENANT_UUID>" \
  http://localhost:8080/api/v1/dashboard-layouts/$LAYOUT_ID | jq .
```
Expected: `{ "success": true, "message": "Layout eliminado correctamente" }`

---

## 4. Test error scenarios

### Duplicate name → 409
```bash
# Create "Dashboard Principal" twice — segunda llamada devuelve:
# { "success": false, "error": "DUPLICATE_NAME" }
```

### Limit reached → 403
```bash
# Crear 3 layouts, luego intentar un 4to — devuelve:
# { "success": false, "error": "LIMIT_REACHED" }
```

### Delete last layout → 400
```bash
# Con un solo layout activo — devuelve:
# { "success": false, "error": "No se puede eliminar el único layout" }
```

### No auth → 401
```bash
curl -s http://localhost:8080/api/v1/dashboard-layouts
# { "success": false, "error": "No autorizado" }
```

### No tenant header → 400
```bash
curl -s -H "Authorization: Bearer <TOKEN>" http://localhost:8080/api/v1/dashboard-layouts
# { "error": "missing X-Tenant-ID header" }
```

### No role in tenant → 403
```bash
# X-Tenant-ID de un tenant donde el usuario no tiene rol activo en user_tenant_roles
# { "error": "tenant access denied" }
```
