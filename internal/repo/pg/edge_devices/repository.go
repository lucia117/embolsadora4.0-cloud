package edge_devices

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/tu-org/embolsadora-api/internal/domain/edge_devices"
)

// PostgresRepository implements edge_devices.Repository using PostgreSQL.
type PostgresRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresRepository creates a new PostgreSQL repository.
func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

// List returns all edge devices for a tenant.
func (r *PostgresRepository) List(ctx context.Context, tenantID uuid.UUID) ([]*edge_devices.EdgeDevice, error) {
	query := `
		SELECT id, tenant_id, name, description, machine_id, edge_type, raspberry_base_url,
		       plc_address, status, last_seen_at, last_health_check_at, last_health_status,
		       last_health_summary, created_at, updated_at
		FROM edge_devices
		WHERE tenant_id = $1
		ORDER BY created_at DESC
	`

	rows, err := r.pool.Query(ctx, query, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var devices []*edge_devices.EdgeDevice
	for rows.Next() {
		device, err := scanEdgeDevice(rows)
		if err != nil {
			return nil, err
		}
		devices = append(devices, device)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return devices, nil
}

// GetByID returns a single device by ID for a tenant.
func (r *PostgresRepository) GetByID(ctx context.Context, tenantID, deviceID uuid.UUID) (*edge_devices.EdgeDevice, error) {
	query := `
		SELECT id, tenant_id, name, description, machine_id, edge_type, raspberry_base_url,
		       plc_address, status, last_seen_at, last_health_check_at, last_health_status,
		       last_health_summary, created_at, updated_at
		FROM edge_devices
		WHERE tenant_id = $1 AND id = $2
	`

	row := r.pool.QueryRow(ctx, query, tenantID, deviceID)
	device, err := scanEdgeDevice(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, edge_devices.ErrDeviceNotFound
		}
		return nil, err
	}
	return device, nil
}

// Create persists a new edge device.
func (r *PostgresRepository) Create(ctx context.Context, device *edge_devices.EdgeDevice) error {
	query := `
		INSERT INTO edge_devices (id, tenant_id, name, description, machine_id, edge_type,
		                          raspberry_base_url, plc_address, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING created_at, updated_at
	`

	var createdAt, updatedAt interface{}
	err := r.pool.QueryRow(ctx, query,
		device.ID, device.TenantID, device.Name, device.Description, device.MachineID,
		device.EdgeType, device.RaspberryBaseURL, device.PLCAddress, device.Status,
		device.CreatedAt, device.UpdatedAt).Scan(&createdAt, &updatedAt)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" { // unique constraint violation
			return edge_devices.ErrMachineIDConflict
		}
		return err
	}

	return nil
}

// Update persists changes to an existing device.
func (r *PostgresRepository) Update(ctx context.Context, device *edge_devices.EdgeDevice) error {
	query := `
		UPDATE edge_devices
		SET name = $1, description = $2, plc_address = $3, status = $4, updated_at = $5
		WHERE tenant_id = $6 AND id = $7
		RETURNING updated_at
	`

	err := r.pool.QueryRow(ctx, query,
		device.Name, device.Description, device.PLCAddress, device.Status,
		device.UpdatedAt, device.TenantID, device.ID).Scan(&device.UpdatedAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return edge_devices.ErrDeviceNotFound
		}
		return err
	}

	return nil
}

// SetStatus updates the device status and returns the updated device.
func (r *PostgresRepository) SetStatus(ctx context.Context, tenantID, deviceID uuid.UUID, status string) (*edge_devices.EdgeDevice, error) {
	query := `
		UPDATE edge_devices
		SET status = $1, updated_at = CURRENT_TIMESTAMP
		WHERE tenant_id = $2 AND id = $3
		RETURNING id, tenant_id, name, description, machine_id, edge_type, raspberry_base_url,
		          plc_address, status, last_seen_at, last_health_check_at, last_health_status,
		          last_health_summary, created_at, updated_at
	`

	row := r.pool.QueryRow(ctx, query, status, tenantID, deviceID)
	device, err := scanEdgeDevice(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, edge_devices.ErrDeviceNotFound
		}
		return nil, err
	}

	return device, nil
}

// UpdateHealthState updates the device's health check metadata.
func (r *PostgresRepository) UpdateHealthState(ctx context.Context, tenantID, deviceID uuid.UUID, status, summary string) error {
	query := `
		UPDATE edge_devices
		SET last_health_check_at = CURRENT_TIMESTAMP, last_health_status = $1, last_health_summary = $2,
		    updated_at = CURRENT_TIMESTAMP
		WHERE tenant_id = $3 AND id = $4
	`

	result, err := r.pool.Exec(ctx, query, status, summary, tenantID, deviceID)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return edge_devices.ErrDeviceNotFound
	}

	return nil
}

// SaveEvent persists an immutable device event.
func (r *PostgresRepository) SaveEvent(ctx context.Context, event *edge_devices.DeviceEvent) error {
	detailsJSON, err := json.Marshal(event.Details)
	if err != nil {
		detailsJSON = []byte("{}")
	}

	query := `
		INSERT INTO device_events (id, device_id, tenant_id, check_type, checked_at, overall_status,
		                           summary, details, user_id, user_email)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`

	_, err = r.pool.Exec(ctx, query,
		event.ID, event.DeviceID, event.TenantID, event.CheckType, event.CheckedAt,
		event.OverallStatus, event.Summary, detailsJSON, event.UserID, event.UserEmail)

	return err
}

// ListEvents returns all events for a device, ordered newest-first.
func (r *PostgresRepository) ListEvents(ctx context.Context, tenantID, deviceID uuid.UUID) ([]*edge_devices.DeviceEvent, error) {
	query := `
		SELECT id, device_id, tenant_id, check_type, checked_at, overall_status, summary, details, user_id, user_email
		FROM device_events
		WHERE device_id = $1 AND tenant_id = $2
		ORDER BY checked_at DESC
	`

	rows, err := r.pool.Query(ctx, query, deviceID, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []*edge_devices.DeviceEvent
	for rows.Next() {
		event, err := scanDeviceEvent(rows)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return events, nil
}

// Helper functions

type scanner interface {
	Scan(dest ...interface{}) error
}

func scanEdgeDevice(s scanner) (*edge_devices.EdgeDevice, error) {
	device := &edge_devices.EdgeDevice{}
	err := s.Scan(
		&device.ID, &device.TenantID, &device.Name, &device.Description, &device.MachineID,
		&device.EdgeType, &device.RaspberryBaseURL, &device.PLCAddress, &device.Status,
		&device.LastSeenAt, &device.LastHealthCheckAt, &device.LastHealthStatus,
		&device.LastHealthSummary, &device.CreatedAt, &device.UpdatedAt)
	return device, err
}

func scanDeviceEvent(s scanner) (*edge_devices.DeviceEvent, error) {
	event := &edge_devices.DeviceEvent{}
	var detailsJSON []byte

	err := s.Scan(
		&event.ID, &event.DeviceID, &event.TenantID, &event.CheckType, &event.CheckedAt,
		&event.OverallStatus, &event.Summary, &detailsJSON, &event.UserID, &event.UserEmail)

	if err == nil && detailsJSON != nil {
		if err := json.Unmarshal(detailsJSON, &event.Details); err != nil {
			event.Details = make(map[string]interface{})
		}
	} else {
		event.Details = make(map[string]interface{})
	}

	return event, err
}
