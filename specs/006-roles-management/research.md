# Investigación: API de Gestión de Roles

**Feature**: `006-roles-management`
**Fecha**: 2026-04-03

---

## Decisión 1: Estrategia de IDs para Roles Custom

**Decisión**: Usar strings del tipo `custom_<6 hex chars>` (por ejemplo: `custom_3a9f12`)

**Fundamento**: La tabla `roles` tiene PK `VARCHAR(50)`. Los roles del sistema usan strings legibles ("admin", "operario"). Usar un formato similar para roles custom mantiene consistencia en el tipo y permite que los IDs sean descriptivos. Un UUID sería inconsistente con el esquema existente.

**Generación**: `"custom_" + hex.EncodeToString(rand.Read(3 bytes))` — 6 caracteres hex, probabilidad de colisión despreciable con límite de 3 roles por tenant.

**Alternativas descartadas**:
- UUID: inconsistente con PK VARCHAR(50) existente, requeriría cambio de tipo
- Slug del nombre: riesgo de colisión, complica el rename

---

## Decisión 2: Almacenamiento de Permisos

**Decisión**: Columna `permissions JSONB NOT NULL DEFAULT '[]'` en la tabla `roles`

**Fundamento**: Los permisos siempre se leen y escriben como conjunto completo (reemplazo total en update). No existen consultas del tipo "dame todos los roles que tienen el permiso X". Una tabla separada agregaría complejidad sin beneficio real para este caso de uso.

**Alternativas descartadas**:
- Tabla `role_permissions`: mayor complejidad, joins innecesarios, sin ventaja para el patrón de acceso actual
- Array PostgreSQL (`TEXT[]`): JSONB ya usado en el proyecto (widgets en dashboard_layouts), más flexible

---

## Decisión 3: Scoping del Listado

**Decisión**: Devolver roles del sistema (is_global = TRUE) + roles custom del tenant (tenant_id = X) en un único listado ordenado

**Fundamento**: El frontend necesita ver todos los roles disponibles para asignar a usuarios. Los roles del sistema son globales y siempre visibles. Ordenar: roles del sistema primero (is_system_role DESC), luego alfabético por nombre.

**Alternativas descartadas**:
- Endpoint separado para roles del sistema: innecesario, el Pact muestra un único GET /roles
- Filtro opcional `?type=system|custom`: sobre-ingeniería para el caso de uso actual

---

## Decisión 4: Validación de Permisos

**Decisión**: Permisos como strings opacos — el servidor almacena y devuelve sin validar contra catálogo

**Fundamento**: No existe un catálogo de permisos en la BD (el RBAC es estático en `security/rbac.go`). Validar contra el mapa estático acoplaría los roles custom al código, dificultando la extensión futura. El spec explícitamente define los permisos como opacos.

**Implicación futura**: Cuando se implemente la feature de Permisos (006+), la validación puede agregarse como paso adicional en el servicio sin cambiar la interfaz.

---

## Decisión 5: Unicidad de Nombre

**Decisión**: Índice parcial `UNIQUE (tenant_id, name) WHERE deleted_at IS NULL AND is_system_role = FALSE`

**Fundamento**:
- El soft-delete requiere índice parcial (permite reusar nombres de roles eliminados)
- Los roles del sistema están excluidos (no tienen tenant_id y sus nombres son globales)
- La restricción aplica solo a roles custom dentro del mismo tenant

---

## Decisión 6: Soft Delete

**Decisión**: Soft delete con columna `deleted_at TIMESTAMPTZ`

**Fundamento**: Consistente con el patrón del proyecto (dashboard_layouts, users, invitations). Permite auditoría. Los roles eliminados no aparecen en la lista pero su historial se preserva en `user_tenant_roles`.

---

## Estado del RBAC Estático

El mapa `rolePermissions` en `security/rbac.go` **no se modifica** en esta feature. El RBAC en tiempo de request sigue siendo estático. Los nuevos roles custom creados vía API no tienen efecto en la autorización hasta que se integre el RBAC dinámico (feature futura).

**Justificación**: Separación de concerns — la gestión de roles (este feature) es independiente de la autorización (RBAC). Extender el RBAC para leer roles de la BD es una feature separada con mayor complejidad e impacto en performance.
