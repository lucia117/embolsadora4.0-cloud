# Guía de Inicio Rápido - Embolsadora API

Esta guía te ayudará a levantar la API rápidamente usando Docker Compose.

## Prerequisitos

- Docker Desktop instalado y en ejecución
- Docker Compose (incluido con Docker Desktop)

## Pasos para levantar la API

### 1. Clonar el repositorio (si aún no lo hiciste)

```powershell
git clone <url-del-repositorio>
cd embolsadora-api
```

### 2. Levantar los servicios con Docker Compose

```powershell
# Construir las imágenes (solo la primera vez o cuando cambies el código)
docker-compose -f docker-compose.yml build --no-cache

# Levantar todos los servicios (PostgreSQL, Redis y API)
docker-compose -f docker-compose.yml up
```

**Nota para Windows PowerShell:** Si el comando anterior falla con error de sintaxis, ejecutá los comandos por separado:

```powershell
docker-compose -f docker-compose.yml build --no-cache
docker-compose -f docker-compose.yml up
```

### 3. Verificar que la API está funcionando

Abrí otra terminal y ejecutá:

```powershell
# Health check
curl http://localhost:8080/healthz

# Ready check
curl http://localhost:8080/readyz
```

Si no tenés `curl`, podés abrir en tu navegador:

- http://localhost:8080/healthz
- http://localhost:8080/readyz

Ambos deberían devolver un código 200 OK.

## Servicios disponibles

Una vez que los contenedores estén corriendo, tendrás acceso a:

- **API REST**: http://localhost:8080
- **PostgreSQL**: localhost:5432
  - Usuario: `embolsadora_user`
  - Password: `embolsadora_password`
  - Base de datos: `embolsadora_dev`
- **Redis**: localhost:6379
  - Password: `embolsadora_redis_pass`

## Comandos útiles

### Ver logs de los servicios

```powershell
# Ver logs de todos los servicios
docker-compose -f docker-compose.yml logs -f

# Ver logs solo de la API
docker-compose -f docker-compose.yml logs -f api

# Ver logs solo de PostgreSQL
docker-compose -f docker-compose.yml logs -f db
```

### Detener los servicios

```powershell
# Presionar Ctrl+C en la terminal donde están corriendo

# O ejecutar en otra terminal:
docker-compose -f docker-compose.yml down
```

### Reiniciar los servicios

```powershell
# Detener
docker-compose -f docker-compose.yml down

# Levantar nuevamente
docker-compose -f docker-compose.yml up
```

### Eliminar todo (incluyendo datos de la base de datos)

```powershell
# Detener y eliminar volúmenes
docker-compose -f docker-compose.yml down -v
```

⚠️ **Advertencia:** Esto eliminará todos los datos de la base de datos.

## Reconstruir la API después de cambios en el código

Si modificás el código de la API, necesitás reconstruir la imagen:

```powershell
# Detener los servicios
docker-compose -f docker-compose.yml down

# Reconstruir solo el servicio de la API
docker-compose -f docker-compose.yml build api

# Levantar nuevamente
docker-compose -f docker-compose.yml up
```

## Solución de problemas

### La API no se conecta a la base de datos

Verificá que los contenedores de PostgreSQL y Redis estén corriendo y saludables:

```powershell
docker ps
```

Deberías ver tres contenedores: `embolsadora_api`, `embolsadora_db`, y `embolsadora_redis`.

### Puerto 8080 ya está en uso

Si el puerto 8080 ya está siendo usado por otra aplicación, podés cambiar el puerto en el archivo `docker-compose.yml`:

```yaml
api:
  ports:
    - "3000:8080" # Cambia 3000 por el puerto que prefieras
```

### Errores de construcción de la imagen

Si tenés problemas al construir la imagen, intentá limpiar las imágenes antiguas:

```powershell
# Eliminar imágenes no utilizadas
docker image prune -a

# Reconstruir desde cero
docker-compose -f docker-compose.yml build --no-cache
```

## Próximos pasos

- Lee el [README.md](README.md) completo para más detalles sobre la arquitectura
- Revisa [SETUP_MIGRATIONS.md](SETUP_MIGRATIONS.md) para configurar las migraciones de base de datos
- Consulta [TESTING_AUTH.md](TESTING_AUTH.md) para probar la autenticación

## Endpoints disponibles

### Health Checks

- `GET /healthz` - Health check básico
- `GET /readyz` - Ready check

### Autenticación

- `POST /api/auth/register` - Registrar usuario
- `POST /api/auth/login` - Iniciar sesión
- `POST /api/auth/refresh` - Refrescar token

### API v1 (requiere autenticación)

- `GET /api/v1/users` - Listar usuarios
- `POST /api/v1/users` - Crear usuario
- `GET /api/v1/machines` - Listar máquinas
- `POST /api/v1/machines` - Crear máquina
- `GET /api/v1/tenants` - Listar tenants
- `POST /api/v1/tenants` - Crear tenant

Para más detalles sobre los endpoints, consultá la documentación OpenAPI en `docs/openapi.yaml`.
