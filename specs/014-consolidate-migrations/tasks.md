---
description: "Task list for feature 014-consolidate-migrations"
---

# Tasks: Consolidación de migraciones para deploy en Koyeb

**Input**: Design documents from `/specs/014-consolidate-migrations/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, quickstart.md

**Tests**: La spec define un smoke test manual end-to-end (no tests Go nuevos). Las tareas de verificación están incluidas como ejecución del smoke test documentado en `quickstart.md`, no como código de test nuevo (la spec marca "sin cambios de código" como Out of Scope).

**Organization**: Tareas agrupadas por user story (US1: aplicar esquema, US2: seeds esenciales, US3: seeds opcionales) para implementación e integración incremental.

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Preparar el entorno de trabajo para regenerar y validar las migraciones consolidadas.

- [X] T001 Verificar herramientas locales: `migrate -version` ≥ v4.19, `psql --version` ≥ 16, `pg_dump --version` ≥ 16, `docker info` responde. Si alguna falta, instalar según el bloque "Pre-requisitos" de `specs/014-consolidate-migrations/quickstart.md`.
- [X] T002 Crear directorios temporales de trabajo: `mkdir -p /tmp/mig-stage1 /tmp/mig-output` para staging del dump y archivos consolidados intermedios.
- [X] T003 Confirmar branch correcto: `git branch --show-current` debe retornar `014-consolidate-migrations`. Si no, `git checkout 014-consolidate-migrations`.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Construir la fuente de verdad del esquema final consolidado a partir del historial actual. Bloquea TODAS las user stories porque el dump generado aquí alimenta a US1, y los INSERTs reales de US2 dependen del esquema correcto.

- [X] T004 Levantar Postgres 16 efímero en `localhost:55432` ejecutando el bloque docker del Paso 1 de `specs/014-consolidate-migrations/quickstart.md`. Validar con `psql "postgres://postgres:test@localhost:55432/postgres?sslmode=disable" -c "SELECT 1;"`.
- [X] T005 Copiar las 20 migraciones actuales a `/tmp/mig-stage1/` resolviendo el conflicto de prefijo `000019` (renombrar `000019_seed_mrg_platform_tenant.*` → `000020_seed_mrg.*` y `000020_seed_test_city_tenants.*` → `000021.*`), exactamente como se describe en el Paso 1 del `quickstart.md`.
- [X] T006 Aplicar el historial completo a la DB efímera: `migrate -path /tmp/mig-stage1/ -database "postgres://postgres:test@localhost:55432/postgres?sslmode=disable" up`. Verificar `SELECT version FROM schema_migrations` retorna `21` y `dirty=false`.
- [X] T007 Generar el dump del esquema final: `pg_dump --schema-only --no-owner --no-privileges --exclude-table=schema_migrations "postgres://postgres:test@localhost:55432/postgres?sslmode=disable" > /tmp/mig-output/initial_schema.sql`.
- [X] T008 Limpiar el dump (`/tmp/mig-output/initial_schema.sql`): eliminar líneas `-- Dumped by …`, `SET …` superfluos, `OWNER TO …`; asegurar que `CREATE EXTENSION` use `IF NOT EXISTS` para `pgcrypto`/`uuid-ossp` si aparecen. Resultado debe ser idempotente y autocontenido.
- [X] T009 Auditar el dump contra `specs/014-consolidate-migrations/data-model.md`: las 15 entidades listadas deben aparecer; las tablas con `tenant_id` deben tener índice por `tenant_id` (Principio II). Documentar cualquier discrepancia y resolverla antes de continuar.
- [X] T010 Bajar el contenedor de staging: `docker rm -f pg-consolidate`.

**Checkpoint**: `/tmp/mig-output/initial_schema.sql` validado y listo. Puede proceder cualquier user story.

---

## Phase 3: User Story 1 — Aplicar esquema consolidado en DB vacía (Priority: P1) 🎯 MVP

**Goal**: Tener una migración inicial única que, aplicada sobre Postgres vacío, deje el esquema final equivalente al historial actual, con su contraparte `down` que limpia todo.

**Independent Test**: Sobre un Postgres 16 limpio, ejecutar `migrate -path migrations/ -database "$URL" up` y verificar que aparecen las 15 entidades de `data-model.md`; ejecutar `migrate down -all` y verificar `\dt` reporta 0 tablas; reaplicar `up` exitosamente.

- [X] T011 [US1] Eliminar todas las migraciones legacy: `git rm migrations/000001_*.sql migrations/000002_*.sql migrations/000003_*.sql migrations/000004_*.sql migrations/000005_*.sql migrations/000006_*.sql migrations/000007_*.sql migrations/000008_*.sql migrations/000009_*.sql migrations/000010_*.sql migrations/000011_*.sql migrations/000012_*.sql migrations/000013_*.sql migrations/000014_*.sql migrations/000015_*.sql migrations/000016_*.sql migrations/000017_*.sql migrations/000018_*.sql migrations/000019_*.sql migrations/000020_*.sql`. Esto resuelve el conflicto de prefijo `000019` (FR-005).
- [X] T012 [US1] Mover el dump validado a su ubicación final: `mv /tmp/mig-output/initial_schema.sql migrations/000001_initial_schema.up.sql`.
- [X] T013 [P] [US1] Crear `migrations/000001_initial_schema.down.sql` con `DROP TABLE IF EXISTS … CASCADE` por cada tabla del schema + `DROP FUNCTION` para cualquier función que cree el `up` (orden FK-safe). Esta forma es la efectivamente versionada en el PR; permite un rollback granular sin requerir privilegios de owner del schema.
- [X] T014 [US1] Smoke test parcial de US1: levantar Postgres 16 efímero (`docker run --rm -d --name pg-us1 -p 55433:5432 -e POSTGRES_PASSWORD=test postgres:16`), aplicar **solo** `000001`: `migrate -path migrations/ -database "postgres://postgres:test@localhost:55433/postgres?sslmode=disable" up 1`. Verificar que las 15 entidades de `data-model.md` están presentes con `psql … -c "\dt"`.
- [X] T015 [US1] Smoke test reverso de US1: `migrate … down 1`; `psql … -c "\dt"` debe reportar 0 tablas. Reaplicar `up 1` (idempotencia). Bajar contenedor: `docker rm -f pg-us1`.

**Checkpoint**: US1 entregable como MVP. Una DB nueva queda con el esquema final aplicando una sola migración. La aplicación arrancaría pero sin RBAC ni admin (eso es US2).

---

## Phase 4: User Story 2 — Seeds esenciales para producción (Priority: P1)

**Goal**: Cargar roles, permisos y tenant MRG de forma idempotente para que la app pueda autenticar y autorizar tras un deploy. El usuario admin MRG NO se siembra en SQL: se crea en Supabase Auth post-deploy y se auto-provisiona en `users` vía `auth_usecase.ProvisionUser` en el primer login (R3 actualizada). La asignación admin↔`super_admin`↔tenant MRG se hace como paso documentado post-deploy.

**Independent Test**: Sobre una DB con `000001` aplicado, ejecutar `migrate up 1` (que aplica `000002`); verificar `SELECT count(*) FROM roles WHERE is_system_role=true` = 6, `SELECT count(*) FROM permissions WHERE is_system_permission=true` = 17, `SELECT count(*) FROM tenants WHERE id='11b36b85-033d-4bb3-9e31-4c92161887c0'` = 1, `SELECT count(*) FROM users` = 0 (no se siembra usuario), `SELECT count(*) FROM user_tenant_roles` = 0. Re-ejecutar `down 1; up 1` sin errores (idempotencia).

**Depends on**: Phase 3 (T012–T015) completa — el seed referencia tablas que sólo existen tras `000001`.

- [X] T016 [US2] Extraer el catálogo completo de permisos del repo legacy revisando `migrations/000017_create_permissions_table.up.sql` (lectura desde `git show HEAD:migrations/000017_create_permissions_table.up.sql` si ya fue eliminado en T011) y volcarlo en una lista de tuplas `(code, description)` que se usará en T018.
- [X] T017 [US2] Extraer el contenido de RBAC global del legacy `migrations/000019_add_global_roles_and_mrg_tenant.up.sql` (vía `git show`) — específicamente las filas insertadas en `roles` con `scope='global'` y las asignaciones en `role_permissions` — y consolidarlas para T018.
- [X] T018 [US2] Crear `migrations/000002_seed_essentials.up.sql` con el bloque del Paso 2 del `quickstart.md`, completando los `INSERT` de roles (T017) y permisos (T016). Cada `INSERT` DEBE usar `ON CONFLICT … DO NOTHING` o `WHERE NOT EXISTS` (decisión R4 de `research.md`). NO insertar usuarios ni filas en `user_tenant_roles`: el admin MRG se crea post-deploy en Supabase y se auto-provisiona vía `auth_usecase.ProvisionUser` (decisión R3 actualizada — sin credenciales en el repo, FR-007 + FR-008).
- [X] T019 [P] [US2] Crear `migrations/000002_seed_essentials.down.sql` con el bloque DELETE en orden inverso (FK-safe). El borrado de `user_tenant_roles` debe scopearse al tenant MRG con AND (no OR) para no arrastrar asignaciones de otros tenants que reusen los mismos `role_id`: `WHERE tenant_id=<MRG> AND role_id IN (...)`. Luego `tenants WHERE id=<MRG>`, luego `roles WHERE id IN (...)`, finalmente `permissions WHERE is_system_permission=true`. (Sin DELETE de `users` ni de `role_permissions` — esas filas no se siembran, ver T018 actualizada.)
- [X] T020 [US2] Smoke test de US2: levantar Postgres 16 efímero, aplicar `migrate up` (las 2 migraciones), correr las queries de validación del bloque "Independent Test" arriba. Probar idempotencia: `migrate down 1; migrate up 1` debe terminar sin error y dejar el mismo conteo de filas.

**Checkpoint**: US1+US2 entregable. La DB queda lista para que la app arranque, un admin MRG sea invitado vía Supabase, y todo funcione end-to-end.

---

## Phase 5: User Story 3 — Seeds opcionales de tenants de prueba (Priority: P2)

**Goal**: Mantener los datos de QA/demos versionados en el repo pero **fuera del flujo de migraciones**, ejecutables solo manualmente en entornos no productivos.

**Independent Test**: En una DB con `000001`+`000002` aplicadas, ejecutar `psql "$URL" -f scripts/seed_test_city_tenants.sql`; verificar que aparecen los tenants de ciudades y sus usuarios. En una DB que solo aplicó las migraciones (sin ejecutar el script), verificar `SELECT count(*) FROM tenants WHERE is_platform=false` = 0.

**Depends on**: Phase 3 (esquema). Independiente de Phase 4 funcionalmente, pero por orden de Definition of Done conviene ejecutar tras US2.

- [X] T021 [P] [US3] Crear `scripts/seed_test_city_tenants.sql` consolidando el contenido de los archivos legacy `migrations/000020_seed_test_city_tenants.up.sql` y `scripts/seed_city_tenants_users.sql` (leerlos vía `git show HEAD:…` si ya fueron eliminados). Cabecera del archivo: comentario explicando que es **solo para entornos no productivos** y se ejecuta manualmente con `psql -f`.
- [X] T022 [P] [US3] Eliminar los seeds sueltos legacy: `git rm scripts/seed_mrg_users.sql scripts/seed_city_tenants_users.sql`. (El contenido de `seed_mrg_users.sql` ya fue absorbido en `000002` durante US2.)
- [X] T023 [US3] Smoke test de US3: sobre la DB con US1+US2 aplicados, ejecutar `psql "$URL" -f scripts/seed_test_city_tenants.sql`; validar conteos de tenants y usuarios de prueba. Confirmar que SC-006 se cumple: si NO se ejecuta el script, `SELECT count(*) FROM tenants WHERE is_platform=false` retorna 0.

**Checkpoint**: Las 3 user stories funcionan independientemente. Repo limpio: 4 archivos en `migrations/` + README, 1 archivo en `scripts/` para QA.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Documentación, ADR, y verificación end-to-end del compromiso constitucional (smoke test completo).

- [X] T024 [P] Crear `docs/adr/ADR-014-consolidate-migrations.md` con el contenido del Paso 6 del `quickstart.md` (Status, Context, Decision, Consequences). Documenta el por qué y cierra el requerimiento de ADR del Constitution Check.
- [X] T025 [P] Actualizar `migrations/README.md`: reemplazar la sección de comandos para reflejar el flujo simplificado (solo dos migraciones), agregar bloque "Deploy a Koyeb" copiado del Paso 4 de `quickstart.md` (FR-006).
- [X] T026 [P] Actualizar `CLAUDE.md` sección "Pending Manual Steps": eliminar referencias obsoletas a T008, T034, T048, T051 y `013` (todas mencionan migraciones específicas eliminadas); agregar entrada "Deploy a Koyeb: aplicar `migrations/000001`+`000002` post-merge — ver `specs/014-consolidate-migrations/quickstart.md`".
- [ ] T027 Smoke test end-to-end completo (compromiso constitucional IV): ejecutar **íntegramente** los Pasos 3.1–3.5 de `specs/014-consolidate-migrations/quickstart.md` sobre un Postgres 16 efímero limpio. Criterio de aceptación: pasos 1, 2, 3, 4 y 5 verdes; en particular el `time migrate … up` debe reportar < 30 s (SC-001), `go test ./...` debe pasar sin cambios de código (SC-003, FR-008), y la query de auditoría de `tenant_id` debe retornar 0 filas (Principio II).
- [ ] T028 Verificación final de scope: `git diff --stat $(git merge-base develop HEAD)...HEAD -- '*.go' 'cmd/**' 'internal/**'` debe estar vacío (FR-008). La rama 014 nace de un punto avanzado de `develop`; comparar contra `main` arrastra features 005–013 ya mergeadas y produce falso-positivo. Debe listar únicamente cambios en `migrations/`, `scripts/`, `docs/adr/`, `CLAUDE.md` y `specs/014-consolidate-migrations/`. Si aparece otro archivo, revertirlo o justificarlo en el PR.
- [ ] T029 Definition of Done: marcar todos los items del bloque "Definition of Done" al final de `quickstart.md`. Si alguno no se cumple, NO mergear hasta resolverlo.

---

## Dependencies

```text
Phase 1 (Setup)
  → Phase 2 (Foundational: dump source-of-truth)
      → Phase 3 (US1) ──────────┐
            ↓                   ↓
      → Phase 4 (US2)     Phase 5 (US3)
            └──────┬──────────┘
                   ↓
            Phase 6 (Polish)
