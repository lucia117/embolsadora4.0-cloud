# Guía para probar la API en local

## Requisitos previos

1. Docker corriendo (`docker compose up -d`)
2. Postman instalado (v10+)
3. Archivo de colección importado: `embolsadora-api.postman_collection.json`

---

## 1. Importar la colección

1. Abrir Postman
2. **File → Import** (o arrastrar el archivo a la ventana)
3. Seleccionar `specs/postman/embolsadora-api.postman_collection.json`
4. Confirmar — aparecerán las carpetas en el panel izquierdo

---

## 2. Configurar las variables de colección

Hacer click en el nombre de la colección → pestaña **Variables**:

| Variable | Valor a completar |
|---|---|
| `baseUrl` | `http://localhost:8080` (ya viene configurado) |
| `bearerToken` | Se llena **automáticamente** al ejecutar Login |
| `tenantId` | UUID del tenant de tu base de datos (ver paso 4) |

El resto (`invitationId`, `userId`, `shellId`, `deviceId`) se llenan automáticamente cuando ejecutás los requests de creación.

---

## 3. Verificar que la API está corriendo

Ejecutar **Health / Ping**

```
GET http://localhost:8080/ping
```

Respuesta esperada:
```json
{
  "postgres": { "status": "ok" },
  "redis":    { "status": "ok" },
  "mongo":    { "status": "ok" }
}
```

Si alguno aparece como `"disabled"` o `"degraded"`, revisar que los contenedores estén corriendo:
```bash
docker compose ps
docker compose logs postgres   # o redis / mongo
```

---

## 4. Obtener un token (Login)

Ejecutar **Auth / Login**

Editar el body con las credenciales de un usuario que exista en Supabase:
```json
{
  "email": "tu-usuario@example.com",
  "password": "tu-password"
}
```

Si el login es exitoso, el test script guarda el token automáticamente en `{{bearerToken}}`.
No hace falta copiar nada a mano.

> **Nota**: Supabase debe estar accesible. Si usás Supabase local, asegurate de que
> `SUPABASE_URL` en tu `.env` apunte al contenedor correcto.

---

## 5. Obtener un tenantId válido

Opción A — Crear uno nuevo:
1. Ejecutar **Tenants / Create Tenant** (llena `{{tenantId}}` automáticamente)

Opción B — Usar uno existente:
```bash
# Conectarse a Postgres
docker exec -it <contenedor-postgres> psql -U embolsadora_user -d embolsadora_dev
SELECT id, name FROM tenants LIMIT 10;
```
Copiar el UUID en la variable `tenantId` de la colección.

---

## 6. Flujo de prueba recomendado

### Happy path completo (en orden)

```
1. Health / Ping
2. Auth / Login                        ← guarda bearerToken
3. Tenants / Create Tenant             ← guarda tenantId
4. Me / Get Me
5. Invitations / Create Invitation     ← guarda invitationId
6. Invitations / List Invitations
7. Invitations / Resend Invitation
8. Invitations / Revoke Invitation
9. AAS Shells / Create Shell           ← guarda shellId (requiere MongoDB)
10. AAS Shells / List Shells
11. AAS Shells / Get Shell
12. AAS Shells / Update Shell
13. AAS Shells / Delete Shell
14. Edge Devices / Create Device       ← guarda deviceId
15. Edge Devices / List Devices
16. Edge Devices / Get Device
17. Edge Devices / Enable Device
18. Edge Devices / Disable Device
19. Edge Devices / Status Check
20. Edge Devices / Health Check
21. Edge Devices / Get Telemetry
22. Edge Devices / List Events
```

### Casos de error (Auth — Casos de Error)

Estos no requieren token ni setup:

| Request | Resultado esperado |
|---|---|
| Request sin token | 401 Unauthorized |
| Request con token inválido | 401 Unauthorized |
| Request sin X-Tenant-ID | 400 Bad Request |

### Casos de error específicos

| Request | Resultado esperado |
|---|---|
| AAS Shells / Create Shell — Conflict | 409 (ejecutar Create dos veces con el mismo `globalAssetId`) |
| AAS Shells / Get Shell — Not Found | 404 |
| AAS Shells / List Shells — Sin X-Tenant-ID | 400 |
| Edge Devices / Create Device — edgeType inválido | 400 |

---

## 7. Ejecutar toda la colección de una vez

1. Click derecho en la colección → **Run collection**
2. En Collection Runner: mantener el orden predeterminado
3. Click **Run**

Postman ejecuta los requests en secuencia y los test scripts encadenan los IDs automáticamente.

> **Tip**: Si algún request falla por falta de datos (ej. `shellId` vacío), ejecutar
> primero el request de creación correspondiente de forma individual.

---

## 8. Colección Bruno — AAS Shells (MongoDB) y endpoints completos

El archivo `Embolsadora API — Local - brunoo.json` es una colección en formato Postman v2.1
compatible con **Bruno** (importar desde *File → Import Collection → Postman Format*).

Cubre todos los endpoints incluyendo AAS Shells (MongoDB) y Consumers (API Key).

### 8.1 Importar y configurar el entorno

1. Abrir Bruno → **Import Collection** → seleccionar `Embolsadora API — Local - brunoo.json`
2. Importar el entorno → **Environments → Import** → seleccionar `Embolsadora API — Local.environment.json`
3. Activar el entorno **Local** en el selector superior

Completar estas variables en el entorno antes de correr el runner:

