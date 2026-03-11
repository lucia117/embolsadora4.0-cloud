-- Drop device_events table
DROP TABLE IF EXISTS device_events;

-- Drop edge_devices table
DROP TABLE IF EXISTS edge_devices;

-- Drop trigger and function
DROP TRIGGER IF EXISTS trg_edge_devices_updated_at ON edge_devices;
DROP FUNCTION IF EXISTS update_edge_devices_updated_at();
