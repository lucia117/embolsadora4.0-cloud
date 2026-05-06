package dto

// UpdateUserRequest represents a request to update a user
type UpdateUserRequest struct {
	FirstName *string `json:"firstName" binding:"omitempty,max=100"`
	LastName  *string `json:"lastName" binding:"omitempty,max=100"`
	Role      *string `json:"role" binding:"omitempty"`
	Image     *string `json:"image"`
}

// UpdateUserResponse is the same as UserResponse
type UpdateUserResponse = UserResponse
