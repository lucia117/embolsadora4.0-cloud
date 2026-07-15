# ADR-015: Acceso cross-tenant para operadores del tenant plataforma

**Status**: Accepted
**Date**: 2026-07-14
**Branch**: `fix/frontend-users-tenants-support`
**PR**: [#40](https://github.com/lucia117/embolsadora4.0-cloud/pull/40)

## Context

`TenantFromHeader` exigía una fila activa en `user_tenant_roles` para el tenant del
header `X-Tenant-ID`, sin excepciones. Eso dejaba a los operadores de MRG (el tenant
plataforma, `tenants.is_platform_tenant = TRUE`) sin poder:

- invitar usuarios a un tenant cliente (`POST /invitations` toma el tenant del header),
- invitar al admin de un tenant recién creado (el creador no es miembro),
- listar usuarios/membresías de un tenant cliente,
- editar un tenant cliente (además, RBAC solo daba `tenants:write` a `super_admin`).

La seed (`000002`) define `super_admin` y `tenant_manager` como roles globales
("pueden acceder a múltiples tenants") pero el middleware nunca implementó esa
semántica. El requerimiento de producto pide además que los **admins de MRG** puedan
gestionar tenants.

Esto matiza el Principio II de la constitution ("ninguna operación puede cruzar
límites de tenants"): el aislamiento aplica a los tenants **clientes** entre sí; el
tenant plataforma existe precisamente para operar la plataforma. Este ADR documenta
esa excepción de forma explícita, como exige la constitution para cambios de auth.

## Decision

1. **Fallback de operador de plataforma en `TenantFromHeader`**: si el usuario no
   tiene membresía directa en el tenant destino, se le permite operar solo si tiene
   una membresía activa **en el tenant plataforma** con un rol global o `admin`
   (query con `roles.deleted_at IS NULL`). El tenant destino debe existir (si no:
   404, distinguido del 403 de acceso denegado). Los usuarios de tenants clientes no
   tienen ningún camino a este fallback.
2. **Rol efectivo `platform_admin`** (`internal/security/rbac.go`): permisos de
   `admin` + `tenants:write`. No existe en el catálogo `roles` ni puede asignarse
   vía API: solo lo computa el middleware cuando la membresía `admin` pertenece al
   tenant plataforma. Un `admin` de un tenant cliente conserva exactamente sus
   permisos actuales.
3. **`X-Tenant-ID` se valida como UUID** antes de tocar la base (400 si es inválido).

## Consequences

### Positivas

- Los flujos de plataforma (alta de tenants con invitación de admin, soporte
  cross-tenant, gestión de usuarios de clientes) funcionan sin filas de membresía
  artificiales en cada tenant cliente.
- La semántica "rol global" declarada en la seed pasa a estar implementada y
  auditable en un único punto (`resolvePlatformOperator`).

### Negativas / Riesgos

- Un `admin` del tenant MRG gana `tenants:write` sobre todos los tenants: la
  pertenencia al tenant plataforma se vuelve una frontera de seguridad de primer
  orden. Mitigación: la membresía en MRG solo puede otorgarla quien ya tiene
  `users:write` en MRG, y el trigger de `000004` sigue impidiendo roles globales
  fuera del tenant plataforma.
- El acceso elevado se loguea solo en denegación; queda como mejora futura loguear
  también las concesiones cross-tenant para auditoría.
