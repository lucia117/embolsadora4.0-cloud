-- Migration: 000004_create_log_entries_table (DOWN)
DROP TABLE IF EXISTS log_retention_policies;
DROP TABLE IF EXISTS log_entries;
