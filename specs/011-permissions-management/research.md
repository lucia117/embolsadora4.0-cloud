# Research: Permissions Management API

**Feature**: 011-permissions-management  
**Date**: 2026-04-10  
**Status**: Complete — sin NEEDS CLARIFICATION pendientes

---

## Decisión 1: Estrategia de almacenamiento de permisos de sistema

**Decisión**: Los permisos de sistema se almacenan en la tabla `permissions` con un flag `is_system_permission = TRUE` y `tenant_id = NULL`. Se insertan via migración de base de datos (seed en `000017_create_permissions_table.up.sql`).

**Rationale**: 
- Consistente con el patrón ya establecido en `roles` (ver `is_system_role`, `is_global` en `internal/domain/roles.go`)
- Permite consultas unificadas: `WHERE tenant_id = $1 OR is_system_permission = TRUE`
- Los permisos de sistema no tienen `tenant_id`, son globales
- El seed en la migración garantiza disponibilidad desde el inicio sin lógica especial en la aplicación

**Alternativas consideradas**:
- **Hardcoded en Go**: Más simple pero no consultable via SQL, no versionable con migraciones, requeriría lógica especial para mezclar con custom permisos en la respuesta
- **Tabla separada `system_permissions`**: Más compleja, requiere UNION en queries, sin ventaja real dado que el flag ya los diferencia

---

## Decisión 2: Formato de IDs para permisos

**Decisión**: Dos estrategias según tipo:
- **Permisos de sistema**: IDs con prefijo fijo (`perm_dashboard`, `perm_alerts`, etc.) — exactamente como los define el contrato Pact
- **Permisos custom**: UUID v4 generado por la BD (`gen_random_uuid()`) — consistente con el resto de entidades del proyecto

**Rationale**: 
- Los IDs de sistema deben coincidir exactamente con el contrato Pact (los tests verifican `perm_dashboard`, etc.)
- UUID para custom evita colisiones y es el patrón estándar del proyecto
- La BD maneja la generación automática igual que `alarm_rules`, `roles`, etc.

**Alternativas consideradas**:
- **UUID para todos**: No compatible con el Pact que espera `perm_dashboard` como ID de sistema
- **Prefijo `perm_custom_` + timestamp**: El Pact lo usa como ejemplo ilustrativo (`perm_custom_1709380000000`) pero UUID es más robusto

---

## Decisión 3: Aislamiento multi-tenant para permisos custom

**Decisión**: Las queries de listado usan `WHERE (tenant_id = $1 OR is_system_permission = TRUE)`, exactamente el mismo patrón que `roles.List()`:

```sql
SELECT id, name, section, description, is_system_permission, tenant_id, created_at, updated_at
FROM permissions
WHERE (tenant_id = $1 OR is_system_permission = TRUE)
  AND deleted_at IS NULL
ORDER BY is_system_permission DESC, name ASC
```

**Rationale**: Patrón ya validado en el proyecto (roles usa `is_global = TRUE OR tenant_id = $1`). Garantiza que permisos custom de un tenant jamás sean visibles para otro.

---

## Decisión 4: Soft-delete vs hard-delete para permisos custom

**Decisión**: **Hard-delete** para permisos custom. Sin columna `deleted_at`.

**Rationale**: 
- La spec FR-007 lo especifica explícitamente: "eliminar de forma permanente (no soft-delete)"
- Consistente con `alarm_rules` que también usa hard-delete
- Los permisos custom no tienen auditoría requerida en este scope MVP

**Alternativas consideradas**:
- **Soft-delete**: Permite auditaría pero agrega complejidad. El contrato Pact no exige ningún comportamiento post-eliminación que requiera soft-delete.

---

## Decisión 5: Validación de nombre mínimo 3 caracteres

**Decisión**: Validación en la capa de servicio (app layer), no en la BD. Error retornado con formato `{"error": "Validation failed", "errors": [{"path": "name", "message": "name must be at least 3 characters"}]}`.

**Rationale**: 
- El Pact especifica exactamente ese error en el body de la respuesta 400
- La validación en servicio es testeable unitariamente
- Consistente con el patrón de validación de otros handlers del proyecto

---

## Decisión 6: Endpoint de permisos en la superficie ABM

**Decisión**: Los endpoints se montan en `/api/v1/permissions` bajo la cadena de middleware JWTAuth + TenantFromHeader + PasswordChangeGuard. La restricción de solo admins para mutaciones se implementa via `RBACCheck`.

**Rationale**: 
- Consistente con todos los otros endpoints de la superficie ABM
- El contrato Pact usa `Bearer valid-token` → requiere JWTAuth
- FR-013 indica que solo admin puede crear/modificar/eliminar

---

## Decisión 7: Estructura de código

**Decisión**: Seguir exactamente el patrón establecido en `roles/`:

```
internal/
  domain/permissions.go            — entidad Permission, errores de dominio
  app/permissions/service.go       — lógica de negocio
  repo/pg/permissions/repository.go — acceso a BD
  api/handler/permissions/handler.go — HTTP handlers + DTOs
```

**Rationale**: El proyecto tiene un patrón consolidado de hexagonal layout. Desviarse sin justificación violaría la Constitución (Principio I).

---

## Decisión 8: Número de migración

**Decisión**: `000017_create_permissions_table` (siguiente disponible después de `000016_create_notifications_table`).

**Rationale**: Numeración secuencial estricta del proyecto.

---

## Sin NEEDS CLARIFICATION pendientes

Todas las ambigüedades de la spec están resueltas por contexto del proyecto o por el contrato Pact:
- IDs de permisos de sistema: definidos en el Pact
- Formato de error 400: definido en el Pact
- Patrón multi-tenant: establecido por roles (precedente en el mismo proyecto)
- Hard-delete: especificado en FR-007
