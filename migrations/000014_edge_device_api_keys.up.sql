-- ============================================================================
-- Migration 000008: API keys de edge devices
-- ============================================================================
-- El Edge Pi Service autentica contra POST /api/v1/consumers/events con el
-- header X-Api-Key. El cloud resuelve tenant y device server-side a partir de
-- esa key: el Pi nunca manda X-Tenant-Id. Esta tabla es ese punto de anclaje.
--
-- El secreto en claro NO se guarda nunca: sólo sha256(secreto) en key_hash.
-- key_id es la parte publica e indexada, que permite el lookup directo sin
-- tener que comparar hashes contra toda la tabla.
--
-- Varias keys activas por device es intencional: es lo que permite rotar sin
-- downtime (crear la nueva, desplegarla en el Pi, recien ahi revocar la vieja).
-- ============================================================================

CREATE TABLE public.edge_device_api_keys (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id    uuid NOT NULL REFERENCES public.tenants(id)      ON DELETE CASCADE,
    device_id    uuid NOT NULL REFERENCES public.edge_devices(id) ON DELETE CASCADE,
    key_id       character varying(32) NOT NULL UNIQUE,
    key_hash     bytea NOT NULL,
    name         character varying(255),
    created_at   timestamp with time zone NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_by   uuid REFERENCES public.users(id),
    expires_at   timestamp with time zone,
    revoked_at   timestamp with time zone,
    last_used_at timestamp with time zone
);

CREATE INDEX idx_edge_device_api_keys_device ON public.edge_device_api_keys(device_id);
