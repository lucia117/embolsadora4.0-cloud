// Package logwriter defines the write path for log entries.
// Log entries are written internally (e.g., by event processors or alarm handlers),
// never via a public HTTP endpoint.
package logwriter

import (
	"context"

	"github.com/tu-org/embolsadora-api/internal/domain"
)

// LogWriter is the interface for writing log entries into the system.
// Implementations may write directly to PostgreSQL, buffer to a queue, etc.
type LogWriter interface {
	Write(ctx context.Context, entry *domain.LogEntry) error
}
