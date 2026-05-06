package notifications

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/tu-org/embolsadora-api/internal/domain"
)

// ListParams contiene los filtros opcionales y parámetros de paginación para List.
type ListParams struct {
	Status   *string
	Severity *string
	Limit    int
	Offset   int
}

// Repository define las operaciones de persistencia para notificaciones.
type Repository interface {
	List(ctx context.Context, tenantID uuid.UUID, params ListParams) ([]*domain.Notification, int, error)
	CountUnread(ctx context.Context, tenantID uuid.UUID) (int, error)
	GetByID(ctx context.Context, id uuid.UUID, tenantID uuid.UUID) (*domain.Notification, error)
	Ack(ctx context.Context, id uuid.UUID, tenantID uuid.UUID) (*domain.Notification, error)
	Close(ctx context.Context, id uuid.UUID, tenantID uuid.UUID) (*domain.Notification, error)
}

// PostgresRepository implementa Repository usando PostgreSQL.
type PostgresRepository struct {
	pool *pgxpool.Pool
}

// New crea un nuevo repositorio PostgreSQL para notificaciones.
func New(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

// List devuelve notificaciones del tenant con filtros opcionales y paginación.
// Retorna la lista, el total de registros que coinciden (sin paginación) y un error.
func (r *PostgresRepository) List(ctx context.Context, tenantID uuid.UUID, params ListParams) ([]*domain.Notification, int, error) {
	const countQuery = `
		SELECT COUNT(*)
		FROM notifications
		WHERE tenant_id = $1
		  AND ($2::text IS NULL OR status = $2)
		  AND ($3::text IS NULL OR severity = $3)
	`
	var total int
	if err := r.pool.QueryRow(ctx, countQuery, tenantID, params.Status, params.Severity).Scan(&total); err != nil {
		return nil, 0, err
	}

	const listQuery = `
		SELECT id, tenant_id, title, message, severity, status,
		       alarm_rule_id, machine_id, created_at, acknowledged_at, closed_at
		FROM notifications
		WHERE tenant_id = $1
		  AND ($2::text IS NULL OR status = $2)
		  AND ($3::text IS NULL OR severity = $3)
		ORDER BY created_at DESC
		LIMIT $4 OFFSET $5
	`
	rows, err := r.pool.Query(ctx, listQuery, tenantID, params.Status, params.Severity, params.Limit, params.Offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var items []*domain.Notification
	for rows.Next() {
		n, err := scanNotification(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, n)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	if items == nil {
		items = []*domain.Notification{}
	}
	return items, total, nil
}

// CountUnread retorna el número de notificaciones con status='unread' del tenant.
func (r *PostgresRepository) CountUnread(ctx context.Context, tenantID uuid.UUID) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM notifications WHERE tenant_id = $1 AND status = 'unread'`,
		tenantID,
	).Scan(&count)
	return count, err
}

// GetByID devuelve una notificación por ID verificando que pertenezca al tenant.
func (r *PostgresRepository) GetByID(ctx context.Context, id uuid.UUID, tenantID uuid.UUID) (*domain.Notification, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, tenant_id, title, message, severity, status,
		       alarm_rule_id, machine_id, created_at, acknowledged_at, closed_at
		FROM notifications
		WHERE id = $1 AND tenant_id = $2
	`, id, tenantID)

	n, err := scanNotification(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotificationNotFound
		}
		return nil, err
	}
	return n, nil
}

// Ack marca una notificación como acknowledged de forma idempotente.
// Si ya está acknowledged o closed, retorna la notificación sin modificarla.
func (r *PostgresRepository) Ack(ctx context.Context, id uuid.UUID, tenantID uuid.UUID) (*domain.Notification, error) {
	row := r.pool.QueryRow(ctx, `
		UPDATE notifications
		SET status          = CASE WHEN status = 'unread' THEN 'acknowledged' ELSE status END,
		    acknowledged_at = CASE WHEN status = 'unread' THEN NOW() ELSE acknowledged_at END
		WHERE id = $1 AND tenant_id = $2
		RETURNING id, tenant_id, title, message, severity, status,
		          alarm_rule_id, machine_id, created_at, acknowledged_at, closed_at
	`, id, tenantID)

	n, err := scanNotification(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotificationNotFound
		}
		return nil, err
	}
	return n, nil
}

// Close marca una notificación como closed de forma idempotente.
// Puede aplicarse desde cualquier estado; si ya está closed, retorna sin modificar.
func (r *PostgresRepository) Close(ctx context.Context, id uuid.UUID, tenantID uuid.UUID) (*domain.Notification, error) {
	row := r.pool.QueryRow(ctx, `
		UPDATE notifications
		SET status    = 'closed',
		    closed_at = CASE WHEN status != 'closed' THEN NOW() ELSE closed_at END
		WHERE id = $1 AND tenant_id = $2
		RETURNING id, tenant_id, title, message, severity, status,
		          alarm_rule_id, machine_id, created_at, acknowledged_at, closed_at
	`, id, tenantID)

	n, err := scanNotification(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotificationNotFound
		}
		return nil, err
	}
	return n, nil
}

// scanNotification escanea una fila de la tabla notifications.
func scanNotification(row interface {
	Scan(dest ...any) error
}) (*domain.Notification, error) {
	var n domain.Notification
	err := row.Scan(
		&n.ID,
		&n.TenantID,
		&n.Title,
		&n.Message,
		&n.Severity,
		&n.Status,
		&n.AlarmRuleID,
		&n.MachineID,
		&n.CreatedAt,
		&n.AcknowledgedAt,
		&n.ClosedAt,
	)
	if err != nil {
		return nil, err
	}
	return &n, nil
}
