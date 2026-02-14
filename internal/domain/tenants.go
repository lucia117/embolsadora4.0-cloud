package domain

import (
	"time"

	"github.com/google/uuid"
)

// Tenant representa una organización/empresa en el sistema
type Tenant struct {
	ID          uuid.UUID `json:"id" db:"id"`
	Name        string    `json:"name" db:"name"`
	Description string    `json:"description" db:"description"`
	Domain      string    `json:"domain" db:"domain"`
	Active      bool      `json:"active" db:"active"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}
