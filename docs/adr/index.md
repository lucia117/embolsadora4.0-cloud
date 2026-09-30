---
title: Índice de ADRs — embolsadora4.0-cloud
status: vigente
owner: Lucia Scharff
last_reviewed: 2026-09-29
---

# Architecture Decision Records

Un archivo por decisión, formato MADR, con prefijo `CLOUD-`. Un ADR aceptado **no se
edita**: se crea uno nuevo que lo reemplaza, y el anterior pasa a `status: reemplazada`
con `superseded_by` en el front-matter. Plantilla: [`_template.md`](_template.md).

Merece ADR una decisión difícil de revertir que afecte contratos, persistencia, seguridad
o despliegue. No merecen ADR: elección de librerías de lint o UI, naming, detalles
internos reversibles en un PR, tareas y bugs.

| ID | Decisión | Estado | Fecha | Relación |
|---|---|---|---|---|
| [CLOUD-ADR-001](CLOUD-ADR-001-monolito-modular-superficies.md) | Monolito modular con superficies separadas (ABM vs Consumers) | aceptada | 2025-10-17 | — |
| [CLOUD-ADR-002](CLOUD-ADR-002-tenant-desde-jwt-y-api-key.md) | Tenant desde claim del JWT y API key | ↻ reemplazada | 2025-10-17 | → 016 |
| [CLOUD-ADR-003](CLOUD-ADR-003-eventos-en-postgres.md) | Eventos en Postgres (JSONB, particionado, 90 días) | ↻ reemplazada | 2025-10-17 | → 017 |
| [CLOUD-ADR-004](CLOUD-ADR-004-ingesta-http-batch.md) | Ingesta HTTP batch con idempotencia y rate limit | ↻ reemplazada | 2025-10-17 | → 018 |
| [CLOUD-ADR-005](CLOUD-ADR-005-supabase-auth.md) | Reemplazar el auth propio con Supabase Auth | aceptada | 2026-03-07 | — |
| [CLOUD-ADR-014](CLOUD-ADR-014-consolidar-migraciones.md) | Consolidación de migraciones para el primer deploy | aceptada | 2026-05-08 | — |
| [CLOUD-ADR-015](CLOUD-ADR-015-plataforma-cross-tenant.md) | Acceso cross-tenant para operadores del tenant plataforma | aceptada | 2026-07-14 | — |
| [CLOUD-ADR-016](CLOUD-ADR-016-resolucion-de-tenant.md) | Resolución de tenant por header, path y API key | aceptada | 2026-09-29 | reemplaza 002 |
| [CLOUD-ADR-017](CLOUD-ADR-017-mediciones-en-mongodb.md) | Mediciones en MongoDB; identidad en Postgres | aceptada | 2026-09-29 | reemplaza 003 |
| [CLOUD-ADR-018](CLOUD-ADR-018-ingesta-http-batch.md) | Ingesta HTTP batch con contrato congelado del Edge | aceptada | 2026-09-29 | reemplaza 004 |
| [CLOUD-ADR-019](CLOUD-ADR-019-aas-fuera-del-cloud.md) | El AAS no se implementa en el cloud | aceptada | 2026-09-29 | — |

## Notas de numeración y renombres

- **006 a 013 no se usaron.** El número 014 viene de la spec `014-consolidate-migrations`
  y 015 le siguió. Los huecos se dejan porque código, specs y PRs citan los números
  existentes (por ejemplo, `ADR-015` en `internal/api/middleware`).
- **CLOUD-ADR-005 se llamaba "ADR 002"** (`002-replace-auth-system.md`) y colisionaba con
  `ADR-002.md`. Tomó el primer número libre, que además respeta el orden cronológico. Su
  título interno sigue diciendo "ADR 002" porque el cuerpo de un ADR no se edita.
- El campo `former_file` del front-matter guarda el nombre anterior de cada archivo
  (renombrados el 2026-09-29).

## Notas de vigencia

Detalles desactualizados que no cambian la decisión, así que no justifican un ADR nuevo:

- **CLOUD-ADR-014** habla de "deploy en Koyeb". El deploy real es Google Cloud Run
  (`docs/operations.md`). La decisión (consolidar migraciones) sigue vigente, y la
  plataforma de deploy no tiene ADR propio.
- **CLOUD-ADR-005** dice que los JWT se validan con RS256. El proyecto de Supabase firma hoy
  con **ES256** (clave EC P-256, JWKS público); el verificador acepta ES256 y RS256 (issue
  #94). La decisión (Supabase Auth, firma asimétrica por JWKS) no cambió.
- **CLOUD-ADR-015** nombra permisos `tenants:write`/`users:write` y dice que
  `platform_admin` no está en el catálogo de `roles`. Desde la migración `000011` los
  permisos son `perm_tenants_manage`/`perm_users_manage` y `platform_admin` es una fila de
  `roles`. La regla de acceso cross-tenant no cambió.

## Candidatos a ADR

Decisiones abiertas con issue: generación del OpenAPI con swag (#100) y versionado con
release-please frente a la constitution (#101).

- Deploy en Google Cloud Run con Workload Identity Federation (reemplazó a Koyeb; la razón
  no está documentada).
- Permisos dinámicos leídos de `roles.permissions` en cada request (migración `000011`).
