# Implementation Plan: Consolidación de migraciones para deploy en Koyeb

**Branch**: `014-consolidate-migrations` | **Date**: 2026-05-08 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/014-consolidate-migrations/spec.md`

## Summary

Reemplazar las 20 migraciones históricas (`000001`–`000020`, con conflicto de prefijo en `000019`) por **dos migraciones limpias**: una con el esquema final (`000001_initial_schema`) y otra con los seeds esenciales para producción (`000002_seed_essentials`: roles, permisos, tenant MRG, usuario admin). Los seeds de tenants de prueba (ciudades) salen del flujo de migraciones y quedan como un script SQL independiente bajo `scripts/`. La migración inicial se construye haciendo `pg_dump --schema-only` de una DB intermedia donde se aplicó la cadena histórica completa, garantizando equivalencia funcional. La verificación end-to-end es: `migrate up` sobre Postgres vacío → arrancar `cmd/api` → `go test ./...` verde + login del admin MRG retorna 200 en `/api/v1/me`.

## Technical Context

**Language/Version**: Go 1.24+ (sin cambios de código en este feature; solo SQL y docs).
**Primary Dependencies**: `golang-migrate` (CLI v4.19.x) para versionado de migraciones; `psql` y `pg_dump` (postgres-client 16) para extracción del esquema.
**Storage**: PostgreSQL 16 (compatible con Koyeb Managed Postgres). Esquema único `public`.
**Testing**: `go test ./...` (suite existente, sin cambios); test manual de migración aplicando `migrate up` + `migrate down` + `migrate up` sobre contenedor Postgres efímero; smoke test con `curl /api/v1/me` autenticado como admin MRG.
**Target Platform**: Koyeb Managed Postgres (producción) + Docker Compose Postgres (local/CI).
**Project Type**: Backend monolito modular (cambio limitado a `migrations/`, `scripts/`, `migrations/README.md`).
**Performance Goals**: Aplicación completa de esquema + seeds esenciales sobre DB vacía en < 30 s (SC-001).
**Constraints**:
- No se permiten cambios en código Go (FR-008).
- Idempotencia de seeds (FR-003 + edge case): usar `ON CONFLICT DO NOTHING` o `INSERT … WHERE NOT EXISTS`.
- Credencial inicial del admin MRG no puede ir hardcodeada en el repo (FR-007): el seed crea el usuario en estado `pending_invitation` (sin password local) o usa una variable `MRG_ADMIN_PASSWORD_HASH` resuelta en deploy time.
- El conflicto de prefijo `000019` debe quedar eliminado del repo (FR-005).
**Scale/Scope**: ~15 tablas en el modelo final, ~30 permisos, 8 roles base, 1 tenant MRG, 1 usuario admin MRG.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Constitución: `.specify/memory/constitution.md` v1.1.0.

| Principio | Aplicabilidad | Evaluación |
|-----------|---------------|-----------|
| I. Arquitectura hexagonal limpia | Solo aplica si tocamos `transport/app/domain/repo`. **No tocamos código Go.** | ✅ N/A |
| II. Aislamiento de tenants (NO NEGOCIABLE) | El esquema consolidado debe preservar todas las constraints de `tenant_id` que existen hoy. | ✅ Garantizado por `pg_dump` del estado actual: no se introducen ni eliminan FK ni constraints. |
| III. Observabilidad (NO NEGOCIABLE) | Aplica al código de la app, no al SQL. | ✅ N/A |
| IV. Testing de integración dirigido por contrato | "Cambios de schema de base de datos deben incluir tests de migración contra contenedores Postgres de test." | ⚠️ **Aplica.** Mitigación: el `quickstart.md` define un smoke test reproducible (`docker run postgres:16` + `migrate up` + `migrate down` + `migrate up` + `go test ./...`). No introducimos tests automáticos nuevos en CI por estar fuera del scope (Out of Scope §spec), pero el smoke test queda documentado y será obligatorio antes de mergear. |
| V. Versionado semántico | Cambio de schema es ruptura para entornos preexistentes. | ✅ La spec asume DB de prod recreable (Assumption confirmada). El cambio NO afecta la versión semver de la API (no cambian endpoints ni contratos). No requiere bump MAJOR. |
| Requerimiento de ADR | "migraciones de schema requieren ADRs". | ⚠️ **Aplica.** Se agregará `docs/adr/ADR-014-consolidate-migrations.md` documentando el por qué (conflicto `000019`, simplificación deploy Koyeb, equivalencia validada por `pg_dump`). |

**Resultado**: ✅ Pasa con dos compromisos documentados (smoke test obligatorio en quickstart + ADR-014). Ninguna violación que requiera Complexity Tracking.

## Project Structure

### Documentation (this feature)

```text
specs/014-consolidate-migrations/
├── plan.md              # Este archivo
├── research.md          # Phase 0 — decisiones técnicas resueltas
├── data-model.md        # Phase 1 — modelo de datos consolidado (entidades + relaciones)
├── quickstart.md        # Phase 1 — pasos reproducibles para aplicar a Koyeb y verificar
├── contracts/           # Vacío — feature sin contratos HTTP/CLI nuevos
└── checklists/
    └── requirements.md  # Generado en /speckit.specify
