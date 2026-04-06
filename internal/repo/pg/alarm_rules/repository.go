package alarm_rules

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/tu-org/embolsadora-api/internal/domain"
)

// Repository define las operaciones de persistencia para reglas de alarma.
type Repository interface {
	List(ctx context.Context, tenantID uuid.UUID) ([]*domain.AlarmRule, error)
	GetByID(ctx context.Context, id uuid.UUID, tenantID uuid.UUID) (*domain.AlarmRule, error)
	Create(ctx context.Context, rule *domain.AlarmRule) error
	Update(ctx context.Context, rule *domain.AlarmRule) error
	Delete(ctx context.Context, id uuid.UUID, tenantID uuid.UUID) error
}

// PostgresRepository implementa Repository usando PostgreSQL.
type PostgresRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresRepository crea un nuevo repositorio PostgreSQL para alarm_rules.
func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

// List devuelve todas las reglas de alarma activas del tenant.
func (r *PostgresRepository) List(ctx context.Context, tenantID uuid.UUID) ([]*domain.AlarmRule, error) {
	query := `
		SELECT id, tenant_id, name, description, metric, operator, threshold, severity, enabled, created_at, updated_at
		FROM alarm_rules
		WHERE tenant_id = $1
		ORDER BY created_at ASC
	`
	rows, err := r.pool.Query(ctx, query, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []*domain.AlarmRule
	for rows.Next() {
		rule, err := scanAlarmRule(rows)
		if err != nil {
			return nil, err
		}
		rules = append(rules, rule)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if rules == nil {
		rules = []*domain.AlarmRule{}
	}
	return rules, nil
}

// GetByID devuelve una regla por ID, verificando que pertenezca al tenant.
func (r *PostgresRepository) GetByID(ctx context.Context, id uuid.UUID, tenantID uuid.UUID) (*domain.AlarmRule, error) {
	query := `
		SELECT id, tenant_id, name, description, metric, operator, threshold, severity, enabled, created_at, updated_at
		FROM alarm_rules
		WHERE id = $1 AND tenant_id = $2
	`
	row := r.pool.QueryRow(ctx, query, id, tenantID)
	rule, err := scanAlarmRule(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrAlarmRuleNotFound
		}
		return nil, err
	}
	return rule, nil
}

// Create persiste una nueva regla de alarma.
func (r *PostgresRepository) Create(ctx context.Context, rule *domain.AlarmRule) error {
	return r.pool.QueryRow(ctx, `
		INSERT INTO alarm_rules (tenant_id, name, description, metric, operator, threshold, severity, enabled)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, created_at, updated_at
	`,
		rule.TenantID,
		rule.Name,
		nullableString(rule.Description),
		rule.Metric,
		rule.Operator,
		rule.Threshold,
		rule.Severity,
		rule.Enabled,
	).Scan(&rule.ID, &rule.CreatedAt, &rule.UpdatedAt)
}

// Update actualiza los campos modificables de una regla.
func (r *PostgresRepository) Update(ctx context.Context, rule *domain.AlarmRule) error {
	err := r.pool.QueryRow(ctx, `
		UPDATE alarm_rules
		SET name = $1, description = $2, metric = $3, operator = $4,
		    threshold = $5, severity = $6, enabled = $7, updated_at = NOW()
		WHERE id = $8 AND tenant_id = $9
		RETURNING updated_at
	`,
		rule.Name,
		nullableString(rule.Description),
		rule.Metric,
		rule.Operator,
		rule.Threshold,
		rule.Severity,
		rule.Enabled,
		rule.ID,
		rule.TenantID,
	).Scan(&rule.UpdatedAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrAlarmRuleNotFound
		}
		return err
	}
	return nil
}

// Delete elimina permanentemente una regla de alarma del tenant.
func (r *PostgresRepository) Delete(ctx context.Context, id uuid.UUID, tenantID uuid.UUID) error {
	result, err := r.pool.Exec(ctx,
		`DELETE FROM alarm_rules WHERE id = $1 AND tenant_id = $2`,
		id, tenantID,
	)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return domain.ErrAlarmRuleNotFound
	}
	return nil
}

// scanAlarmRule escanea una fila de alarm_rules.
func scanAlarmRule(row interface {
	Scan(dest ...any) error
}) (*domain.AlarmRule, error) {
	var rule domain.AlarmRule
	var description *string

	err := row.Scan(
		&rule.ID,
		&rule.TenantID,
		&rule.Name,
		&description,
		&rule.Metric,
		&rule.Operator,
		&rule.Threshold,
		&rule.Severity,
		&rule.Enabled,
		&rule.CreatedAt,
		&rule.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if description != nil {
		rule.Description = *description
	}
	return &rule, nil
}

// nullableString convierte un string vacío a nil para columnas TEXT nullable.
func nullableString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
