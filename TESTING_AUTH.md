# Testing Auth Endpoints

Guía para probar los endpoints de autenticación implementados según el pacto.

## Prerequisitos

1. **Instalar migrate CLI** (si no lo hiciste):
   ```powershell
   scoop install migrate
   # O usando Go:
   go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
   ```

2. **Levantar la base de datos**:
   ```powershell
   make db-up
   ```

3. **Ejecutar migraciones**:
   ```powershell
   make migrate-up
   ```

4. **Iniciar el servidor**:
   ```powershell
   make run
   # O directamente:
   go run cmd/api/main.go
   ```

El servidor debería iniciar en `http://localhost:8080`

## Endpoints Disponibles

### 1. Login (POST /api/auth/callback/credentials)

```bash
curl -X POST http://localhost:8080/api/auth/callback/credentials \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "email=user@example.com&password=password&redirect=false&csrfToken=abc123&callbackUrl=http://localhost:3000/dashboard&json=true" \
  -c cookies.txt
```

**Respuesta esperada**:
```json
{
  "url": "/dashboard"
}
```

La cookie `next-auth.session-token` se guarda en `cookies.txt`.

### 2. Obtener Sesión (GET /api/auth/session)

```bash
curl -X GET http://localhost:8080/api/auth/session \
  -b cookies.txt
```

**Respuesta esperada**:
```json
{
  "user": {
    "id": "00000000-0000-0000-0000-000000000001",
    "name": "John Doe",
    "email": "user@example.com",
    "image": "https://example.com/avatar.jpg"
  },
  "expires": "2025-12-05T10:30:00Z",
  "tenant": {
    "id": "demo",
    "name": "demo",
    "companyName": "Demo Company",
    "subdomain": "demo"
  }
}
```

### 3. Logout (POST /api/auth/signout)

```bash
curl -X POST http://localhost:8080/api/auth/signout \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "csrfToken=abc123&callbackUrl=http://localhost:3000/auth/login&json=true" \
  -b cookies.txt \
  -c cookies.txt
```

**Respuesta esperada**:
```json
{
  "url": "/auth/login",
  "success": true
}
```

### 4. Forgot Password (POST /api/auth/forgot-password)

```bash
curl -X POST http://localhost:8080/api/auth/forgot-password \
  -H "Content-Type: application/json" \
  -d '{"email":"user@example.com"}'
```

**Respuesta esperada**:
```json
{
  "message": "Password reset email sent"
}
```

**Nota**: El token se imprime en la consola del servidor (MockEmailService).

### 5. Reset Password (POST /api/auth/reset-password)

Primero, copia el token que apareció en la consola del servidor después del forgot-password.

```bash
curl -X POST http://localhost:8080/api/auth/reset-password \
  -H "Content-Type: application/json" \
  -d '{
    "token": "TOKEN_AQUI",
    "password": "newPassword123!",
    "passwordConfirmation": "newPassword123!"
  }'
```

**Respuesta esperada**:
```json
{
  "message": "Password updated successfully"
}
```

## Flujo Completo de Prueba

### Escenario 1: Login exitoso
```powershell
# 1. Login
curl -X POST http://localhost:8080/api/auth/callback/credentials `
  -H "Content-Type: application/x-www-form-urlencoded" `
  -d "email=user@example.com&password=password&redirect=false&csrfToken=abc123&callbackUrl=http://localhost:3000/dashboard&json=true" `
  -c cookies.txt

# 2. Verificar sesión
curl -X GET http://localhost:8080/api/auth/session -b cookies.txt

# 3. Logout
curl -X POST http://localhost:8080/api/auth/signout `
  -H "Content-Type: application/x-www-form-urlencoded" `
  -d "csrfToken=abc123&callbackUrl=http://localhost:3000/auth/login&json=true" `
  -b cookies.txt
```

### Escenario 2: Login fallido
```powershell
curl -X POST http://localhost:8080/api/auth/callback/credentials `
  -H "Content-Type: application/x-www-form-urlencoded" `
  -d "email=wrong@example.com&password=wrongpassword&redirect=false&csrfToken=abc123&callbackUrl=http://localhost:3000/dashboard&json=true"
```

