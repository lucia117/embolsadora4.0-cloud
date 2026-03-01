-- Create roles catalog table
CREATE TABLE IF NOT EXISTS roles (
    id VARCHAR(50) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Seed predefined roles
INSERT INTO roles (id, name) VALUES
    ('admin', 'Admin'),
    ('operario', 'Operario'),
    ('cliente_admin', 'Cliente Admin'),
    ('cliente_operario', 'Cliente Operario')
ON CONFLICT (id) DO NOTHING;

-- Create user-tenant-role assignments table
CREATE TABLE IF NOT EXISTS user_tenant_roles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    role_id VARCHAR(50) REFERENCES roles(id),
    status VARCHAR(20) NOT NULL DEFAULT 'pending'
        CHECK (status IN ('active', 'pending', 'revoked')),
    assigned_by UUID REFERENCES users(id),
    assigned_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Enforce one active role per user+tenant (partial unique index)
CREATE UNIQUE INDEX idx_utr_active_unique
    ON user_tenant_roles (user_id, tenant_id)
    WHERE status = 'active';

-- Performance indexes
CREATE INDEX idx_utr_tenant_id ON user_tenant_roles (tenant_id);
CREATE INDEX idx_utr_user_id ON user_tenant_roles (user_id);
CREATE INDEX idx_utr_status ON user_tenant_roles (status);
