# User Management API - Postman Collection

Documentación completa de la API de User Management en Postman. Incluye todos los endpoints CRUD con ejemplos de requests/responses y manejo de errores.

## 📋 Contenido

La colección incluye 5 operaciones principales:

1. **List Users** (GET) - Listado paginado de usuarios del tenant
2. **Get User** (GET) - Obtener perfil completo de un usuario específico
3. **Create User** (POST) - Crear un nuevo usuario (requiere rol admin)
4. **Update User** (PATCH) - Actualizar parcialmente un usuario (requiere rol admin)
5. **Delete User** (DELETE) - Soft-delete de un usuario (requiere rol admin)

## 🚀 Configuración Rápida

### Opción 1: Importar en Postman Desktop (Recomendado)

1. Abre **Postman Desktop**
2. Ve a **File** → **Import**
3. Selecciona la pestaña **Upload Files**
4. Carga: `User-Management-API.postman_collection.json`
5. Haz clic en **Import**

### Opción 2: Importar desde URL

```bash
# Si la colección está hospedada en un repositorio Git
curl -X GET https://raw.githubusercontent.com/tu-org/embolsadora-api/main/postman/User-Management-API.postman_collection.json | pbpaste
```

Luego **Cmd/Ctrl + V** → **Import** en Postman

## 🔐 Configuración de Variables

La colección contiene 4 variables de entorno que DEBES configurar:

| Variable | Valor Ejemplo | Descripción |
|----------|---------------|-------------|
| `base_url` | `http://localhost:8080/api/v1` | URL base del API |
| `tenant_id` | `550e8400-e29b-41d4-a716-446655440000` | UUID del tenant (reemplaza con tu UUID) |
| `jwt_token` | `eyJhbGciOiJIUzI1NiIs...` | Token JWT con rol admin |
| `user_id` | `323e4567-e89b-12d3-a456-426614174002` | UUID del usuario a manipular |

### Cómo Establecer Variables

#### En Postman Desktop:

1. Abre la colección importada
2. Haz clic en la pestaña **Variables**
3. Edita cada variable:
   - `base_url`: URL del servidor (ej: `http://localhost:8080/api/v1`)
   - `tenant_id`: UUID de tu tenant de prueba
   - `jwt_token`: Token JWT válido con rol admin
   - `user_id`: UUID del usuario a probar

#### Crear un JWT válido para testing:

```bash
# Opción 1: Usando jwt.io (https://jwt.io)
# Crea un token con claims:
# {
#   "sub": "admin-user-id",
#   "name": "Admin User",
#   "role": "admin",
#   "iat": 1677000000
# }

# Opción 2: Si tu API tiene endpoint de login
# POST http://localhost:8080/api/v1/login
# Body: {"email": "admin@example.com", "password": "..."}
# Copia el token del response
```

## 📡 Headers Automáticos

Todos los requests incluyen automáticamente estos headers (configurados en la colección):

```
X-Tenant-ID: {{tenant_id}}        # Requerido - Aislamiento multi-tenant
Authorization: Bearer {{jwt_token}}  # Requerido - Autenticación JWT
Content-Type: application/json     # Para POST/PATCH
Accept: application/json           # Para todas las operaciones
```

## 🧪 Ejemplos de Uso

### 1. Listar Usuarios del Tenant

```
GET {{base_url}}/users?limit=20&offset=0
```

