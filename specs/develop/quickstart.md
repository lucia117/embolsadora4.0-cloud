# Quickstart: POST /users con Rol Inicial

**Feature**: `develop`  
**Fecha**: 2026-04-11

---

## Prerequisitos

```bash
# Servidor corriendo en localhost:8080
# Variables de entorno configuradas
export BASE_URL="http://localhost:8080/api/v1"
export TENANT_ID="<uuid-del-tenant>"
export JWT_TOKEN="<token-jwt-de-admin>"
```

---

## Escenario 1: Crear usuario con rol del sistema (happy path)

```bash
curl -X POST "$BASE_URL/users" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $JWT_TOKEN" \
  -H "X-Tenant-ID: $TENANT_ID" \
  -d '{
    "firstName": "Juan",
    "lastName": "Pérez",
    "email": "juan.perez@ejemplo.com",
    "role": "operario"
  }'
```

**Respuesta esperada (201)**:
```json
{
  "id": "<uuid-generado>",
  "firstName": "Juan",
  "lastName": "Pérez",
  "email": "juan.perez@ejemplo.com",
  "role": "operario",
  "tenantId": "<TENANT_ID>",
  "image": null,
  "createdAt": "2026-04-11T...",
  "updatedAt": "2026-04-11T...",
  "deletedAt": null
}
```

**Verificar UTR creado**:
```sql
SELECT id, user_id, tenant_id, role_id, status, assigned_by, assigned_at
FROM user_tenant_roles
WHERE user_id = '<uuid-generado>' AND tenant_id = '<TENANT_ID>';
-- Debe retornar 1 fila con status = 'active'
```

---

## Escenario 2: Crear usuario con rol custom (UUID)

```bash
export ROLE_ID="<uuid-de-rol-custom-del-tenant>"

curl -X POST "$BASE_URL/users" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $JWT_TOKEN" \
  -H "X-Tenant-ID: $TENANT_ID" \
  -d "{
    \"firstName\": \"María\",
    \"lastName\": \"García\",
    \"email\": \"maria.garcia@ejemplo.com\",
    \"role\": \"$ROLE_ID\"
  }"
```

**Respuesta esperada (201)**: igual que escenario 1, con `role` = UUID del rol custom.

---

## Escenario 3: Rol inexistente → 400

```bash
curl -X POST "$BASE_URL/users" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $JWT_TOKEN" \
  -H "X-Tenant-ID: $TENANT_ID" \
  -d '{
    "firstName": "Test",
    "lastName": "User",
    "email": "test@ejemplo.com",
    "role": "rol_que_no_existe"
  }'
```

**Respuesta esperada (400)**:
```json
{
  "error": "INVALID_ROLE",
  "message": "el rol especificado no existe",
  "status": 400
}
```

**Verificar atomicidad**: el usuario NO debe haber sido creado en la tabla `users`.

---

## Escenario 4: Email duplicado en el tenant → 409

```bash
# Repetir el mismo request del Escenario 1 con el mismo email

curl -X POST "$BASE_URL/users" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $JWT_TOKEN" \
  -H "X-Tenant-ID: $TENANT_ID" \
  -d '{
    "firstName": "Otro",
    "lastName": "User",
    "email": "juan.perez@ejemplo.com",
    "role": "operario"
  }'
```

**Respuesta esperada (409)**:
```json
{
  "error": "EMAIL_TAKEN",
  "message": "el email ya está registrado en este tenant",
  "status": 409
}
```

---

## Escenario 5: Sin autenticación → 401

```bash
curl -X POST "$BASE_URL/users" \
  -H "Content-Type: application/json" \
  -H "X-Tenant-ID: $TENANT_ID" \
  -d '{
    "firstName": "Test",
    "lastName": "User",
    "email": "noauth@ejemplo.com",
    "role": "operario"
  }'
```

**Respuesta esperada (401)**:
```json
{
  "error": "UNAUTHORIZED",
  "message": "token de autenticación requerido",
  "status": 401
}
```

---

## Verificación de regresiones

```bash
# GET /users — debe seguir funcionando igual
curl "$BASE_URL/users" \
  -H "Authorization: Bearer $JWT_TOKEN" \
  -H "X-Tenant-ID: $TENANT_ID"

# GET /users/:id?include=roles — debe mostrar el rol recién asignado
curl "$BASE_URL/users/<uuid-del-usuario-creado>?include=roles" \
  -H "Authorization: Bearer $JWT_TOKEN" \
  -H "X-Tenant-ID: $TENANT_ID"
# El campo "roles" debe contener el rol asignado en la creación
```
