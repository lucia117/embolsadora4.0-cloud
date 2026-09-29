# Completar cobertura de OpenAPI (docs/openapi.yaml) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Documentar en `docs/openapi.yaml` los 38 endpoints que existen en el código pero no están en el spec (login, users/pending, permissions, roles, alarm-rules, dashboard-layouts, dashboard-metrics, edge-devices), sin tocar código de negocio.

**Architecture:** Trabajo puramente de documentación sobre un único archivo (`docs/openapi.yaml`). Cada tarea agrega un grupo de endpoints (paths + schemas de `components/schemas`) siguiendo las convenciones ya establecidas en el archivo. La validación de cada tarea es `redocly lint docs/openapi.yaml` (vía `make lint`), no tests de Go — no se modifica código, así que `go build`/`go test` no aplican a este plan.

**Tech Stack:** OpenAPI 3.1.0 (YAML), `@redocly/cli` (ya instalado globalmente, `redocly lint`) como linter — mismo que usa `make lint`.

**Spec:** No hay spec formal separada; el spec vivo es este mismo plan más el relevamiento de endpoints hecho en la conversación que lo generó (comparación de `internal/routes/url_mappings.go`, `internal/api/router.go` y cada `internal/api/handler/*/routes.go` contra `docs/openapi.yaml`).

## Global Constraints

- No se toca código Go. Solo `docs/openapi.yaml`.
- Todas las rutas nuevas van bajo `security: [{ bearerAuth: [] }]` salvo `POST /api/v1/auth/login` (público, `security: []`, igual que `GET /api/v1/public/tenants/{idOrSubdomain}` ya documentado).
- **No** se declara un parámetro `X-Tenant-ID` explícito por operación — el archivo actual nunca lo hace (el tenant se resuelve del header pero eso se documenta una sola vez en la `description` de `bearerAuth`, ver `docs/openapi.yaml:1008-1009`). Mantener esa convención para que las secciones nuevas no desentonen con las existentes.
- Los grupos de Edge Devices son la única excepción: el tenant viene del **path** (`/api/v1/tenants/{tenantId}/edge-devices/...`), no del header — ahí `tenantId` sí es un parámetro de path explícito.
- 401 y 403 de **middleware** (JWTAuth/RBACCheck) siempre se documentan con `$ref: '#/components/responses/Unauthorized'` y `$ref: '#/components/responses/Forbidden'` — son el mismo middleware compartido en todos los grupos nuevos. Los 400/403/404/409 que arma el **handler** de cada feature (ej. "no se puede borrar un rol del sistema") llevan su propio schema de error, específico de esa feature.
- Estilo YAML: mappings de una línea (`{ name: x, in: path, ... }`) para objetos simples, como ya hace el archivo en `Logs`/`Notifications`; multilínea solo cuando hay anidamiento real (arrays de objetos, oneOf-like).
- `nullable: true` es la convención ya usada en todo el archivo (ver `User.image`, `Tenant`, etc.) aunque técnicamente no es 100% puro OpenAPI 3.1 JSON Schema — **no** se migra a `type: [x, "null"]` en este plan, eso es un cambio de estilo global fuera de alcance.
- Validación de cada tarea: `redocly lint docs/openapi.yaml`. El archivo tiene un bug preexistente (línea 1 con un espacio inicial) que hoy hace que el lint **ni siquiera parsee** — se arregla en la Tarea 1. Después de ese fix, el baseline conocido (preexistente, no atribuible a este plan) es **19 errores y 54 warnings**, todos de dos categorías: `struct` (`nullable` no es válido en modo estricto 3.1) y `operation-operationId` (falta `operationId` en operaciones ya documentadas). Ninguna tarea de este plan debe introducir una categoría de error **nueva** — solo más instancias de esas dos mismas categorías (aceptable, es el estilo ya establecido) son esperables al sumar contenido nuevo.

---

## Task 1: Branch, fix bloqueante de parseo y setup de tags

**Files:**
- Modify: `docs/openapi.yaml:1` (bug de indentación)
- Modify: `docs/openapi.yaml:15-26` (lista de tags)

**Interfaces:**
- Produces: baseline de lint (19 errores / 54 warnings) que las tareas siguientes usan como referencia para confirmar que no rompieron nada nuevo.

- [ ] **Step 1: Crear la rama**

```bash
git fetch origin
git checkout develop
git pull origin develop
git checkout -b docs/openapi-missing-endpoints
```

- [ ] **Step 2: Confirmar que el lint hoy ni siquiera parsea**

Run: `redocly lint docs/openapi.yaml`

Expected: falla con `Failed to parse API description... end of the stream or a document separator is expected... (2:1)`. Esto confirma el bug antes de tocarlo.

- [ ] **Step 3: Arreglar la línea 1**

El archivo hoy empieza así (nótese el espacio inicial antes de `openapi`):

```yaml
 openapi: 3.1.0
info:
```

Editar para que quede sin el espacio inicial:

```yaml
openapi: 3.1.0
info:
```

- [ ] **Step 4: Re-correr el lint y anotar el baseline**

Run: `redocly lint docs/openapi.yaml`

Expected: ya parsea. Termina con una línea del estilo `❌ Validation failed with 19 errors and 54 warnings.` Confirmar que son **19 errores**, todos de la regla `struct` (`Property 'nullable' is not expected here`), y que los warnings son de `operation-operationId` y `tag-description` (preexistentes). Si el conteo difiere mucho de 19/54, alguien ya tocó el archivo desde que se escribió este plan — parar y avisar antes de seguir.

- [ ] **Step 5: Agregar los tags nuevos**

En `docs/openapi.yaml`, la lista de tags actual (líneas 15-26) es:

```yaml
tags:
  - name: Auth
  - name: Me
  - name: Invitations
  - name: Users
  - name: UserRoles
  - name: Machines
  - name: Tenants
  - name: Consumers
  - name: Logs
  - name: Retention
```

Reemplazar por (agrega `Notifications` —usado por 5 operaciones ya existentes pero nunca declarado, bug preexistente separado— y los 6 tags que usan las tareas siguientes):

```yaml
tags:
  - name: Auth
  - name: Me
  - name: Invitations
  - name: Users
  - name: UserRoles
  - name: Machines
  - name: Tenants
  - name: Consumers
  - name: Logs
  - name: Retention
  - name: Notifications
  - name: Permissions
  - name: Roles
  - name: AlarmRules
  - name: DashboardLayouts
  - name: DashboardMetrics
  - name: EdgeDevices
```

- [ ] **Step 6: Commit**

```bash
git add docs/openapi.yaml
git commit -m "docs(openapi): fix bloqueo de parseo y declarar tags para endpoints faltantes"
```

---

## Task 2: Documentar `POST /api/v1/auth/login` y `GET /api/v1/users/pending`

**Files:**
- Modify: `docs/openapi.yaml` (paths + schemas)

**Interfaces:**
- Consumes: tag `Auth`, tag `Users`, schema `User` (ya existe en el archivo, `docs/openapi.yaml:1084`), `$ref: '#/components/responses/Unauthorized'`, `$ref: '#/components/responses/Forbidden'`.
- Produces: schemas `LoginRequest`, `SupabaseAuthResponse`, `PendingUserList`.

- [ ] **Step 1: Agregar el path de login**

`internal/routes/url_mappings.go:82` registra `POST /api/v1/auth/login` fuera del grupo `v1` (sin `X-Tenant-ID`, sin `PasswordChangeGuard`). El handler (`internal/api/handler/auth/login/`) es un proxy transparente contra `POST {SUPABASE_URL}/auth/v1/token?grant_type=password`: reenvía el body y el status de Supabase tal cual.

Insertar **antes** de `  /api/v1/me:` (línea 31):

```yaml
  /api/v1/auth/login:
    post:
      tags: [Auth]
      summary: Login contra Supabase Auth (password grant)
      description: >
        Proxy transparente a `POST {SUPABASE_URL}/auth/v1/token?grant_type=password`.
        La respuesta (status y body) de Supabase se reenvía tal cual — no hay
        transformación ni envelope propio. Si el fetch a Supabase falla (red,
        timeout), responde 502.
      security: []
      requestBody:
        required: true
        content:
          application/json:
            schema: { $ref: '#/components/schemas/LoginRequest' }
      responses:
        '200':
          description: Login exitoso — body pasado tal cual desde Supabase (access_token, refresh_token, token_type, expires_in, user, ...)
          content:
            application/json:
              schema: { $ref: '#/components/schemas/SupabaseAuthResponse' }
        '400':
          description: Body inválido (bind falló) o error 4xx pasado tal cual desde Supabase (ej. credenciales incorrectas)
          content:
            application/json:
              schema: { type: object, additionalProperties: true }
        '502':
          description: No se pudo contactar a Supabase Auth
          content:
            application/json:
              schema: { type: object, additionalProperties: true }

```

- [ ] **Step 2: Agregar el path de users/pending**

`internal/api/router.go:73` registra `GET /users/pending` (bajo `/api/v1`) con `middleware.RBACCheck("perm_users_view")`. El handler es `ListPendingUsers` — reutiliza el schema `User` ya existente.

Insertar **antes** de `  /api/v1/users:` (línea 218):

