# Research: Alarm Rules Service API (008)

**Fecha**: 2026-04-06  
**Branch**: `008-alarm-rules`

## Decisiones Técnicas

### 1. Estructura de tabla — sin soft-delete

**Decision**: Eliminación permanente (DELETE FROM). No se agrega `deleted_at`.  
**Rationale**: La spec lo indica explícitamente ("no se implementa soft-delete en MVP"). Las reglas de alarma son configuración pura; no hay auditoría requerida sobre reglas eliminadas en este ciclo.  
**Alternatives considered**: Soft-delete con `deleted_at` (patrón usado en roles, invitaciones). Rechazado porque agrega complejidad innecesaria sin beneficio claro en MVP.

### 2. PATCH vs PUT para actualización

**Decision**: `PATCH` parcial — solo se actualizan los campos presentes en el body.  
**Rationale**: Los contratos Pact del frontend especifican `PATCH /api/alarm-rules/{id}`. Además, las reglas pueden tener muchos campos y un PATCH reduce el payload necesario.  
**Alternatives considered**: PUT completo (requiere todos los campos). Más simple de implementar pero obliga al cliente a conocer y reenviar el estado completo.

### 3. Validación de `operator` y `severity`

**Decision**: Validación en la capa de servicio de aplicación con constantes en domain.  
**Rationale**: Mantiene consistencia con el patrón de features anteriores. Las reglas de negocio viven en domain/app, no en el handler.  
**Values**:
- Operators: `gt`, `lt`, `gte`, `lte`, `eq`
- Severities: `info`, `warning`, `critical`

### 4. ID de regla — UUID

**Decision**: UUID v4 generado por la base de datos al crear (`DEFAULT gen_random_uuid()`) y recuperado desde el `INSERT ... RETURNING id`.  
**Rationale**: Consistente con todas las demás entidades del sistema (users, tenants, roles, edge devices, dashboard layouts).  
**Alternatives considered**: Generar UUID en capa de aplicación (válido) → no implementado. ID secuencial (más simple) → rechazado por inconsistencia y exposición de volumen.

### 5. Desajuste de prefijo Pact `/api/alarm-rules` vs backend `/api/v1/alarm-rules`

**Decision**: El backend expone bajo `/api/v1/alarm-rules`. El frontend debe tener configurado `basePath=/api/v1`.  
**Rationale**: El mismo desajuste existe en `user-service-api` y `tenant-service-api` (documentado en PACTS_ANALYSIS.md). La solución es configuración del frontend, no del backend.  
**Action**: Documentar en quickstart.md y OpenAPI spec.

### 6. RBAC — permisos requeridos

**Decision**: Operaciones de escritura (POST, PATCH, DELETE) requieren permiso `users:write` (reutilizando el permiso de admin existente).  
**Rationale**: No hay un permiso `alarm-rules:write` definido aún en el sistema RBAC estático. Usar `users:write` (que poseen admins) es consistente con cómo 006-roles maneja sus writes.  
**Note**: Cuando se implemente `permissions-service-api` (feature futura), se podrá granularizar con `alarm-rules:write`.

### 7. Threshold como NUMERIC en PostgreSQL

**Decision**: `NUMERIC(15,4)` para el valor umbral.  
**Rationale**: Las métricas industriales pueden ser temperaturas (ej: 85.5), presiones (ej: 1013.25), conteos (ej: 1000). NUMERIC evita errores de punto flotante.  
**Alternatives considered**: `FLOAT8` — rechazado por imprecisión en comparaciones de igualdad.

### 8. Paginación — omitida en MVP

**Decision**: `GET /alarm-rules` devuelve la lista completa sin paginación.  
**Rationale**: Conforme a la spec (Assumptions: "pocos rules por tenant"). Los contratos Pact no incluyen parámetros de paginación para este servicio.
