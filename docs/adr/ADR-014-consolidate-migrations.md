# ADR-014: Consolidación de migraciones para deploy en Koyeb

**Status**: Accepted
**Date**: 2026-05-08
**Branch**: `014-consolidate-migrations`
**Spec**: [`specs/014-consolidate-migrations/spec.md`](../../specs/014-consolidate-migrations/spec.md)

## Context

El repo acumulaba 20 migraciones (`000001`–`000020`) con dos archivos compartiendo el prefijo `000019` (`add_global_roles_and_mrg_tenant` y `seed_mrg_platform_tenant`), lo que rompía cualquier intento de aplicarlas desde cero con `golang-migrate`. Adicionalmente, la migración `000007` usaba `CREATE TABLE IF NOT EXISTS users` sobre la tabla ya creada en `000001`, dejando la cadena incapaz de aplicarse contra una DB vacía (los índices posteriores referenciaban columnas que el `IF NOT EXISTS` silenciosamente nunca agregaba).

Se necesitaba un primer deploy a Koyeb Managed Postgres reproducible y verificable. La base de datos de producción no tenía datos preexistentes, así que la opción más simple era colapsar el historial.

## Decision

Reemplazar las 20 migraciones por dos:

- **`000001_initial_schema.up/down.sql`** — DDL final, generado vía `pg_dump --schema-only` contra una base intermedia donde se aplicó la cadena histórica completa (con un parche local en `000007` para que la cadena fuera aplicable). El `down` hace `DROP TABLE IF EXISTS … CASCADE` en orden inverso a las dependencias, preservando `schema_migrations`.
- **`000002_seed_essentials.up/down.sql`** — Catálogo de 17 permisos del sistema, 6 roles (2 globales + 4 tenant-scoped), y el tenant MRG (`11b36b85-033d-4bb3-9e31-4c92161887c0`). Todos los inserts son idempotentes (`ON CONFLICT … DO NOTHING`).

Los seeds de tenants de prueba (Córdoba, Mendoza, Rosario) salen del flujo de migraciones y viven en `scripts/seed_test_city_tenants.sql`, ejecutables manualmente con `psql -f` solo en entornos no productivos. El admin MRG NO se inserta en el seed: su UUID viene de Supabase Auth post-deploy y el middleware existente (`internal/api/usecases/auth_usecase.go::ProvisionUser`) lo provisiona automáticamente en el primer login.

## Consequences

### Positivas

- Deploy a Koyeb es un único `migrate up` (~3.4s, muy por debajo de los 30s de SC-001).
- Conflicto de prefijo `000019` resuelto.
- Cualquier desarrollador nuevo levanta el esquema completo sin pasos manuales.
- 40+ archivos en `migrations/` reducidos a 4 (95% de reducción, supera SC-004).

### Negativas / trade-offs

- Pérdida de granularidad histórica del esquema en `migrations/` (mitigado: `git log` y `git show HEAD~N:migrations/…` retienen el historial completo).
- Cualquier entorno preexistente con migraciones aplicadas necesita `migrate force 2` antes de un `up` (no aplica a producción Koyeb por la asunción de DB recreable).
- El parche de `000007` aplicado en staging (no commiteado) queda sin trazabilidad formal — está documentado aquí y en `specs/014-consolidate-migrations/quickstart.md`.

## Verification

- `migrate up` sobre Postgres 16 vacío → 14 tablas, `schema_migrations.version=2 dirty=f`.
- `migrate down -all` deja el esquema vacío (solo `schema_migrations`) sin errores.
- Re-aplicación `up` después de `down` → estado limpio (idempotencia verificada).
- Sin script opcional, `SELECT subdomain FROM tenants` retorna solo `mrgsrl` (cumple SC-006).
- Cero archivos `*.go` modificados (FR-008).

## References

- Spec: `specs/014-consolidate-migrations/spec.md`
- Plan: `specs/014-consolidate-migrations/plan.md`
- Research: `specs/014-consolidate-migrations/research.md`
- Quickstart (procedimiento reproducible): `specs/014-consolidate-migrations/quickstart.md`
