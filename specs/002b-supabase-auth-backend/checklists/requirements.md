# Specification Quality Checklist: Supabase Auth — Backend

**Purpose**: Validate specification completeness and quality before proceeding to planning
**Created**: 2026-03-06
**Feature**: [spec.md](../spec.md)

## Content Quality

- [x] No implementation details (languages, frameworks, APIs)
- [x] Focused on user value and business needs
- [x] Written for non-technical stakeholders
- [x] All mandatory sections completed

## Requirement Completeness

- [x] No [NEEDS CLARIFICATION] markers remain
- [x] Requirements are testable and unambiguous
- [x] Success criteria are measurable
- [x] Success criteria are technology-agnostic (no implementation details)
- [x] All acceptance scenarios are defined
- [x] Edge cases are identified
- [x] Scope is clearly bounded
- [x] Dependencies and assumptions identified

## Feature Readiness

- [x] All functional requirements have clear acceptance criteria
- [x] User scenarios cover primary flows
- [x] Feature meets measurable outcomes defined in Success Criteria
- [x] No implementation details leak into specification

## Decisiones documentadas

- **Opción A confirmada**: Se elimina `internal/auth/` completo, tablas `sessions`, `password_reset_tokens`, y campo `password_hash`.
- **Deployment**: Koyeb (Go API) + Neon (PostgreSQL) + Upstash (Redis) + Supabase Cloud.
- **Self-hosted path**: Solo env vars, sin cambio de código (FR-001, SC-005).
- **Auto-provisioning**: Primer request de usuario nuevo crea el registro en `users` de forma idempotente (US2, FR-004, edge case de concurrencia).
- **Tenant scope**: El backend resuelve el tenant desde `X-Tenant-ID` header (enviado por el frontend), NO desde el JWT de Supabase. Validado contra `user_tenant_roles` (US7, FR-013).
- **Invitaciones**: `POST /api/v1/invitations` llama a Supabase Admin API (`inviteUserByEmail`) con `redirect_to: {APP_BASE_URL}/s/{tenantId}/auth/callback`. Requiere `APP_BASE_URL` y `SUPABASE_SERVICE_ROLE_KEY` como env vars (US5, FR-015).
- **Force password change**: Admin llama a `POST /api/v1/users/:id/force-password-change`. Backend setea `password_change_required = true` y delega el envío del email de reset a Supabase Admin API (US8, FR-014, FR-016).
- **super_admin global**: Fuera de scope de esta feature.

## Listo para `/speckit.plan`
