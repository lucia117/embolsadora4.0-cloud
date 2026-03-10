# User Management API - Testing Guide

Guía práctica para testear la API de User Management usando la colección Postman. Incluye escenarios happy path y edge cases.

## 📋 Prerequisitos

- Postman Desktop instalado
- Colección `User-Management-API.postman_collection.json` importada
- Environment `User-Management-API.postman_environment.json` importado
- Servidor API ejecutándose en `http://localhost:8080`
- Base de datos PostgreSQL con migraciones aplicadas

## 🔑 Obtener Credenciales de Testing

### 1. Obtener JWT Token (si tu API tiene endpoint de login)

```bash
# Si tienes endpoint de login:
curl -X POST http://localhost:8080/api/v1/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "admin@example.com",
    "password": "secure-password-here"
  }'
```

Copia el `token` del response y reemplaza `{{jwt_token}}` en Postman.

### 2. Crear JWT válido manualmente (para testing)

Ve a https://jwt.io y crea un token con payload:

```json
{
  "sub": "admin-user-id-123",
  "name": "Admin User",
  "email": "admin@example.com",
  "role": "admin",
  "iat": 1677000000,
  "exp": 2000000000
}
```

Usa cualquier secret para signing (el servidor debería validarlo).

### 3. Obtener Tenant ID

El tenant ID corresponde a la organización/cliente. Puedes:
- Usar uno existente de tu base de datos
- Crear uno nuevo si tienes endpoint de creación de tenants
- Usar UUIDs de ejemplo proporcionados

## 🧪 Scenario 1: Happy Path (Flujo Normal)

### Paso 1: Configurar Postman

1. Abre Postman
2. Selecciona **Environment**: "User Management API - Development"
3. Verifica que `base_url`, `tenant_id` y `jwt_token` están configurados
4. Abre la **Colección**: "User Management API"

### Paso 2: Listar Usuarios Existentes

```
Request: GET /users
```

1. Haz clic en request "List Users (with pagination)"
2. Haz clic en **Send**
3. Verifica response **200 OK**

**Response esperado**:
```json
{
  "data": [],
  "pagination": {
    "total": 0,
    "count": 0,
    "limit": 20,
    "offset": 0
  }
}
```

Si no hay usuarios, continuamos al siguiente paso.

### Paso 3: Crear Primer Usuario

```
Request: POST /users
```

1. Haz clic en request "Create User"
2. Verifica el **Body**:
```json
{
  "firstName": "Juan",
  "lastName": "Pérez",
  "email": "juan.perez@example.com",
  "role": "admin",
  "image": "https://example.com/avatar/juan.jpg"
}
```

3. Haz clic en **Send**
4. Verifica response **201 Created**
5. **Copia el `id` de la respuesta** para los próximos tests

**Response**:
```json
{
  "id": "123e4567-e89b-12d3-a456-426614174000",
  "firstName": "Juan",
  "lastName": "Pérez",
  "email": "juan.perez@example.com",
  "role": "admin",
  "tenantId": "550e8400-e29b-41d4-a716-446655440000",
  "image": "https://example.com/avatar/juan.jpg",
  "createdAt": "2026-03-02T12:00:00Z",
  "updatedAt": "2026-03-02T12:00:00Z",
  "deletedAt": null
}
```

### Paso 4: Obtener Usuario por ID

```
Request: GET /users/:id
```

1. Haz clic en request "Get User by ID"
2. En **Params**, verifica que `user_id` está configurado con el ID del usuario creado
3. Haz clic en **Send**
4. Verifica response **200 OK** con los datos correctos

### Paso 5: Actualizar Usuario

```
Request: PATCH /users/:id
```

1. Haz clic en request "Update User"
2. Edita el **Body**:
```json
{
  "firstName": "Juan Carlos",
  "role": "user"
}
```

3. Haz clic en **Send**
4. Verifica response **200 OK**
5. Verifica que `firstName` cambió a "Juan Carlos" y `role` es "user"

**Response esperado**:
```json
{
  "id": "123e4567-e89b-12d3-a456-426614174000",
  "firstName": "Juan Carlos",
  "lastName": "Pérez",
  "email": "juan.perez@example.com",
  "role": "user",
  "tenantId": "550e8400-e29b-41d4-a716-446655440000",
  "image": "https://example.com/avatar/juan.jpg",
  "createdAt": "2026-03-02T12:00:00Z",
  "updatedAt": "2026-03-02T12:05:00Z",
  "deletedAt": null
}
```

**Nota**: `updatedAt` cambió pero `createdAt` se mantiene igual.

