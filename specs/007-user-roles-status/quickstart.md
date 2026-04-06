# Quickstart: Extensión de Gestión de Usuarios (007)

**Objetivo**: Validar las 3 nuevas funcionalidades contra el servidor local.  
**Prerequisitos**: Servidor corriendo en `http://localhost:8080`, usuario admin autenticado, tenant con datos de prueba.

---

## Setup: Variables

```bash
BASE_URL="http://localhost:8080/api/v1"
JWT="<token del POST /api/v1/auth/login>"
TENANT_ID="<UUID del tenant>"
USER_ID="<UUID de un usuario del tenant con rol asignado>"
ADMIN_ID="<UUID del admin autenticado (tu propio usuario)>"
```

---

## Historia 1: GET /users/:id?include=roles

### Escenario 1a — Usuario con rol activo

```bash
curl -s -X GET "$BASE_URL/users/$USER_ID?include=roles" \
  -H "Authorization: Bearer $JWT" \
  -H "X-Tenant-ID: $TENANT_ID" | jq .
```

**Respuesta esperada** (200):
```json
{
  "id": "<USER_ID>",
  "firstName": "Juan",
  "lastName": "Pérez",
  "email": "juan@ejemplo.com",
  "role": "admin",
  "tenantId": "<TENANT_ID>",
  "image": null,
  "createdAt": "...",
  "updatedAt": "...",
  "deletedAt": null,
  "roles": [
    {
      "id": "admin",
      "name": "Administrador",
      "permissions": ["users:read", "users:write", "tenants:read"]
    }
  ]
}
```

### Escenario 1b — Sin parámetro include (backward-compatible)

```bash
curl -s -X GET "$BASE_URL/users/$USER_ID" \
  -H "Authorization: Bearer $JWT" \
  -H "X-Tenant-ID: $TENANT_ID" | jq .
```

**Respuesta esperada** (200): Response idéntico al anterior **sin el campo `roles`**.

### Escenario 1c — Usuario de otro tenant (aislamiento)

```bash
OTHER_USER_ID="<UUID de usuario de otro tenant>"
curl -s -X GET "$BASE_URL/users/$OTHER_USER_ID?include=roles" \
  -H "Authorization: Bearer $JWT" \
  -H "X-Tenant-ID: $TENANT_ID" | jq .
```

**Respuesta esperada** (404):
```json
{"success": false, "error": "NOT_FOUND", "message": "usuario no encontrado"}
```

### Escenario 1d — Usuario sin rol asignado

```bash
NO_ROLE_USER_ID="<UUID de usuario sin UTR activo>"
curl -s -X GET "$BASE_URL/users/$NO_ROLE_USER_ID?include=roles" \
  -H "Authorization: Bearer $JWT" \
  -H "X-Tenant-ID: $TENANT_ID" | jq .
```

**Respuesta esperada** (200): Response del usuario con `"roles": []`.

---

## Historia 2: PATCH /users/:id/status

### Escenario 2a — Desactivar usuario activo

```bash
curl -s -X PATCH "$BASE_URL/users/$USER_ID/status" \
  -H "Authorization: Bearer $JWT" \
  -H "X-Tenant-ID: $TENANT_ID" \
  -H "Content-Type: application/json" \
  -d '{"status": "inactive"}' | jq .
```

**Respuesta esperada** (200): Response del usuario con `updatedAt` actualizado.

### Escenario 2b — Reactivar usuario inactivo

```bash
curl -s -X PATCH "$BASE_URL/users/$USER_ID/status" \
  -H "Authorization: Bearer $JWT" \
  -H "X-Tenant-ID: $TENANT_ID" \
  -H "Content-Type: application/json" \
  -d '{"status": "active"}' | jq .
```

**Respuesta esperada** (200): Response del usuario con status actualizado.

### Escenario 2c — Estado inválido

```bash
curl -s -X PATCH "$BASE_URL/users/$USER_ID/status" \
  -H "Authorization: Bearer $JWT" \
  -H "X-Tenant-ID: $TENANT_ID" \
  -H "Content-Type: application/json" \
  -d '{"status": "deleted"}' | jq .
```

**Respuesta esperada** (400):
```json
{
  "success": false,
  "error": "INVALID_STATUS",
  "message": "el estado debe ser active, inactive o suspended"
}
```

### Escenario 2d — Admin intentando desactivarse a sí mismo (RF-006)

```bash
curl -s -X PATCH "$BASE_URL/users/$ADMIN_ID/status" \
  -H "Authorization: Bearer $JWT" \
  -H "X-Tenant-ID: $TENANT_ID" \
  -H "Content-Type: application/json" \
  -d '{"status": "inactive"}' | jq .
```

**Respuesta esperada** (400):
```json
{
  "success": false,
  "error": "CANNOT_DEACTIVATE_SELF",
  "message": "un administrador no puede desactivarse a sí mismo"
}
```

### Escenario 2e — Sin permiso de administrador (403)

```bash
OPERATOR_JWT="<token de un usuario sin rol admin>"
curl -s -X PATCH "$BASE_URL/users/$USER_ID/status" \
  -H "Authorization: Bearer $OPERATOR_JWT" \
  -H "X-Tenant-ID: $TENANT_ID" \
  -H "Content-Type: application/json" \
  -d '{"status": "inactive"}' | jq .
```

**Respuesta esperada** (403).

---

## Historia 3: GET /users/pending

### Escenario 3a — Tenant con usuarios pendientes

```bash
curl -s -X GET "$BASE_URL/users/pending" \
  -H "Authorization: Bearer $JWT" \
  -H "X-Tenant-ID: $TENANT_ID" | jq .
```

**Respuesta esperada** (200):
```json
{
  "data": [
    {
      "id": "...",
      "firstName": "Carlos",
      "lastName": "López",
      "email": "carlos@ejemplo.com",
      "role": "user",
      "tenantId": "<TENANT_ID>",
      "image": null,
      "createdAt": "...",
      "updatedAt": "...",
      "deletedAt": null
    }
  ],
  "total": 1
}
```

### Escenario 3b — Tenant sin usuarios pendientes

```bash
curl -s -X GET "$BASE_URL/users/pending" \
  -H "Authorization: Bearer $JWT" \
  -H "X-Tenant-ID: $TENANT_ID" | jq .
```

**Respuesta esperada** (200):
```json
{"data": [], "total": 0}
```

### Escenario 3c — Sin permiso de administrador

```bash
curl -s -X GET "$BASE_URL/users/pending" \
  -H "Authorization: Bearer $OPERATOR_JWT" \
  -H "X-Tenant-ID: $TENANT_ID" | jq .
```

**Respuesta esperada** (403).

---

## Checklist de validación Pact

| Interacción Pact | Escenario | Estado |
|---|---|---|
| GET /users/{id}?include=roles | 1a | ⬜ |
| GET /users/{id} sin include | 1b | ⬜ |
| PATCH /users/{id}/status → 200 | 2a / 2b | ⬜ |
| GET /users/pending → 200 | 3a / 3b | ⬜ |

Marcar ✅ cuando el curl retorne el response esperado.
