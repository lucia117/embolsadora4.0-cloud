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

// UpdateMeRequest represents a self-service profile update request.
// Deliberadamente no tiene campo Role: a diferencia de UpdateUserRequest (que sí lo
// tiene, protegido por RBAC + EnsureAssignable), este DTO alimenta un endpoint sin
// RBAC — la ausencia del campo, no una validación, es lo que impide la escalada.
type UpdateMeRequest struct {
	FirstName *string `json:"firstName" binding:"omitempty,max=100"`
	LastName  *string `json:"lastName" binding:"omitempty,max=100"`
}
