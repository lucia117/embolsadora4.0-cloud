# Seed Data (Test Fixtures)

Esta carpeta contiene scripts SQL para cargar datos de prueba/demostración en la base de datos.

## Archivos

### `initial_seed.sql`
Carga datos de prueba para:
- **3 usuarios** con diferentes roles y tenants
- **3 asignaciones de roles** (user_tenant_roles)

#### Usuarios incluidos:
| Email | Nombre | Rol | Tenant |
|-------|--------|-----|--------|
| juan.garcia@demo.com | Juan García | admin | Demo Tenant |
| maria.rodriguez@demo.com | María Rodríguez | user | Demo Tenant |
| carlos.lopez@techsolutions.com | Carlos López | admin | Tech Solutions |

## Cómo usar

### Opción 1: Desde psql (recomendado)
```bash
# Localmente si psql está instalado
psql -h localhost -U embolsadora_user -d embolsadora_dev -f seeds/initial_seed.sql

# O dentro del contenedor Docker
docker exec embolsadora_db psql -U embolsadora_user -d embolsadora_dev -f /dev/stdin < seeds/initial_seed.sql
```

### Opción 2: Copiar el contenido en pgAdmin o DBeaver
1. Abre pgAdmin o DBeaver
2. Conéctate a la BD `embolsadora_dev`
3. Abre una nueva query
4. Copia y pega el contenido del archivo
5. Ejecuta (Ctrl+Enter)

### Opción 3: Desde Go (en tu aplicación)
```go
// Puedes incorporar este archivo en tu aplicación usando `embed`
//go:embed initial_seed.sql
var seedSQL string

db.Exec(context.Background(), seedSQL)
```

## Limpiar datos (opcional)

Si necesitas eliminar los datos insertados y reiniciar:

```sql
-- Eliminar en orden (respetar foreign keys)
DELETE FROM user_tenant_roles WHERE user_id IN (
  SELECT id FROM users WHERE email LIKE '%.com'
);
DELETE FROM users WHERE email LIKE '%.com';
```

## Verificación

Para confirmar que los datos fueron insertados correctamente:

```sql
SELECT 'users' as tabla, COUNT(*) as registros FROM users
UNION ALL
SELECT 'user_tenant_roles', COUNT(*) FROM user_tenant_roles;
```

Debería mostrar:
```
       tabla         | registros
---------------------+-----------
 users               |         3
 user_tenant_roles   |         3
```

## Notas

- Los IDs de tenants (`550e8400-e29b-41d4-a716-446655440001` y `550e8400-e29b-41d4-a716-446655440002`) deben existir en la tabla `tenants`
- Los roles (`admin`, `operario`) deben existir en la tabla `roles`
- Ejecuta después de las migraciones, antes de los tests
- La autenticación es manejada por Supabase Auth — este seed no crea sesiones ni tokens

