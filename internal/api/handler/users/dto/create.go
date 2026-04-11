package dto

// CreateUserRequest represents a request to create a new user
type CreateUserRequest struct {
	FirstName string  `json:"firstName" binding:"required,max=100"`
	LastName  string  `json:"lastName" binding:"required,max=100"`
	Email     string  `json:"email" binding:"required,email"`
	Role      string  `json:"role" binding:"required,max=50"`
	Image     *string `json:"image"`
}

// CreateUserResponse is the same as UserResponse (uses list.go)
type CreateUserResponse = UserResponse
