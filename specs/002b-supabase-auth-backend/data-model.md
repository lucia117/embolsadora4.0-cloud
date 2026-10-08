# Data Model: Supabase Auth — Backend

**Branch**: `002-supabase-auth-backend` | **Date**: 2026-03-06

---

## Entidades modificadas

### `users` (tabla existente — modificada)

**Cambios en migración 004**:
- ELIMINAR: `password_hash TEXT`
- AGREGAR: `supabase_user_id TEXT UNIQUE NOT NULL` (después de backfill; ver nota de migración)
- AGREGAR: `auth_provider TEXT` (ej: `"email"`, `"google"`, `"github"`)
- AGREGAR: `email_verified_at TIMESTAMPTZ`
- AGREGAR: `last_login_at TIMESTAMPTZ`
- AGREGAR: `password_change_required BOOLEAN NOT NULL DEFAULT FALSE`

**Schema post-migración**:
```sql
CREATE TABLE users (
  id                       UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  supabase_user_id         TEXT UNIQUE,           -- FK lógica a auth.users de Supabase
  email                    TEXT,                  -- puede ser NULL en OAuth sin email público
  name                     TEXT,
  status                   TEXT NOT NULL DEFAULT 'invited'
                           CHECK (status IN ('invited', 'active', 'revoked', 'disabled')),
  auth_provider            TEXT,
  email_verified_at        TIMESTAMPTZ,
  last_login_at            TIMESTAMPTZ,
  password_change_required BOOLEAN NOT NULL DEFAULT FALSE,
  created_at               TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at               TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_users_supabase_user_id ON users(supabase_user_id);
```

**Estado de `status`**:
```
invited ──────► active ──────► revoked   (acción manual del admin — reversible)
                   │
                   └──────────► disabled  (acción automática del sistema — requiere intervención técnica)
```

**Nota de migración**: Los usuarios existentes en la tabla `users` (si los hay) tendrán `supabase_user_id = NULL` hasta que hagan login. El campo es `UNIQUE` pero no `NOT NULL` para permitir la migración sin downtime.

---

### `sessions` (tabla existente — ELIMINADA)

```sql
DROP TABLE sessions CASCADE;
```

---

### `password_reset_tokens` (tabla existente — ELIMINADA)

```sql
DROP TABLE password_reset_tokens CASCADE;
```

---

## Entidades nuevas

### `user_invitations` (tabla nueva — migración 005)

```sql
CREATE TABLE user_invitations (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id   TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  email       TEXT NOT NULL,
  role_id     UUID NOT NULL REFERENCES roles(id),
  status      TEXT NOT NULL DEFAULT 'pending'
              CHECK (status IN ('pending', 'accepted', 'revoked', 'expired')),
  invited_by  UUID NOT NULL REFERENCES users(id),
  created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  expires_at  TIMESTAMPTZ NOT NULL DEFAULT (NOW() + INTERVAL '7 days')
);

-- Solo puede haber una invitación pendiente por email+tenant
CREATE UNIQUE INDEX idx_user_invitations_pending
  ON user_invitations(tenant_id, email)
  WHERE status = 'pending';

CREATE INDEX idx_user_invitations_tenant ON user_invitations(tenant_id);
CREATE INDEX idx_user_invitations_email  ON user_invitations(email);
```

**State transitions**:
```
pending ──────► accepted   (primer login del usuario invitado — automático)
   │
   ├──────────► revoked    (admin revoca manualmente — DELETE /api/v1/invitations/:id)
   │
   └──────────► expired    (expires_at < NOW() — detectado en el momento de activación)
```

---

## Entidades existentes (sin cambios de schema)

### `user_tenant_roles` (existente — usada por RBAC y X-Tenant-ID validation)

```sql
-- Ya existe. Campos relevantes:
-- user_id      UUID FK users
-- tenant_id    TEXT FK tenants
-- role_id      UUID FK roles
-- status       TEXT ('active', 'revoked')
-- Índice único parcial: UNIQUE (user_id, tenant_id) WHERE status = 'active'
```

### `roles` (existente — roles de negocio)

