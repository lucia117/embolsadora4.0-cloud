# Research: Supabase Auth — Backend

**Branch**: `002-supabase-auth-backend` | **Date**: 2026-03-06

---

## 1. Librería JWKS para Go

**Decision**: `MicahParks/keyfunc/v3`

**Rationale**: Es la librería Go más adoptada específicamente para el patrón JWKS + `golang-jwt/jwt`. Maneja automáticamente: cache local de claves públicas, refresh bajo demanda cuando el `kid` del JWT no está en cache, rotación de claves sin reiniciar el servidor, y soporte para múltiples algoritmos (RS256, ES256 — ambos usados por Supabase según la configuración). El proyecto ya importa `golang-jwt/jwt/v5`, y `keyfunc` es el complemento canónico.

**Alternatives considered**:
- `lestrrat-go/jwx/v2`: Más completo pero más verboso. Requiere escribir el cache manualmente. Excesivo para este caso de uso.
- Fetch manual con `net/http`: Mayor superficie de bugs (concurrencia, rotación de claves, expiración). Rechazado.

**Usage pattern**:
```go
// internal/security/jwt.go
import "github.com/MicahParks/keyfunc/v3"

func NewJWKSVerifier(jwksURL, issuer, audience string) (Verifier, error) {
    k, err := keyfunc.NewDefault([]string{jwksURL})
    if err != nil {
        return nil, err
    }
    return &jwksVerifier{keyfunc: k, issuer: issuer, audience: audience}, nil
}

func (v *jwksVerifier) Verify(tokenString string) (*jwt.Token, error) {
    return jwt.Parse(tokenString, v.keyfunc.Keyfunc,
        jwt.WithIssuer(v.issuer),
        jwt.WithAudience(v.audience),
        jwt.WithExpirationRequired(),
    )
}
```

---

## 2. Supabase Admin API — Invitaciones

**Decision**: `POST /auth/v1/admin/invite` con `Authorization: Bearer <service_role_key>`

**Rationale**: Es el endpoint oficial de Supabase para invitaciones. Envía el email con el magic link. El campo `data.redirect_to` permite configurar la URL de redirect post-confirmación, que debe incluir el `tenantId` para que el callback del frontend sepa a qué tenant asociar al usuario.

**Request**:
```
POST {SUPABASE_URL}/auth/v1/admin/invite
Authorization: Bearer {SUPABASE_SERVICE_ROLE_KEY}
Content-Type: application/json

{
  "email": "user@example.com",
  "data": {
    "redirect_to": "{APP_BASE_URL}/s/{tenantId}/auth/callback"
  }
}
```

**Response 200**: `{ "id": "...", "email": "...", "created_at": "..." }`
**Response 422**: Email ya registrado con contraseña (no es un usuario nuevo sin password)
**Response 429**: Rate limit de Supabase Cloud (free tier: 3 emails/hora por defecto en SMTP compartido)

**Important**: Si el usuario ya existe en Supabase (creado previamente), `inviteUserByEmail` reenvía el magic link. Comportamiento idempotente.

---

## 3. Supabase Admin API — Force Password Reset

**Decision**: `POST /auth/v1/admin/generate-link` con `type: "recovery"`

**Rationale**: Genera un recovery link para el usuario sin requerir que el usuario lo solicite. El backend puede obtener el link y enviarlo por email, o simplemente triggear el envío via Supabase.

**Alternative**: `PUT /auth/v1/admin/users/{id}` con `password: null` — fuerza al usuario a setear contraseña en el próximo login, pero no envía email. Menos UX-friendly.

**Request**:
```
POST {SUPABASE_URL}/auth/v1/admin/generate-link
Authorization: Bearer {SUPABASE_SERVICE_ROLE_KEY}
Content-Type: application/json

{
  "type": "recovery",
  "email": "{user_email}"
}
```

**Response 200**: `{ "action_link": "...", "email": "...", "hashed_token": "..." }`

El link generado se puede enviar via el SMTP de Supabase o retornar al backend para usar un SMTP propio (si se requiere en el futuro). Por ahora, Supabase gestiona el envío.

---

## 4. Rate Limiting con Redis (Upstash)

**Decision**: Contador Redis con ventana deslizante simple (INCR + EXPIRE)