```

**Notas**:
- US2 depende de US1 (los `INSERT` necesitan las tablas).
- US3 depende de US1 (idem). US3 puede ejecutarse antes o después de US2 — no comparten datos.
- Phase 6 requiere las 3 user stories completas.

## Parallel execution opportunities

- Dentro de **Phase 3**: T013 puede correr en paralelo con T012 (archivo distinto, sin dependencia funcional).
- Dentro de **Phase 4**: T019 puede correr en paralelo con T018 (archivos distintos). T016 y T017 son lecturas independientes y también paralelizables.
- Dentro de **Phase 5**: T021 y T022 son archivos independientes — paralelizables.
- Dentro de **Phase 6**: T024, T025, T026 son docs en archivos distintos — paralelizables. T027/T028/T029 son secuenciales (verificación final).

## Implementation strategy: MVP first

- **MVP (entregable mínimo)** = Phase 1 + Phase 2 + Phase 3 (US1). Deja la DB con esquema final aplicable. Si hubiera urgencia, esto solo ya destraba el deploy a Koyeb (la app se levanta, aunque sin admin hasta ejecutar US2).
- **Increment 2** = + Phase 4 (US2). Producción usable: admin MRG puede ser invitado y operar.
- **Increment 3** = + Phase 5 (US3). Habilita QA/demos en dev/staging sin contaminar prod.
- **Final** = + Phase 6. ADR mergeado, docs actualizadas, smoke test completo grabado en el PR.

Recomendación: mergear el PR completo (todas las phases) en un único cambio, dado que las 3 user stories son chicas y la separación es para validación incremental durante el desarrollo, no para releases independientes.

## Format validation

Total: **29 tareas**, todas con formato `- [ ] T### [P?] [Story?] descripción + path`.

| Phase | Tareas | Story label | Paralelas |
|-------|--------|-------------|-----------|
| 1 Setup | T001–T003 | — | T003 implícito (independiente de T001/T002 una vez verificado) |
| 2 Foundational | T004–T010 | — | secuencial (cada paso alimenta al siguiente) |
| 3 US1 | T011–T015 | [US1] | T013 [P] |
| 4 US2 | T016–T020 | [US2] | T016 [P] T017 [P] T019 [P] |
| 5 US3 | T021–T023 | [US3] | T021 [P] T022 [P] |
| 6 Polish | T024–T029 | — | T024 [P] T025 [P] T026 [P] |

**Independent test criteria por story**: documentado al inicio de cada Phase 3/4/5.
**MVP sugerido**: User Story 1 (Phases 1+2+3, T001–T015).
