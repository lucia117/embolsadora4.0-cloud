package models

// TaskRequest define la estructura de la solicitud para crear una tarea
type TaskRequest struct {
	Title       string `json:"title" binding:"required"`
	Description string `json:"description"`
	Completed   bool   `json:"completed"`
}
