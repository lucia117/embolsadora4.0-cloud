# Quickstart: Dashboard Layouts API

**Feature**: 005-dashboard-layouts
**Date**: 2026-03-24

## Prerequisites

- Docker running
- `DATABASE_URL` env var pointing to a local Postgres instance
- A valid JWT token from Supabase (use existing dev token)
- A tenant with subdomain `acme` seeded in the `tenants` table

---

## 1. Apply the migration

```bash
migrate -path migrations/ -database $DATABASE_URL up 1
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

Replace `<TOKEN>` with your JWT.

### List layouts (empty)
```bash
curl -s -H "Authorization: Bearer <TOKEN>" \
  http://localhost:8080/api/tenants/acme/dashboard-layouts | jq .
```
Expected: `{ "success": true, "data": [], "meta": { "total": 0, "limit": 3 } }`

### Create a layout
```bash
curl -s -X POST \
  -H "Authorization: Bearer <TOKEN>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Dashboard Principal",
    "widgets": [
      {
        "id": "w1",
        "type": "machine-status",
        "name": "Estado",
        "title": "Estado de Máquinas",
        "description": "Estado general",
        "category": "overview",
        "icon": "Activity",
        "position": { "x": 0, "y": 0, "w": 6, "h": 3, "i": "w1" }
      }
    ]
  }' \
  http://localhost:8080/api/tenants/acme/dashboard-layouts | jq .
```
Expected: `{ "success": true, "data": { "id": "...", "name": "Dashboard Principal", ... } }`

### Get by ID
```bash
LAYOUT_ID="<id-from-create-response>"
curl -s -H "Authorization: Bearer <TOKEN>" \
  http://localhost:8080/api/tenants/acme/dashboard-layouts/$LAYOUT_ID | jq .
```

### Update
```bash
curl -s -X PUT \
  -H "Authorization: Bearer <TOKEN>" \
  -H "Content-Type: application/json" \
  -d '{ "name": "Dashboard Principal v2", "widgets": [] }' \
  http://localhost:8080/api/tenants/acme/dashboard-layouts/$LAYOUT_ID | jq .
```

### Delete
```bash
curl -s -X DELETE \
  -H "Authorization: Bearer <TOKEN>" \
  http://localhost:8080/api/tenants/acme/dashboard-layouts/$LAYOUT_ID | jq .
```
Expected: `{ "success": true, "message": "Layout eliminado correctamente" }`

---

## 4. Test error scenarios

### Duplicate name → 409
```bash
# Create "Dashboard Principal" twice
curl -s -X POST ... -d '{ "name": "Dashboard Principal", "widgets": [] }' ...
# Second call should return:
# { "success": false, "error": "DUPLICATE_NAME" }
```

### Limit reached → 403
```bash
# Create 3 layouts, then attempt a 4th
# Response: { "success": false, "error": "LIMIT_REACHED" }
```

### Delete last layout → 400
```bash
# With only one layout remaining:
# Response: { "success": false, "error": "No se puede eliminar el único layout" }
```

### No auth → 401
```bash
curl -s http://localhost:8080/api/tenants/acme/dashboard-layouts
# Response: { "success": false, "error": "No autorizado" }
```