```yaml
  /api/v1/users/pending:
    get:
      tags: [Users]
      summary: Listar usuarios con invitación pendiente de activación
      description: Requiere permiso `perm_users_view`.
      security: [{ bearerAuth: [] }]
      responses:
        '200':
          description: Usuarios pendientes
          content:
            application/json:
              schema: { $ref: '#/components/schemas/PendingUserList' }
        '401': { $ref: '#/components/responses/Unauthorized' }
        '403': { $ref: '#/components/responses/Forbidden' }

```

- [ ] **Step 3: Agregar los schemas nuevos**

Insertar en `components/schemas`, justo **antes** del comentario `  ########################\n  # Standard responses` (línea 1448):

```yaml
    LoginRequest:
      type: object
      properties:
        email: { type: string, format: email }
        password: { type: string }
      required: [email, password]

    SupabaseAuthResponse:
      type: object
      description: Passthrough del body de Supabase Auth — no es un contrato propio de esta API.
      properties:
        access_token: { type: string }
        token_type: { type: string, example: bearer }
        expires_in: { type: integer }
        refresh_token: { type: string }
        user: { type: object, additionalProperties: true }
      additionalProperties: true

    PendingUserList:
      type: object
      required: [data, total]
      properties:
        data:
          type: array
          items: { $ref: '#/components/schemas/User' }
        total: { type: integer }

```

- [ ] **Step 4: Lint**

Run: `redocly lint docs/openapi.yaml`

Expected: sigue parseando (sin errores de sintaxis nuevos). El conteo de errores/warnings puede subir levemente solo por `operation-operationId` en las 2 operaciones nuevas (esperable, ya es la convención del archivo) — no debe aparecer ninguna categoría de error nueva.

- [ ] **Step 5: Commit**

```bash
git add docs/openapi.yaml
git commit -m "docs(openapi): documentar POST /auth/login y GET /users/pending"
```

---

## Task 3: Documentar Permissions (5 endpoints)

**Files:**
- Modify: `docs/openapi.yaml` (paths + schemas)

**Interfaces:**
- Consumes: tag `Permissions` (de Task 1), `$ref: '#/components/responses/Unauthorized'`, `$ref: '#/components/responses/Forbidden'`.
- Produces: schemas `Permission`, `PermissionCreate`, `PermissionError`, `PermissionValidationError`.

Handlers: `internal/api/handler/permissions/`. Registro: `internal/routes/url_mappings.go:310-314`. GET sin RBAC extra; POST/PUT/DELETE requieren `perm_users_manage`.

- [ ] **Step 1: Agregar los paths**

Insertar como nueva sección de `paths:`, justo **antes** de la línea `components:` (línea 999):

```yaml
  ########################
  # Permissions
  ########################
  /api/v1/permissions:
    get:
      tags: [Permissions]
      summary: Listar permisos del catálogo
      description: Sin RBAC adicional — cualquier usuario autenticado puede consultar.
      security: [{ bearerAuth: [] }]
      responses:
        '200':
          description: Lista de permisos
          content:
            application/json:
              schema:
                type: array
                items: { $ref: '#/components/schemas/Permission' }
        '401': { $ref: '#/components/responses/Unauthorized' }
    post:
      tags: [Permissions]
      summary: Crear un permiso
      description: Requiere `perm_users_manage`.
      security: [{ bearerAuth: [] }]
      requestBody:
        required: true
        content:
          application/json:
            schema: { $ref: '#/components/schemas/PermissionCreate' }
      responses:
        '201':
          description: Permiso creado
          content:
            application/json:
              schema: { $ref: '#/components/schemas/Permission' }
        '400':
          description: Validación fallida
          content:
            application/json:
              schema: { $ref: '#/components/schemas/PermissionValidationError' }
        '401': { $ref: '#/components/responses/Unauthorized' }
        '403': { $ref: '#/components/responses/Forbidden' }

  /api/v1/permissions/{id}:
    get:
      tags: [Permissions]
      summary: Obtener un permiso por ID
      description: Sin RBAC adicional.
      security: [{ bearerAuth: [] }]
      parameters:
        - { name: id, in: path, required: true, schema: { type: string } }
      responses:
        '200':
          description: Permiso encontrado
          content:
            application/json:
              schema: { $ref: '#/components/schemas/Permission' }
        '401': { $ref: '#/components/responses/Unauthorized' }
        '404':
          description: Permiso no encontrado
          content:
            application/json:
              schema: { $ref: '#/components/schemas/PermissionError' }
    put:
      tags: [Permissions]
      summary: Actualizar un permiso
      description: Requiere `perm_users_manage`. Los permisos del sistema (`isSystemPermission=true`) no se pueden modificar.
      security: [{ bearerAuth: [] }]
      parameters:
        - { name: id, in: path, required: true, schema: { type: string } }
      requestBody:
        required: true
        content:
          application/json:
            schema: { $ref: '#/components/schemas/PermissionCreate' }
      responses:
        '200':
          description: Permiso actualizado
          content:
            application/json:
              schema: { $ref: '#/components/schemas/Permission' }
        '401': { $ref: '#/components/responses/Unauthorized' }
        '403':
          description: No se pueden modificar permisos del sistema
          content:
            application/json:
              schema: { $ref: '#/components/schemas/PermissionError' }
        '404':
          description: Permiso no encontrado
          content:
            application/json:
              schema: { $ref: '#/components/schemas/PermissionError' }
    delete:
      tags: [Permissions]
      summary: Eliminar un permiso
      description: Requiere `perm_users_manage`. Los permisos del sistema no se pueden eliminar.
      security: [{ bearerAuth: [] }]
      parameters:
        - { name: id, in: path, required: true, schema: { type: string } }
      responses:
        '200':
          description: Permiso eliminado
          content:
            application/json:
              schema:
                type: object
                properties: { success: { type: boolean } }
                required: [success]
        '401': { $ref: '#/components/responses/Unauthorized' }
        '403':
          description: No se pueden eliminar permisos del sistema
          content:
            application/json:
              schema: { $ref: '#/components/schemas/PermissionError' }
        '404':
          description: Permiso no encontrado
          content:
            application/json:
              schema: { $ref: '#/components/schemas/PermissionError' }

```

- [ ] **Step 2: Agregar los schemas**

Insertar en `components/schemas`, antes de `# Standard responses` (mismo punto que Task 2):

```yaml
    Permission:
      type: object
      properties:
        id: { type: string }
        name: { type: string }
        section: { type: string }
        description: { type: string }
        isSystemPermission: { type: boolean }
        createdAt: { type: string, format: date-time }
        updatedAt: { type: string, format: date-time }
      required: [id, name, section, description, isSystemPermission]

    PermissionCreate:
      type: object
      properties:
        name: { type: string }
        section: { type: string }
        description: { type: string }
      required: [name, section, description]

    PermissionError:
      type: object
      properties:
        error: { type: string }
      required: [error]

    PermissionValidationError:
      type: object
      properties:
        error: { type: string, example: "Validation failed" }
        errors:
          type: array
          items:
            type: object
            properties:
              path: { type: string }
              message: { type: string }
      required: [error, errors]

```

- [ ] **Step 3: Lint**

Run: `redocly lint docs/openapi.yaml`

Expected: parsea sin errores de sintaxis nuevos; los nuevos errores/warnings solo pueden ser `operation-operationId` (esperable).

- [ ] **Step 4: Commit**

```bash
git add docs/openapi.yaml
git commit -m "docs(openapi): documentar /api/v1/permissions"
```

---

## Task 4: Documentar Roles (5 endpoints) + schema `ErrorResponse` compartido

**Files:**
- Modify: `docs/openapi.yaml` (paths + schemas)

**Interfaces:**
- Consumes: tag `Roles` (de Task 1).
- Produces: schema `ErrorResponse` (`{error, message, status}`) — **reutilizado también por Tasks 5 y 6** (Alarm Rules, Dashboard Layouts), que devuelven el mismo shape de error de dominio. Produce también `Role`, `RoleWrite`.

Handlers: `internal/api/handler/roles/`. Registro: `internal/routes/url_mappings.go:280-282`. GET sin RBAC extra; POST/PUT/DELETE requieren `perm_users_manage`.

- [ ] **Step 1: Agregar el schema compartido `ErrorResponse`**

Insertar en `components/schemas`, antes de `# Standard responses`:

```yaml
    ErrorResponse:
      type: object
      description: Formato de error de dominio compartido por Roles, Alarm Rules y Dashboard Layouts — no confundir con JWTError (401/403 de middleware).
      properties:
        error: { type: string, description: "Código de error, ej. ROLE_NOT_FOUND" }
        message: { type: string }
        status: { type: integer }
      required: [error, message, status]

    Role:
      type: object
      properties:
        id: { type: string }
        name: { type: string }
        description: { type: string }
        permissions:
          type: array
          items: { type: string }
        isSystemRole: { type: boolean }
        isGlobal: { type: boolean }
        tenantId: { type: string, format: uuid, nullable: true }
        createdAt: { type: string, format: date-time }
        updatedAt: { type: string, format: date-time }
      required: [id, name, permissions, isSystemRole, isGlobal, createdAt, updatedAt]

    RoleWrite:
      type: object
      properties:
        name: { type: string }
        description: { type: string }
        permissions:
          type: array
          items: { type: string }
      required: [name]

```

- [ ] **Step 2: Agregar los paths**

Insertar como nueva sección de `paths:`, antes de `components:`:

