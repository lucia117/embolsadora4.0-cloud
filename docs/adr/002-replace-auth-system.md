# ADR 002: Reemplazar sistema de auth propio con Supabase Auth

**Date**: 2026-03-07
**Status**: Accepted
**Deciders**: Equipo backend

---

## Context

El sistema actual implementa auth propio en `internal/auth/`: sesiones en DB, password hashing con bcrypt, JWT HS256 con secret compartido, y password reset tokens. Este sistema requiere mantenimiento continuo, es propenso a errores de seguridad y duplica funcionalidad que Supabase Cloud ya provee de forma robusta.

## Decision

Reemplazar el sistema de auth propio por Supabase Auth Cloud:

- **JWT validation**: RS256 via JWKS endpoint público de Supabase (sin secret compartido)
- **Session management**: Supabase gestiona sesiones; el backend solo valida tokens JWT
- **Invitations**: Supabase Admin API (`POST /auth/v1/admin/invite`)
- **Password reset**: Supabase Admin API (`POST /auth/v1/admin/generate-link` con `type:"recovery"`)
- **Auto-provisioning**: El primer request con token válido crea el registro en `users` localmente (idempotente via UPSERT)

## Consequences

### Breaking Changes (MAJOR — v2.0.0)
- Eliminación de `internal/auth/` (handlers, service, repository, models, routes)
- Eliminación de `POST /api/auth/login`, `POST /api/auth/logout`, `POST /api/auth/refresh`
- Eliminación de columna `password_hash` en `users`
- Eliminación de tablas `sessions` y `password_reset_tokens`
- Nuevo header requerido: `X-Tenant-ID` en todos los endpoints `/api/v1/*` (excepto `/me` y `/auth/change-password`)

### New Capabilities
- Soporte OAuth (Google, GitHub) sin código adicional en el backend
- Rotación automática de claves JWT via JWKS
- Flujo de invitaciones end-to-end con email gestionado por Supabase
- `password_change_required` flag para forzar cambio de credenciales

### Libraries Added
- `github.com/MicahParks/keyfunc/v3`: JWKS cache + auto-refresh para validación JWT RS256

### Environment Variables Required
- `SUPABASE_JWKS_URL`: URL del endpoint JWKS de Supabase
- `SUPABASE_JWT_ISSUER`: Issuer esperado en los JWT (`https://{project}.supabase.co/auth/v1`)
- `SUPABASE_JWT_AUDIENCE`: Audience esperado (`authenticated`)
- `SUPABASE_URL`: Base URL del proyecto Supabase
- `SUPABASE_SERVICE_ROLE_KEY`: Service role key para Admin API
- `APP_BASE_URL`: Base URL del frontend (para `redirect_to` en invitaciones)
- `INVITATION_RATE_LIMIT_PER_HOUR`: Máximo de invitaciones por tenant por hora (default: 20)
