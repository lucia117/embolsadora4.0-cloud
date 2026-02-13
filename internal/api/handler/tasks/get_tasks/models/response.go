package models

// TaskResponse define la estructura de respuesta para las tareas
type TaskResponse struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Completed   bool   `json:"completed"`
}

// Response define la estructura de respuesta para listar múltiples tareas
type Response struct {
	Tasks []TaskResponse `json:"tasks"`
}
