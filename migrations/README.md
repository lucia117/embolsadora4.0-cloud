# Database Migrations

Este directorio contiene las migraciones de base de datos del proyecto. Tras la consolidación de mayo 2026 (ver [`ADR-014`](../docs/adr/ADR-014-consolidate-migrations.md)) el historial fue colapsado a dos migraciones (`000001`, `000002`); las que siguen (`000003`–`000010`) se agregaron después, incrementalmente.

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
| 3 | `000003_add_platform_tenant_flag` | Agrega `tenants.is_platform_tenant`. |
| 4 | `000004_enforce_platform_role_tenant` | Define `tenant_can_use_role(uuid, boolean)` y el trigger `trg_enforce_platform_role_tenant`: los roles `is_global=TRUE` (`super_admin`, `tenant_manager`) solo pueden asignarse dentro del tenant plataforma de MRG. |
| 5 | `000005_translate_permissions_and_seed_role_permissions` | Traduce el catálogo de permisos y siembra `role_permissions`. |
| 6 | `000006_add_tenant_settings` | Agrega configuración por tenant. |
| 7 | `000007_translate_role_descriptions` | Traduce las descripciones de roles. |
| 8 | `000008_remove_all_tenants_permission` | Elimina el permiso `perm_all_tenants`. |
| 9 | `000009_grant_logs_view_to_admin` | Otorga `perm_logs_view` al rol `admin`. |
| 10 | `000010_platform_only_roles` | **Ver "⚠️ Orden de deploy" abajo.** Extiende `tenant_can_use_role` (ahora `tenant_can_use_role(uuid, text)`, reemplaza la firma `(uuid, boolean)` de la 000004) para que también trate `admin`/`operario` como platform-only, no solo `is_global=TRUE`. Antes de esta migración, un admin de un tenant cliente podía asignarse a sí mismo o a otros el rol `admin` dentro de su propio tenant. |
| 11 | `000011_dynamic_role_permissions` | **Ver "⚠️ Orden de deploy" abajo.** Extiende el catálogo de permisos con `perm_users_view`/`perm_users_manage`/`perm_tenants_view`/`perm_tenants_manage`, reemplazando los permisos gruesos `perm_users`/`perm_tenants`. Agrega la fila `platform_admin` a `roles` (antes solo se calculaba en runtime). Resiembra `permissions` de los 7 roles de sistema. |
| 12 | `000012_cascade_user_tenant_roles_tenant_fkey` | Cambia `user_tenant_roles_tenant_id_fkey` a `ON DELETE CASCADE`, igual que el resto de las FKs que referencian `tenants(id)`. Era la única sin CASCADE desde la `000001` — borrar un tenant con asignaciones de rol activas fallaba con una violación de FK. Sin riesgo de orden de deploy: no requiere cambios de código Go. |

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

## ⚠️ Orden de deploy: migración 000010

La migración `000010_platform_only_roles` **tiene que aplicarse antes de deployar el
binario** que incluye los cambios de Go de la rama `fix/uat-role-scoping`. El código Go
de esa rama llama a `tenant_can_use_role(uuid, text)` — la firma nueva que introduce
`000010`, no la `(uuid, boolean)` de la `000004`. Si el binario nuevo corre contra una DB
que todavía tiene solo la `000004` aplicada, `GET /roles` y toda ruta de asignación de
roles (`POST /user-roles`, `PUT /user-roles/:id`, `POST /user-roles/bulk`, `POST /users`)
devuelven 500 (función inexistente con esa firma).

Simétricamente, un **rollback** del código de esta rama también requiere revertir esta
migración (`migrate ... down 1`), no solo el binario: si el binario vuelve a la versión
anterior pero `000010` queda aplicada, ese binario anterior sigue esperando la firma
`(uuid, boolean)` — que `000010` reemplazó por `(uuid, text)` — y se rompe de la misma
forma, solo que en la dirección contraria.

Orden correcto:
- **Deploy**: aplicar `000010` → deployar el binario nuevo.
- **Rollback**: revertir al binario anterior → `migrate ... down 1` (revierte `000010`).

### Antes de deployar 000010: auditar datos existentes

`000010` empieza a rechazar asignaciones de `admin`/`operario` fuera del tenant
plataforma de MRG. No toca filas existentes (no hay backfill/DELETE en el `up.sql`), pero
sí bloquea *cambios futuros* sobre membresías que ya estén en ese estado, y dos casos
concretos donde eso importa antes del deploy:

```sql
-- Membresías activas existentes que van a quedar bloqueadas para futuros cambios
-- de rol admin/operario (el UPDATE de esa fila fallaría después de esta migración,
-- aunque la fila en sí no se toca):
SELECT utr.tenant_id, utr.role_id, count(*) FROM user_tenant_roles utr
  JOIN tenants t ON t.id = utr.tenant_id
 WHERE utr.role_id IN ('admin','operario') AND NOT t.is_platform_tenant
   AND utr.status = 'active' GROUP BY 1,2;

-- Invitaciones pendientes que van a fallar silenciosamente al activarse (el usuario
-- acepta, pero no se crea la membresía):
SELECT i.tenant_id, i.role_id, count(*) FROM user_invitations i
  JOIN tenants t ON t.id = i.tenant_id
 WHERE i.role_id IN ('admin','operario') AND NOT t.is_platform_tenant
   AND i.status = 'pending' GROUP BY 1,2;
```

Si la segunda consulta devuelve filas, es un gap conocido y **fuera de alcance** de esta
tanda de fixes: `ActivatePendingInvitations` (`internal/api/usecases/invitation_usecase.go`)
hoy solo loguea un warning y sigue de largo cuando el trigger rechaza la asignación —
la invitación queda `pending` para siempre, sin error visible para el usuario ni para
quien invitó. Si la primera auditoría (arriba) devuelve filas antes de deployar, un
humano tiene que decidir qué hacer con esas invitaciones pendientes — no se intenta
arreglar ese código acá.

## ⚠️ Orden de deploy: migración 000011

A diferencia de `000010` (backend-binario vs. DB), el riesgo de `000011` es
**backend-DB vs. frontend**. El código Go de esta rama chequea permisos
exclusivamente vía ids `perm_*` (`security.Can()`), pero el frontend
actualmente deployado (`embolsadora-frontend`, repo separado) todavía filtra
la nav de admin y el acceso a sus rutas con los ids viejos `perm_users`/
`perm_tenants` — que esta migración **borra** del catálogo.

Entre el momento en que `000011` se aplica en producción y el momento en que
el fix propio del frontend (deploy separado, con su propio plan en el repo
frontend) sale, los `admin`/`super_admin` van a ver desaparecer sus ítems de
nav de administración y van a chocar con access-denied del lado del cliente
en esas rutas — aunque la API del backend siga permitiendo esas llamadas sin
problema. Es una ventana de degradación de UX en el admin, no pérdida de
datos ni un agujero de seguridad: la API sigue correctamente enforced todo
el tiempo.

Orden correcto:
- Aplicar `000011`.
- Deployar el binario nuevo del backend (junto con la migración o muy cerca —
  el código Go de esta rama asume el catálogo que introduce `000011`).
- Deployar el frontend lo antes posible después, idealmente espalda con
  espalda con el paso anterior, para minimizar la ventana de degradación.

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