| Variable | Valor | Cómo obtenerlo |
|---|---|---|
| `baseUrl` | `http://localhost:8080` | ya viene configurado |
| `bearerToken` | *(se llena automáticamente al hacer Login)* | ejecutar Auth / Login |
| `tenantId` | UUID del tenant | usar `550e8400-e29b-41d4-a716-446655440001` (demo seed) o ejecutar Get Me |
| `tenantSlug` | slug del tenant | usar `demo` (demo seed) o leer de la respuesta del tenant |
| `roleId` | ID del rol | usar `admin` (roles: `admin`, `operario`, `cliente_admin`, `cliente_operario`) |
| `apiKey` | `dev-api-key` | ya viene configurado |

### 8.2 Prerequisito: seed de datos en MongoDB

El test *Create Shell — Conflict* requiere que exista un shell previo con `globalAssetId: "urn:example:machine:001"`.
Si la base de datos está limpia, insertarlo manualmente:

```bash
docker exec embolsadora_mongo mongosh embolsadora_dev --eval '
db.asset_administration_shells.insertOne({
  _id: "seed-conflict-001",
  tenantId: UUID("550e8400-e29b-41d4-a716-446655440001"),
  globalAssetId: "urn:example:machine:001",
  assetKind: "Instance",
  assetType: "BolagsacoEmbolsadora",
  submodelRefs: [],
  createdAt: new Date(),
  updatedAt: new Date()
})'
```

Para verificar el estado actual de la colección:
```bash
docker exec embolsadora_mongo mongosh embolsadora_dev --quiet --eval \
  'db.asset_administration_shells.find({}, {_id:1, globalAssetId:1}).toArray()'
```

### 8.3 Endpoints AAS Shells (MongoDB)

Todos los endpoints requieren `Authorization: Bearer {{bearerToken}}` y `X-Tenant-ID: {{tenantId}}`.

| Método | Path | Descripción | Status esperado |
|---|---|---|---|
| `POST` | `/api/v1/aas/shells` | Crear shell | 201 — guarda `shellId` automáticamente |
| `GET` | `/api/v1/aas/shells` | Listar shells (paginado) | 200 — body: `{ data, total, limit, offset }` |
| `GET` | `/api/v1/aas/shells/:id` | Obtener shell por ID | 200 |
| `PUT` | `/api/v1/aas/shells/:id` | Actualizar shell | 200 |
| `DELETE` | `/api/v1/aas/shells/:id` | Eliminar shell | 204 |

**Body de creación:**
```json
{
  "globalAssetId": "urn:example:machine:001",
  "assetKind": "Instance",
  "assetType": "BolagsacoEmbolsadora",
  "description": "Máquina de prueba",
  "administration": { "version": "1", "revision": "0" },
  "submodelRefs": []
}
```

**Respuesta de creación (201):**
```json
{
  "ID": "uuid-generado",
  "TenantID": "550e8400-...",
  "GlobalAssetID": "urn:example:machine:001",
  "AssetKind": "Instance",
  "AssetType": "BolagsacoEmbolsadora",
  "SubmodelRefs": [],
  "CreatedAt": "2026-04-11T...",
  "UpdatedAt": "2026-04-11T..."
}
```

> **Nota**: Los campos de la respuesta usan PascalCase porque el struct Go no tiene json tags.

### 8.4 Endpoint Consumer — Get Shell (API Key)

Permite que un dispositivo IoT lea su propio shell usando una API key en lugar de JWT.

```
GET /api/v1/consumers/shells/:id
Headers:
  X-API-Key: dev-api-key
  X-Tenant-ID: {{tenantId}}
```

### 8.5 Orden del runner en la colección Bruno

El runner ejecuta las carpetas en el orden del archivo. Orden correcto para el happy path:

```
1.  Auth / Login                             ← guarda bearerToken
2.  AAS Shells / Create Shell — Conflict     ← espera 409 (requiere seed)
3.  AAS Shells / Create Shell                ← guarda shellId
4.  AAS Shells / Get Shell
5.  AAS Shells / Update Shell
6.  AAS Shells / Delete Shell
7.  AAS Shells / Get Shell — Not Found       ← espera 404
8.  AAS Shells / List Shells — Sin tenant    ← espera 400
9.  AAS Shells / List Shells
10. Consumers / Get Shell (consumer)         ← usa shellId (debe existir)
11. Me / Get Me                              ← guarda tenantId
12. Invitations / Create Invitation          ← requiere roleId en env
...
```

> **Tip**: Si el runner falla en un request de creación, los siguientes que dependen
> del ID van a fallar en cascada. Ejecutar el request fallido de forma individual
> primero para diagnosticar.

---

## 9. Headers comunes

| Header | Cuándo va |
|---|---|
| `Authorization: Bearer {{bearerToken}}` | Todos los endpoints protegidos |
| `X-Tenant-ID: {{tenantId}}` | Endpoints bajo `/api/v1/` (excepto `/me` y `/auth/change-password`) |
| `Content-Type: application/json` | Requests con body |

Los endpoints de Edge Devices (`/api/tenants/:tenantId/...`) toman el tenant del path, **no** del header.

---

## 10. Troubleshooting rápido

| Síntoma | Causa probable | Solución |
|---|---|---|
| 401 en todos los requests | Token vencido | Ejecutar Login de nuevo |
| 400 "missing X-Tenant-ID" | Variable `tenantId` vacía | Completar la variable o ejecutar Create Tenant |
| 409 en Create Shell | `globalAssetId` ya existe | Cambiar el valor en el body |
| 503 en Login | Supabase no accesible | Verificar `SUPABASE_URL` y conectividad |
| 500 en AAS Shells | MongoDB no disponible | Verificar `docker compose ps` y `MONGO_URI` |
| `mongo: disabled` en /ping | `MONGO_URI` no configurado | Agregar `MONGO_URI` al `.env` y reiniciar |
