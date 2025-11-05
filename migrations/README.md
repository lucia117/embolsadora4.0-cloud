# Database Migrations

Este directorio contiene las migraciones de base de datos para el proyecto.

## Requisitos

Instalar la CLI de `golang-migrate`:

```bash
# Windows (usando scoop)
scoop install migrate

# macOS
brew install golang-migrate

# Linux
curl -L https://github.com/golang-migrate/migrate/releases/download/v4.19.0/migrate.linux-amd64.tar.gz | tar xvz
sudo mv migrate /usr/local/bin/
```

## Comandos disponibles

### Ejecutar migraciones
```bash
make migrate-up
```

### Revertir última migración
```bash
make migrate-down
```

### Crear nueva migración
```bash
make migrate-create
# Luego ingresa el nombre de la migración
```

### Levantar solo la base de datos
```bash
make db-up
```

## Migraciones actuales

### 000001_create_auth_tables
Crea las tablas necesarias para autenticación:
- `tenants` - Información de organizaciones/tenants
- `users` - Usuarios del sistema
- `sessions` - Tokens de sesión (cookie-based)
- `password_reset_tokens` - Tokens para recuperación de contraseña

### 000002_seed_demo_data
Inserta datos de prueba:
- Tenant: `demo` (Demo Company)
- Usuario: `user@example.com` / `password`

## Estructura de tablas

### users
- `id` (UUID, PK)
- `email` (VARCHAR, UNIQUE)
- `name` (VARCHAR)
- `password_hash` (VARCHAR) - bcrypt hash
- `image` (TEXT, nullable)
- `tenant_id` (VARCHAR, FK -> tenants.id)
- `status` (VARCHAR) - 'active', 'inactive', etc.
- `created_at`, `updated_at` (TIMESTAMP)

### tenants
- `id` (VARCHAR, PK)
- `name` (VARCHAR)
- `company_name` (VARCHAR)
- `subdomain` (VARCHAR, UNIQUE)
- `created_at`, `updated_at` (TIMESTAMP)

### sessions
- `token` (VARCHAR, PK) - El valor de la cookie `next-auth.session-token`
- `user_id` (UUID, FK -> users.id)
- `expires_at` (TIMESTAMP)
- `created_at` (TIMESTAMP)

### password_reset_tokens
- `id` (UUID, PK)
- `user_id` (UUID, FK -> users.id)
- `token` (VARCHAR, UNIQUE)
- `expires_at` (TIMESTAMP)
- `used_at` (TIMESTAMP, nullable)
- `created_at` (TIMESTAMP)

## Usuario de prueba

Después de ejecutar las migraciones, puedes usar estas credenciales para probar:

- **Email**: `user@example.com`
- **Password**: `password`
- **Tenant**: `demo`
