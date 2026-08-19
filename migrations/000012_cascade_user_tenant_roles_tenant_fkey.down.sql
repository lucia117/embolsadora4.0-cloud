-- Revierte a NO ACTION (comportamiento original de la migración 000001).

ALTER TABLE public.user_tenant_roles
    DROP CONSTRAINT user_tenant_roles_tenant_id_fkey,
    ADD CONSTRAINT user_tenant_roles_tenant_id_fkey
        FOREIGN KEY (tenant_id) REFERENCES public.tenants(id);
