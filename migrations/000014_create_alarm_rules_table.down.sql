-- Migration: 000014_create_alarm_rules_table.down.sql
DROP TABLE IF EXISTS alarm_rules;
DROP FUNCTION IF EXISTS update_alarm_rules_updated_at();
