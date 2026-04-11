package aas

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// AssetAdministrationShell is the digital twin of a physical machine asset.
type AssetAdministrationShell struct {
	ID            string         `bson:"_id"`
	TenantID      uuid.UUID      `bson:"tenantId"`
	GlobalAssetID string         `bson:"globalAssetId"`
	AssetKind     string         `bson:"assetKind"`
	AssetType     string         `bson:"assetType"`
	Description   *string        `bson:"description,omitempty"`
	Administration *Administration `bson:"administration,omitempty"`
	SubmodelRefs  []SubmodelRef  `bson:"submodelRefs"`
	CreatedAt     time.Time      `bson:"createdAt"`
	UpdatedAt     time.Time      `bson:"updatedAt"`
}

// Administration holds version information for the shell.
type Administration struct {
	Version  string `bson:"version"`
	Revision string `bson:"revision"`
}

// SubmodelRef is a reference to a submodel by its server-assigned ID.
type SubmodelRef struct {
	SubmodelID string `bson:"submodelId"`
}

// ShellUpdate contains the fields that can be modified in an update operation.
// A nil pointer means "do not change this field".
type ShellUpdate struct {
	Description    *string
	Administration *Administration
	AssetKind      *string
	AssetType      *string
	SubmodelRefs   []SubmodelRef // nil = no change; empty slice = clear refs
}

// ShellRepository defines the persistence contract for AAS shells.
// All operations require a tenantID to enforce multi-tenant isolation.
type ShellRepository interface {
	Create(ctx context.Context, shell *AssetAdministrationShell) (*AssetAdministrationShell, error)
	GetByID(ctx context.Context, tenantID uuid.UUID, shellID string) (*AssetAdministrationShell, error)
	Update(ctx context.Context, tenantID uuid.UUID, shellID string, update *ShellUpdate) (*AssetAdministrationShell, error)
	Delete(ctx context.Context, tenantID uuid.UUID, shellID string) error
	// ListByTenant returns a page of shells for the tenant plus the total count (unpaged).
	ListByTenant(ctx context.Context, tenantID uuid.UUID, limit, offset int) ([]*AssetAdministrationShell, int64, error)
}
