# Setup de Migraciones - Instrucciones

## 1. Instalar golang-migrate CLI

### Opción A: Usando scoop (recomendado para Windows)
```powershell
# Si no tienes scoop instalado:
Set-ExecutionPolicy RemoteSigned -Scope CurrentUser
irm get.scoop.sh | iex

# Instalar migrate
scoop install migrate
```

### Opción B: Descarga directa
1. Ve a https://github.com/golang-migrate/migrate/releases/latest
2. Descarga `migrate.windows-amd64.zip`
3. Extrae el archivo `migrate.exe`
4. Muévelo a una carpeta en tu PATH (ej: `C:\Windows\System32` o `C:\Program Files\migrate\`)

### Opción C: Usando Go
```powershell
go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
```

## 2. Verificar instalación
```powershell
migrate -version
```

## 3. Levantar la base de datos
```powershell
make db-up
```

O manualmente:
```powershell
docker compose -f docker-compose.dev.yml up -d db
```

## 4. Ejecutar migraciones
```powershell
make migrate-up
```

Esto creará las tablas:
- ✅ `tenants`
- ✅ `users`
- ✅ `sessions`
- ✅ `password_reset_tokens`

Y agregará datos de prueba:
- 👤 Usuario: `user@example.com` / `password`
- 🏢 Tenant: `demo`

## 5. Verificar que funcionó

Conéctate a la base de datos:
```powershell
docker exec -it embolsadora-api-db-1 psql -U postgres -d embolsadora
```

Luego ejecuta:
```sql
\dt  -- Ver todas las tablas
SELECT * FROM users;  -- Ver el usuario de prueba
SELECT * FROM tenants;  -- Ver el tenant demo
```

## Próximos pasos

Una vez que las migraciones estén ejecutadas, podemos implementar el endpoint de login:
- `POST /api/auth/callback/credentials`

Este endpoint usará las tablas que acabamos de crear para autenticar usuarios.