**Rationale**: Simple, atómico, y suficiente para el caso de uso. El patrón de ventana fija por hora (no deslizante) es aceptable para rate limiting de invitaciones (no es un API de alto volumen).

**Pattern**:
```go
// internal/api/usecases/invitation_usecase.go
key := fmt.Sprintf("invitations:ratelimit:%s:%s", tenantID, time.Now().Format("2006-01-02-15"))
count, err := redis.Incr(ctx, key)
if count == 1 {
    redis.Expire(ctx, key, time.Hour)
}
if count > invitationRateLimit {
    return ErrRateLimitExceeded
}
```

**Alternative**: Sliding window con Redis sorted sets — más preciso pero innecesariamente complejo para este throughput. Rechazado.

---

## 5. Auto-provisioning idempotente (concurrencia)

**Decision**: `INSERT ... ON CONFLICT (supabase_user_id) DO UPDATE SET email = EXCLUDED.email, last_login_at = NOW() RETURNING *`

**Rationale**: Una sola query que es atómica en PostgreSQL. Dos requests simultáneos del mismo `supabase_user_id` producen exactamente un registro. El `RETURNING *` evita un SELECT posterior.

**Pattern**:
```sql
INSERT INTO users (id, supabase_user_id, email, status, created_at, updated_at, last_login_at)
VALUES ($1, $2, $3, 'invited', NOW(), NOW(), NOW())
ON CONFLICT (supabase_user_id)
DO UPDATE SET
  email = EXCLUDED.email,
  last_login_at = NOW(),
  updated_at = NOW()
RETURNING *;
```

---

## 6. RBAC — Mapa de permisos

**Decision**: Mapa de permisos hardcodeado en código como constantes Go, no en base de datos

**Rationale**: Los roles y sus permisos son estables y conocidos (4 roles: `admin`, `operario`, `cliente_admin`, `cliente_operario`). Hardcodear evita una query adicional en cada request. Los permisos se pueden evolucionar en una feature posterior si se necesita configurabilidad dinámica.

**Structure**:
```go
// internal/security/rbac.go
var rolePermissions = map[string][]string{
    "admin":            {"users:read", "users:write", "invitations:write", "machines:read", "machines:write", "tenants:read"},
    "operario":         {"machines:read", "machines:write"},
    "cliente_admin":    {"users:read", "invitations:write", "machines:read"},
    "cliente_operario": {"machines:read"},
}
```

**Alternative**: Tabla de permisos en DB (`role_permissions`) — más flexible pero requiere query por request (o cache). Diferido a feature posterior.

---

## 7. Supabase JWT Claims

Los tokens JWT de Supabase tienen esta estructura relevante:

```json
{
  "sub": "uuid-supabase-user-id",
  "email": "user@example.com",
  "aud": "authenticated",
  "iss": "https://{project}.supabase.co/auth/v1",
  "exp": 1234567890,
  "role": "authenticated",
  "app_metadata": { "provider": "google" },
  "user_metadata": { "name": "..." }
}
```

- `sub` → `supabase_user_id` en la tabla `users`
- `email` → puede estar vacío en algunos providers OAuth (ej: GitHub sin email público)
- `role` en el JWT es el rol de Supabase (siempre `"authenticated"` para usuarios normales) — **distinto** del rol de negocio en `user_tenant_roles`
- `aud` debe validarse: `"authenticated"` para la superficie ABM

---

## 8. Configuración de Gin Middleware Chain

El middleware chain actual para `/api/v1`:
```
JWTAuth() → TenantFromJWT() → RequestID() → Logger() → CORS()
```

El nuevo chain propuesto:
```
JWTAuth()           # Valida token + auto-provision + status check
TenantFromHeader()  # Lee X-Tenant-ID + valida membresía (excepto GET /api/v1/me)
PasswordChangeGuard() # Bloquea si password_change_required (excepto /me y /auth/change-password)
RequestID()         # Sin cambios
Logger()            # Sin cambios
CORS()              # Sin cambios
```

`TenantFromHeader` debe ser bypasseable para `GET /api/v1/me` porque ese endpoint devuelve todos los tenants del usuario (no necesita un tenant específico). La implementación puede usar un path-based skip o un middleware condicional.