```sql
-- Roles actuales: admin, operario, cliente_admin, cliente_operario
-- No se modifican en esta feature
```

---

## Domain types (Go)

### `domain.User`

```go
// internal/domain/user.go (extender)
type UserStatus string

const (
    UserStatusInvited  UserStatus = "invited"
    UserStatusActive   UserStatus = "active"
    UserStatusRevoked  UserStatus = "revoked"   // acción manual del admin
    UserStatusDisabled UserStatus = "disabled"  // acción automática del sistema
)

type User struct {
    ID                     string
    SupabaseUserID         string
    Email                  string
    Name                   string
    Status                 UserStatus
    AuthProvider           string
    EmailVerifiedAt        *time.Time
    LastLoginAt            *time.Time
    PasswordChangeRequired bool
    CreatedAt              time.Time
    UpdatedAt              time.Time
}
```

### `domain.UserInvitation` (nuevo)

```go
// internal/domain/invitation.go (nuevo)
type InvitationStatus string

const (
    InvitationStatusPending  InvitationStatus = "pending"
    InvitationStatusAccepted InvitationStatus = "accepted"
    InvitationStatusRevoked  InvitationStatus = "revoked"
    InvitationStatusExpired  InvitationStatus = "expired"
)

type UserInvitation struct {
    ID         string
    TenantID   string
    Email      string
    RoleID     string
    Status     InvitationStatus
    InvitedBy  string
    CreatedAt  time.Time
    UpdatedAt  time.Time
    ExpiresAt  time.Time
}

func (i *UserInvitation) IsExpired() bool {
    return time.Now().After(i.ExpiresAt)
}
```

---

## Migraciones

### 000004_supabase_auth_migration.up.sql

```sql
-- Agregar columnas nuevas a users
ALTER TABLE users
  ADD COLUMN IF NOT EXISTS supabase_user_id         TEXT UNIQUE,
  ADD COLUMN IF NOT EXISTS auth_provider            TEXT,
  ADD COLUMN IF NOT EXISTS email_verified_at        TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS last_login_at            TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS password_change_required BOOLEAN NOT NULL DEFAULT FALSE;

CREATE INDEX IF NOT EXISTS idx_users_supabase_user_id ON users(supabase_user_id);

-- Eliminar columna de password del sistema anterior
ALTER TABLE users DROP COLUMN IF EXISTS password_hash;

-- Eliminar tablas del sistema de auth anterior
DROP TABLE IF EXISTS sessions CASCADE;
DROP TABLE IF EXISTS password_reset_tokens CASCADE;
```

### 000004_supabase_auth_migration.down.sql

```sql
ALTER TABLE users
  DROP COLUMN IF EXISTS supabase_user_id,
  DROP COLUMN IF EXISTS auth_provider,
  DROP COLUMN IF EXISTS email_verified_at,
  DROP COLUMN IF EXISTS last_login_at,
  DROP COLUMN IF EXISTS password_change_required,
  ADD COLUMN IF NOT EXISTS password_hash TEXT;

-- Nota: sessions y password_reset_tokens NO se recrean en down (datos perdidos intencionalmente)
```

### 000005_user_invitations.up.sql

```sql
CREATE TABLE IF NOT EXISTS user_invitations (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id   TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  email       TEXT NOT NULL,
  role_id     UUID NOT NULL REFERENCES roles(id),
  status      TEXT NOT NULL DEFAULT 'pending'
              CHECK (status IN ('pending', 'accepted', 'revoked', 'expired')),
  invited_by  UUID NOT NULL REFERENCES users(id),
  created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  expires_at  TIMESTAMPTZ NOT NULL DEFAULT (NOW() + INTERVAL '7 days')
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_user_invitations_pending
  ON user_invitations(tenant_id, email)
  WHERE status = 'pending';

CREATE INDEX IF NOT EXISTS idx_user_invitations_tenant ON user_invitations(tenant_id);
CREATE INDEX IF NOT EXISTS idx_user_invitations_email  ON user_invitations(email);
```

### 000005_user_invitations.down.sql

```sql
DROP TABLE IF EXISTS user_invitations CASCADE;
```
