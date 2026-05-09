# Phase 0 — Research: Consolidación de migraciones para Koyeb

Decisiones tomadas para resolver los puntos abiertos del Technical Context. Cada bloque lleva: **Decisión / Rationale / Alternativas consideradas**.

---

## R1. Cómo generar el esquema consolidado de forma verificable

**Decisión**: Construir `migrations/000001_initial_schema.up.sql` haciendo `pg_dump --schema-only --no-owner --no-privileges` contra una base de datos PostgreSQL 16 efímera donde se aplicaron en orden las 20 migraciones actuales (`000001`–`000020` con ambos archivos `000019`). Limpiar el dump para que sea idempotente y compatible con `golang-migrate`: remover `SET` de owner, comentarios de `pg_dump`, líneas `CREATE EXTENSION` que duplican defaults de Postgres, y dejar `CREATE EXTENSION IF NOT EXISTS` para `pgcrypto` / `uuid-ossp` si están en uso.

**Rationale**:
- Garantiza equivalencia funcional 1:1 con el estado actual (FR-001) sin riesgo de transcripción manual.
- `--schema-only` separa DDL de datos, lo que se alinea con la separación esquema/seeds del spec.
- Reproducible y auditable: el procedimiento se documenta en `quickstart.md` y cualquiera puede regenerar el dump y diff-earlo.

**Alternativas consideradas**:
- **Reescribir el SQL a mano consolidando las 20 migraciones**: descartado. Alto riesgo de divergencia (ej: la migración 18 elimina un check que la 1 introdujo; reflejarlo manualmente y olvidar un detalle es probable).
- **Mantener las 20 migraciones tal cual y solo arreglar el conflicto `000019`**: descartado. No cumple el objetivo del usuario ("una sola migración") y deja deuda técnica.
- **Usar `pg_dump` con datos**: descartado. Mezclaría datos de demo (000002) con DDL y haría imposible separar seeds esenciales de los de prueba.

---

## R2. Resolución del conflicto de prefijo `000019`

**Decisión**: Eliminar ambos archivos (`000019_add_global_roles_and_mrg_tenant.up/down.sql` y `000019_seed_mrg_platform_tenant.up/down.sql`). Su contenido queda absorbido:
- DDL de roles globales (columnas/constraints) → cae naturalmente dentro del `pg_dump` que genera `000001_initial_schema`.
- INSERTs del tenant MRG y del usuario admin → reescritos de forma idempotente en `000002_seed_essentials.up.sql`.

**Rationale**: `golang-migrate` exige números únicos. Mantener cualquiera de los dos rompe la otra historia. Como vamos a colapsar todo el historial igualmente, eliminar ambos es consistente.

**Alternativas consideradas**:
- **Renumerar uno a `000019b` o `000021`**: descartado. `golang-migrate` no soporta sufijos no numéricos y renumerar fuerza a editar `schema_migrations` en cualquier DB que ya los tenga aplicados.
- **Squash en un solo `000019_*` combinado**: descartado por el mismo motivo del item anterior — se borran de todos modos al consolidar.

---

## R3. Entrega segura de la credencial inicial del admin MRG

**Decisión** (actualizada post-implementación): El seed `000002` NO inserta ningún usuario. El admin MRG se crea en el dashboard de Supabase post-deploy; el primer request autenticado dispara `auth_usecase.ProvisionUser` que upsertea la fila en `users` (idempotente vía `ON CONFLICT (supabase_user_id)`). La asignación `user_tenant_roles` (admin MRG ↔ tenant MRG ↔ rol `super-admin`) se hace con un `psql` one-shot o un `curl POST /api/v1/invitations` desde un script de bootstrap, documentado en `quickstart.md` Paso 5. Esta variante reemplaza la decisión original de sembrar el usuario con `status='pending_invitation'`; el motivo es evitar cualquier material rotable (incluso UUIDs de admins) en el repo y reusar el flujo de provisión ya existente sin agregar código (FR-008).

**Rationale**:
- Cumple FR-007: cero credenciales en repo.
- Reusa infraestructura existente — no requiere código nuevo (FR-008).
- El primer login fuerza el cambio de password vía el guard `PasswordChangeGuard()` ya implementado.

**Alternativas consideradas**:
- **Hash bcrypt hardcodeado en el seed**: descartado. Aunque sea hash, sigue siendo material rotable que terminaría en `git log`.
- **Variable `MRG_ADMIN_PASSWORD_HASH` resuelta por un script de envoltura que hace `envsubst` antes de `migrate up`**: viable como fallback pero agrega un paso frágil al deploy. Queda como Plan B documentado en `quickstart.md` para entornos sin Supabase configurado.
- **Crear el usuario manualmente vía Supabase Dashboard tras el deploy**: descartado. No reproducible en CI/CD y propenso a olvido.

---

## R4. Idempotencia de los seeds esenciales

**Decisión**: Todos los `INSERT` en `000002_seed_essentials.up.sql` usan `ON CONFLICT (<unique-key>) DO NOTHING`. Para tablas sin unique key natural sobre el seed (ej: relación `user_tenant_roles`), se usa `INSERT … WHERE NOT EXISTS (SELECT 1 FROM …)`.

