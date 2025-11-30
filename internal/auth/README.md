# Auth Service - Endpoints de Autenticación

Este módulo implementa los endpoints de autenticación según el pacto `auth-service-api.json` del frontend.

## Endpoints Implementados

### 1. POST `/api/auth/callback/credentials`
**Login con credenciales**

- **Content-Type**: `application/x-www-form-urlencoded`
- **Request Body**:
  ```
  email=user@example.com
  password=password
  redirect=false
  csrfToken=abc123
  callbackUrl=http://localhost:3000/dashboard
  json=true
  ```

- **Response 200** (Login exitoso):
  ```json
  {
    "url": "/dashboard"
  }
  ```
  - Setea cookie: `next-auth.session-token` (HttpOnly, válida por 30 días)

- **Response 401** (Credenciales inválidas):
  ```json
  {
    "error": "Invalid credentials",
    "statusCode": 401
  }
  ```

### 2. GET `/api/auth/session`
**Obtener sesión actual**

- **Headers**: Cookie `next-auth.session-token=<token>`

- **Response 200**:
  ```json
  {
    "user": {
      "id": "1",
      "name": "John Doe",
      "email": "user@example.com",
      "image": "https://example.com/avatar.jpg"
    },
    "expires": "2023-12-31T23:59:59.999Z",
    "tenant": {
      "id": "demo",
      "name": "demo",
      "companyName": "Demo Company",
      "subdomain": "demo"
    }
  }
  ```

- **Response 401** (Sin sesión o sesión inválida):
  ```json
  {
    "error": "Invalid session",
    "statusCode": 401
  }
  ```

### 3. POST `/api/auth/signout`
**Cerrar sesión**

- **Content-Type**: `application/x-www-form-urlencoded`
- **Request Body**:
  ```
  csrfToken=abc123
  callbackUrl=http://localhost:3000/auth/login
  json=true
  ```

- **Response 200**:
  ```json
  {
    "url": "/auth/login",
    "success": true
  }
  ```
  - Borra cookie `next-auth.session-token`

### 4. POST `/api/auth/forgot-password`
**Solicitar reseteo de contraseña**

- **Content-Type**: `application/json`
- **Request Body**:
  ```json
  {
    "email": "user@example.com"
  }
  ```

- **Response 200** (Siempre retorna éxito, no revela si el email existe):
  ```json
  {
    "message": "Password reset email sent"
  }
  ```

### 5. POST `/api/auth/reset-password`
**Resetear contraseña con token**

- **Content-Type**: `application/json`
- **Request Body**:
  ```json
  {
    "token": "valid-reset-token",
    "password": "newSecurePassword123!",
    "passwordConfirmation": "newSecurePassword123!"
  }
  ```

- **Response 200**:
  ```json
  {
    "message": "Password updated successfully"
  }
  ```

- **Response 400** (Token inválido o contraseñas no coinciden):
  ```json
  {
    "error": "Invalid or expired token",
    "statusCode": 400
  }
  ```

## Arquitectura

### Capas

```
handlers.go       -> Maneja HTTP requests/responses
    ↓
service.go        -> Lógica de negocio (auth, sessions, password reset)
    ↓
repository.go     -> Acceso a datos (users, sessions, tenants, tokens)
    ↓
PostgreSQL        -> Base de datos
```

### Archivos

- **`models.go`** - Modelos de dominio y DTOs
- **`repository.go`** - Repositorios para acceso a datos
- **`service.go`** - Servicios de autenticación y lógica de negocio
- **`handlers.go`** - Handlers HTTP para cada endpoint
- **`routes.go`** - Registro de rutas

## Seguridad

### Passwords
- Hasheados con **bcrypt** (cost 10)
- Nunca se exponen en JSON responses

### Sesiones
- Token aleatorio de 32 bytes (base64 encoded)
- Almacenados en tabla `sessions` con expiración
- Cookie `HttpOnly` para prevenir XSS
- Válidas por 30 días

### Password Reset Tokens
- Token aleatorio de 32 bytes (base64 encoded)
- Válidos por 1 hora
- Uso único (se marcan como `used_at` después de usar)
- Se invalidan todos los tokens anteriores al generar uno nuevo

### Cookies
- **Name**: `next-auth.session-token`
- **HttpOnly**: `true` (no accesible desde JavaScript)
- **Secure**: `false` en dev, `true` en producción
- **Path**: `/`
- **SameSite**: Por defecto (Lax)

## Usuario de Prueba

Después de ejecutar las migraciones:

- **Email**: `user@example.com`
- **Password**: `password`
- **Tenant**: `demo`

## Próximos Pasos

### Pendientes
- [ ] Implementar EmailService real (SendGrid, SES, SMTP)
- [ ] Agregar validación CSRF
- [ ] Agregar rate limiting en login y forgot-password
- [ ] Configurar cookie `Secure=true` en producción
- [ ] Agregar logs de auditoría
- [ ] Implementar refresh de sesiones
- [ ] Agregar tests de contrato Pact

### Mejoras Opcionales
- [ ] Soporte para OAuth (Google, GitHub, etc.)
- [ ] 2FA (Two-Factor Authentication)
- [ ] Bloqueo de cuenta después de N intentos fallidos
- [ ] Historial de sesiones activas
- [ ] Notificación de login desde nuevo dispositivo
