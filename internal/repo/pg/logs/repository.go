package logs

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/tu-org/embolsadora-api/internal/domain"
)

// Repository defines the persistence interface for log entries.
type Repository interface {
	Write(ctx context.Context, entry *domain.LogEntry) (*domain.LogEntry, error)
	List(ctx context.Context, params ListParams) ([]domain.LogEntry, int, error)
	Get(ctx context.Context, tenantID, id uuid.UUID) (*domain.LogEntry, error)
	GetContext(ctx context.Context, tenantID, id uuid.UUID, windowSize int) ([]domain.LogEntry, *domain.LogEntry, []domain.LogEntry, error)
	Export(ctx context.Context, params ExportParams) ([]domain.LogEntry, int, error)
	GetRetention(ctx context.Context, tenantID uuid.UUID) (*domain.RetentionPolicy, error)
	UpsertRetention(ctx context.Context, policy *domain.RetentionPolicy) (*domain.RetentionPolicy, error)
}

// ListParams holds filter and pagination parameters for listing logs.
type ListParams struct {
	TenantID  uuid.UUID
	EventType string
	Severity  string
	MachineID *uuid.UUID
	From      *time.Time
	To        *time.Time
	Q         string
	Cursor    string
	Limit     int
}

// ExportParams holds filter parameters for exporting logs (no cursor, hard limit).
type ExportParams struct {
	TenantID  uuid.UUID
	EventType string
	Severity  string
	MachineID *uuid.UUID
	From      *time.Time
	To        *time.Time
	Q         string
	MaxRows   int
}

// cursorData is the internal structure encoded into the pagination cursor.
type cursorData struct {
	CreatedAt time.Time `json:"created_at"`
	ID        uuid.UUID `json:"id"`
}

// encodeCursor creates an opaque base64 cursor from a log entry.
func encodeCursor(entry domain.LogEntry) string {
	data, _ := json.Marshal(cursorData{CreatedAt: entry.CreatedAt, ID: entry.ID})
	return base64.StdEncoding.EncodeToString(data)
}

// decodeCursor parses an opaque cursor into its components.
func decodeCursor(cursor string) (*cursorData, error) {
	raw, err := base64.StdEncoding.DecodeString(cursor)
	if err != nil {
		return nil, domain.ErrInvalidCursor
	}
	var cd cursorData
	if err := json.Unmarshal(raw, &cd); err != nil {
		return nil, domain.ErrInvalidCursor
	}
	return &cd, nil
}

// pgRepository is the PostgreSQL implementation of Repository.
type pgRepository struct {
	db *pgxpool.Pool
}

// New creates a new PostgreSQL log repository.
func New(db *pgxpool.Pool) Repository {
	return &pgRepository{db: db}
}

func (r *pgRepository) Write(ctx context.Context, entry *domain.LogEntry) (*domain.LogEntry, error) {
	sql := `
		INSERT INTO log_entries (tenant_id, severity, event_type, source_id, machine_id, message, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, tenant_id, created_at, severity, event_type, source_id, machine_id, message, metadata`

	row := r.db.QueryRow(ctx, sql,
		entry.TenantID, entry.Severity, entry.EventType,
		entry.SourceID, entry.MachineID, entry.Message, entry.Metadata,
	)
	result, err := scanRow(row)
	if err != nil {
		return nil, fmt.Errorf("logs write: %w", err)
	}
	return result, nil
}

func (r *pgRepository) List(ctx context.Context, params ListParams) ([]domain.LogEntry, int, error) {
	limit := params.Limit
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	where, args := buildFilterClauses(params.TenantID, params.EventType, params.Severity, params.MachineID, params.From, params.To, params.Q)

	// Apply cursor for keyset pagination
	if params.Cursor != "" {
		cd, err := decodeCursor(params.Cursor)
		if err != nil {
			return nil, 0, err
		}
		n := len(args) + 1
		where = append(where, fmt.Sprintf("(created_at, id) < ($%d, $%d)", n, n+1))
		args = append(args, cd.CreatedAt, cd.ID)
	}

	whereClause := ""
	if len(where) > 0 {
		whereClause = "WHERE " + strings.Join(where, " AND ")
	}

	// Count query
	countSQL := fmt.Sprintf(`SELECT COUNT(*) FROM log_entries %s`, whereClause)
	var total int
	if err := r.db.QueryRow(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("logs count: %w", err)
	}

	// Data query
	n := len(args) + 1
	dataSQL := fmt.Sprintf(`
		SELECT id, tenant_id, created_at, severity, event_type, source_id, machine_id, message, metadata
		FROM log_entries
		%s
		ORDER BY created_at DESC, id DESC
		LIMIT $%d`, whereClause, n)
	args = append(args, limit)

	rows, err := r.db.Query(ctx, dataSQL, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("logs list: %w", err)
	}
	defer rows.Close()

	entries, err := scanRows(rows)
	if err != nil {
		return nil, 0, err
	}
	return entries, total, nil
}