```

### Source Code (repository root)

```text
migrations/
├── 000001_initial_schema.up.sql     # NUEVO — DDL completo (pg_dump --schema-only)
├── 000001_initial_schema.down.sql   # NUEVO — DROP SCHEMA public CASCADE; CREATE SCHEMA public;
├── 000002_seed_essentials.up.sql    # NUEVO — RBAC + tenant MRG + admin MRG
├── 000002_seed_essentials.down.sql  # NUEVO — DELETE en orden inverso, idempotente
├── README.md                        # ACTUALIZADO — flujo Koyeb, comando ejemplo
└── (todos los 000001-000020 anteriores: ELIMINADOS)

scripts/
├── seed_test_city_tenants.sql       # NUEVO — consolida 000020 + seed_city_tenants_users.sql
├── seed_mrg_users.sql               # ELIMINADO (su contenido pasa a 000002)
└── seed_city_tenants_users.sql      # ELIMINADO (consolidado en seed_test_city_tenants.sql)

docs/adr/
└── ADR-014-consolidate-migrations.md  # NUEVO — registro de la decisión

CLAUDE.md
└── Sección "Pending Manual Steps" — ACTUALIZADA: removidos T008/T034/013, agregado paso "aplicar 000001+000002 contra Koyeb post-deploy"
```

**Structure Decision**: Cambio quirúrgico, contenido en `migrations/`, `scripts/`, `docs/adr/` y un párrafo de `CLAUDE.md`. **Cero archivos `.go` modificados.** No se introducen nuevas dependencias ni se mueven directorios existentes.

## Complexity Tracking

> No hay violaciones constitucionales que justificar. Sección omitida intencionalmente.

## Phase 0 Output

Ver [`research.md`](./research.md). Resuelve:

1. Cómo construir el esquema consolidado de forma verificable (decisión: `pg_dump --schema-only` de una DB intermedia con el historial completo aplicado).
2. Cómo entregar la credencial del admin MRG sin filtrarla (decisión: usuario en `status='pending_invitation'`, password se setea vía flujo de invitación de Supabase post-deploy).
3. Cómo resolver el conflicto de prefijo `000019` (decisión: ambos archivos viejos se eliminan; su contenido se reincorpora en `000001` (DDL de roles globales) y `000002` (INSERT del tenant MRG)).
4. Cómo organizar los seeds de tests sin contaminar prod (decisión: archivo único `scripts/seed_test_city_tenants.sql`, ejecución manual `psql -f`).
5. Comando concreto para Koyeb (decisión: `migrate -path migrations/ -database "$KOYEB_DATABASE_URL?sslmode=require" up`).

## Phase 1 Output

- [`data-model.md`](./data-model.md): Lista de entidades del esquema final con campos clave, relaciones y constraints de aislamiento por tenant. No introduce entidades nuevas — refleja el estado tras 000001-000020.
- [`quickstart.md`](./quickstart.md): Pasos numerados para (a) regenerar `000001_initial_schema.up.sql` desde el historial actual, (b) aplicar las dos migraciones a una DB local de prueba, (c) validar con `migrate down/up` y `go test ./...`, (d) aplicar a Koyeb prod.
- `contracts/`: vacío. Este feature no cambia ningún contrato externo (HTTP, CLI, eventos).
- **Agent context update**: omitido. El script `.specify/scripts/bash/update-agent-context.sh` no existe en este repo (solo está la versión PowerShell, incompatible con macOS). Además, el feature no introduce stack nuevo — `CLAUDE.md` ya documenta `golang-migrate`, Postgres y la convención de `migrations/`. Único ajuste manual aplicado en Phase 2: limpiar referencias obsoletas a migraciones específicas en la sección "Pending Manual Steps" de `CLAUDE.md`.

## Re-evaluation: Constitution Check (post-design)

Re-revisado tras Phase 1: ningún output de Phase 1 modifica la evaluación previa. Compromisos siguen vigentes (smoke test en quickstart + ADR-014). ✅ Sigue pasando.