```yaml
  ########################
  # Roles
  ########################
  /api/v1/roles:
    get:
      tags: [Roles]
      summary: Listar roles del tenant
      description: Sin RBAC adicional — cualquier usuario autenticado puede listar/ver roles.
      security: [{ bearerAuth: [] }]
      responses:
        '200':
          description: Lista de roles
          content:
            application/json:
              schema:
                type: array
                items: { $ref: '#/components/schemas/Role' }
        '401': { $ref: '#/components/responses/Unauthorized' }
    post:
      tags: [Roles]
      summary: Crear un rol
      description: Requiere `perm_users_manage`.
      security: [{ bearerAuth: [] }]
      requestBody:
        required: true
        content:
          application/json:
            schema: { $ref: '#/components/schemas/RoleWrite' }
      responses:
        '201':
          description: Rol creado
          content:
            application/json:
              schema: { $ref: '#/components/schemas/Role' }
        '400':
          description: "Validación fallida o límite de roles alcanzado (LIMIT_REACHED)"
          content:
            application/json:
              schema: { $ref: '#/components/schemas/ErrorResponse' }
        '401': { $ref: '#/components/responses/Unauthorized' }
        '403': { $ref: '#/components/responses/Forbidden' }
        '409':
          description: Ya existe un rol con ese nombre (DUPLICATE_NAME)
          content:
            application/json:
              schema: { $ref: '#/components/schemas/ErrorResponse' }

  /api/v1/roles/{id}:
    get:
      tags: [Roles]
      summary: Obtener un rol por ID
      security: [{ bearerAuth: [] }]
      parameters:
        - { name: id, in: path, required: true, schema: { type: string } }
      responses:
        '200':
          description: Rol encontrado
          content:
            application/json:
              schema: { $ref: '#/components/schemas/Role' }
        '401': { $ref: '#/components/responses/Unauthorized' }
        '404':
          description: Rol no encontrado (ROLE_NOT_FOUND)
          content:
            application/json:
              schema: { $ref: '#/components/schemas/ErrorResponse' }
    put:
      tags: [Roles]
      summary: Actualizar un rol
      description: Requiere `perm_users_manage`. Los roles del sistema no se pueden modificar (SYSTEM_ROLE).
      security: [{ bearerAuth: [] }]
      parameters:
        - { name: id, in: path, required: true, schema: { type: string } }
      requestBody:
        required: true
        content:
          application/json:
            schema: { $ref: '#/components/schemas/RoleWrite' }
      responses:
        '200':
          description: Rol actualizado
          content:
            application/json:
              schema: { $ref: '#/components/schemas/Role' }
        '400': { $ref: '#/components/responses/BadRequest' }
        '401': { $ref: '#/components/responses/Unauthorized' }
        '403':
          description: Rol del sistema, no modificable (SYSTEM_ROLE)
          content:
            application/json:
              schema: { $ref: '#/components/schemas/ErrorResponse' }
        '404':
          description: Rol no encontrado (ROLE_NOT_FOUND)
          content:
            application/json:
              schema: { $ref: '#/components/schemas/ErrorResponse' }
        '409':
          description: Ya existe un rol con ese nombre (DUPLICATE_NAME)
          content:
            application/json:
              schema: { $ref: '#/components/schemas/ErrorResponse' }
    delete:
      tags: [Roles]
      summary: Eliminar un rol
      description: Requiere `perm_users_manage`. Falla si el rol tiene asignaciones activas (ROLE_HAS_ASSIGNMENTS).
      security: [{ bearerAuth: [] }]
      parameters:
        - { name: id, in: path, required: true, schema: { type: string } }
      responses:
        '200':
          description: Rol eliminado
          content:
            application/json:
              schema: { type: object }
        '401': { $ref: '#/components/responses/Unauthorized' }
        '403':
          description: Rol del sistema, no eliminable (SYSTEM_ROLE)
          content:
            application/json:
              schema: { $ref: '#/components/schemas/ErrorResponse' }
        '404':
          description: Rol no encontrado (ROLE_NOT_FOUND)
          content:
            application/json:
              schema: { $ref: '#/components/schemas/ErrorResponse' }
        '409':
          description: El rol tiene usuarios asignados (ROLE_HAS_ASSIGNMENTS)
          content:
            application/json:
              schema:
                allOf:
                  - $ref: '#/components/schemas/ErrorResponse'
                  - type: object
                    properties:
                      usersAffected: { type: integer }

```

- [ ] **Step 3: Lint**

Run: `redocly lint docs/openapi.yaml`

Expected: sin errores de sintaxis nuevos.

- [ ] **Step 4: Commit**

```bash
git add docs/openapi.yaml
git commit -m "docs(openapi): documentar /api/v1/roles"
```

---

## Task 5: Documentar Alarm Rules (5 endpoints)

**Files:**
- Modify: `docs/openapi.yaml` (paths + schemas)

**Interfaces:**
- Consumes: tag `AlarmRules` (Task 1), schema `ErrorResponse` (Task 4).
- Produces: schemas `AlarmRule`, `AlarmRuleCreate`, `AlarmRuleUpdate`.

Handlers: `internal/api/handler/alarm_rules/`. Registro: `internal/routes/url_mappings.go:287-290`. GET sin RBAC extra; POST/PATCH/DELETE requieren `perm_users_manage`.

- [ ] **Step 1: Agregar los schemas**

Insertar en `components/schemas`, antes de `# Standard responses`:

```yaml
    AlarmRule:
      type: object
      properties:
        id: { type: string, format: uuid }
        tenantId: { type: string, format: uuid }
        name: { type: string }
        description: { type: string }
        metric: { type: string }
        operator: { type: string, enum: [gt, lt, gte, lte, eq] }
        threshold: { type: number }
        severity: { type: string, enum: [info, warning, critical] }
        enabled: { type: boolean }
        createdAt: { type: string, format: date-time }
        updatedAt: { type: string, format: date-time }
      required: [id, tenantId, name, metric, operator, threshold, severity, enabled, createdAt, updatedAt]

    AlarmRuleCreate:
      type: object
      properties:
        name: { type: string }
        description: { type: string }
        metric: { type: string }
        operator: { type: string, enum: [gt, lt, gte, lte, eq] }
        threshold: { type: number }
        severity: { type: string, enum: [info, warning, critical] }
        enabled: { type: boolean, default: true }
      required: [name, metric, operator, threshold, severity]

    AlarmRuleUpdate:
      type: object
      description: Todos los campos opcionales (actualización parcial).
      properties:
        name: { type: string }
        description: { type: string }
        metric: { type: string }
        operator: { type: string, enum: [gt, lt, gte, lte, eq] }
        threshold: { type: number }
        severity: { type: string, enum: [info, warning, critical] }
        enabled: { type: boolean }
      minProperties: 1

```

- [ ] **Step 2: Agregar los paths**

Insertar como nueva sección de `paths:`, antes de `components:`:

```yaml
  ########################
  # Alarm Rules
  ########################
  /api/v1/alarm-rules:
    get:
      tags: [AlarmRules]
      summary: Listar reglas de alarma del tenant
      description: Sin RBAC adicional — cualquier usuario autenticado del tenant puede listar/ver reglas.
      security: [{ bearerAuth: [] }]
      responses:
        '200':
          description: Lista de reglas
          content:
            application/json:
              schema:
                type: array
                items: { $ref: '#/components/schemas/AlarmRule' }
        '401': { $ref: '#/components/responses/Unauthorized' }
    post:
      tags: [AlarmRules]
      summary: Crear una regla de alarma
      description: Requiere `perm_users_manage`.
      security: [{ bearerAuth: [] }]
      requestBody:
        required: true
        content:
          application/json:
            schema: { $ref: '#/components/schemas/AlarmRuleCreate' }
      responses:
        '201':
          description: Regla creada
          content:
            application/json:
              schema: { $ref: '#/components/schemas/AlarmRule' }
        '400':
          description: Validación fallida (VALIDATION_ERROR)
          content:
            application/json:
              schema: { $ref: '#/components/schemas/ErrorResponse' }
        '401': { $ref: '#/components/responses/Unauthorized' }
        '403': { $ref: '#/components/responses/Forbidden' }

  /api/v1/alarm-rules/{id}:
    get:
      tags: [AlarmRules]
      summary: Obtener una regla de alarma por ID
      security: [{ bearerAuth: [] }]
      parameters:
        - { name: id, in: path, required: true, schema: { type: string, format: uuid } }
      responses:
        '200':
          description: Regla encontrada
          content:
            application/json:
              schema: { $ref: '#/components/schemas/AlarmRule' }
        '401': { $ref: '#/components/responses/Unauthorized' }
        '404':
          description: Regla no encontrada (ALARM_RULE_NOT_FOUND)
          content:
            application/json:
              schema: { $ref: '#/components/schemas/ErrorResponse' }
    patch:
      tags: [AlarmRules]
      summary: Actualizar parcialmente una regla de alarma
      description: Requiere `perm_users_manage`.
      security: [{ bearerAuth: [] }]
      parameters:
        - { name: id, in: path, required: true, schema: { type: string, format: uuid } }
      requestBody:
        required: true
        content:
          application/json:
            schema: { $ref: '#/components/schemas/AlarmRuleUpdate' }
      responses:
        '200':
          description: Regla actualizada
          content:
            application/json:
              schema: { $ref: '#/components/schemas/AlarmRule' }
        '400':
          description: Validación fallida (VALIDATION_ERROR)
          content:
            application/json:
              schema: { $ref: '#/components/schemas/ErrorResponse' }
        '401': { $ref: '#/components/responses/Unauthorized' }
        '403': { $ref: '#/components/responses/Forbidden' }
        '404':
          description: Regla no encontrada (ALARM_RULE_NOT_FOUND)
          content:
            application/json:
              schema: { $ref: '#/components/schemas/ErrorResponse' }
    delete:
      tags: [AlarmRules]
      summary: Eliminar una regla de alarma
      description: Requiere `perm_users_manage`.
      security: [{ bearerAuth: [] }]
      parameters:
        - { name: id, in: path, required: true, schema: { type: string, format: uuid } }
      responses:
        '200':
          description: Regla eliminada
          content:
            application/json:
              schema: { type: object }
        '401': { $ref: '#/components/responses/Unauthorized' }
        '403': { $ref: '#/components/responses/Forbidden' }
        '404':
          description: Regla no encontrada (ALARM_RULE_NOT_FOUND)
          content:
            application/json:
              schema: { $ref: '#/components/schemas/ErrorResponse' }

```

