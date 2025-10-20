package pg

import (
    "context"

    "github.com/tu-org/embolsadora-api/internal/domain"
    "github.com/tu-org/embolsadora-api/internal/platform"
)

// EventsRepo provides access to events storage.
type EventsRepo struct{
    // TODO: hold pgx pool/conn here when wiring real DB.
}

// BatchInsert ingests events in batch for the tenant in context.
// TODO: Use COPY or batched INSERTs, ensure idempotency constraints, proper indexes on (tenant_id, machine_id, ts), and partitions with 90d retention.
func (r *EventsRepo) BatchInsert(ctx context.Context /* items []EventItem */) error {
    if platform.TenantID(ctx) == "" {
        return domain.ErrForbidden
    }
    // TODO: INSERT/COPY into events (tenant_id, machine_id, ts, seq, kind, schema_version, payload JSONB)
    return nil
}
