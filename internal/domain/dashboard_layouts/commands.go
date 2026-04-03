package dashboard_layouts

// CreateLayoutCommand carries the input data for creating a new dashboard layout.
type CreateLayoutCommand struct {
	Name    string
	Widgets []Widget
}

// UpdateLayoutCommand carries the input data for updating an existing dashboard layout.
type UpdateLayoutCommand struct {
	Name    string
	Widgets []Widget
}