### Paso 6: Listar Usuarios (verificar cambios)

```
Request: GET /users
```

1. Haz clic en request "List Users (with pagination)"
2. Haz clic en **Send**
3. Verifica que aparece el usuario actualizado en la lista

**Response esperado**:
```json
{
  "data": [
    {
      "id": "123e4567-e89b-12d3-a456-426614174000",
      "firstName": "Juan Carlos",
      "lastName": "Pérez",
      "email": "juan.perez@example.com",
      "role": "user",
      "tenantId": "550e8400-e29b-41d4-a716-446655440000",
      "image": "https://example.com/avatar/juan.jpg",
      "createdAt": "2026-03-02T12:00:00Z",
      "updatedAt": "2026-03-02T12:05:00Z",
      "deletedAt": null
    }
  ],
  "pagination": {
    "total": 1,
    "count": 1,
    "limit": 20,
    "offset": 0
  }
}
```

### Paso 7: Eliminar Usuario

```
Request: DELETE /users/:id
```

1. Haz clic en request "Delete User"
2. Verifica que `user_id` está configurado correctamente
3. Haz clic en **Send**
4. Verifica response **204 No Content**

### Paso 8: Verificar Soft Delete

```
Request: GET /users/:id
```

1. Haz clic en request "Get User by ID"
2. Haz clic en **Send**
3. Verifica que response es **404 Not Found** (soft deleted)

```json
{
  "error": "USER_NOT_FOUND",
  "message": "User not found",
  "status": 404
}
```

### Paso 9: Listar Usuarios (verificar soft delete)

```
Request: GET /users
```

1. Haz clic en request "List Users (with pagination)"
2. Haz clic en **Send**
3. Verifica que el usuario NO aparece (soft delete filtra `deleted_at IS NULL`)

**Response esperado**:
```json
{
  "data": [],
  "pagination": {
    "total": 0,
    "count": 0,
    "limit": 20,
    "offset": 0
  }
}
```

---

## 🛑 Scenario 2: Error Cases

### Test 2.1: Missing X-Tenant-ID Header

```
Request: GET /users (sin X-Tenant-ID)
```

1. En la colección, selecciona "List Users (with pagination)"
2. En **Headers**, deshabilita temporalmente la línea `X-Tenant-ID`
3. Haz clic en **Send**
4. Verifica response **400 Bad Request**

**Response esperado**:
```json
{
  "error": "MISSING_HEADER",
  "message": "X-Tenant-ID header is required",
  "status": 400
}
```

### Test 2.2: Duplicate Email

```
Request: POST /users (email duplicado)
```

1. Crea un usuario: `juan@example.com`
2. Intenta crear otro usuario con el mismo email
3. Verifica response **409 Conflict**

**Response esperado**:
```json
{
  "error": "DUPLICATE_EMAIL",
  "message": "Email already exists in this tenant",
  "status": 409
}
```

### Test 2.3: Invalid Email Format

```
Request: POST /users (email inválido)
```

1. Haz clic en request "Create User"
2. Edita el **Body**:
```json
{
  "firstName": "Test",
  "lastName": "User",
  "email": "not-an-email",
  "role": "admin"
}
```

3. Haz clic en **Send**
4. Verifica response **400 Bad Request**

**Response esperado**:
```json
{
  "error": "VALIDATION_ERROR",
  "message": "Key: 'CreateUserRequest.Email' Error:Field validation for 'Email' failed on the 'email' tag",
  "status": 400
}
```

### Test 2.4: Missing Required Field (firstName)

```
Request: POST /users (sin firstName)
```

1. Haz clic en request "Create User"
2. Edita el **Body**:
```json
{
  "lastName": "User",
  "email": "test@example.com",
  "role": "admin"
}
```

3. Haz clic en **Send**
4. Verifica response **400 Bad Request**

### Test 2.5: Non-Existent User

```
Request: GET /users/:id (ID no existe)
```

1. Haz clic en request "Get User by ID"
2. Edita `user_id` a: `00000000-0000-0000-0000-000000000000`
3. Haz clic en **Send**
4. Verifica response **404 Not Found**

```json
{
  "error": "USER_NOT_FOUND",
  "message": "User not found",
  "status": 404
}
```

### Test 2.6: Immutable Field Update (email)

```
Request: PATCH /users/:id (intentar cambiar email)
```

1. Haz clic en request "Update User"
2. Edita el **Body**:
```json
{
  "email": "new-email@example.com"
}
```

3. Haz clic en **Send**
4. Verifica response **400 Bad Request**

```json
{
  "error": "IMMUTABLE_FIELD",
  "message": "Email and tenantId cannot be modified",
  "status": 400
}
```

