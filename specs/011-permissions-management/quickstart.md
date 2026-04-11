# Quickstart: Permissions Management API

**Feature**: 011-permissions-management  
**Date**: 2026-04-10  
**Prerequisito**: servidor corriendo en `http://localhost:8080` con JWT válido y `X-Tenant-ID` del tenant de prueba

---

## Variables de entorno para los curl

```bash
JWT="<token JWT válido>"
TENANT_ID="<UUID del tenant>"
BASE_URL="http://localhost:8080/api/v1"
```

---

## 1. Listar permisos (GET /permissions)

Verifica que los 17 permisos de sistema están disponibles y el auth es requerido.

```bash
# Happy path — debe retornar >= 17 permisos de sistema
curl -s -X GET "$BASE_URL/permissions" \
  -H "Authorization: Bearer $JWT" \
  -H "X-Tenant-ID: $TENANT_ID" | jq 'length'
# Esperado: >= 17

# Verificar que perm_dashboard existe con isSystemPermission=true
curl -s -X GET "$BASE_URL/permissions" \
  -H "Authorization: Bearer $JWT" \
  -H "X-Tenant-ID: $TENANT_ID" | jq '.[] | select(.id == "perm_dashboard")'
# Esperado: { "id": "perm_dashboard", "name": "View Dashboard", ..., "isSystemPermission": true }

# Sin auth — debe retornar 401
curl -s -o /dev/null -w "%{http_code}" -X GET "$BASE_URL/permissions"
# Esperado: 401
```

---

## 2. Obtener permiso por ID (GET /permissions/:id)

```bash
# Permiso de sistema existente
curl -s -X GET "$BASE_URL/permissions/perm_dashboard" \
  -H "Authorization: Bearer $JWT" \
  -H "X-Tenant-ID: $TENANT_ID" | jq .
# Esperado: 200 con datos del permiso

# Permiso inexistente
curl -s -o /dev/null -w "%{http_code}" \
  -X GET "$BASE_URL/permissions/nonexistent-perm" \
  -H "Authorization: Bearer $JWT" \
  -H "X-Tenant-ID: $TENANT_ID"
# Esperado: 404
```

---

## 3. Crear permiso custom (POST /permissions)

```bash
# Crear permiso custom válido
PERM_ID=$(curl -s -X POST "$BASE_URL/permissions" \
  -H "Authorization: Bearer $JWT" \
  -H "X-Tenant-ID: $TENANT_ID" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Export Production Data",
    "section": "analytics",
    "description": "Allows exporting raw production data for external analysis"
  }' | jq -r '.id')

echo "Permiso creado con ID: $PERM_ID"
# Esperado: UUID como "550e8400-e29b-41d4-a716-446655440000"

# Verificar que isSystemPermission=false
curl -s -X GET "$BASE_URL/permissions/$PERM_ID" \
  -H "Authorization: Bearer $JWT" \
  -H "X-Tenant-ID: $TENANT_ID" | jq '.isSystemPermission'
# Esperado: false

# Validación: nombre demasiado corto
curl -s -X POST "$BASE_URL/permissions" \
  -H "Authorization: Bearer $JWT" \
  -H "X-Tenant-ID: $TENANT_ID" \
  -H "Content-Type: application/json" \
  -d '{"name": "AB", "section": "analytics", "description": "Too short"}' | jq .
# Esperado: 400 con errors[].path = "name"

# Validación: sección ausente
curl -s -X POST "$BASE_URL/permissions" \
  -H "Authorization: Bearer $JWT" \
  -H "X-Tenant-ID: $TENANT_ID" \
  -H "Content-Type: application/json" \
  -d '{"name": "Valid Name", "description": "No section"}' | jq .
# Esperado: 400 con errors[].path = "section"
```

---

## 4. Actualizar permiso custom (PUT /permissions/:id)

