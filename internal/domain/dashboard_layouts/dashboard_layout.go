package dashboard_layouts

import (
	"time"

	"github.com/google/uuid"
)

// MaxLayoutsPerTenant is the maximum number of active layouts allowed per tenant.
const MaxLayoutsPerTenant = 3

// DashboardLayout represents a named dashboard configuration for a (tenant, user) pair.
type DashboardLayout struct {
	ID        uuid.UUID
	TenantID  uuid.UUID
	UserID    uuid.UUID
	Name      string
	Widgets   []Widget
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time
}

// Widget represents a visual component within a dashboard layout.
type Widget struct {
	ID          string   `json:"id"`
	Type        string   `json:"type"`
	Name        string   `json:"name"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Category    string   `json:"category"`
	Icon        string   `json:"icon"`
	Position    Position `json:"position"`
}

// Position defines the grid coordinates and dimensions of a widget.
type Position struct {
	X int    `json:"x"`
	Y int    `json:"y"`
	W int    `json:"w"`
	H int    `json:"h"`
	I string `json:"i"`
}
