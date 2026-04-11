package dto

// UpdateUserStatusRequest is the request body for PATCH /users/:id/status.
type UpdateUserStatusRequest struct {
	Status string `json:"status" binding:"required"`
}
