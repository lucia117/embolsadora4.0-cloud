# Quickstart — Consolidación de migraciones

Procedimiento end-to-end para regenerar las migraciones consolidadas, validarlas localmente y aplicarlas a Koyeb. Pensado para ser ejecutado por una persona con `docker`, `psql`, `migrate` CLI v4.19+ y acceso a la URL de Postgres de Koyeb.

> **Pre-requisitos**
> - `migrate` CLI: `brew install golang-migrate` (macOS) — versión `v4.19+`.
> - `psql` y `pg_dump` 16: `brew install postgresql@16`.
> - Docker corriendo.
> - Repo en branch `014-consolidate-migrations`.

---

## Paso 1 — Generar el dump del esquema final desde el historial actual

Levantamos un Postgres 16 efímero, aplicamos las 20 migraciones tal cual están hoy (resolviendo el conflicto `000019` con un workaround temporal), y dumpeamos el esquema.

```bash
# Workaround para el conflicto: aplicar primero add_global_roles, luego seed_mrg
mkdir -p /tmp/mig-stage1 /tmp/mig-stage2
cp migrations/000001_*.sql migrations/000002_*.sql migrations/000003_*.sql \
   migrations/000004_*.sql migrations/000005_*.sql migrations/000006_*.sql \
   migrations/000007_*.sql migrations/000008_*.sql migrations/000009_*.sql \
   migrations/000010_*.sql migrations/000011_*.sql migrations/000012_*.sql \
   migrations/000013_*.sql migrations/000014_*.sql migrations/000015_*.sql \
   migrations/000016_*.sql migrations/000017_*.sql migrations/000018_*.sql \
   /tmp/mig-stage1/
cp migrations/000019_add_global_roles_and_mrg_tenant.up.sql /tmp/mig-stage1/000019.up.sql
cp migrations/000019_add_global_roles_and_mrg_tenant.down.sql /tmp/mig-stage1/000019.down.sql
cp migrations/000019_seed_mrg_platform_tenant.up.sql /tmp/mig-stage1/000020_seed_mrg.up.sql
cp migrations/000019_seed_mrg_platform_tenant.down.sql /tmp/mig-stage1/000020_seed_mrg.down.sql
cp migrations/000020_seed_test_city_tenants.up.sql /tmp/mig-stage1/000021.up.sql
cp migrations/000020_seed_test_city_tenants.down.sql /tmp/mig-stage1/000021.down.sql

# Levantar Postgres efímero
docker run --rm -d --name pg-consolidate \
  -p 55432:5432 -e POSTGRES_PASSWORD=test \
  postgres:16
sleep 3

export STAGE_URL="postgres://postgres:test@localhost:55432/postgres?sslmode=disable"

# Aplicar todo el historial
migrate -path /tmp/mig-stage1/ -database "$STAGE_URL" up

# Dumpear solo el esquema
pg_dump --schema-only --no-owner --no-privileges \
  --exclude-table=schema_migrations \
  "$STAGE_URL" > /tmp/initial_schema.sql

docker rm -f pg-consolidate
```

Limpieza manual del dump (`/tmp/initial_schema.sql`):
- Eliminar líneas `-- Dumped by …`, `SET …` que no sean necesarias.
- Verificar que `CREATE EXTENSION` queden con `IF NOT EXISTS`.
- Quitar `OWNER TO …` si quedó alguno.

---

## Paso 2 — Crear las nuevas migraciones

```bash
# Borrar todas las migraciones viejas (incluyendo el conflicto 000019)
git rm migrations/000001_*.sql migrations/000002_*.sql migrations/000003_*.sql \
       migrations/000004_*.sql migrations/000005_*.sql migrations/000006_*.sql \
       migrations/000007_*.sql migrations/000008_*.sql migrations/000009_*.sql \
       migrations/000010_*.sql migrations/000011_*.sql migrations/000012_*.sql \
       migrations/000013_*.sql migrations/000014_*.sql migrations/000015_*.sql \
       migrations/000016_*.sql migrations/000017_*.sql migrations/000018_*.sql \
       migrations/000019_*.sql migrations/000020_*.sql

# Crear el nuevo schema
mv /tmp/initial_schema.sql migrations/000001_initial_schema.up.sql

cat > migrations/000001_initial_schema.down.sql <<'SQL'
DROP SCHEMA public CASCADE;
CREATE SCHEMA public;
GRANT ALL ON SCHEMA public TO public;
SQL
```

Crear `migrations/000002_seed_essentials.up.sql` con (extraído de las migraciones 000017 + 000019_add_global_roles + 000019_seed_mrg + parte relevante de `scripts/seed_mrg_users.sql`):