- [ ] **Step 3: Lint**

Run: `redocly lint docs/openapi.yaml`

Expected: sin errores de sintaxis nuevos.

- [ ] **Step 4: Commit**

```bash
git add docs/openapi.yaml
git commit -m "docs(openapi): documentar /api/v1/alarm-rules"
```

---

## Task 6: Documentar Dashboard Layouts (5 endpoints)

**Files:**
- Modify: `docs/openapi.yaml` (paths + schemas)

**Interfaces:**
- Consumes: tag `DashboardLayouts` (Task 1), schema `ErrorResponse` (Task 4).
- Produces: schemas `LayoutWidgetPosition`, `LayoutWidget`, `DashboardLayout`, `DashboardLayoutWrite`, `DashboardLayoutList`.

Handlers: `internal/api/handler/dashboard_layouts/`. Registro: `internal/routes/url_mappings.go:275`. Sin RBAC más allá de estar autenticado (registrado directo sobre `v1`, sin `RBACCheck`). `tenant_id` viene de `X-Tenant-ID`, `user_id` del JWT — si el JWT no trae user id, 401 `UNAUTHORIZED` (chequeo de la app, no del middleware).

- [ ] **Step 1: Agregar los schemas**

Insertar en `components/schemas`, antes de `# Standard responses`:

```yaml
    LayoutWidgetPosition:
      type: object
      properties:
        x: { type: integer }
        y: { type: integer }
        w: { type: integer }
        h: { type: integer }
        i: { type: string }
      required: [x, y, w, h, i]

    LayoutWidget:
      type: object
      properties:
        id: { type: string }
        type: { type: string }
        name: { type: string }
        title: { type: string }
        description: { type: string }
        category: { type: string }
        icon: { type: string }
        position: { $ref: '#/components/schemas/LayoutWidgetPosition' }
      required: [id, type, position]

    DashboardLayout:
      type: object
      properties:
        id: { type: string, format: uuid }
        name: { type: string }
        widgets:
          type: array
          items: { $ref: '#/components/schemas/LayoutWidget' }
        createdAt: { type: string, format: date-time }
        updatedAt: { type: string, format: date-time }
      required: [id, name, widgets, createdAt, updatedAt]

    DashboardLayoutWrite:
      type: object
      properties:
        name: { type: string }
        widgets:
          type: array
          items: { $ref: '#/components/schemas/LayoutWidget' }
      required: [name]

    DashboardLayoutList:
      type: object
      properties:
        data:
          type: array
          items: { $ref: '#/components/schemas/DashboardLayout' }
        meta:
          type: object
          properties:
            total: { type: integer }
            limit: { type: integer, description: "Hardcodeado a 3 hoy — no es un query param configurable." }
          required: [total, limit]
      required: [data, meta]

```

- [ ] **Step 2: Agregar los paths**

Insertar como nueva sección de `paths:`, antes de `components:`:

```yaml
  ########################
  # Dashboard Layouts
  ########################
  /api/v1/dashboard-layouts:
    get:
      tags: [DashboardLayouts]
      summary: Listar layouts de dashboard del usuario
      description: tenant_id viene de `X-Tenant-ID`, user_id del JWT. Sin RBAC adicional más allá de estar autenticado.
      security: [{ bearerAuth: [] }]
      responses:
        '200':
          description: Lista de layouts (máx. 3 por usuario/tenant)
          content:
            application/json:
              schema: { $ref: '#/components/schemas/DashboardLayoutList' }
        '401':
          description: Usuario no identificado en el JWT (UNAUTHORIZED)
          content:
            application/json:
              schema: { $ref: '#/components/schemas/ErrorResponse' }
    post:
      tags: [DashboardLayouts]
      summary: Crear un layout de dashboard
      security: [{ bearerAuth: [] }]
      requestBody:
        required: true
        content:
          application/json:
            schema: { $ref: '#/components/schemas/DashboardLayoutWrite' }
      responses:
        '200':
          description: Layout creado
          content:
            application/json:
              schema: { $ref: '#/components/schemas/DashboardLayout' }
        '400': { $ref: '#/components/responses/BadRequest' }
        '401':
          description: Usuario no identificado en el JWT (UNAUTHORIZED)
          content:
            application/json:
              schema: { $ref: '#/components/schemas/ErrorResponse' }
        '403':
          description: Límite de layouts alcanzado (LIMIT_REACHED)
          content:
            application/json:
              schema: { $ref: '#/components/schemas/ErrorResponse' }
        '409':
          description: Ya existe un layout con ese nombre (DUPLICATE_NAME)
          content:
            application/json:
              schema: { $ref: '#/components/schemas/ErrorResponse' }

  /api/v1/dashboard-layouts/{layoutId}:
    get:
      tags: [DashboardLayouts]
      summary: Obtener un layout por ID
      security: [{ bearerAuth: [] }]
      parameters:
        - { name: layoutId, in: path, required: true, schema: { type: string, format: uuid } }
      responses:
        '200':
          description: Layout encontrado
          content:
            application/json:
              schema: { $ref: '#/components/schemas/DashboardLayout' }
        '401': { $ref: '#/components/responses/Unauthorized' }
        '404':
          description: Layout no encontrado (LAYOUT_NOT_FOUND)
          content:
            application/json:
              schema: { $ref: '#/components/schemas/ErrorResponse' }
    put:
      tags: [DashboardLayouts]
      summary: Actualizar un layout
      security: [{ bearerAuth: [] }]
      parameters:
        - { name: layoutId, in: path, required: true, schema: { type: string, format: uuid } }
      requestBody:
        required: true
        content:
          application/json:
            schema: { $ref: '#/components/schemas/DashboardLayoutWrite' }
      responses:
        '200':
          description: Layout actualizado
          content:
            application/json:
              schema: { $ref: '#/components/schemas/DashboardLayout' }
        '400': { $ref: '#/components/responses/BadRequest' }
        '401': { $ref: '#/components/responses/Unauthorized' }
        '404':
          description: Layout no encontrado (LAYOUT_NOT_FOUND)
          content:
            application/json:
              schema: { $ref: '#/components/schemas/ErrorResponse' }
        '409':
          description: Ya existe un layout con ese nombre (DUPLICATE_NAME)
          content:
            application/json:
              schema: { $ref: '#/components/schemas/ErrorResponse' }
    delete:
      tags: [DashboardLayouts]
      summary: Eliminar un layout
      description: No se puede eliminar el último layout del usuario (CANNOT_DELETE_LAST_LAYOUT).
      security: [{ bearerAuth: [] }]
      parameters:
        - { name: layoutId, in: path, required: true, schema: { type: string, format: uuid } }
      responses:
        '200':
          description: Layout eliminado
          content:
            application/json:
              schema: { type: object }
        '400':
          description: No se puede eliminar el último layout (CANNOT_DELETE_LAST_LAYOUT)
          content:
            application/json:
              schema: { $ref: '#/components/schemas/ErrorResponse' }
        '401': { $ref: '#/components/responses/Unauthorized' }
        '404':
          description: Layout no encontrado (LAYOUT_NOT_FOUND)
          content:
            application/json:
              schema: { $ref: '#/components/schemas/ErrorResponse' }

```

- [ ] **Step 3: Lint**

Run: `redocly lint docs/openapi.yaml`

Expected: sin errores de sintaxis nuevos.

- [ ] **Step 4: Commit**

```bash
git add docs/openapi.yaml
git commit -m "docs(openapi): documentar /api/v1/dashboard-layouts"
```

---

## Task 7: Documentar Dashboard Metrics (3 endpoints)

**Files:**
- Modify: `docs/openapi.yaml` (paths + schemas)

**Interfaces:**
- Consumes: tag `DashboardMetrics` (Task 1).
- Produces: schemas `MetricSpec`, `MetricsQueryRequest`, `MetricPoint`, `MetricScalarResult`, `MetricSeriesPoint`, `MetricGroupResult`, `MetricsQueryResponse`, `MetricsQueryEnvelope`, `MetricsErrorEnvelope`, `MetricsCatalogResponse`, `MetricsBatchQueryItem`, `MetricsBatchRequest`, `MetricsBatchResultItem`, `MetricsBatchResponse`.