**Response (200 OK)**:
```json
{
  "data": [
    {
      "id": "123e4567-e89b-12d3-a456-426614174000",
      "firstName": "Juan",
      "lastName": "Pérez",
      "email": "juan.perez@example.com",
      "role": "admin",
      "tenantId": "550e8400-e29b-41d4-a716-446655440000",
      "image": "https://example.com/avatar/juan.jpg",
      "createdAt": "2026-01-15T10:30:00Z",
      "updatedAt": "2026-02-20T14:45:00Z",
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

### 2. Obtener Usuario por ID

```
GET {{base_url}}/users/{{user_id}}
```

**Response (200 OK)**:
```json
{
  "id": "323e4567-e89b-12d3-a456-426614174002",
  "firstName": "Carlos",
  "lastName": "López",
  "email": "carlos.lopez@example.com",
  "role": "admin",
  "tenantId": "550e8400-e29b-41d4-a716-446655440000",
  "image": "https://example.com/avatar/carlos.jpg",
  "createdAt": "2026-03-02T12:00:00Z",
  "updatedAt": "2026-03-02T12:00:00Z",
  "deletedAt": null
}
```

### 3. Crear Usuario (Solo Admin)

```
POST {{base_url}}/users
Content-Type: application/json
```

**Body**:
```json
{
  "firstName": "Carlos",
  "lastName": "López",
  "email": "carlos.lopez@example.com",
  "role": "admin",
  "image": "https://example.com/avatar/carlos.jpg"
}
```

**Response (201 Created)**:
```json
{
  "id": "323e4567-e89b-12d3-a456-426614174002",
  "firstName": "Carlos",
  "lastName": "López",
  "email": "carlos.lopez@example.com",
  "role": "admin",
  "tenantId": "550e8400-e29b-41d4-a716-446655440000",
  "image": "https://example.com/avatar/carlos.jpg",
  "createdAt": "2026-03-02T12:00:00Z",
  "updatedAt": "2026-03-02T12:00:00Z",
  "deletedAt": null
}
```

### 4. Actualizar Usuario (Solo Admin)

```
PATCH {{base_url}}/users/{{user_id}}
Content-Type: application/json
```

**Body** (actualización parcial):
```json
{
  "firstName": "Carlos Eduardo",
  "role": "user",
  "image": "https://example.com/avatar/carlos-updated.jpg"
}
```

**Response (200 OK)**:
```json
{
  "id": "323e4567-e89b-12d3-a456-426614174002",
  "firstName": "Carlos Eduardo",
  "lastName": "López",
  "email": "carlos.lopez@example.com",
  "role": "user",
  "tenantId": "550e8400-e29b-41d4-a716-446655440000",
  "image": "https://example.com/avatar/carlos-updated.jpg",
  "createdAt": "2026-03-02T12:00:00Z",
  "updatedAt": "2026-03-02T13:15:00Z",
  "deletedAt": null
}
```

### 5. Eliminar Usuario (Solo Admin)

```
DELETE {{base_url}}/users/{{user_id}}
```

**Response (204 No Content)**:
```
(sin body)
```

## ⚠️ Códigos de Error

| Status | Error | Causa | Solución |
|--------|-------|-------|----------|
| 400 | `MISSING_HEADER` | Falta `X-Tenant-ID` | Agrega header `X-Tenant-ID: <uuid>` |
| 400 | `VALIDATION_ERROR` | JSON inválido o campos requeridos faltantes | Verifica el body del request |
| 400 | `IMMUTABLE_FIELD` | Intentaste modificar email o tenantId | Solo se pueden actualizar: firstName, lastName, role, image |
| 403 | `INSUFFICIENT_PERMISSIONS` | Usuario no tiene rol admin | POST, PATCH, DELETE requieren rol admin |
| 404 | `USER_NOT_FOUND` | El usuario no existe en el tenant | Verifica el user_id |
| 409 | `DUPLICATE_EMAIL` | Email ya existe en el tenant | Usa un email único |
| 500 | `INTERNAL_ERROR` | Error del servidor | Revisa los logs del backend |

## 🔍 Detalles de Validación

### CreateUserRequest
```json
{
  "firstName": "string (requerido, max 100 chars)",
  "lastName": "string (requerido, max 100 chars)",
  "email": "string (requerido, formato email, único por tenant)",
  "role": "enum: 'admin' | 'user'",
  "image": "string (opcional, URL válida)"
}
```

### UpdateUserRequest
```json
{
  "firstName": "string (opcional, max 100 chars)",
  "lastName": "string (opcional, max 100 chars)",
  "role": "enum: 'admin' | 'user' (opcional)",
  "image": "string (opcional, URL válida)",
  // NO PERMITIDOS:
  // "email": "❌ Campo inmutable",
  // "tenantId": "❌ Campo inmutable"
}
```

## 📊 Parámetros de Query

### ListUsers - Paginación

```
GET /users?limit=20&offset=0
```

| Parámetro | Tipo | Default | Range | Descripción |
|-----------|------|---------|-------|-------------|
| `limit` | int | 20 | 1-100 | Resultados por página |
| `offset` | int | 0 | ≥0 | Número de resultados a saltar |

## 🔐 Multi-Tenant Isolation

**IMPORTANTE**: La plataforma aísla completamente los datos por tenant.

- Cada request DEBE incluir `X-Tenant-ID`
- Los usuarios de un tenant NO pueden listar/acceder usuarios de otro tenant
- Las emails son únicas POR TENANT (no globalmente)
- Los tenant_id en los usuarios siempre coinciden con el header del request

## 🧩 Flujo de Testing Completo

```
1. Configurar variables (base_url, tenant_id, jwt_token)
2. Listar usuarios → GET /users (verificar aislamiento)
3. Crear usuario → POST /users (admin only)
4. Obtener usuario → GET /users/:id
5. Actualizar usuario → PATCH /users/:id (cambiar role)
6. Eliminar usuario → DELETE /users/:id
7. Listar usuarios → GET /users (verificar soft-delete)
8. Obtener usuario eliminado → GET /users/:id (404)
```

## 📝 Notas de Implementación

- **Soft Delete**: DELETE no elimina el registro, solo marca `deletedAt`
- **Timestamps**: `createdAt` y `updatedAt` se generan automáticamente en el servidor
- **UUIDs**: Todos los IDs son UUID v4 generados por el servidor
- **Timestamps**: Formato ISO 8601 UTC (ej: `2026-03-02T12:00:00Z`)
- **Role-Based Access**: POST/PATCH/DELETE requieren JWT con `"role": "admin"`

## 🆘 Troubleshooting

### "X-Tenant-ID header is required"
- Verifica que el header esté presente en la colección
- Asegúrate que la variable `{{tenant_id}}` está configurada

### "User role not found in token"
- El JWT debe incluir claim `"role"` con valor `"admin"` o `"user"`
- Usa el JWT de ejemplo o crea uno en https://jwt.io

### "Email already exists in this tenant"
- Usa un email diferente para crear usuarios
- Recuerda: emails son únicos POR TENANT, no globalmente

### "Only admin users can perform this action"
- POST, PATCH, DELETE requieren rol admin
- Verifica que tu JWT tiene `"role": "admin"`

### "User not found"
- Verifica que el `{{user_id}}` corresponde a un usuario existente
- Recuerda que usuarios soft-deleted (deleted_at ≠ null) no se encuentran

## 📞 Soporte

Para preguntas sobre la API:
- Revisa el archivo `contracts/user-service-api.openapi.yaml` en el repo
- Consulta la especificación en `specs/002-user-management/spec.md`
- Revisa el plan de implementación en `specs/002-user-management/plan.md`

---

**Última actualización**: 2026-03-02
**Versión API**: v1
**Status**: Producción-Ready (MVP)
