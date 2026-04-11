package aas

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Submodel represents a specific aspect of an AAS (e.g. TechnicalData, OperationalData).
type Submodel struct {
	ID               string            `bson:"_id"`
	TenantID         uuid.UUID         `bson:"tenantId"`
	ShellID          string            `bson:"shellId"`
	IDShort          string            `bson:"idShort"`
	SemanticID       *SemanticReference `bson:"semanticId,omitempty"`
	SubmodelElements []SubmodelElement  `bson:"submodelElements"`
	UpdatedAt        time.Time          `bson:"updatedAt"`
}

// SemanticReference is an optional pointer to a standardised submodel definition (e.g. IDTA URN).
type SemanticReference struct {
	Type string        `bson:"type"`
	Keys []SemanticKey `bson:"keys"`
}

// SemanticKey is a single key within a SemanticReference.
type SemanticKey struct {
	Type  string `bson:"type"`
	Value string `bson:"value"`
}

// SubmodelElement is a single data unit within a submodel.
// It is recursive: SubmodelElementCollection contains Children of the same type.
type SubmodelElement struct {
	ModelType string            `bson:"modelType"`
	IDShort   string            `bson:"idShort"`
	Value     interface{}       `bson:"value,omitempty"`
	ValueType *string           `bson:"valueType,omitempty"`
	Unit      *string           `bson:"unit,omitempty"`
	Children  []SubmodelElement `bson:"children,omitempty"`
}

// SubmodelRepository defines the persistence contract for AAS submodels.
// All operations require a tenantID to enforce multi-tenant isolation.
type SubmodelRepository interface {
	Create(ctx context.Context, submodel *Submodel) (*Submodel, error)
	GetByID(ctx context.Context, tenantID uuid.UUID, submodelID string) (*Submodel, error)
	// ListByShell returns a page of submodels for the given shell plus the total count (unpaged).
	ListByShell(ctx context.Context, tenantID uuid.UUID, shellID string, limit, offset int) ([]*Submodel, int64, error)
	// UpsertElement inserts or replaces a SubmodelElement identified by its IDShort within the submodel.
	UpsertElement(ctx context.Context, tenantID uuid.UUID, submodelID string, element SubmodelElement) error
	Delete(ctx context.Context, tenantID uuid.UUID, submodelID string) error
}
