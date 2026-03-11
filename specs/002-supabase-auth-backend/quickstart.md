# Quickstart: Probar el backend con Postman

Guía completa para levantar el entorno local y probar los endpoints desde cero.

---

## Paso 1: Crear un proyecto en Supabase

1. Ir a [supabase.com](https://supabase.com) y crear una cuenta (gratis).
2. Click en **New project**.
3. Completar:
   - **Name**: `embolsadora-dev`
   - **Database password**: elegí una contraseña segura (guardala aparte)
   - **Region**: la más cercana (ej: South America)
4. Esperar ~2 minutos a que el proyecto esté listo.

### Obtener las credenciales

Ir a **Settings → API** y copiar:

| Variable | Dónde encontrarla en el dashboard |
|----------|-----------------------------------|
| `SUPABASE_URL` | "Project URL" (ej: `https://abcxyz.supabase.co`) |
| `SUPABASE_SERVICE_ROLE_KEY` | Sección "Project API keys" → "service_role" → click en "Reveal" |
| `anon key` | Sección "Project API keys" → "anon public" (se usa solo en el Paso 6 para obtener tokens) |

El resto se deriva de la URL:
- `SUPABASE_JWKS_URL` = `{SUPABASE_URL}/auth/v1/.well-known/jwks.json`
- `SUPABASE_JWT_ISSUER` = `{SUPABASE_URL}/auth/v1`

### Crear un usuario de prueba

Ir a **Authentication → Users → Add user → Create new user**:
- Email: `admin@test.com`
- Password: `Test1234!`
- Tildar **"Auto confirm email"**

---

## Paso 2: Crear el archivo `.env`

En la raíz del proyecto:

```bash
cp .env.example .env
```

Editar `.env` reemplazando los valores de Supabase con los del paso anterior:

```env
# Base
PORT=8080
APP_ENV=development

# Base de datos (Docker local)
DATABASE_URL=postgres://embolsadora_user:embolsadora_password@localhost:5432/embolsadora_dev?sslmode=disable

# Redis (Docker local)
REDIS_URL=redis://:embolsadora_redis_pass@localhost:6379/0

# Supabase — reemplazar con tus valores reales
SUPABASE_URL=https://<tu-project-id>.supabase.co
SUPABASE_JWKS_URL=https://<tu-project-id>.supabase.co/auth/v1/.well-known/jwks.json
SUPABASE_JWT_ISSUER=https://<tu-project-id>.supabase.co/auth/v1
SUPABASE_JWT_AUDIENCE=authenticated
SUPABASE_SERVICE_ROLE_KEY=<tu-service-role-key>

# URL del frontend (para links en emails de invitación)
APP_BASE_URL=http://localhost:3000

# Rate limiting de invitaciones
INVITATION_RATE_LIMIT_PER_HOUR=20
```

---

## Paso 3: Levantar Postgres y Redis

```bash
docker compose up db redis -d
```

Verificar que ambos estén healthy:

```bash
docker compose ps
```

Ambos deben mostrar `(healthy)` en la columna Status.

---

## Paso 4: Aplicar las migraciones

```bash
docker run --rm \
  --network host \
  -v $(pwd)/migrations:/migrations \
  migrate/migrate \
  -path=/migrations \
  -database "postgres://embolsadora_user:embolsadora_password@localhost:5432/embolsadora_dev?sslmode=disable" \
  up
```

Deberías ver algo como:
```
1/u 000001_initial_schema (Xms)
2/u 000002_...
...
5/u 000005_user_invitations (Xms)
```

---

## Paso 5: Correr el servidor

```bash
docker compose build api
docker compose up api
```

Deberías ver en los logs:
```
Database connection established
Starting server on :8080
```

> Si da error por variables de entorno faltantes, verificar el archivo `.env` del Paso 2.

---

## Paso 6: Obtener un JWT de Supabase (en Postman)

Crear un request en Postman:

- **Method**: `POST`
- **URL**: `https://<tu-project-id>.supabase.co/auth/v1/token?grant_type=password`
- **Headers**:
  ```
  Content-Type: application/json
  apikey: <tu-anon-key>
  ```
- **Body** (raw JSON):
  ```json
  {
    "email": "admin@test.com",
    "password": "Test1234!"
  }
  ```

De la respuesta, copiar el valor de `access_token`.

> **Tip**: En Postman, crear un Environment con una variable `jwt` y guardar ahí el token para reutilizarlo en todos los requests.

---

## Paso 7: Probar los endpoints

En todos los requests agregar el header:
```
Authorization: Bearer {{jwt}}
```

### GET /api/v1/me — Perfil propio

No requiere `X-Tenant-ID`.

```
GET http://localhost:8080/api/v1/me
Authorization: Bearer {{jwt}}
```

Respuesta esperada (primera vez — usuario sin tenant asignado):
```json
{
  "user": {
    "id": "uuid-del-usuario",
    "email": "admin@test.com",
    "name": null,
    "password_change_required": false
  },
  "tenant": null,
  "role": null,
  "permissions": []
}
```

> El primer `GET /me` auto-provisionó el usuario en la tabla `users` de tu Postgres local.

### Sin token → 401

```
GET http://localhost:8080/api/v1/me
```
```json
{ "error": "missing token" }
```

### Con token inválido → 401

```
GET http://localhost:8080/api/v1/me
Authorization: Bearer token_inventado
```
```json
{ "error": "invalid token" }
```

---

## Paso 8: Asignar tenant y rol (para probar más endpoints)

Conectarse a Postgres para insertar datos de prueba:

```bash
docker exec -it embolsadora_db psql -U embolsadora_user -d embolsadora_dev
```

```sql
-- Insertar un tenant de prueba
INSERT INTO tenants (id, name, subdomain)
VALUES ('11111111-1111-1111-1111-111111111111', 'Tenant Demo', 'demo');

-- Ver el ID del usuario auto-provisionado en el paso anterior
SELECT id, email, supabase_user_id FROM users;

-- Asignar rol admin al usuario (reemplazar <user-id> con el id del SELECT)
INSERT INTO user_tenant_roles (user_id, tenant_id, role_id, status)
VALUES (
  '<user-id>',
  '11111111-1111-1111-1111-111111111111',
  'admin',
  'active'
);
```

Ahora repetir `GET /api/v1/me` con el tenant en el header:

```
GET http://localhost:8080/api/v1/me
Authorization: Bearer {{jwt}}
X-Tenant-ID: 11111111-1111-1111-1111-111111111111
```

Respuesta esperada:
```json
{
  "user": { "id": "...", "email": "admin@test.com", "password_change_required": false },
  "tenant": { "id": "11111111-...", "name": "Tenant Demo", "subdomain": "demo" },
  "role": { "id": "admin", "name": "admin" },
  "permissions": [
    "users:read", "users:write", "invitations:write",
    "machines:read", "machines:write", "tenants:read"
  ]
}
```

---

## Paso 9: Probar el resto de endpoints

### GET /api/v1/invitations — Listar invitaciones

```
GET http://localhost:8080/api/v1/invitations
Authorization: Bearer {{jwt}}
X-Tenant-ID: 11111111-1111-1111-1111-111111111111
```

### POST /api/v1/invitations — Crear invitación

Requiere permiso `invitations:write` (rol admin lo tiene).

```
POST http://localhost:8080/api/v1/invitations
Authorization: Bearer {{jwt}}
X-Tenant-ID: 11111111-1111-1111-1111-111111111111
Content-Type: application/json

{
  "email": "nuevo@test.com",
  "role_id": "operario"
}
```

Respuesta esperada: `201` con los datos de la invitación. Supabase envía un email al usuario invitado.

Segundo POST con el mismo email → `409` (invitación pendiente ya existe).

### DELETE /api/v1/invitations/:id — Revocar invitación

```
DELETE http://localhost:8080/api/v1/invitations/<invitation-id>
Authorization: Bearer {{jwt}}
X-Tenant-ID: 11111111-1111-1111-1111-111111111111
```

### POST /api/v1/users/:id/force-password-change

Requiere permiso `users:write`.

```
POST http://localhost:8080/api/v1/users/<user-id>/force-password-change
Authorization: Bearer {{jwt}}
X-Tenant-ID: 11111111-1111-1111-1111-111111111111
```

---

## Referencia rápida de endpoints

| Método | Path | `X-Tenant-ID` | Permiso requerido |
|--------|------|:-------------:|:-----------------:|
| GET | `/api/v1/me` | No | — |
| POST | `/api/v1/auth/change-password` | No | — |
| GET | `/api/v1/invitations` | Sí | — |
| POST | `/api/v1/invitations` | Sí | `invitations:write` |
| POST | `/api/v1/invitations/:id/resend` | Sí | — |
| DELETE | `/api/v1/invitations/:id` | Sí | `invitations:write` |
| POST | `/api/v1/users/:id/force-password-change` | Sí | `users:write` |

---

## Roles disponibles y sus permisos

| Rol | Permisos |
|-----|----------|
| `admin` | users:read, users:write, invitations:write, machines:read, machines:write, tenants:read |
| `operario` | machines:read, machines:write |
| `cliente_admin` | users:read, invitations:write, machines:read |
| `cliente_operario` | machines:read |

---

## Paso 10: Benchmark de performance (T051 — SC-003)

**Herramienta**: Apache Bench (`ab`)  
**Fecha**: 2026-03-09  
**Entorno**: Docker local (API + Postgres en contenedores en la misma máquina)

### Resultados

#### 10 VUs concurrentes — 200 requests

```
ab -n 200 -c 10 -H "Authorization: Bearer <jwt>" -H "X-Tenant-ID: <tenant-id>" http://127.0.0.1:8080/api/v1/me
```

| Métrica | Valor |
|---------|-------|
| Req/s | 41.9 |
| P50 | 222ms |
| P95 | 374ms |
| P99 | 459ms |
| Min | 61ms |
| Max | 472ms |
| Failures | 0/200 |

#### 50 VUs concurrentes — 500 requests

| Métrica | Valor |
|---------|-------|
| Req/s | 27.6 |
| P50 | 1583ms |
| P95 | 2536ms |
| Min | 842ms |
| Failures | 0/500 |

### Análisis

- **SC-003 (P95 < 300ms)**: No cumplido en entorno local Docker.
- **Causa principal con 50 VUs**: `DB_MAX_CONNS=10` — con 50 concurrentes los requests hacen cola esperando conexiones disponibles al pool de Postgres.
- **Causa del overhead base (~60ms min)**: latencia de red Docker bridge entre el proceso `ab` (host) y el contenedor API, más el salto al contenedor de Postgres.
- **En producción (Koyeb + Neon)**: el overhead de Docker desaparece. Se recomienda subir `DB_MAX_CONNS` a 25-50 y re-medir en Koyeb antes de dar SC-003 por cumplido.

### Recomendaciones para producción

1. Ajustar `DB_MAX_CONNS=25` en las env vars de Koyeb (Neon free tier soporta hasta 100 conexiones).
2. Re-ejecutar el benchmark con k6 desde una máquina externa al servidor de producción.
3. El target P95 < 300ms es alcanzable en producción dado que el P50 local sin contención es 222ms.
