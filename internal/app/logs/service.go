package logs

import (
	"context"
	"sync"

	"github.com/google/uuid"
	"github.com/tu-org/embolsadora-api/internal/domain"
	logsRepo "github.com/tu-org/embolsadora-api/internal/repo/pg/logs"
	"go.uber.org/zap"
)

// Service handles business logic for log operations.
type Service struct {
	repo   logsRepo.Repository
	log    *zap.Logger
	mu     sync.RWMutex
	subs   map[uuid.UUID][]chan *domain.LogEntry
}

// New creates a new logs Service.
func New(repo logsRepo.Repository, log *zap.Logger) *Service {
	return &Service{
		repo: repo,
		log:  log,
		subs: make(map[uuid.UUID][]chan *domain.LogEntry),
	}
}

// ListResult holds the paginated result of a list operation.
type ListResult struct {
	Entries    []domain.LogEntry
	Total      int
	NextCursor *string
}

// List returns a paginated list of log entries for the given tenant.
func (s *Service) List(ctx context.Context, params logsRepo.ListParams) (*ListResult, error) {
	if params.Limit <= 0 || params.Limit > 100 {
		params.Limit = 50
	}

	entries, total, err := s.repo.List(ctx, params)
	if err != nil {
		return nil, err
	}

	var nextCursor *string
	if len(entries) == params.Limit {
		c := logsRepo.EncodeCursor(entries[len(entries)-1])
		nextCursor = &c
	}

	return &ListResult{Entries: entries, Total: total, NextCursor: nextCursor}, nil
}

// Get returns a single log entry by ID, enforcing tenant isolation.
func (s *Service) Get(ctx context.Context, tenantID, id uuid.UUID) (*domain.LogEntry, error) {
	return s.repo.Get(ctx, tenantID, id)
}

// GetContext returns a window of log entries around the given anchor log.
func (s *Service) GetContext(ctx context.Context, tenantID, id uuid.UUID, windowSize int) ([]domain.LogEntry, *domain.LogEntry, []domain.LogEntry, error) {
	if windowSize <= 0 || windowSize > 50 {
		windowSize = 10
	}
	return s.repo.GetContext(ctx, tenantID, id, windowSize)
}

// ExportResult holds the result of an export operation.
type ExportResult struct {
	Entries        []domain.LogEntry
	TotalAvailable int
	Truncated      bool
}

// Export returns log entries for export, capped at 50,000 rows.
func (s *Service) Export(ctx context.Context, params logsRepo.ExportParams) (*ExportResult, error) {
	const maxExport = 50000
	params.MaxRows = maxExport + 1 // fetch one extra to detect truncation

	entries, total, err := s.repo.Export(ctx, params)
	if err != nil {
		return nil, err
	}

	truncated := len(entries) > maxExport
	if truncated {
		entries = entries[:maxExport]
	}

	return &ExportResult{
		Entries:        entries,
		TotalAvailable: total,
		Truncated:      truncated,
	}, nil
}

// GetRetention returns the retention policy for a tenant (defaults to 90 days if not set).
func (s *Service) GetRetention(ctx context.Context, tenantID uuid.UUID) (*domain.RetentionPolicy, error) {
	policy, err := s.repo.GetRetention(ctx, tenantID)
	if err == domain.ErrRetentionNotFound {
		// Return default policy without persisting
		return &domain.RetentionPolicy{
			TenantID:      tenantID,
			RetentionDays: 90,
		}, nil
	}
	return policy, err
}

// UpdateRetention upserts the retention policy for a tenant.
func (s *Service) UpdateRetention(ctx context.Context, tenantID uuid.UUID, retentionDays int) (*domain.RetentionPolicy, error) {
	if retentionDays < 1 || retentionDays > 3650 {
		return nil, domain.ErrInvalidRetentionDays
	}
	return s.repo.UpsertRetention(ctx, &domain.RetentionPolicy{
		TenantID:      tenantID,
		RetentionDays: retentionDays,
	})
}

// Subscribe registers a channel to receive new log entries for a tenant.
// The caller must call Unsubscribe when done.
func (s *Service) Subscribe(tenantID uuid.UUID) chan *domain.LogEntry {
	ch := make(chan *domain.LogEntry, 64)
	s.mu.Lock()
	s.subs[tenantID] = append(s.subs[tenantID], ch)
	s.mu.Unlock()
	return ch
}

// Unsubscribe removes the channel from the tenant's subscriber list.
func (s *Service) Unsubscribe(tenantID uuid.UUID, ch chan *domain.LogEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	channels := s.subs[tenantID]
	for i, c := range channels {
		if c == ch {
			s.subs[tenantID] = append(channels[:i], channels[i+1:]...)
			close(ch)
			return
		}
	}
}

// Write persists a new log entry and publishes it to SSE subscribers.
// Implements the logwriter.LogWriter interface.
func (s *Service) Write(ctx context.Context, entry *domain.LogEntry) error {
	persisted, err := s.repo.Write(ctx, entry)
	if err != nil {
		return err
	}
	s.Publish(persisted.TenantID, persisted)
	return nil
}

// Publish sends a log entry to all subscribers of a tenant.
// Called by the write path (future use or internal event processing).
func (s *Service) Publish(tenantID uuid.UUID, entry *domain.LogEntry) {
	s.mu.RLock()
	channels := s.subs[tenantID]
	s.mu.RUnlock()
	for _, ch := range channels {
		select {
		case ch <- entry:
		default:
			// Drop if subscriber is too slow
		}
	}
}