### Test 2.7: Immutable Field Update (tenantId)

```
Request: PATCH /users/:id (intentar cambiar tenantId)
```

1. Haz clic en request "Update User"
2. Edita el **Body**:
```json
{
  "tenantId": "99999999-9999-9999-9999-999999999999"
}
```

3. Haz clic en **Send**
4. Verifica response **400 Bad Request**

---

## 🔐 Scenario 3: RBAC (Role-Based Access Control)

### Test 3.1: Non-Admin User (Read Operations)

```
Request: GET /users (usuario non-admin)
```

**Nota**: Los endpoints de lectura (GET) NO requieren admin.

1. Cambia `jwt_token` a un token con `"role": "user"` (no admin)
2. Haz clic en request "List Users"
3. Haz clic en **Send**
4. Verifica response **200 OK** (lectura permitida)

### Test 3.2: Non-Admin User (Create Operation)

```
Request: POST /users (usuario non-admin)
```

1. Cambia `jwt_token` a un token con `"role": "user"` (no admin)
2. Haz clic en request "Create User"
3. Haz clic en **Send**
4. Verifica response **403 Forbidden**

```json
{
  "error": "INSUFFICIENT_PERMISSIONS",
  "message": "Only admin users can perform this action",
  "status": 403
}
```

### Test 3.3: Non-Admin User (Update Operation)

```
Request: PATCH /users/:id (usuario non-admin)
```

1. Cambia `jwt_token` a un token con `"role": "user"`
2. Haz clic en request "Update User"
3. Haz clic en **Send**
4. Verifica response **403 Forbidden**

### Test 3.4: Non-Admin User (Delete Operation)

```
Request: DELETE /users/:id (usuario non-admin)
```

1. Cambia `jwt_token` a un token con `"role": "user"`
2. Haz clic en request "Delete User"
3. Haz clic en **Send**
4. Verifica response **403 Forbidden**

---

## 📊 Scenario 4: Pagination

### Test 4.1: Limit Parameter

```
Request: GET /users?limit=1&offset=0
```

1. Crea 3 usuarios diferentes
2. Haz clic en "List Users (with pagination)"
3. En **Params**, cambia `limit` a `1`
4. Haz clic en **Send**
5. Verifica que solo devuelve 1 usuario pero `total: 3`

**Response esperado**:
```json
{
  "data": [
    { "id": "..." }
  ],
  "pagination": {
    "total": 3,
    "count": 1,
    "limit": 1,
    "offset": 0
  }
}
```

### Test 4.2: Offset Parameter

```
Request: GET /users?limit=1&offset=1
```

1. En **Params**, cambia `offset` a `1`
2. Haz clic en **Send**
3. Verifica que devuelve el 2do usuario

### Test 4.3: Invalid Limit (menor que 1)

```
Request: GET /users?limit=0&offset=0
```

1. En **Params**, cambia `limit` a `0`
2. Haz clic en **Send**
3. Verifica que retorna **400 Bad Request** con `VALIDATION_ERROR` y mensaje `"limit must be an integer between 1 and 100"`

### Test 4.4: Invalid Offset (negativo)

```
Request: GET /users?limit=20&offset=-1
```

1. En **Params**, cambia `offset` a `-1`
2. Haz clic en **Send**
3. Verifica que retorna **400 Bad Request** con `VALIDATION_ERROR` y mensaje `"offset must be a non-negative integer"`

---

## 🔀 Scenario 5: Multi-Tenant Isolation

### Test 5.1: Usuario A no ve usuarios de Tenant B

```
Request: GET /users (diferentes tenant_ids)
```

**Setup**:
- Crea usuario con `tenant_id: AAAA-AAAA-AAAA-AAAA`
- Cambia `tenant_id` a `BBBB-BBBB-BBBB-BBBB`
- Request: `GET /users`

1. En **Environment Variables**, cambia `tenant_id` a otro UUID
2. Haz clic en request "List Users"
3. Haz clic en **Send**
4. Verifica que NO aparece el usuario creado antes (pertenece a otro tenant)

### Test 5.2: Tenant ID Forzado en Response

```
Request: GET /users/:id
```

1. Crea usuario en `tenant_id: AAAA-AAAA-AAAA-AAAA`
2. Cambia a `tenant_id: BBBB-BBBB-BBBB-BBBB`
3. Intenta recuperar usuario creado
4. Verifica que response es **404 Not Found**

---

## 📈 Scenario 6: Load Testing (Opcional)

### Test 6.1: Crear 100 Usuarios