Handlers: `internal/api/handler/dashboards/`. Montado en `v1.Group("/dashboards/metrics", RBACCheck("perm_metrics_view"), DashboardRateLimit(...))` (`internal/routes/url_mappings.go:337-341`) → paths reales `/api/v1/dashboards/metrics/query`, `/catalog`, `/query/batch`. Enums y límites vienen de `internal/domain/metrics/metrics.go` y del `Limits` configurado (`MaxSpecs=10`, `MaxBuckets=1000`, `MaxRawPoints=5000`, `MaxGroups=200`, `MaxBatchQueries=50`).

- [ ] **Step 1: Agregar los schemas**

Insertar en `components/schemas`, antes de `# Standard responses`:

```yaml
    MetricSpec:
      type: object
      properties:
        aasPath: { type: string }
        agg: { type: string, enum: [avg, sum, min, max, last, count, delta, raw] }
      required: [aasPath, agg]

    MetricsQueryRequest:
      type: object
      properties:
        machineId: { type: string }
        range: { type: string, enum: ["15m", "30m", "1h", "8h", "24h", "7d", "30d"] }
        from: { type: string, format: date-time }
        to: { type: string, format: date-time }
        bucket: { type: string, enum: ["1m", "5m", "15m", "1h", "6h", "1d"] }
        metrics:
          type: array
          items: { $ref: '#/components/schemas/MetricSpec' }
          minItems: 1
          maxItems: 10
        groupBy: { type: string }
        filter:
          type: object
          properties:
            valueEquals: {}
        maxPoints: { type: integer }
      required: [machineId, metrics]

    MetricPoint:
      type: object
      properties:
        ts: { type: string, format: date-time }
        value: {}
      required: [ts, value]

    MetricScalarResult:
      type: object
      properties:
        aasPath: { type: string }
        agg: { type: string }
        value: { type: number }
        sampleCount: { type: integer }
      required: [aasPath, agg, value, sampleCount]

    MetricSeriesPoint:
      type: object
      properties:
        ts: { type: string, format: date-time }
        results:
          type: array
          items: { $ref: '#/components/schemas/MetricScalarResult' }
      required: [ts, results]

    MetricGroupResult:
      type: object
      properties:
        key: { type: string }
        value: { type: number }
      required: [key, value]

    MetricsQueryResponse:
      type: object
      description: >
        Forma discriminada por `mode`: `scalar` usa `results`, `series` usa
        `series`, `raw` usa `points`, `grouped` usa `groups`. Los campos no
        aplicables al modo actual se omiten.
      properties:
        mode: { type: string, enum: [scalar, series, raw, grouped] }
        machineId: { type: string }
        from: { type: string, format: date-time }
        to: { type: string, format: date-time }
        dataAsOf: { type: string, format: date-time, nullable: true }
        bucket: { type: string }
        results:
          type: array
          items: { $ref: '#/components/schemas/MetricScalarResult' }
        series:
          type: array
          items: { $ref: '#/components/schemas/MetricSeriesPoint' }
        aasPath: { type: string }
        points:
          type: array
          items: { $ref: '#/components/schemas/MetricPoint' }
        agg: { type: string }
        groupBy: { type: string }
        groups:
          type: array
          items: { $ref: '#/components/schemas/MetricGroupResult' }
      required: [mode, machineId, from, to]

    MetricsQueryEnvelope:
      type: object
      properties:
        success: { type: boolean, example: true }
        data: { $ref: '#/components/schemas/MetricsQueryResponse' }
      required: [success, data]

    MetricsErrorEnvelope:
      type: object
      properties:
        success: { type: boolean, example: false }
        error: { type: string }
        code: { type: string, enum: [INVALID_PARAMS, TOO_MANY_METRICS, RAW_MODE_CONFLICT, GROUP_BY_CONFLICT, RANGE_TOO_WIDE, TOO_MANY_GROUPS, TOO_MANY_QUERIES, QUERY_TIMEOUT] }
      required: [success, error, code]

    MetricsCatalogResponse:
      type: object
      properties:
        success: { type: boolean, example: true }
        data:
          type: object
          properties:
            machineId: { type: string }
            aasPaths:
              type: array
              items: { type: string }
          required: [machineId, aasPaths]
      required: [success, data]

    MetricsBatchQueryItem:
      allOf:
        - $ref: '#/components/schemas/MetricsQueryRequest'
        - type: object
          properties:
            id: { type: string }
          required: [id]

    MetricsBatchRequest:
      type: object
      properties:
        queries:
          type: array
          items: { $ref: '#/components/schemas/MetricsBatchQueryItem' }
          minItems: 1
          maxItems: 50
      required: [queries]

    MetricsBatchResultItem:
      type: object
      description: Los errores por ítem no abortan el batch — solo errores a nivel batch (ej. ids duplicados) devuelven un envelope de error top-level.
      properties:
        id: { type: string }
        success: { type: boolean }
        data: { $ref: '#/components/schemas/MetricsQueryResponse' }
        error: { type: string }
        code: { type: string }
      required: [id, success]

    MetricsBatchResponse:
      type: object
      properties:
        success: { type: boolean, example: true }
        data:
          type: object
          properties:
            results:
              type: array
              items: { $ref: '#/components/schemas/MetricsBatchResultItem' }
          required: [results]
      required: [success, data]

```

- [ ] **Step 2: Agregar los paths**

Insertar como nueva sección de `paths:`, antes de `components:`:

```yaml
  ########################
  # Dashboard Metrics
  ########################
  /api/v1/dashboards/metrics/query:
    post:
      tags: [DashboardMetrics]
      summary: Consultar métricas de una máquina
      description: >
        Requiere `perm_metrics_view`. Sujeto a rate limit por tenant
        (`DashboardRateLimit`, responde 429 con header `Retry-After`).
        `metrics` acepta hasta `MaxSpecs` (10) entradas.
      security: [{ bearerAuth: [] }]
      requestBody:
        required: true
        content:
          application/json:
            schema: { $ref: '#/components/schemas/MetricsQueryRequest' }
      responses:
        '200':
          description: Resultado de la consulta
          content:
            application/json:
              schema: { $ref: '#/components/schemas/MetricsQueryEnvelope' }
        '400':
          description: "Parámetros inválidos, X-Tenant-ID ausente/inválido, JSON malformado, o conflicto de modo (ej. RAW_MODE_CONFLICT, GROUP_BY_CONFLICT, RANGE_TOO_WIDE, TOO_MANY_METRICS, TOO_MANY_GROUPS)"
          content:
            application/json:
              schema: { $ref: '#/components/schemas/MetricsErrorEnvelope' }
        '401': { $ref: '#/components/responses/Unauthorized' }
        '403': { $ref: '#/components/responses/Forbidden' }
        '429': { $ref: '#/components/responses/RateLimited' }
        '504':
          description: Timeout consultando Mongo (QUERY_TIMEOUT)
          content:
            application/json:
              schema: { $ref: '#/components/schemas/MetricsErrorEnvelope' }
        '500': { $ref: '#/components/responses/InternalError' }

  /api/v1/dashboards/metrics/catalog:
    get:
      tags: [DashboardMetrics]
      summary: Listar los aasPaths disponibles para una máquina
      description: Requiere `perm_metrics_view`. Sujeto al mismo rate limit que `/query`.
      security: [{ bearerAuth: [] }]
      parameters:
        - { name: machineId, in: query, required: true, schema: { type: string } }
      responses:
        '200':
          description: Catálogo de métricas disponibles
          content:
            application/json:
              schema: { $ref: '#/components/schemas/MetricsCatalogResponse' }
        '400':
          description: X-Tenant-ID ausente/inválido o machineId ausente (INVALID_PARAMS)
          content:
            application/json:
              schema: { $ref: '#/components/schemas/MetricsErrorEnvelope' }
        '401': { $ref: '#/components/responses/Unauthorized' }
        '403': { $ref: '#/components/responses/Forbidden' }
        '429': { $ref: '#/components/responses/RateLimited' }
        '500': { $ref: '#/components/responses/InternalError' }

  /api/v1/dashboards/metrics/query/batch:
    post:
      tags: [DashboardMetrics]
      summary: Consultar métricas de varias máquinas/consultas en un solo request
      description: >
        Requiere `perm_metrics_view`. Hasta `MaxBatchQueries` (50) consultas
        por batch, cada una con su propio `id`. Los errores de una consulta
        individual no abortan el resto del batch.
      security: [{ bearerAuth: [] }]
      requestBody:
        required: true
        content:
          application/json:
            schema: { $ref: '#/components/schemas/MetricsBatchRequest' }
      responses:
        '200':
          description: Resultados del batch (por ítem, ver `MetricsBatchResultItem.success`)
          content:
            application/json:
              schema: { $ref: '#/components/schemas/MetricsBatchResponse' }
        '400':
          description: "Parámetros inválidos a nivel batch (ej. ids duplicados, más de MaxBatchQueries consultas)"
          content:
            application/json:
              schema: { $ref: '#/components/schemas/MetricsErrorEnvelope' }
        '401': { $ref: '#/components/responses/Unauthorized' }
        '403': { $ref: '#/components/responses/Forbidden' }
        '429': { $ref: '#/components/responses/RateLimited' }
        '500': { $ref: '#/components/responses/InternalError' }

```

