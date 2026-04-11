-- Migration: 000017_create_permissions_table.down.sql
-- Feature: 011-permissions-management

DROP TABLE IF EXISTS permissions CASCADE;
DROP FUNCTION IF EXISTS update_permissions_updated_at CASCADE;
