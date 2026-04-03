-- Drop trigger and function first (before tables)
DROP TRIGGER IF EXISTS trg_edge_devices_updated_at ON edge_devices;
DROP FUNCTION IF EXISTS update_edge_devices_updated_at();

-- Drop device_events table (references edge_devices via foreign key)
DROP TABLE IF EXISTS device_events;

-- Drop edge_devices table last
DROP TABLE IF EXISTS edge_devices;
