package models

// TaskUpdateRequest define la estructura para actualizar una tarea (con campos opcionales)
type TaskUpdateRequest struct {
	Title       *string `json:"title"`
	Description *string `json:"description"`
	Completed   *bool   `json:"completed"`
}
