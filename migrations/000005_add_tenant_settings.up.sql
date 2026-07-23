-- ============================================================================
-- Migration 000005: Add tenant settings fields (contact + localization)
-- ============================================================================
-- The frontend /settings page already sends contactEmail, companyWebsite,
-- locale, timezone, dateFormat, timeFormat, and currency on tenant update,
-- but the backend has never persisted them. This adds 7 flat columns,
-- following the existing Theme/Address precedent (no JSONB), with CHECK
-- constraints on the 5 fields that have a fixed catalog matching the
-- frontend's own <Select> options exactly.
-- ============================================================================

ALTER TABLE tenants
    ADD COLUMN contact_email character varying(255) NOT NULL DEFAULT '',
    ADD COLUMN company_website character varying(255) NOT NULL DEFAULT '',
    ADD COLUMN locale character varying(10) NOT NULL DEFAULT 'es-AR'
        CHECK (locale IN ('es-AR', 'es-ES', 'en-US', 'pt-BR')),
    ADD COLUMN timezone character varying(64) NOT NULL DEFAULT 'America/Argentina/Buenos_Aires'
        CHECK (timezone IN ('America/Argentina/Buenos_Aires', 'America/Sao_Paulo', 'America/Santiago', 'America/Lima', 'America/Bogota', 'America/Mexico_City', 'UTC')),
    ADD COLUMN date_format character varying(20) NOT NULL DEFAULT 'dd/MM/yyyy'
        CHECK (date_format IN ('dd/MM/yyyy', 'MM/dd/yyyy', 'yyyy-MM-dd')),
    ADD COLUMN time_format character varying(10) NOT NULL DEFAULT 'HH:mm'
        CHECK (time_format IN ('HH:mm', 'hh:mm a')),
    ADD COLUMN currency character varying(3) NOT NULL DEFAULT 'ARS'
        CHECK (currency IN ('ARS', 'USD', 'EUR', 'BRL', 'CLP', 'MXN'));
