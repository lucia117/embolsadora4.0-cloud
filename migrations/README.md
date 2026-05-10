# Database Migrations

Este directorio contiene las migraciones de base de datos del proyecto. Tras la consolidación de mayo 2026 (ver [`ADR-014`](../docs/adr/ADR-014-consolidate-migrations.md)) el historial fue colapsado a dos migraciones.

## Requisitos

Instalar la CLI de `golang-migrate` v4.19+:

```bash
# macOS
brew install golang-migrate

# Linux
curl -L https://github.com/golang-migrate/migrate/releases/download/v4.19.0/migrate.linux-amd64.tar.gz | tar xvz
sudo mv migrate /usr/local/bin/
```

## Migraciones actuales

| # | Archivo | Contenido |
|---|---------|-----------|
| 1 | `000001_initial_schema` | DDL completo: 13 tablas (tenants, users, roles, permissions, user_tenant_roles, user_invitations, edge_devices, device_events, alarm_rules, log_entries, log_retention_policies, notifications, dashboard_layouts), índices, FKs, triggers. |
| 2 | `000002_seed_essentials` | Catálogo del sistema: 17 permisos `is_system_permission=TRUE`, 6 roles (`super_admin`, `tenant_manager`, `admin`, `operario`, `cliente_admin`, `cliente_operario`), tenant MRG (`11b36b85-033d-4bb3-9e31-4c92161887c0`). Idempotente (`ON CONFLICT DO NOTHING`). |

## Comandos

### Aplicar todas las migraciones

```bash
migrate -path migrations/ -database "$DATABASE_URL" up
```

### Revertir

```bash
migrate -path migrations/ -database "$DATABASE_URL" down 1     # revierte la última
migrate -path migrations/ -database "$DATABASE_URL" down -all  # revierte todo (preserva schema_migrations)
```

### Crear una nueva migración

```bash
migrate create -ext sql -dir migrations -seq nombre_de_la_feature
```

## Deploy a Koyeb (producción)

```bash
export KOYEB_DATABASE_URL="postgres://USER:PASS@HOST:PORT/DB?sslmode=require"

# 1. Verificar conectividad
psql "$KOYEB_DATABASE_URL" -c "SELECT current_database(), version();"

# 2. Aplicar migraciones
migrate -path migrations/ -database "$KOYEB_DATABASE_URL" up

# 3. Verificar
psql "$KOYEB_DATABASE_URL" -c "SELECT version, dirty FROM schema_migrations;"
# → version=2, dirty=f
```

`sslmode=require` es obligatorio en Koyeb Managed Postgres.

## Activación del admin MRG (post-deploy)

El usuario admin MRG **no** está en el seed: su UUID lo genera Supabase Auth. Pasos:

1. Crear el usuario admin en Supabase Auth (dashboard o API), obtener su UUID.
2. Que el usuario complete el flujo de invitación / set password vía Supabase.
3. En el primer login el middleware (`internal/api/usecases/auth_usecase.go::ProvisionUser`) crea automáticamente la fila en `users`.
4. Asignar el rol `super_admin` dentro del tenant MRG:

   ```sql
   INSERT INTO user_tenant_roles (id, user_id, tenant_id, role_id, status, assigned_at, created_at, updated_at)
   VALUES (
       gen_random_uuid(),
       '<UUID-DEL-ADMIN>',
       '11b36b85-033d-4bb3-9e31-4c92161887c0',
       'super_admin',
       'active', NOW(), NOW(), NOW()
   );
   ```

5. Validar: `curl "$API_URL/api/v1/me" -H "Authorization: Bearer $TOKEN"` debe retornar 200 con permisos de super_admin.

## Seeds opcionales (UAT / dev)

`scripts/seed_test_city_tenants.sql` carga 3 tenants de prueba (Córdoba, Mendoza, Rosario). **No ejecutar en producción.**

```bash
# Solo tenants
psql "$DATABASE_URL" -f scripts/seed_test_city_tenants.sql

# Tenants + usuarios (requiere UUIDs de Supabase, sin comillas extras)
psql "$DATABASE_URL" \
     -v cordoba_admin=<uuid> -v cordoba_op=<uuid> \
     -v mendoza_admin=<uuid> -v mendoza_op=<uuid> \
     -v rosario_admin=<uuid> -v rosario_op=<uuid> \
     -v with_users=1 \
     -f scripts/seed_test_city_tenants.sql
```

## Estado dirty / recuperación

Si una migración falla a medias, `schema_migrations.dirty` queda en `true`:

```bash
# Ver estado
psql "$DATABASE_URL" -c "SELECT version, dirty FROM schema_migrations;"

# Forzar a una versión conocida (después de arreglar manualmente)
migrate -path migrations/ -database "$DATABASE_URL" force <version>
```

## Historial

El historial granular previo (20 migraciones del periodo enero–mayo 2026) está en `git log` y `git show HEAD~N:migrations/…`. Ver `ADR-014` para el contexto completo de la consolidación.
