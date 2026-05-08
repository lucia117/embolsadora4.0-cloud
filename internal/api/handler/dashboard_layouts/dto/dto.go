package dto

import (
	"time"

	"github.com/google/uuid"
	domain "github.com/tu-org/embolsadora-api/internal/domain/dashboard_layouts"
)

// PositionDTO represents the grid position of a widget.
type PositionDTO struct {
	X int    `json:"x"`
	Y int    `json:"y"`
	W int    `json:"w"`
	H int    `json:"h"`
	I string `json:"i"`
}

// WidgetDTO represents a widget in JSON responses and requests.
type WidgetDTO struct {
	ID          string      `json:"id"`
	Type        string      `json:"type"`
	Name        string      `json:"name"`
	Title       string      `json:"title"`
	Description string      `json:"description"`
	Category    string      `json:"category"`
	Icon        string      `json:"icon"`
	Position    PositionDTO `json:"position"`
}

// LayoutDTO represents a dashboard layout in JSON responses.
type LayoutDTO struct {
	ID        uuid.UUID   `json:"id"`
	Name      string      `json:"name"`
	Widgets   []WidgetDTO `json:"widgets"`
	CreatedAt time.Time   `json:"createdAt"`
	UpdatedAt time.Time   `json:"updatedAt"`
}

// MetaDTO carries pagination/limit metadata for list responses.
type MetaDTO struct {
	Total int `json:"total"`
	Limit int `json:"limit"`
}

// ListLayoutsResponse is the response for GET /dashboard-layouts.
type ListLayoutsResponse struct {
	Data []LayoutDTO `json:"data"`
	Meta MetaDTO     `json:"meta"`
}

// CreateLayoutRequest is the request body for POST /dashboard-layouts.
type CreateLayoutRequest struct {
	Name    string      `json:"name" binding:"required"`
	Widgets []WidgetDTO `json:"widgets"`
}

// UpdateLayoutRequest is the request body for PUT /dashboard-layouts/:id.
type UpdateLayoutRequest struct {
	Name    string      `json:"name" binding:"required"`
	Widgets []WidgetDTO `json:"widgets"`
}

// ToLayoutDTO converts a domain DashboardLayout to a LayoutDTO.
func ToLayoutDTO(layout *domain.DashboardLayout) LayoutDTO {
	widgets := make([]WidgetDTO, len(layout.Widgets))
	for i, w := range layout.Widgets {
		widgets[i] = WidgetDTO{
			ID:          w.ID,
			Type:        w.Type,
			Name:        w.Name,
			Title:       w.Title,
			Description: w.Description,
			Category:    w.Category,
			Icon:        w.Icon,
			Position: PositionDTO{
				X: w.Position.X,
				Y: w.Position.Y,
				W: w.Position.W,
				H: w.Position.H,
				I: w.Position.I,
			},
		}
	}
	return LayoutDTO{
		ID:        layout.ID,
		Name:      layout.Name,
		Widgets:   widgets,
		CreatedAt: layout.CreatedAt,
		UpdatedAt: layout.UpdatedAt,
	}
}

// ToWidgetsDomain converts a slice of WidgetDTO to domain Widgets.
func ToWidgetsDomain(dtos []WidgetDTO) []domain.Widget {
	if dtos == nil {
		return []domain.Widget{}
	}
	widgets := make([]domain.Widget, len(dtos))
	for i, d := range dtos {
		widgets[i] = domain.Widget{
			ID:          d.ID,
			Type:        d.Type,
			Name:        d.Name,
			Title:       d.Title,
			Description: d.Description,
			Category:    d.Category,
			Icon:        d.Icon,
			Position: domain.Position{
				X: d.Position.X,
				Y: d.Position.Y,
				W: d.Position.W,
				H: d.Position.H,
				I: d.Position.I,
			},
		}
	}
	return widgets
}