```sql
-- Roles del sistema
INSERT INTO roles (id, name, scope, description, is_system) VALUES
  (gen_random_uuid(), 'super_admin',  'global', 'Acceso total a la plataforma', true),
  (gen_random_uuid(), 'tenant_admin', 'tenant', 'Administra un tenant', true),
  (gen_random_uuid(), 'operator',     'tenant', 'Operación día a día', true),
  (gen_random_uuid(), 'viewer',       'tenant', 'Solo lectura', true)
ON CONFLICT (name, scope) DO NOTHING;

-- Permisos del sistema
INSERT INTO permissions (id, code, description) VALUES
  (gen_random_uuid(), 'users.create', 'Crear usuarios'),
  (gen_random_uuid(), 'users.read',   'Listar usuarios'),
  -- … completar con catálogo de 000017
ON CONFLICT (code) DO NOTHING;

-- Asignación rol↔permisos (super_admin = todos)
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r CROSS JOIN permissions p
WHERE r.name='super_admin' AND r.scope='global'
ON CONFLICT DO NOTHING;

-- Tenant plataforma MRG
INSERT INTO tenants (id, name, slug, is_platform, status) VALUES
  ('00000000-0000-0000-0000-000000000001', 'MRG Platform', 'mrg', true, 'active')
ON CONFLICT (slug) DO NOTHING;

-- Usuario admin MRG (sin password — se invita post-deploy)
INSERT INTO users (id, email, full_name, status) VALUES
  ('00000000-0000-0000-0000-000000000010',
   'admin@mrg.local', 'MRG Admin', 'pending_invitation')
ON CONFLICT (email) DO NOTHING;

-- Vincular admin MRG ↔ tenant MRG ↔ super_admin
INSERT INTO user_tenant_roles (id, user_id, tenant_id, role_id, status)
SELECT gen_random_uuid(),
       '00000000-0000-0000-0000-000000000010',
       '00000000-0000-0000-0000-000000000001',
       r.id, 'active'
FROM roles r WHERE r.name='super_admin' AND r.scope='global'
AND NOT EXISTS (
  SELECT 1 FROM user_tenant_roles utr
  WHERE utr.user_id='00000000-0000-0000-0000-000000000010'
    AND utr.tenant_id='00000000-0000-0000-0000-000000000001'
);
```

Y el `down`:

```sql
-- migrations/000002_seed_essentials.down.sql
DELETE FROM user_tenant_roles
  WHERE user_id='00000000-0000-0000-0000-000000000010';
DELETE FROM users WHERE id='00000000-0000-0000-0000-000000000010';
DELETE FROM tenants WHERE id='00000000-0000-0000-0000-000000000001';
DELETE FROM role_permissions
  WHERE role_id IN (SELECT id FROM roles WHERE is_system=true);
DELETE FROM permissions;
DELETE FROM roles WHERE is_system=true;
```

Mover los seeds de prueba a `scripts/seed_test_city_tenants.sql` consolidando `000020_seed_test_city_tenants.up.sql` + `scripts/seed_city_tenants_users.sql`. Borrar los originales:

```bash
git rm scripts/seed_mrg_users.sql scripts/seed_city_tenants_users.sql
```

---

## Paso 3 — Smoke test local (compromiso constitucional IV)

```bash
docker run --rm -d --name pg-smoke -p 55433:5432 -e POSTGRES_PASSWORD=test postgres:16
sleep 3
export SMOKE_URL="postgres://postgres:test@localhost:55433/postgres?sslmode=disable"

# 1. Up desde DB vacía
time migrate -path migrations/ -database "$SMOKE_URL" up
# → debe terminar en < 30s y reportar "2/u seed_essentials (XXXms)"

# 2. Down completo
migrate -path migrations/ -database "$SMOKE_URL" down -all
psql "$SMOKE_URL" -c "\dt" # → "Did not find any relations."

# 3. Up de nuevo (verifica reproducibilidad)
migrate -path migrations/ -database "$SMOKE_URL" up

# 4. Verificación de aislamiento de tenants (Principio II)
psql "$SMOKE_URL" <<'SQL'
SELECT table_name FROM information_schema.columns
WHERE column_name='tenant_id' AND table_schema='public'
GROUP BY table_name
EXCEPT
SELECT tablename FROM pg_indexes
WHERE schemaname='public' AND indexdef LIKE '%tenant_id%';
-- Debe retornar 0 filas: toda tabla con tenant_id tiene índice por tenant_id.
SQL

# 5. Tests Go
docker run --rm -v /tmp/go-mod-cache:/go/pkg/mod -v $(pwd):/app -w /app \
  -e DATABASE_URL="postgres://postgres:test@host.docker.internal:55433/postgres?sslmode=disable" \
  golang:1.24-alpine sh -c "go test ./..."
# → todo verde

# 6. Smoke de auth: levantar API, invitar admin MRG, completar invitación, GET /me
# (Detallado en docs/runbooks/post-migrate-smoke.md — fuera de scope crear ese archivo aquí)

docker rm -f pg-smoke
```

