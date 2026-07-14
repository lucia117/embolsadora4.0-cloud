-- Reverts 000001_initial_schema by dropping every object created above
-- while preserving schema_migrations (managed by golang-migrate).

DROP TABLE IF EXISTS public.notifications CASCADE;
DROP TABLE IF EXISTS public.log_entries CASCADE;
DROP TABLE IF EXISTS public.log_retention_policies CASCADE;
DROP TABLE IF EXISTS public.alarm_rules CASCADE;
DROP TABLE IF EXISTS public.dashboard_layouts CASCADE;
DROP TABLE IF EXISTS public.device_events CASCADE;
DROP TABLE IF EXISTS public.edge_devices CASCADE;
DROP TABLE IF EXISTS public.user_invitations CASCADE;
DROP TABLE IF EXISTS public.user_tenant_roles CASCADE;
DROP TABLE IF EXISTS public.permissions CASCADE;
DROP TABLE IF EXISTS public.roles CASCADE;
DROP TABLE IF EXISTS public.users CASCADE;
DROP TABLE IF EXISTS public.tenants CASCADE;

DROP FUNCTION IF EXISTS public.update_users_updated_at() CASCADE;
DROP FUNCTION IF EXISTS public.update_alarm_rules_updated_at() CASCADE;
DROP FUNCTION IF EXISTS public.update_dashboard_layouts_updated_at() CASCADE;
DROP FUNCTION IF EXISTS public.update_edge_devices_updated_at() CASCADE;
DROP FUNCTION IF EXISTS public.update_permissions_updated_at() CASCADE;
DROP FUNCTION IF EXISTS public.update_updated_at_column() CASCADE;