**Rationale**:
- Permite re-ejecutar el seed sin errores si quedó a medio aplicar (edge case del spec).
- Compatible con `golang-migrate`: la tabla `schema_migrations` evita la doble ejecución en condiciones normales, pero ante un rollback parcial el operador puede forzar `migrate force <v>` y volver a correr.

**Alternativas consideradas**:
- **Asumir ejecución única (sin idempotencia)**: descartado por edge case explícito en el spec.
- **`TRUNCATE … RESTART IDENTITY` antes de insertar**: descartado. Destruye datos en una re-ejecución accidental en un entorno con datos reales (justo lo que queremos evitar).

---

## R5. Organización de los seeds opcionales (tenants de prueba)

**Decisión**: Un único archivo `scripts/seed_test_city_tenants.sql` que consolida el contenido de `migrations/000020_seed_test_city_tenants.up.sql` + `scripts/seed_city_tenants_users.sql`. Se ejecuta manualmente con `psql "$DATABASE_URL" -f scripts/seed_test_city_tenants.sql` en dev/staging. **No** se invoca desde `migrate up`.

**Rationale**:
- Cumple FR-004 y SC-006 (producción no contiene datos de prueba si no se ejecuta el script).
- Mantiene los datos versionados en el repo para QA reproducible.
- Separar de `migrations/` evita el riesgo de aplicarlos por accidente con `migrate up` en prod.

**Alternativas consideradas**:
- **Migración `000003` opcional gateada por una variable**: descartado. `golang-migrate` no soporta migraciones condicionales; un flag interpretado por la app es complejidad innecesaria.
- **Tags Make targets distintos (`make seed-prod` vs `make seed-dev`)**: complementario, no excluyente. Documentado como mejora opcional en `quickstart.md` pero no entra en el alcance.

---

## R6. Comando concreto para aplicar a Koyeb

**Decisión**: Desde una máquina con `migrate` CLI v4.19+ y conectividad a Koyeb:

```bash
export KOYEB_DATABASE_URL="postgres://USER:PASS@HOST:PORT/DB?sslmode=require"
migrate -path migrations/ -database "$KOYEB_DATABASE_URL" up
```

`sslmode=require` es obligatorio en Koyeb Managed Postgres. Verificación post-aplicación:

```bash
psql "$KOYEB_DATABASE_URL" -c "SELECT version FROM schema_migrations;"
# → debe mostrar '2' y dirty='f'
```

**Rationale**: Mantiene coherencia con `migrations/README.md` actual y con `CLAUDE.md`. No introduce herramientas nuevas.

**Alternativas consideradas**:
- **Embeber las migraciones en el binario y aplicarlas al arrancar `cmd/api`**: viable pero invasivo (requiere código Go nuevo) — viola FR-008. Puede ser un follow-up.
- **Job de Koyeb ejecutado vía `koyeb deploy`**: opcional para CI/CD, queda como Out of Scope del spec.

---

## R7. Tests de migración (compromiso constitucional IV)

**Decisión**: Smoke test manual obligatorio antes de mergear, documentado paso a paso en `quickstart.md`:

1. `docker run --rm -d -p 5432:5432 -e POSTGRES_PASSWORD=test --name pg-smoke postgres:16`
2. `migrate -path migrations/ -database "postgres://postgres:test@localhost:5432/postgres?sslmode=disable" up`
3. `migrate … down` → DB queda vacía sin errores
4. `migrate … up` (re-aplicación) → estado limpio
5. `go test ./...` con `DATABASE_URL` apuntando a la DB → verde
6. Levantar `cmd/api`, invitar al admin MRG, completar invitación, `curl /api/v1/me` → 200 con permisos esperados
7. `docker rm -f pg-smoke`

**Rationale**: Cumple el principio IV sin requerir nueva infra de CI. Auditable porque queda en el quickstart.

**Alternativas consideradas**:
- **Test de integración Go nuevo que aplica migraciones y verifica esquema**: ideal pero excede el scope (Out of Scope §spec: "sin cambios de código"). Anotado como follow-up.

---

## Resumen de NEEDS CLARIFICATION resueltos

| # | Pregunta original | Resolución |
|---|-------------------|-----------|
| R1 | ¿Cómo asegurar equivalencia con el historial actual? | `pg_dump --schema-only` de DB intermedia |
| R2 | ¿Qué hacer con el doble `000019`? | Eliminar ambos; contenido absorbido en 000001/000002 |
| R3 | ¿Cómo entregar la credencial admin MRG? | `pending_invitation` + flujo Supabase existente |
| R4 | ¿Cómo hacer seeds idempotentes? | `ON CONFLICT DO NOTHING` / `WHERE NOT EXISTS` |
| R5 | ¿Dónde viven los seeds de prueba? | `scripts/seed_test_city_tenants.sql`, ejecución manual |
| R6 | ¿Comando para Koyeb? | `migrate -path migrations/ -database "$URL?sslmode=require" up` |
| R7 | ¿Cómo cumplir testing constitucional? | Smoke test manual documentado en quickstart |

Sin `NEEDS CLARIFICATION` pendientes.
