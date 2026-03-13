package users

import (
	"fmt"
	"net/mail"
	"time"
)

// User represents a platform user within a tenant
type User struct {
	ID        string     // UUID
	TenantID  string     // UUID - which tenant owns this user
	FirstName string     // max 100 chars
	LastName  string     // max 100 chars
	Email     string     // unique per tenant, immutable
	Role      string     // 'admin' or 'user'
	Image     *string    // optional avatar URL
	CreatedAt time.Time  // server-generated at creation
	UpdatedAt time.Time  // server-generated, updated on any change
	DeletedAt *time.Time // null if active, timestamp if soft-deleted
}

// Role constants
const (
	RoleAdmin = "admin"
	RoleUser  = "user"
)

// IsActive returns true if user is not soft-deleted
func (u *User) IsActive() bool {
	return u.DeletedAt == nil
}

// Validate checks if user data is valid
func (u *User) Validate() error {
	if u.TenantID == "" {
		return fmt.Errorf("tenant_id is required")
	}

	if err := u.ValidateFirstName(); err != nil {
		return err
	}

	if err := u.ValidateLastName(); err != nil {
		return err
	}

	if err := u.ValidateEmail(); err != nil {
		return err
	}

	if err := u.ValidateRole(); err != nil {
		return err
	}

	return nil
}

// ValidateFirstName validates first name
func (u *User) ValidateFirstName() error {
	if u.FirstName == "" {
		return fmt.Errorf("first_name is required")
	}
	if len(u.FirstName) > 100 {
		return fmt.Errorf("first_name must be at most 100 characters")
	}
	return nil
}

// ValidateLastName validates last name
func (u *User) ValidateLastName() error {
	if u.LastName == "" {
		return fmt.Errorf("last_name is required")
	}
	if len(u.LastName) > 100 {
		return fmt.Errorf("last_name must be at most 100 characters")
	}
	return nil
}

// ValidateEmail validates email format
func (u *User) ValidateEmail() error {
	if u.Email == "" {
		return fmt.Errorf("email is required")
	}
	if len(u.Email) > 254 {
		return fmt.Errorf("email must be at most 254 characters")
	}
	addr, err := mail.ParseAddress(u.Email)
	if err != nil || addr.Address != u.Email {
		return fmt.Errorf("email format is invalid")
	}
	return nil
}

// ValidateRole validates role value
func (u *User) ValidateRole() error {
	if u.Role == "" {
		return fmt.Errorf("role is required")
	}
	if u.Role != RoleAdmin && u.Role != RoleUser {
		return fmt.Errorf("role must be 'admin' or 'user'")
	}
	return nil
}

// IsAdmin returns true if user has admin role
func (u *User) IsAdmin() bool {
	return u.Role == RoleAdmin
}
