-- Migration 005: Create user_invitations table

CREATE TABLE IF NOT EXISTS user_invitations (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id   UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  email       TEXT NOT NULL,
  role_id     VARCHAR(50) NOT NULL REFERENCES roles(id),
  status      TEXT NOT NULL DEFAULT 'pending'
              CHECK (status IN ('pending', 'accepted', 'revoked', 'expired')),
  invited_by  UUID NOT NULL REFERENCES users(id),
  created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  expires_at  TIMESTAMPTZ NOT NULL DEFAULT (NOW() + INTERVAL '7 days')
);

-- Only one pending invitation per email+tenant
CREATE UNIQUE INDEX IF NOT EXISTS idx_user_invitations_pending
  ON user_invitations(tenant_id, email)
  WHERE status = 'pending';

CREATE INDEX IF NOT EXISTS idx_user_invitations_tenant ON user_invitations(tenant_id);
CREATE INDEX IF NOT EXISTS idx_user_invitations_email  ON user_invitations(email);
