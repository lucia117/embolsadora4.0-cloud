# Guía de Prueba Rápida: API de Gestión de Roles

**Rama**: `006-roles-management`
**Requisitos previos**:
- Servidor corriendo en `localhost:8080`
- Migration 000012 aplicada
- JWT Bearer token válido (`$TOKEN`)
- UUID del tenant activo (`$TENANT_ID`)

---

## 1. Listar roles del tenant

Devuelve los 4 roles del sistema + roles custom del tenant.

```bash
curl -s http://localhost:8080/api/v1/roles \
  -H "Authorization: Bearer $TOKEN" \
  -H "X-Tenant-ID: $TENANT_ID" | jq .
```

**Respuesta esperada (200)**:
```json
{
  "success": true,
  "data": [
    {
      "id": "admin",
      "name": "Admin",
      "permissions": ["users:read", "users:write", "machines:read"],
      "isSystemRole": true,
      "isGlobal": true,
      "tenantId": null
    },
    { "id": "operario", "isSystemRole": true, "isGlobal": true },
    { "id": "cliente_admin", "isSystemRole": true, "isGlobal": true },
    { "id": "cliente_operario", "isSystemRole": true, "isGlobal": true }
  ]
}
```

---

## 2. Crear un rol personalizado

```bash
curl -s -X POST http://localhost:8080/api/v1/roles \
  -H "Authorization: Bearer $TOKEN" \
  -H "X-Tenant-ID: $TENANT_ID" \
  -H "Content-Type: application/json" \
  -d '{"name": "Supervisor", "description": "Acceso de solo lectura", "permissions": ["machines:read"]}' | jq .
```

**Respuesta esperada (201)**:
```json
{
  "success": true,
  "data": {
    "id": "custom_3a9f12",
    "name": "Supervisor",
    "description": "Acceso de solo lectura",
    "permissions": ["machines:read"],
    "isSystemRole": false,
    "isGlobal": false,
    "tenantId": "550e8400-...",
    "createdAt": "2026-04-03T10:00:00Z",
    "updatedAt": "2026-04-03T10:00:00Z"
  }
}
```

Guardar el ID: `export ROLE_ID=custom_3a9f12`

---

## 3. Obtener rol por ID

```bash
curl -s http://localhost:8080/api/v1/roles/$ROLE_ID \
  -H "Authorization: Bearer $TOKEN" \
  -H "X-Tenant-ID: $TENANT_ID" | jq .
```

**404 si el ID no existe**:
```json
{ "success": false, "error": "NOT_FOUND", "message": "El rol no existe" }
```

---

## 4. Actualizar rol

```bash
curl -s -X PUT http://localhost:8080/api/v1/roles/$ROLE_ID \
  -H "Authorization: Bearer $TOKEN" \
  -H "X-Tenant-ID: $TENANT_ID" \
  -H "Content-Type: application/json" \
  -d '{"name": "Supervisor Senior", "permissions": ["machines:read", "users:read"]}' | jq .
```

**403 si se intenta actualizar un rol del sistema**:
```bash
curl -s -X PUT http://localhost:8080/api/v1/roles/admin \
  -H "Authorization: Bearer $TOKEN" \
  -H "X-Tenant-ID: $TENANT_ID" \
  -H "Content-Type: application/json" \
  -d '{"name": "Super Admin"}' | jq .
# → {"success": false, "error": "SYSTEM_ROLE"}
```

---

## 5. Eliminar rol personalizado

```bash
curl -s -X DELETE http://localhost:8080/api/v1/roles/$ROLE_ID \
  -H "Authorization: Bearer $TOKEN" \
  -H "X-Tenant-ID: $TENANT_ID" | jq .
```

**Respuesta esperada (200)**:
```json
{ "success": true }
```

---

## 6. Escenarios de error

### Límite alcanzado (403)
```bash
# Después de crear 3 roles custom, intentar crear un 4°
curl -s -X POST http://localhost:8080/api/v1/roles \
  -H "Authorization: Bearer $TOKEN" \
  -H "X-Tenant-ID: $TENANT_ID" \
  -H "Content-Type: application/json" \
  -d '{"name": "Cuarto Rol"}' | jq .
# → {"success": false, "error": "LIMIT_REACHED"}
```

### Nombre duplicado (409)
```bash
curl -s -X POST http://localhost:8080/api/v1/roles \
  -H "Authorization: Bearer $TOKEN" \
  -H "X-Tenant-ID: $TENANT_ID" \
  -H "Content-Type: application/json" \
  -d '{"name": "Supervisor"}' | jq .
# → {"success": false, "error": "DUPLICATE_NAME"}
```

### Eliminar rol del sistema (403)
```bash
curl -s -X DELETE http://localhost:8080/api/v1/roles/admin \
  -H "Authorization: Bearer $TOKEN" \
  -H "X-Tenant-ID: $TENANT_ID" | jq .
# → {"success": false, "error": "SYSTEM_ROLE"}
```

### Eliminar rol con usuarios asignados (409)
```bash
curl -s -X DELETE http://localhost:8080/api/v1/roles/$ROLE_ID \
  -H "Authorization: Bearer $TOKEN" \
  -H "X-Tenant-ID: $TENANT_ID" | jq .
# → {"success": false, "error": "ROLE_HAS_ASSIGNMENTS", "usersAffected": 2}
```

### Sin autenticación (401)
```bash
curl -s http://localhost:8080/api/v1/roles \
  -H "X-Tenant-ID: $TENANT_ID" | jq .
# → {"success": false, "error": "No autorizado"}
```
