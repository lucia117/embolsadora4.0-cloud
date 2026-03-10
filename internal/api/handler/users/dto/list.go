package dto

import "time"

// ListUsersResponse represents the paginated list response
type ListUsersResponse struct {
	Data       []UserResponse    `json:"data"`
	Pagination PaginationMeta    `json:"pagination"`
}

// PaginationMeta contains pagination metadata
type PaginationMeta struct {
	Total  int64 `json:"total"`  // Total count of users in tenant
	Count  int   `json:"count"`  // Count of returned users in this page
	Limit  int   `json:"limit"`  // Limit used for this request
	Offset int   `json:"offset"` // Offset used for this request
}

// UserResponse represents a user in API responses
type UserResponse struct {
	ID        string     `json:"id"`
	FirstName string     `json:"firstName"`
	LastName  string     `json:"lastName"`
	Email     string     `json:"email"`
	Role      string     `json:"role"`
	TenantID  string     `json:"tenantId"`
	Image     *string    `json:"image"`
	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
	DeletedAt *time.Time `json:"deletedAt"`
}