- [ ] **Step 3: Lint**

Run: `redocly lint docs/openapi.yaml`

Expected: sin errores de sintaxis nuevos. Vigilar en particular que `MetricsBatchQueryItem` (un `allOf`) no rompa el parseo — es la única composición `allOf` nueva de esta tarea.

- [ ] **Step 4: Commit**

```bash
git add docs/openapi.yaml
git commit -m "docs(openapi): documentar /api/v1/dashboards/metrics"
```

---

## Task 8: Documentar Edge Devices — núcleo (list, create, get, update, enable, disable)

**Files:**
- Modify: `docs/openapi.yaml` (paths + schemas)

**Interfaces:**
- Consumes: tag `EdgeDevices` (Task 1).
- Produces: schemas `EdgeDevice`, `EdgeDeviceCreate`, `EdgeDeviceUpdate`, `EdgeDeviceEnvelope`, `EdgeDeviceListEnvelope`, `EdgeDeviceErrorEnvelope` — **reutilizados también por Task 9**.

Handlers: `internal/api/handler/edge_devices/`. Montado en `internal/routes/url_mappings.go:261-269` como `r.Group("/api/v1/tenants/:tenantId", JWTAuth(...), ResolveTenantAndCheckMembership(db))` → **el tenant viene del path, no de `X-Tenant-ID`**. RBAC por ruta (`internal/api/handler/edge_devices/routes.go`): `perm_edge_devices_view` (GET), `perm_edge_devices_create` (POST crear), `perm_edge_devices_manage` (PUT, enable, disable).

- [ ] **Step 1: Agregar los schemas**

Insertar en `components/schemas`, antes de `# Standard responses`:

```yaml
    EdgeDevice:
      type: object
      properties:
        id: { type: string, format: uuid }
        tenantId: { type: string, format: uuid }
        name: { type: string }
        description: { type: string, nullable: true }
        machineId: { type: string }
        edgeType: { type: string, enum: [RASPBERRY_PLC] }
        raspberryBaseUrl: { type: string }
        plcAddress: { type: string, nullable: true }
        status: { type: string, enum: [ACTIVE, DISABLED] }
        lastSeenAt: { type: string, format: date-time, nullable: true }
        lastHealthCheckAt: { type: string, format: date-time, nullable: true }
        lastHealthStatus: { type: string, enum: [OK, DEGRADED, ERROR, UNKNOWN] }
        lastHealthSummary: { type: string, nullable: true }
        createdAt: { type: string, format: date-time }
        updatedAt: { type: string, format: date-time }
      required: [id, tenantId, name, machineId, edgeType, raspberryBaseUrl, status, lastHealthStatus, createdAt, updatedAt]

    EdgeDeviceCreate:
      type: object
      properties:
        name: { type: string }
        machineId: { type: string }
        edgeType: { type: string, enum: [RASPBERRY_PLC] }
        raspberryBaseUrl: { type: string }
        description: { type: string, nullable: true }
        plcAddress: { type: string, nullable: true }
      required: [name, machineId, edgeType, raspberryBaseUrl]

    EdgeDeviceUpdate:
      type: object
      description: machineId y edgeType son inmutables — no se aceptan en este body.
      properties:
        name: { type: string, nullable: true }
        description: { type: string, nullable: true }
        raspberryBaseUrl: { type: string, nullable: true }
        plcAddress: { type: string, nullable: true }

    EdgeDeviceEnvelope:
      type: object
      properties:
        success: { type: boolean, example: true }
        data: { $ref: '#/components/schemas/EdgeDevice' }
      required: [success, data]

    EdgeDeviceListEnvelope:
      type: object
      properties:
        success: { type: boolean, example: true }
        data:
          type: array
          items: { $ref: '#/components/schemas/EdgeDevice' }
      required: [success, data]

    EdgeDeviceErrorEnvelope:
      type: object
      properties:
        success: { type: boolean, example: false }
        error: { type: string }
      required: [success, error]

```

- [ ] **Step 2: Agregar los paths**

Insertar como nueva sección de `paths:`, antes de `components:`:

```yaml
  ########################
  # Edge Devices — núcleo
  ########################
  /api/v1/tenants/{tenantId}/edge-devices:
    parameters:
      - { name: tenantId, in: path, required: true, schema: { type: string, format: uuid } }
    get:
      tags: [EdgeDevices]
      summary: Listar dispositivos edge del tenant
      description: Requiere `perm_edge_devices_view`. Tenant resuelto desde el path, no desde `X-Tenant-ID`.
      security: [{ bearerAuth: [] }]
      responses:
        '200':
          description: Lista de dispositivos
          content:
            application/json:
              schema: { $ref: '#/components/schemas/EdgeDeviceListEnvelope' }
        '401': { $ref: '#/components/responses/Unauthorized' }
        '403': { $ref: '#/components/responses/Forbidden' }
    post:
      tags: [EdgeDevices]
      summary: Crear un dispositivo edge
      description: Requiere `perm_edge_devices_create`.
      security: [{ bearerAuth: [] }]
      requestBody:
        required: true
        content:
          application/json:
            schema: { $ref: '#/components/schemas/EdgeDeviceCreate' }
      responses:
        '201':
          description: Dispositivo creado
          content:
            application/json:
              schema: { $ref: '#/components/schemas/EdgeDeviceEnvelope' }
        '400':
          description: Body inválido
          content:
            application/json:
              schema: { $ref: '#/components/schemas/EdgeDeviceErrorEnvelope' }
        '401': { $ref: '#/components/responses/Unauthorized' }
        '403': { $ref: '#/components/responses/Forbidden' }
        '409':
          description: "machineId ya existe en el tenant"
          content:
            application/json:
              schema: { $ref: '#/components/schemas/EdgeDeviceErrorEnvelope' }

  /api/v1/tenants/{tenantId}/edge-devices/{deviceId}:
    parameters:
      - { name: tenantId, in: path, required: true, schema: { type: string, format: uuid } }
      - { name: deviceId, in: path, required: true, schema: { type: string, format: uuid } }
    get:
      tags: [EdgeDevices]
      summary: Obtener un dispositivo edge por ID
      description: Requiere `perm_edge_devices_view`.
      security: [{ bearerAuth: [] }]
      responses:
        '200':
          description: Dispositivo encontrado
          content:
            application/json:
              schema: { $ref: '#/components/schemas/EdgeDeviceEnvelope' }
        '401': { $ref: '#/components/responses/Unauthorized' }
        '403': { $ref: '#/components/responses/Forbidden' }
        '404':
          description: Dispositivo no encontrado
          content:
            application/json:
              schema: { $ref: '#/components/schemas/EdgeDeviceErrorEnvelope' }
    put:
      tags: [EdgeDevices]
      summary: Actualizar un dispositivo edge
      description: Requiere `perm_edge_devices_manage`. `machineId` y `edgeType` son inmutables (se ignoran si se envían).
      security: [{ bearerAuth: [] }]
      requestBody:
        required: true
        content:
          application/json:
            schema: { $ref: '#/components/schemas/EdgeDeviceUpdate' }
      responses:
        '200':
          description: Dispositivo actualizado
          content:
            application/json:
              schema: { $ref: '#/components/schemas/EdgeDeviceEnvelope' }
        '400':
          description: "Nombre vacío o raspberryBaseUrl inválida"
          content:
            application/json:
              schema: { $ref: '#/components/schemas/EdgeDeviceErrorEnvelope' }
        '401': { $ref: '#/components/responses/Unauthorized' }
        '403': { $ref: '#/components/responses/Forbidden' }
        '404':
          description: Dispositivo no encontrado
          content:
            application/json:
              schema: { $ref: '#/components/schemas/EdgeDeviceErrorEnvelope' }

  /api/v1/tenants/{tenantId}/edge-devices/{deviceId}/enable:
    post:
      tags: [EdgeDevices]
      summary: Habilitar un dispositivo edge
      description: Requiere `perm_edge_devices_manage`.
      security: [{ bearerAuth: [] }]
      parameters:
        - { name: tenantId, in: path, required: true, schema: { type: string, format: uuid } }
        - { name: deviceId, in: path, required: true, schema: { type: string, format: uuid } }
      responses:
        '200':
          description: "Dispositivo habilitado (status pasa a ACTIVE)"
          content:
            application/json:
              schema: { $ref: '#/components/schemas/EdgeDeviceEnvelope' }
        '401': { $ref: '#/components/responses/Unauthorized' }
        '403': { $ref: '#/components/responses/Forbidden' }
        '404':
          description: Dispositivo no encontrado
          content:
            application/json:
              schema: { $ref: '#/components/schemas/EdgeDeviceErrorEnvelope' }

  /api/v1/tenants/{tenantId}/edge-devices/{deviceId}/disable:
    post:
      tags: [EdgeDevices]
      summary: Deshabilitar un dispositivo edge
      description: Requiere `perm_edge_devices_manage`.
      security: [{ bearerAuth: [] }]
      parameters:
        - { name: tenantId, in: path, required: true, schema: { type: string, format: uuid } }
        - { name: deviceId, in: path, required: true, schema: { type: string, format: uuid } }
      responses:
        '200':
          description: "Dispositivo deshabilitado (status pasa a DISABLED)"
          content:
            application/json:
              schema: { $ref: '#/components/schemas/EdgeDeviceEnvelope' }
        '401': { $ref: '#/components/responses/Unauthorized' }
        '403': { $ref: '#/components/responses/Forbidden' }
        '404':
          description: Dispositivo no encontrado
          content:
            application/json:
              schema: { $ref: '#/components/schemas/EdgeDeviceErrorEnvelope' }

```