```bash
#!/bin/bash
for i in {1..100}; do
  curl -X POST http://localhost:8080/api/v1/users \
    -H "X-Tenant-ID: 550e8400-e29b-41d4-a716-446655440000" \
    -H "Authorization: Bearer $JWT_TOKEN" \
    -H "Content-Type: application/json" \
    -d "{
      \"firstName\": \"User\",
      \"lastName\": \"Test$i\",
      \"email\": \"user.test$i@example.com\",
      \"role\": \"user\"
    }"
  echo "Created user $i"
done
```

### Test 6.2: Listar 100 Usuarios

```
Request: GET /users?limit=100&offset=0
```

1. Haz clic en "List Users"
2. En **Params**, cambia `limit` a `100`
3. Haz clic en **Send**
4. Verifica que response contiene todos los 100 usuarios
5. Revisa el tiempo de respuesta en **Tests** tab

---

## ✅ Checklist de Testing

Marca cada test como completado:

- [ ] Happy Path (Paso 1-9)
  - [ ] 1.1: Listar usuarios (vacío)
  - [ ] 1.2: Crear usuario
  - [ ] 1.3: Obtener usuario
  - [ ] 1.4: Actualizar usuario
  - [ ] 1.5: Listar usuarios (con datos)
  - [ ] 1.6: Eliminar usuario (soft delete)
  - [ ] 1.7: Obtener usuario eliminado (404)
  - [ ] 1.8: Listar usuarios (sin eliminados)

- [ ] Error Cases (Escenario 2)
  - [ ] 2.1: Missing X-Tenant-ID
  - [ ] 2.2: Duplicate Email
  - [ ] 2.3: Invalid Email Format
  - [ ] 2.4: Missing Required Field
  - [ ] 2.5: Non-Existent User
  - [ ] 2.6: Immutable Field (email)
  - [ ] 2.7: Immutable Field (tenantId)

- [ ] RBAC (Escenario 3)
  - [ ] 3.1: Non-Admin Read (permitido)
  - [ ] 3.2: Non-Admin Create (403)
  - [ ] 3.3: Non-Admin Update (403)
  - [ ] 3.4: Non-Admin Delete (403)

- [ ] Pagination (Escenario 4)
  - [ ] 4.1: Limit Parameter
  - [ ] 4.2: Offset Parameter
  - [ ] 4.3: Invalid Limit (default)
  - [ ] 4.4: Invalid Offset (default)

- [ ] Multi-Tenant (Escenario 5)
  - [ ] 5.1: Tenant A no ve Tenant B
  - [ ] 5.2: Tenant ID forzado en respuesta

---

## 🐛 Debugging

Si un test falla, revisa:

1. **Response Status Code**
   - 400: Validación de request (body, headers, query params)
   - 401: Autenticación inválida o JWT expirado
   - 403: Autorización insuficiente (revisa `role` en JWT)
   - 404: Recurso no encontrado (verifica ID)
   - 409: Conflicto (email duplicado, etc)
   - 500: Error servidor (revisa logs del backend)

2. **Response Body**
   - `error`: Código de error específico
   - `message`: Descripción del error
   - `status`: HTTP status code

3. **Headers**
   - Verifica `X-Tenant-ID` esté presente
   - Verifica `Authorization: Bearer <token>` sea válido
   - Verifica `Content-Type: application/json` para POST/PATCH

4. **JWT Token**
   - Verifica expiración (`exp` claim)
   - Verifica `role` sea correcto (`admin` para POST/PATCH/DELETE)
   - Puedes decodificar en https://jwt.io

5. **Database**
   - Verifica que migraciones están aplicadas: `SELECT * FROM users;`
   - Verifica que tenant existe: `SELECT * FROM tenants;`

---

## 📞 Common Issues

### "X-Tenant-ID header is required"
- Colección no está configurada correctamente
- Header está deshabilitado en Postman
- Variable `{{tenant_id}}` no está definida

### "User role not found in token"
- JWT no tiene claim `role`
- Token expirado o inválido
- Falta header `Authorization: Bearer`

### "Email already exists in this tenant"
- Email ya existe para este tenant
- Usa un email diferente en tests
- Elimina datos de tests previos de la BD

### "Only admin users can perform this action"
- JWT tiene `"role": "user"` pero necesita `"role": "admin"`
- Para POST/PATCH/DELETE, necesitas admin
- GET operations no requieren admin

### "User not found"
- ID no existe o pertenece a otro tenant
- Usuario fue soft-deleted (deleted_at ≠ null)
- Verifica tenant_id coincida

---

**Última actualización**: 2026-03-02
**Testing Status**: Ready for Integration Testing
