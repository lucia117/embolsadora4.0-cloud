package models

// TaskResponse define la estructura de respuesta para las tareas
type TaskResponse struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Completed   bool   `json:"completed"`
}

// TaskResponseSingle define la estructura de respuesta para una sola tarea
type TaskResponseSingle struct {
	Task TaskResponse `json:"task"`
}
