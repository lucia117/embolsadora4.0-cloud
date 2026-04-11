package users

import "fmt"

// CreateUserCommand represents a user creation request
type CreateUserCommand struct {
	TenantID   string
	FirstName  string
	LastName   string
	Email      string
	Role       string  // any valid roles.id (system or custom)
	Image      *string // optional
	AssignedBy string  // UUID of the admin creating the user
}

// Validate checks command validity
func (c *CreateUserCommand) Validate() error {
	if c.TenantID == "" {
		return fmt.Errorf("tenant_id is required")
	}
	if c.FirstName == "" {
		return fmt.Errorf("first_name is required")
	}
	if len(c.FirstName) > 100 {
		return fmt.Errorf("first_name must be at most 100 characters")
	}
	if c.LastName == "" {
		return fmt.Errorf("last_name is required")
	}
	if len(c.LastName) > 100 {
		return fmt.Errorf("last_name must be at most 100 characters")
	}
	if c.Email == "" {
		return fmt.Errorf("email is required")
	}
	if len(c.Email) > 254 {
		return fmt.Errorf("email must be at most 254 characters")
	}
	if c.Role == "" {
		return fmt.Errorf("role is required")
	}
	if len(c.Role) > 50 {
		return fmt.Errorf("role must be at most 50 characters")
	}
	if c.AssignedBy == "" {
		return fmt.Errorf("assigned_by is required")
	}
	return nil
}

// UpdateUserCommand represents a user update request
type UpdateUserCommand struct {
	TenantID  string
	UserID    string
	FirstName *string // optional
	LastName  *string // optional
	Role      *string // optional
	Image     *string // optional
}

// Validate checks command validity and prevents immutable field updates
func (c *UpdateUserCommand) Validate() error {
	if c.TenantID == "" {
		return fmt.Errorf("tenant_id is required")
	}
	if c.UserID == "" {
		return fmt.Errorf("user_id is required")
	}

	// Validate provided fields
	if c.FirstName != nil && len(*c.FirstName) > 100 {
		return fmt.Errorf("first_name must be at most 100 characters")
	}
	if c.LastName != nil && len(*c.LastName) > 100 {
		return fmt.Errorf("last_name must be at most 100 characters")
	}
	// role accepts any valid roles.id; FK in user_tenant_roles enforces existence

	return nil
}