func (r *pgRepository) Get(ctx context.Context, tenantID, id uuid.UUID) (*domain.LogEntry, error) {
	sql := `
		SELECT id, tenant_id, created_at, severity, event_type, source_id, machine_id, message, metadata
		FROM log_entries
		WHERE tenant_id = $1 AND id = $2`

	row := r.db.QueryRow(ctx, sql, tenantID, id)
	entry, err := scanRow(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, domain.ErrLogNotFound
		}
		return nil, fmt.Errorf("logs get: %w", err)
	}
	return entry, nil
}

func (r *pgRepository) GetContext(ctx context.Context, tenantID, id uuid.UUID, windowSize int) ([]domain.LogEntry, *domain.LogEntry, []domain.LogEntry, error) {
	anchor, err := r.Get(ctx, tenantID, id)
	if err != nil {
		return nil, nil, nil, err
	}

	// Events before anchor (older), ordered newest-first then reversed for chronological output
	beforeSQL := `
		SELECT id, tenant_id, created_at, severity, event_type, source_id, machine_id, message, metadata
		FROM log_entries
		WHERE tenant_id = $1 AND (created_at, id) < ($2, $3)
		ORDER BY created_at DESC, id DESC
		LIMIT $4`

	beforeRows, err := r.db.Query(ctx, beforeSQL, tenantID, anchor.CreatedAt, anchor.ID, windowSize)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("logs context before: %w", err)
	}
	defer beforeRows.Close()
	before, err := scanRows(beforeRows)
	if err != nil {
		return nil, nil, nil, err
	}
	// Reverse to chronological order
	for i, j := 0, len(before)-1; i < j; i, j = i+1, j-1 {
		before[i], before[j] = before[j], before[i]
	}

	// Events after anchor (newer), chronological order
	afterSQL := `
		SELECT id, tenant_id, created_at, severity, event_type, source_id, machine_id, message, metadata
		FROM log_entries
		WHERE tenant_id = $1 AND (created_at, id) > ($2, $3)
		ORDER BY created_at ASC, id ASC
		LIMIT $4`

	afterRows, err := r.db.Query(ctx, afterSQL, tenantID, anchor.CreatedAt, anchor.ID, windowSize)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("logs context after: %w", err)
	}
	defer afterRows.Close()
	after, err := scanRows(afterRows)
	if err != nil {
		return nil, nil, nil, err
	}

	return before, anchor, after, nil
}

func (r *pgRepository) Export(ctx context.Context, params ExportParams) ([]domain.LogEntry, int, error) {
	maxRows := params.MaxRows
	if maxRows <= 0 {
		maxRows = 50000
	}

	where, args := buildFilterClauses(params.TenantID, params.EventType, params.Severity, params.MachineID, params.From, params.To, params.Q)

	whereClause := ""
	if len(where) > 0 {
		whereClause = "WHERE " + strings.Join(where, " AND ")
	}

	countSQL := fmt.Sprintf(`SELECT COUNT(*) FROM log_entries %s`, whereClause)
	var total int
	if err := r.db.QueryRow(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("logs export count: %w", err)
	}

	n := len(args) + 1
	dataSQL := fmt.Sprintf(`
		SELECT id, tenant_id, created_at, severity, event_type, source_id, machine_id, message, metadata
		FROM log_entries
		%s
		ORDER BY created_at DESC, id DESC
		LIMIT $%d`, whereClause, n)
	args = append(args, maxRows)

	rows, err := r.db.Query(ctx, dataSQL, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("logs export: %w", err)
	}
	defer rows.Close()

	entries, err := scanRows(rows)
	if err != nil {
		return nil, 0, err
	}
	return entries, total, nil
}

