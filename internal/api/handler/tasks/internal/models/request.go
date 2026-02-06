package models

// TaskRequest define la estructura de la solicitud para crear/actualizar una tarea
type TaskRequest struct {
	Title       string `json:"title" binding:"required"`
	Description string `json:"description"`
	Completed   bool   `json:"completed"`
}

// TaskUpdateRequest define la estructura para actualizar una tarea (con campos opcionales)
type TaskUpdateRequest struct {
	Title       *string `json:"title"`
	Description *string `json:"description"`
	Completed   *bool   `json:"completed"`
}