**Respuesta esperada**:
```json
{
  "error": "Invalid credentials",
  "statusCode": 401
}
```

### Escenario 3: Reseteo de contraseña
```powershell
# 1. Solicitar reseteo
curl -X POST http://localhost:8080/api/auth/forgot-password `
  -H "Content-Type: application/json" `
  -d '{"email":"user@example.com"}'

# 2. Copiar token de la consola del servidor

# 3. Resetear contraseña
curl -X POST http://localhost:8080/api/auth/reset-password `
  -H "Content-Type: application/json" `
  -d '{
    "token": "TOKEN_DE_LA_CONSOLA",
    "password": "newPassword123!",
    "passwordConfirmation": "newPassword123!"
  }'

# 4. Login con nueva contraseña
curl -X POST http://localhost:8080/api/auth/callback/credentials `
  -H "Content-Type: application/x-www-form-urlencoded" `
  -d "email=user@example.com&password=newPassword123!&redirect=false&csrfToken=abc123&callbackUrl=http://localhost:3000/dashboard&json=true" `
  -c cookies.txt
```

## Usando Postman

Importa la colección de Postman que ya existe en el proyecto:
```
postman/api-embolsadora-api.postman_collection.json
```

Agrega estos requests a la colección:

1. **Login**
   - Method: POST
   - URL: `{{base_url}}/api/auth/callback/credentials`
   - Body (x-www-form-urlencoded):
     - email: user@example.com
     - password: password
     - redirect: false
     - csrfToken: abc123
     - callbackUrl: http://localhost:3000/dashboard
     - json: true

2. **Get Session**
   - Method: GET
   - URL: `{{base_url}}/api/auth/session`
   - (Las cookies se manejan automáticamente)

3. **Logout**
   - Method: POST
   - URL: `{{base_url}}/api/auth/signout`
   - Body (x-www-form-urlencoded):
     - csrfToken: abc123
     - callbackUrl: http://localhost:3000/auth/login
     - json: true

4. **Forgot Password**
   - Method: POST
   - URL: `{{base_url}}/api/auth/forgot-password`
   - Body (JSON):
     ```json
     {
       "email": "user@example.com"
     }
     ```

5. **Reset Password**
   - Method: POST
   - URL: `{{base_url}}/api/auth/reset-password`
   - Body (JSON):
     ```json
     {
       "token": "{{reset_token}}",
       "password": "newPassword123!",
       "passwordConfirmation": "newPassword123!"
     }
     ```

## Verificar Base de Datos

```powershell
# Conectarse a la base de datos
docker exec -it embolsadora-api-db-1 psql -U postgres -d embolsadora

# Ver sesiones activas
SELECT token, user_id, expires_at, created_at FROM sessions;

# Ver tokens de reseteo
SELECT id, user_id, token, expires_at, used_at FROM password_reset_tokens;

# Ver usuarios
SELECT id, email, name, tenant_id, status FROM users;
```

## Troubleshooting

### Error: "Unable to connect to database"
```powershell
# Verificar que la DB está corriendo
docker ps | grep postgres

# Si no está corriendo:
make db-up
```

### Error: "relation 'users' does not exist"
```powershell
# Ejecutar migraciones
make migrate-up
```

### Error: "Invalid credentials" con usuario correcto
```powershell
# Verificar que el usuario existe
docker exec -it embolsadora-api-db-1 psql -U postgres -d embolsadora -c "SELECT email FROM users;"

# Si no existe, ejecutar seed:
make migrate-down
make migrate-up
```

### Ver logs del servidor
El servidor imprime:
- Conexión a DB establecida
- Tokens de reseteo de contraseña (MockEmailService)
- Errores de autenticación

## Próximos Pasos

1. **Implementar EmailService real** para enviar emails de reseteo
2. **Agregar tests de contrato Pact** para validar contra el pacto del frontend
3. **Configurar HTTPS** y `Secure=true` en cookies para producción
4. **Agregar rate limiting** en endpoints de login y forgot-password
5. **Implementar CSRF validation** real