func (r *pgRepository) GetRetention(ctx context.Context, tenantID uuid.UUID) (*domain.RetentionPolicy, error) {
	sql := `SELECT tenant_id, retention_days, updated_at, next_purge_at FROM log_retention_policies WHERE tenant_id = $1`
	row := r.db.QueryRow(ctx, sql, tenantID)

	var p domain.RetentionPolicy
	err := row.Scan(&p.TenantID, &p.RetentionDays, &p.UpdatedAt, &p.NextPurgeAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, domain.ErrRetentionNotFound
		}
		return nil, fmt.Errorf("logs get retention: %w", err)
	}
	return &p, nil
}

func (r *pgRepository) UpsertRetention(ctx context.Context, policy *domain.RetentionPolicy) (*domain.RetentionPolicy, error) {
	sql := `
		INSERT INTO log_retention_policies (tenant_id, retention_days, next_purge_at)
		VALUES ($1, $2, NOW() + INTERVAL '1 day')
		ON CONFLICT (tenant_id) DO UPDATE
		SET retention_days = EXCLUDED.retention_days,
		    next_purge_at  = NOW() + INTERVAL '1 day'
		RETURNING tenant_id, retention_days, updated_at, next_purge_at`

	var p domain.RetentionPolicy
	err := r.db.QueryRow(ctx, sql, policy.TenantID, policy.RetentionDays).
		Scan(&p.TenantID, &p.RetentionDays, &p.UpdatedAt, &p.NextPurgeAt)
	if err != nil {
		return nil, fmt.Errorf("logs upsert retention: %w", err)
	}
	return &p, nil
}

// buildFilterClauses constructs WHERE clause fragments and args for shared filters.
func buildFilterClauses(tenantID uuid.UUID, eventType, severity string, machineID *uuid.UUID, from, to *time.Time, q string) ([]string, []any) {
	var where []string
	var args []any

	n := func() int { return len(args) + 1 }

	where = append(where, fmt.Sprintf("tenant_id = $%d", n()))
	args = append(args, tenantID)

	if eventType != "" {
		where = append(where, fmt.Sprintf("event_type = $%d", n()))
		args = append(args, eventType)
	}
	if severity != "" {
		where = append(where, fmt.Sprintf("severity = $%d", n()))
		args = append(args, severity)
	}
	if machineID != nil {
		where = append(where, fmt.Sprintf("machine_id = $%d", n()))
		args = append(args, machineID)
	}
	if from != nil {
		where = append(where, fmt.Sprintf("created_at >= $%d", n()))
		args = append(args, from)
	}
	if to != nil {
		where = append(where, fmt.Sprintf("created_at <= $%d", n()))
		args = append(args, to)
	}
	if q != "" {
		where = append(where, fmt.Sprintf("to_tsvector('spanish', message) @@ plainto_tsquery('spanish', $%d)", n()))
		args = append(args, q)
	}

	return where, args
}

// scanRows scans multiple rows into a slice of LogEntry.
func scanRows(rows pgx.Rows) ([]domain.LogEntry, error) {
	var entries []domain.LogEntry
	for rows.Next() {
		entry, err := scanRow(rows)
		if err != nil {
			return nil, err
		}
		entries = append(entries, *entry)
	}
	return entries, rows.Err()
}

// scanRow scans a single row into a LogEntry. Works with both pgx.Row and pgx.Rows.
func scanRow(row interface {
	Scan(dest ...any) error
}) (*domain.LogEntry, error) {
	var e domain.LogEntry
	var metadata []byte
	err := row.Scan(&e.ID, &e.TenantID, &e.CreatedAt, &e.Severity, &e.EventType, &e.SourceID, &e.MachineID, &e.Message, &metadata)
	if err != nil {
		return nil, err
	}
	if len(metadata) > 0 {
		if err := json.Unmarshal(metadata, &e.Metadata); err != nil {
			e.Metadata = map[string]any{}
		}
	} else {
		e.Metadata = map[string]any{}
	}
	return &e, nil
}

// EncodeCursor is exported for use by the service layer.
func EncodeCursor(entry domain.LogEntry) string {
	return encodeCursor(entry)
}