Si algún paso falla, **no mergear**. Investigar la diferencia y regenerar el dump del Paso 1.

---

## Paso 4 — Aplicar a Koyeb (producción)

> ⚠️ **Confirmar con el equipo** que la DB de Koyeb está vacía o se autoriza a recrearla. La spec asume DB vacía (Assumption confirmada por el usuario).

```bash
export KOYEB_DATABASE_URL="postgres://USER:PASS@HOST:PORT/DB?sslmode=require"

# Verificar conectividad
psql "$KOYEB_DATABASE_URL" -c "SELECT current_database(), version();"

# Aplicar migraciones
migrate -path migrations/ -database "$KOYEB_DATABASE_URL" up

# Verificar
psql "$KOYEB_DATABASE_URL" -c "SELECT version, dirty FROM schema_migrations;"
# → version=2, dirty=f
```

---

## Paso 5 — Activar el admin MRG

Vía API (con un service token de Supabase con permisos de admin):

```bash
curl -X POST "$API_URL/api/v1/invitations" \
  -H "Authorization: Bearer $SUPABASE_SERVICE_TOKEN" \
  -H "X-Tenant-Id: 00000000-0000-0000-0000-000000000001" \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@mrg.local","role":"super_admin"}'
```

El admin recibe el correo de Supabase, completa la invitación, setea password, y puede hacer login.

Validación final:

```bash
TOKEN=$(... obtener JWT con admin@mrg.local ...)
curl "$API_URL/api/v1/me" -H "Authorization: Bearer $TOKEN"
# → 200 OK con permisos de super_admin
```

---

## Paso 6 — Crear ADR

```bash
cat > docs/adr/ADR-014-consolidate-migrations.md <<'MD'
# ADR-014: Consolidación de migraciones para deploy en Koyeb

**Status**: Accepted | **Date**: 2026-05-08

## Context
Repo acumulaba 20 migraciones con conflicto de prefijo en `000019` y dependencias
históricas que rompían cualquier intento de aplicar incrementalmente sobre una DB
nueva. Necesitábamos un primer deploy a Koyeb reproducible.

## Decision
Reemplazar el historial por:
- `000001_initial_schema` (DDL final, generado vía `pg_dump --schema-only`)
- `000002_seed_essentials` (RBAC + tenant MRG + admin MRG con `pending_invitation`)
Tenants de prueba salen de `migrations/` y pasan a `scripts/seed_test_city_tenants.sql`.

## Consequences
- (+) Deploy a Koyeb es un único `migrate up`.
- (+) Conflicto `000019` resuelto.
- (−) Pérdida de granularidad histórica del schema (mitigado: `git log` retiene el historial).
- (−) Cualquier entorno preexistente con migraciones aplicadas necesita `migrate force 2` antes de `up`.
MD
```

---

## Rollback / contingencia

Si algo sale mal en Koyeb:

```bash
migrate -path migrations/ -database "$KOYEB_DATABASE_URL" down -all
# Vuelve a DB vacía. Investigar, regenerar dump, reintentar.
```

---

## Definition of Done

- [ ] Paso 1 completado, `000001_initial_schema.up.sql` generado y revisado.
- [ ] Paso 2 completado, `000002_seed_essentials.up/down.sql` creados, idempotencia verificada.
- [ ] Paso 3 completado, smoke test 100% verde, query de auditoría de tenant_id retorna 0 filas.
- [ ] `git status` muestra solamente cambios en `migrations/`, `scripts/`, `docs/adr/`, `CLAUDE.md`. **Cero archivos `.go` modificados** (FR-008).
- [ ] Paso 4 ejecutado contra Koyeb, `schema_migrations` reporta `version=2, dirty=f`.
- [ ] Paso 5 ejecutado, admin MRG puede hacer login y `GET /api/v1/me` retorna 200 (SC-002).
- [ ] ADR-014 mergeado.
- [ ] `migrations/README.md` actualizado con el nuevo flujo (FR-006).
- [ ] CLAUDE.md sección "Pending Manual Steps" depurada de referencias a migraciones obsoletas.
