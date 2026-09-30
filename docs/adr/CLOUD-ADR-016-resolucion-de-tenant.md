---
id: CLOUD-ADR-016
title: "Resolución de tenant por header, path y API key según la superficie"
status: aceptada
date: 2026-09-29
owner: Lucia Scharff
last_reviewed: 2026-09-29
supersedes: [CLOUD-ADR-002]
superseded_by: []
---

# CLOUD-ADR-016 — Resolución de tenant por header, path y API key según la superficie

> ADR retrospectivo: documenta una decisión que ya está implementada. Cada afirmación
> sobre el porqué indica su fuente; donde no hay fuente escrita se dice
> "Razón no documentada".

## Contexto

[CLOUD-ADR-002](CLOUD-ADR-002-tenant-desde-jwt-y-api-key.md) (2025-10-17) decidió que en
la superficie ABM el tenant saliera **exclusivamente** del claim `tenant_id` del JWT, que
se ignorara cualquier header `X-Tenant-Id` y que el acceso de soporte usara un header
`X-Act-As-Tenant`.

Nada de eso se implementó así:

- El JWT lo emite Supabase Auth desde la migración a Supabase
  ([CLOUD-ADR-005](CLOUD-ADR-005-supabase-auth.md), 2026-03-07). El middleware no lee
  ningún claim `tenant_id`.
- CLOUD-ADR-005 declara como consecuencia "Nuevo header requerido: `X-Tenant-ID` en todos
  los endpoints `/api/v1/*` (excepto `/me` y `/auth/change-password`)".
- Un usuario puede tener membresías en varios tenants (`user_tenant_roles`), así que el
  tenant de un request no es un atributo fijo del usuario. **[I]** Es la razón más
  probable para elegir el tenant por request y no por token; no está escrita.
- `X-Act-As-Tenant` no existe en el código. El acceso de soporte lo resolvió
  [CLOUD-ADR-015](CLOUD-ADR-015-plataforma-cross-tenant.md).

## Decisión

El tenant se resuelve distinto en cada superficie, y **siempre se valida en el servidor**
contra datos de Postgres. Nunca se confía en el valor que manda el cliente.

| Superficie | Fuente del tenant | Validación | Código |
|---|---|---|---|
| ABM `/api/v1/**` | Header `X-Tenant-ID` (UUID, se normaliza a minúsculas) | Membresía activa en `user_tenant_roles` o fallback de operador de plataforma (CLOUD-ADR-015). Sin header → 400; UUID inválido → 400; tenant inexistente → 404; sin acceso → 403 | `TenantFromHeader` (`internal/api/middleware/middleware.go`) |
| Edge devices `/api/v1/tenants/:tenantId/**` | Path, con el **subdominio** del tenant (no el UUID) | Mismo criterio de membresía y fallback | `ResolveTenantAndCheckMembership` (`resolve_tenant_path.go`) |
| Ingesta `/api/v1/consumers/**` | La API key (`X-Api-Key`), resuelta en el servidor a tenant y device | Key activa en `edge_device_api_keys`; el body no puede fijar el tenant | `APIKeyAuth`, `security.APIKeyAuthenticator` |
| `/me`, `/auth/change-password` | Ninguna | — | `isExemptFromTenant` |

Por qué cada fuente:

- **Header en ABM:** consecuencia de la migración a Supabase (CLOUD-ADR-005). El resto del
  razonamiento no está documentado; ver la inferencia **[I]** del contexto.
- **Subdominio en el path de edge devices:** el contrato con el frontend (pact) usa el
  slug en todas las interacciones, y así no se exponen UUIDs internos en las URLs
  (`specs/003-edge-device-management/research.md`, decisiones 1 y 2).
- **API key en la ingesta:** un Pi comprometido no puede escribir en nombre de otra
  máquina ni de otro tenant (D-10 del diseño de la ingesta,
  `embolsadora-edge/docs/superpowers/specs/2026-07-30-cloud-ingest-endpoint-design.md`).

## Alternativas consideradas

1. **Tenant en un claim del JWT** (CLOUD-ADR-002). Se abandonó con la migración a
   Supabase. Razón no documentada más allá de lo dicho en el contexto.
2. **UUID en el path de edge devices.** Se descartó para respetar el pact existente
   (`research.md` de la spec 003).

## Consecuencias

- El aislamiento depende de que **toda** ruta con datos de tenant pase por uno de los tres
  middlewares. Una ruta registrada fuera de esos grupos no tiene tenant en el contexto.
- Hay dos formatos de tenant en la API (UUID en el header, slug en el path de edge
  devices). El cliente tiene que conocer los dos. Hoy `docs/openapi.yaml` no documenta
  `X-Tenant-ID` de forma uniforme en todas las rutas; se corrige con el drift check de la
  fase 4 del harness.
- El header se valida en cada request con una consulta a Postgres (membresía y permisos
  del rol), sin caché.
- Las invariantes de CLOUD-ADR-002 que siguen vigentes pasan a este ADR: en la ingesta el
  tenant sale de la API key y el cliente no puede fijarlo; los repositorios reciben el
  tenant por `context.Context`.