```bash
# Actualizar permiso custom (usar $PERM_ID del paso anterior)
curl -s -X PUT "$BASE_URL/permissions/$PERM_ID" \
  -H "Authorization: Bearer $JWT" \
  -H "X-Tenant-ID: $TENANT_ID" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Export Production Data (Updated)",
    "section": "analytics",
    "description": "Allows exporting raw production and quality data"
  }' | jq .
# Esperado: 200 con datos actualizados y updatedAt renovado

# Intentar modificar permiso de sistema — debe retornar 403
curl -s -X PUT "$BASE_URL/permissions/perm_dashboard" \
  -H "Authorization: Bearer $JWT" \
  -H "X-Tenant-ID: $TENANT_ID" \
  -H "Content-Type: application/json" \
  -d '{"name": "Modified Dashboard", "section": "dashboard", "description": "Attempt"}' | jq .
# Esperado: 403 { "error": "Cannot modify system permissions" }

# ID inexistente
curl -s -o /dev/null -w "%{http_code}" \
  -X PUT "$BASE_URL/permissions/nonexistent-id" \
  -H "Authorization: Bearer $JWT" \
  -H "X-Tenant-ID: $TENANT_ID" \
  -H "Content-Type: application/json" \
  -d '{"name": "Test", "section": "analytics", "description": "Test"}'
# Esperado: 404
```

---

## 5. Eliminar permiso custom (DELETE /permissions/:id)

```bash
# Eliminar permiso custom (usar $PERM_ID)
curl -s -X DELETE "$BASE_URL/permissions/$PERM_ID" \
  -H "Authorization: Bearer $JWT" \
  -H "X-Tenant-ID: $TENANT_ID" | jq .
# Esperado: 200 { "success": true }

# Verificar que ya no existe
curl -s -o /dev/null -w "%{http_code}" \
  -X GET "$BASE_URL/permissions/$PERM_ID" \
  -H "Authorization: Bearer $JWT" \
  -H "X-Tenant-ID: $TENANT_ID"
# Esperado: 404

# Intentar eliminar permiso de sistema — debe retornar 403
curl -s -X DELETE "$BASE_URL/permissions/perm_dashboard" \
  -H "Authorization: Bearer $JWT" \
  -H "X-Tenant-ID: $TENANT_ID" | jq .
# Esperado: 403 { "error": "Cannot delete system permissions" }
```

---

## 6. Verificar aislamiento multi-tenant

```bash
# Crear permiso custom con TENANT_A
TENANT_A="<uuid-tenant-a>"
TENANT_B="<uuid-tenant-b>"

PERM_A=$(curl -s -X POST "$BASE_URL/permissions" \
  -H "Authorization: Bearer $JWT" \
  -H "X-Tenant-ID: $TENANT_A" \
  -H "Content-Type: application/json" \
  -d '{"name": "Tenant A Permission", "section": "analytics", "description": "Only for A"}' \
  | jq -r '.id')

# Verificar que TENANT_B no ve el permiso de TENANT_A
curl -s -X GET "$BASE_URL/permissions/$PERM_A" \
  -H "Authorization: Bearer $JWT" \
  -H "X-Tenant-ID: $TENANT_B" | jq .
# Esperado: 404

# Listar permisos de TENANT_B — no debe incluir permisos custom de TENANT_A
curl -s -X GET "$BASE_URL/permissions" \
  -H "Authorization: Bearer $JWT" \
  -H "X-Tenant-ID: $TENANT_B" | jq '[.[] | select(.isSystemPermission == false)] | length'
# Esperado: 0 (si TENANT_B no tiene custom permisos propios)
```

---

## Checklist de verificación Pact (10 interacciones)

| # | Interacción | Método | Path | Status esperado |
|---|-------------|--------|------|----------------|
| 1 | List all permissions | GET | `/permissions` | 200 con array ≥ 17 items |
| 2 | Create custom permission | POST | `/permissions` | 201 con id, isSystemPermission=false |
| 3 | Create permission with invalid name | POST | `/permissions` | 400 con errors[0].path=name |
| 4 | Get specific permission by id | GET | `/permissions/perm_dashboard` | 200 con datos |
| 5 | Get non-existent permission | GET | `/permissions/nonexistent-perm` | 404 |
| 6 | Update custom permission | PUT | `/permissions/:id` | 200 con datos actualizados |
| 7 | Update system permission fails | PUT | `/permissions/perm_dashboard` | 403 |
| 8 | Delete custom permission | DELETE | `/permissions/:id` | 200 `{ success: true }` |
| 9 | Delete system permission fails | DELETE | `/permissions/perm_dashboard` | 403 |
| 10 | Request without auth fails | GET | `/permissions` | 401 |
