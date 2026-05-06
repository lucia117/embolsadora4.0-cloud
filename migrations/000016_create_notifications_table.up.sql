CREATE TABLE notifications (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL,
    title           TEXT NOT NULL,
    message         TEXT NOT NULL,
    severity        VARCHAR(20) NOT NULL CHECK (severity IN ('info', 'warning', 'critical', 'error')),
    status          VARCHAR(20) NOT NULL DEFAULT 'unread' CHECK (status IN ('unread', 'acknowledged', 'closed')),
    alarm_rule_id   UUID,
    machine_id      UUID,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    acknowledged_at TIMESTAMPTZ,
    closed_at       TIMESTAMPTZ
);

-- Listado paginado por tenant (orden cronológico inverso)
CREATE INDEX idx_notifications_tenant_list
    ON notifications(tenant_id, created_at DESC);

-- Filtrado por status (conteo de unread, filtro por status)
CREATE INDEX idx_notifications_tenant_status
    ON notifications(tenant_id, status, created_at DESC);

-- Filtrado por severidad
CREATE INDEX idx_notifications_tenant_severity
    ON notifications(tenant_id, severity, created_at DESC);