- [ ] **Step 3: Lint**

Run: `redocly lint docs/openapi.yaml`

Expected: sin errores de sintaxis nuevos.

- [ ] **Step 4: Commit**

```bash
git add docs/openapi.yaml
git commit -m "docs(openapi): documentar nucleo de /api/v1/tenants/{tenantId}/edge-devices"
```

---

## Task 9: Documentar Edge Devices — operacional (status, health-check, telemetry, events, api-keys)

**Files:**
- Modify: `docs/openapi.yaml` (paths + schemas)

**Interfaces:**
- Consumes: tag `EdgeDevices`, schema `EdgeDeviceErrorEnvelope` (ambos de Task 8).
- Produces: schemas `EdgeDeviceCheckResult`, `EdgeDeviceCheckResultEnvelope`, `EdgeDeviceTelemetry`, `EdgeDeviceTelemetryEnvelope`, `EdgeDeviceEvent`, `EdgeDeviceEventListEnvelope`, `EdgeDeviceAPIKey`, `EdgeDeviceAPIKeyCreate`, `EdgeDeviceAPIKeyCreated`, `EdgeDeviceAPIKeyEnvelope`, `EdgeDeviceAPIKeyListEnvelope`.

RBAC: `perm_edge_devices_check` (status, health-check), `perm_edge_devices_view` (telemetry, events, list api-keys), `perm_edge_devices_manage` (create/revoke api-key).

- [ ] **Step 1: Agregar los schemas**

Insertar en `components/schemas`, antes de `# Standard responses`:

```yaml
    EdgeDeviceCheckResult:
      type: object
      properties:
        checkType: { type: string, enum: [STATUS, HEALTH_CHECK] }
        checkedAt: { type: string, format: date-time }
        overallStatus: { type: string, enum: [OK, DEGRADED, ERROR, UNKNOWN] }
        summary: { type: string }
        details:
          type: object
          additionalProperties: true
      required: [checkType, checkedAt, overallStatus, summary]

    EdgeDeviceCheckResultEnvelope:
      type: object
      properties:
        success: { type: boolean, example: true }
        data: { $ref: '#/components/schemas/EdgeDeviceCheckResult' }
      required: [success, data]

    EdgeDeviceTelemetry:
      type: object
      properties:
        capturedAt: { type: string, format: date-time }
        cpu:
          type: object
          nullable: true
          properties:
            usagePercent: { type: number, nullable: true }
        ram:
          type: object
          nullable: true
          properties:
            usedPercent: { type: number }
            usedMb: { type: number }
            totalMb: { type: number }
        disk:
          type: object
          nullable: true
          properties:
            usedPercent: { type: number }
            usedGb: { type: number }
            totalGb: { type: integer }
        temperatureCelsius: { type: number, nullable: true }
        uptimeSeconds: { type: integer, nullable: true }
        plc:
          type: object
          nullable: true
          properties:
            reachable: { type: boolean }
            latencyMs: { type: integer, nullable: true }
            lastHeartbeatAt: { type: string, format: date-time, nullable: true }
      required: [capturedAt]

    EdgeDeviceTelemetryEnvelope:
      type: object
      properties:
        success: { type: boolean, example: true }
        data: { $ref: '#/components/schemas/EdgeDeviceTelemetry' }
      required: [success, data]

    EdgeDeviceEvent:
      type: object
      properties:
        id: { type: string, format: uuid }
        deviceId: { type: string, format: uuid }
        tenantId: { type: string, format: uuid }
        checkType: { type: string, enum: [STATUS, HEALTH_CHECK] }
        checkedAt: { type: string, format: date-time }
        overallStatus: { type: string, enum: [OK, DEGRADED, ERROR, UNKNOWN] }
        summary: { type: string, nullable: true }
        details:
          type: object
          additionalProperties: true
        userId: { type: string, format: uuid }
        userEmail: { type: string, format: email }
      required: [id, deviceId, tenantId, checkType, checkedAt, overallStatus, userId, userEmail]

    EdgeDeviceEventListEnvelope:
      type: object
      properties:
        success: { type: boolean, example: true }
        data:
          type: array
          items: { $ref: '#/components/schemas/EdgeDeviceEvent' }
      required: [success, data]

    EdgeDeviceAPIKey:
      type: object
      description: Nunca incluye el secreto en texto plano — eso solo aparece en la respuesta de creación (ver `EdgeDeviceAPIKeyCreated`).
      properties:
        id: { type: string }
        keyId: { type: string }
        name: { type: string, nullable: true }
        createdAt: { type: string, format: date-time }
        expiresAt: { type: string, format: date-time, nullable: true }
        revokedAt: { type: string, format: date-time, nullable: true }
        lastUsedAt: { type: string, format: date-time, nullable: true }
        active: { type: boolean }
        deviceStatus: { type: string, enum: [ACTIVE, DISABLED] }
      required: [id, keyId, createdAt, active, deviceStatus]

    EdgeDeviceAPIKeyCreate:
      type: object
      properties:
        name: { type: string, nullable: true }
        expiresAt: { type: string, format: date-time, nullable: true }

    EdgeDeviceAPIKeyCreated:
      description: Igual a `EdgeDeviceAPIKey` más el secreto en texto plano — se muestra una única vez, en esta respuesta.
      allOf:
        - $ref: '#/components/schemas/EdgeDeviceAPIKey'
        - type: object
          properties:
            key: { type: string }
          required: [key]

    EdgeDeviceAPIKeyEnvelope:
      type: object
      properties:
        success: { type: boolean, example: true }
        data: { $ref: '#/components/schemas/EdgeDeviceAPIKeyCreated' }
      required: [success, data]

    EdgeDeviceAPIKeyListEnvelope:
      type: object
      properties:
        success: { type: boolean, example: true }
        data:
          type: array
          items: { $ref: '#/components/schemas/EdgeDeviceAPIKey' }
      required: [success, data]

```

- [ ] **Step 2: Agregar los paths**

Insertar como nueva sección de `paths:`, antes de `components:`:

```yaml
  ########################
  # Edge Devices — operacional
  ########################
  /api/v1/tenants/{tenantId}/edge-devices/{deviceId}/status:
    post:
      tags: [EdgeDevices]
      summary: Disparar un chequeo de status contra el dispositivo
      description: Requiere `perm_edge_devices_check`. Queda registrado en el audit trail (`userId`/`userEmail` del JWT).
      security: [{ bearerAuth: [] }]
      parameters:
        - { name: tenantId, in: path, required: true, schema: { type: string, format: uuid } }
        - { name: deviceId, in: path, required: true, schema: { type: string, format: uuid } }
      responses:
        '200':
          description: Resultado del chequeo
          content:
            application/json:
              schema: { $ref: '#/components/schemas/EdgeDeviceCheckResultEnvelope' }
        '400':
          description: "Dispositivo deshabilitado (EDGE_DEVICE_DISABLED)"
          content:
            application/json:
              schema: { $ref: '#/components/schemas/EdgeDeviceErrorEnvelope' }
        '401': { $ref: '#/components/responses/Unauthorized' }
        '403': { $ref: '#/components/responses/Forbidden' }
        '404':
          description: Dispositivo no encontrado
          content:
            application/json:
              schema: { $ref: '#/components/schemas/EdgeDeviceErrorEnvelope' }

  /api/v1/tenants/{tenantId}/edge-devices/{deviceId}/health-check:
    post:
      tags: [EdgeDevices]
      summary: Disparar un health check contra el dispositivo
      description: Requiere `perm_edge_devices_check`. Igual contrato que `/status`.
      security: [{ bearerAuth: [] }]
      parameters:
        - { name: tenantId, in: path, required: true, schema: { type: string, format: uuid } }
        - { name: deviceId, in: path, required: true, schema: { type: string, format: uuid } }
      responses:
        '200':
          description: Resultado del chequeo
          content:
            application/json:
              schema: { $ref: '#/components/schemas/EdgeDeviceCheckResultEnvelope' }
        '400':
          description: "Dispositivo deshabilitado (EDGE_DEVICE_DISABLED)"
          content:
            application/json:
              schema: { $ref: '#/components/schemas/EdgeDeviceErrorEnvelope' }
        '401': { $ref: '#/components/responses/Unauthorized' }
        '403': { $ref: '#/components/responses/Forbidden' }
        '404':
          description: Dispositivo no encontrado
          content:
            application/json:
              schema: { $ref: '#/components/schemas/EdgeDeviceErrorEnvelope' }

  /api/v1/tenants/{tenantId}/edge-devices/{deviceId}/telemetry:
    get:
      tags: [EdgeDevices]
      summary: Obtener telemetría actual del dispositivo
      description: Requiere `perm_edge_devices_view`.
      security: [{ bearerAuth: [] }]
      parameters:
        - { name: tenantId, in: path, required: true, schema: { type: string, format: uuid } }
        - { name: deviceId, in: path, required: true, schema: { type: string, format: uuid } }
      responses:
        '200':
          description: Telemetría del dispositivo
          content:
            application/json:
              schema: { $ref: '#/components/schemas/EdgeDeviceTelemetryEnvelope' }
        '400':
          description: "Dispositivo deshabilitado (EDGE_DEVICE_DISABLED)"
          content:
            application/json:
              schema: { $ref: '#/components/schemas/EdgeDeviceErrorEnvelope' }
        '401': { $ref: '#/components/responses/Unauthorized' }
        '403': { $ref: '#/components/responses/Forbidden' }
        '404':
          description: Dispositivo no encontrado
          content:
            application/json:
              schema: { $ref: '#/components/schemas/EdgeDeviceErrorEnvelope' }

  /api/v1/tenants/{tenantId}/edge-devices/{deviceId}/events:
    get:
      tags: [EdgeDevices]
      summary: Listar el historial de chequeos (status/health-check) del dispositivo
      description: Requiere `perm_edge_devices_view`.
      security: [{ bearerAuth: [] }]
      parameters:
        - { name: tenantId, in: path, required: true, schema: { type: string, format: uuid } }
        - { name: deviceId, in: path, required: true, schema: { type: string, format: uuid } }
      responses:
        '200':
          description: Historial de eventos
          content:
            application/json:
              schema: { $ref: '#/components/schemas/EdgeDeviceEventListEnvelope' }
        '401': { $ref: '#/components/responses/Unauthorized' }
        '403': { $ref: '#/components/responses/Forbidden' }
        '404':
          description: Dispositivo no encontrado
          content:
            application/json:
              schema: { $ref: '#/components/schemas/EdgeDeviceErrorEnvelope' }

  /api/v1/tenants/{tenantId}/edge-devices/{deviceId}/api-keys:
    parameters:
      - { name: tenantId, in: path, required: true, schema: { type: string, format: uuid } }
      - { name: deviceId, in: path, required: true, schema: { type: string, format: uuid } }
    get:
      tags: [EdgeDevices]
      summary: Listar API keys del dispositivo
      description: Requiere `perm_edge_devices_view`. No expone secretos, solo metadata.
      security: [{ bearerAuth: [] }]
      responses:
        '200':
          description: Lista de API keys
          content:
            application/json:
              schema: { $ref: '#/components/schemas/EdgeDeviceAPIKeyListEnvelope' }
        '401': { $ref: '#/components/responses/Unauthorized' }
        '403': { $ref: '#/components/responses/Forbidden' }
        '404':
          description: Dispositivo no encontrado
          content:
            application/json:
              schema: { $ref: '#/components/schemas/EdgeDeviceErrorEnvelope' }
    post:
      tags: [EdgeDevices]
      summary: Emitir una nueva API key para el dispositivo
      description: >
        Requiere `perm_edge_devices_manage`. El secreto en texto plano solo se
        devuelve en esta respuesta — nunca vuelve a exponerse (ver GET, que
        solo devuelve metadata).
      security: [{ bearerAuth: [] }]
      requestBody:
        required: false
        content:
          application/json:
            schema: { $ref: '#/components/schemas/EdgeDeviceAPIKeyCreate' }
      responses:
        '201':
          description: API key creada (incluye el secreto en texto plano una única vez)
          content:
            application/json:
              schema: { $ref: '#/components/schemas/EdgeDeviceAPIKeyEnvelope' }
        '400':
          description: Body malformado
          content:
            application/json:
              schema: { $ref: '#/components/schemas/EdgeDeviceErrorEnvelope' }
        '401': { $ref: '#/components/responses/Unauthorized' }
        '403': { $ref: '#/components/responses/Forbidden' }
        '404':
          description: Dispositivo no encontrado
          content:
            application/json:
              schema: { $ref: '#/components/schemas/EdgeDeviceErrorEnvelope' }

  /api/v1/tenants/{tenantId}/edge-devices/{deviceId}/api-keys/{keyId}:
    delete:
      tags: [EdgeDevices]
      summary: Revocar una API key del dispositivo
      description: Requiere `perm_edge_devices_manage`. Idempotente.
      security: [{ bearerAuth: [] }]
      parameters:
        - { name: tenantId, in: path, required: true, schema: { type: string, format: uuid } }
        - { name: deviceId, in: path, required: true, schema: { type: string, format: uuid } }
        - { name: keyId, in: path, required: true, schema: { type: string } }
      responses:
        '204':
          description: API key revocada
        '401': { $ref: '#/components/responses/Unauthorized' }
        '403': { $ref: '#/components/responses/Forbidden' }
        '404':
          description: "API key no encontrada"
          content:
            application/json:
              schema: { $ref: '#/components/schemas/EdgeDeviceErrorEnvelope' }

```

- [ ] **Step 3: Lint**

Run: `redocly lint docs/openapi.yaml`

Expected: sin errores de sintaxis nuevos.

- [ ] **Step 4: Commit**

```bash
git add docs/openapi.yaml
git commit -m "docs(openapi): documentar operacional de /api/v1/tenants/{tenantId}/edge-devices"
```

---

## Task 10: Verificación final y PR

**Files:**
- Read-only: `docs/openapi.yaml`, `internal/routes/url_mappings.go`, `internal/api/router.go`, `internal/api/handler/*/routes.go`

**Interfaces:**
- Consumes: todo lo producido en Tasks 1-9.
- Produces: nada nuevo — solo verificación y el PR.

- [ ] **Step 1: Re-extraer la lista de paths documentados**

Run: `grep -nE '^  /' docs/openapi.yaml`

Expected: la lista debe incluir, además de los 30 paths que ya existían, estos 15 paths nuevos (algunos con varios métodos):
`/api/v1/auth/login`, `/api/v1/users/pending`, `/api/v1/permissions`, `/api/v1/permissions/{id}`, `/api/v1/roles`, `/api/v1/roles/{id}`, `/api/v1/alarm-rules`, `/api/v1/alarm-rules/{id}`, `/api/v1/dashboard-layouts`, `/api/v1/dashboard-layouts/{layoutId}`, `/api/v1/dashboards/metrics/query`, `/api/v1/dashboards/metrics/catalog`, `/api/v1/dashboards/metrics/query/batch`, `/api/v1/tenants/{tenantId}/edge-devices` (+ sub-rutas), `/api/v1/tenants/{tenantId}/edge-devices/{deviceId}` (+ sub-rutas).

- [ ] **Step 2: Re-extraer la lista de rutas reales del código y diffear a mano**

Run:
```bash
grep -nE '\.(GET|POST|PUT|PATCH|DELETE)\(' internal/routes/url_mappings.go internal/api/router.go internal/api/handler/alarm_rules/routes.go internal/api/handler/dashboards/routes.go internal/api/handler/dashboard_layouts/routes.go internal/api/handler/edge_devices/routes.go internal/api/handler/logs/routes.go internal/api/handler/notifications/routes.go internal/api/handler/roles/routes.go
```

Expected: cada ruta de esta lista tiene una entrada correspondiente en `docs/openapi.yaml` (verificar a ojo contra el Step 1). Si aparece algo nuevo que no estaba en el relevamiento original de esta conversación, agregarlo siguiendo el mismo patrón que la tarea del grupo correspondiente antes de seguir.

- [ ] **Step 3: Lint completo**

Run: `make lint`

Expected: corre `redocly lint docs/openapi.yaml` y `golangci-lint run ./...` (si está instalado). El OpenAPI lint debe seguir sin errores de sintaxis nuevos respecto del baseline de la Tarea 1 (19 errores / 54 warnings + lo agregado por las 9 tareas, todo de las mismas 2 categorías `struct`/`operation-operationId`). Si aparece una categoría de error nueva (`unresolved-ref`, `no-invalid-media-type-examples`, etc.), hay un `$ref` roto o un typo — corregirlo antes de seguir.

- [ ] **Step 4: Confirmar que no se tocó código Go**

Run: `git diff --stat develop`

Expected: el único archivo modificado en todo el diff es `docs/openapi.yaml`.

- [ ] **Step 5: Push y PR**

```bash
git push -u origin docs/openapi-missing-endpoints
gh pr create --base develop --title "docs(openapi): completar cobertura de endpoints faltantes" --body "$(cat <<'EOF'
## Summary
- Documenta en docs/openapi.yaml los 38 endpoints que existian en el codigo pero no en el spec: POST /auth/login, GET /users/pending, Permissions, Roles, Alarm Rules, Dashboard Layouts, Dashboard Metrics y Edge Devices.
- De paso corrige un bug preexistente (espacio inicial en la linea 1) que hacia fallar el parseo de `redocly lint docs/openapi.yaml` por completo.
- No se toca codigo Go — solo documentacion.

## Test plan
- [ ] `redocly lint docs/openapi.yaml` parsea sin errores de sintaxis nuevos (baseline preexistente: 19 errores / 54 warnings de las categorias `struct`/`operation-operationId`)
- [ ] `make lint` corre limpio (modulo el mismo baseline)
- [ ] Cada path nuevo corresponde 1:1 a una ruta real en `internal/routes/url_mappings.go` / `internal/api/router.go` / `internal/api/handler/*/routes.go`
EOF
)"
```

Expected: PR creado contra `develop` (no `main`), URL devuelta por `gh pr create`.
