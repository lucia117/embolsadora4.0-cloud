# Data Model: MongoDB Infrastructure Layer

**Feature**: 006-mongo-infra  
**Date**: 2026-04-02  
**Store**: MongoDB — colecciones `asset_administration_shells` y `submodels`

---

## Entidades de Dominio (Go types en `internal/domain/aas/`)

### AssetAdministrationShell

Representa el gemelo digital de una máquina. Es el documento raíz del AAS.

```go
// internal/domain/aas/shell.go

package aas

import (
    "time"
    "github.com/google/uuid"
)

type AssetAdministrationShell struct {
    ID              string            // MongoDB _id — server-assigned (URN o UUID string)
    TenantID        uuid.UUID         // Aislamiento multi-tenant (OBLIGATORIO en toda query)
    GlobalAssetID   string            // Vínculo con EdgeDevice.MachineID; único por tenant
    AssetKind       string            // "Instance" | "Type"
    AssetType       string            // ej. "BaggingMachine"
    Description     *string           // Descripción libre, opcional
    Administration  *Administration   // Versión del shell (version, revision)
    SubmodelRefs    []SubmodelRef     // Referencias a submodelos (por ID de submodelo)
    CreatedAt       time.Time
    UpdatedAt       time.Time
}

type Administration struct {
    Version  string // ej. "1"
    Revision string // ej. "0"
}

type SubmodelRef struct {
    SubmodelID string // ID del submodelo referenciado
}
```

**Reglas de unicidad**: `(TenantID, GlobalAssetID)` es único. Un tenant no puede tener dos shells con el mismo `GlobalAssetID`.

**Estado**: No tiene máquina de estados. Se crea, actualiza y elimina. No hay estados "activo/inactivo" en esta feature.

**Vínculo con dominio existente**: `GlobalAssetID` == `EdgeDevice.MachineID`. No hay FK referencial (cross-store); el vínculo es por valor de string.

---

### Submodel

Aspecto específico del AAS. Contiene la colección de datos de un dominio particular (técnico, operativo, mantenimiento, infraestructura edge).

```go
// internal/domain/aas/submodel.go

package aas

import "time"

type Submodel struct {
    ID               string             // MongoDB _id — server-assigned
    TenantID         uuid.UUID          // Aislamiento multi-tenant (OBLIGATORIO)
    ShellID          string             // ID del AAS shell padre (no FK referencial)
    IDShort          string             // Nombre corto del submodelo, ej. "TechnicalData"
    SemanticID       *SemanticReference // Referencia semántica opcional (IDTA URN)
    SubmodelElements []SubmodelElement  // Elementos del submodelo (embebidos)
    UpdatedAt        time.Time
}

type SemanticReference struct {
    Type string // "ExternalReference"
    Keys []SemanticKey
}

type SemanticKey struct {
    Type  string // "GlobalReference"
    Value string // URN IDTA, ej. "urn:idta:submodel:TechnicalData:1.2"
}
```

**Reglas de unicidad**: `(TenantID, ShellID, IDShort)` es único. Un shell no puede tener dos submodelos con el mismo `IDShort`.

**Persistencia de SubmodelElements**: Embebida en el documento `Submodel`. No tienen colección propia en MongoDB.

---

### SubmodelElement (tipo embebido)

Unidad de dato dentro de un submodelo. Diseñada para ser heterogénea (Property, Collection, File, etc.) usando un campo discriminador `ModelType`.

```go
// internal/domain/aas/submodel.go (continuación)

type SubmodelElement struct {
    ModelType string           // "Property" | "SubmodelElementCollection" | "File" | "ReferenceElement"
    IDShort   string           // Identificador dentro del submodelo padre
    Value     interface{}      // Para Property: valor escalar (string, float64, bool, etc.)
    ValueType *string          // Para Property: "xs:float", "xs:int", "xs:string", "xs:boolean", "xs:dateTime"
    Unit      *string          // Unidad de medida opcional (ej. "kg", "%", "°C")
    Children  []SubmodelElement // Para SubmodelElementCollection: elementos hijos
}
```

**Nota**: `SubmodelElement` es un tipo recursivo — `SubmodelElementCollection` contiene `[]SubmodelElement`. Esto es fiel al metamodelo AAS v3. En BSON se persiste tal cual (MongoDB soporta documentos anidados sin límite práctico para este caso de uso).

---

## Interfaces de Repositorio (en `internal/domain/aas/`)

### ShellRepository

```go
type ShellRepository interface {
    Create(ctx context.Context, shell *AssetAdministrationShell) (*AssetAdministrationShell, error)
    GetByID(ctx context.Context, tenantID uuid.UUID, shellID string) (*AssetAdministrationShell, error)
    Update(ctx context.Context, tenantID uuid.UUID, shellID string, update *ShellUpdate) (*AssetAdministrationShell, error)
    Delete(ctx context.Context, tenantID uuid.UUID, shellID string) error
    ListByTenant(ctx context.Context, tenantID uuid.UUID, limit, offset int) ([]*AssetAdministrationShell, int64, error)
}

// ShellUpdate contiene solo los campos modificables (nil = no actualizar)
type ShellUpdate struct {
    Description    *string
    Administration *Administration
    AssetKind      *string
    AssetType      *string
    SubmodelRefs   []SubmodelRef  // nil = no actualizar; []SubmodelRef{} = limpiar refs
}
```

### SubmodelRepository

```go
type SubmodelRepository interface {
    Create(ctx context.Context, submodel *Submodel) (*Submodel, error)
    GetByID(ctx context.Context, tenantID uuid.UUID, submodelID string) (*Submodel, error)
    ListByShell(ctx context.Context, tenantID uuid.UUID, shellID string, limit, offset int) ([]*Submodel, int64, error)
    UpsertElement(ctx context.Context, tenantID uuid.UUID, submodelID string, element SubmodelElement) error
    Delete(ctx context.Context, tenantID uuid.UUID, submodelID string) error
}
```

---

## Documento BSON en MongoDB

### Colección `asset_administration_shells`

```json
{
  "_id": "urn:embolsadora:tenant-abc:machine-001",
  "tenantId": "550e8400-e29b-41d4-a716-446655440000",
  "globalAssetId": "urn:embolsadora:machine-001",
  "assetKind": "Instance",
  "assetType": "BaggingMachine",
  "description": null,
  "administration": { "version": "1", "revision": "0" },
  "submodelRefs": [
    { "submodelId": "urn:embolsadora:sm:technical:001" }
  ],
  "createdAt": "2026-04-02T00:00:00Z",
  "updatedAt": "2026-04-02T00:00:00Z"
}
```

### Colección `submodels`

```json
{
  "_id": "urn:embolsadora:sm:technical:001",
  "tenantId": "550e8400-e29b-41d4-a716-446655440000",
  "shellId": "urn:embolsadora:tenant-abc:machine-001",
  "idShort": "TechnicalData",
  "semanticId": {
    "type": "ExternalReference",
    "keys": [{ "type": "GlobalReference", "value": "urn:idta:submodel:TechnicalData:1.2" }]
  },
  "submodelElements": [
    { "modelType": "Property", "idShort": "MaxBagWeight", "value": 50.0, "valueType": "xs:float", "unit": "kg" },
    {
      "modelType": "SubmodelElementCollection",
      "idShort": "Stations",
      "children": [
        {
          "modelType": "SubmodelElementCollection",
          "idShort": "FillingStation",
          "children": [
            { "modelType": "Property", "idShort": "ValveOpen", "value": true, "valueType": "xs:boolean" }
          ]
        }
      ]
    }
  ],
  "updatedAt": "2026-04-02T00:00:00Z"
}
```

---

## Índices MongoDB

| Colección | Campos | Tipo | Propósito |
|-----------|--------|------|-----------|
| `asset_administration_shells` | `{ tenantId: 1, globalAssetId: 1 }` | Único | Unicidad tenant+asset; aislamiento multi-tenant |
| `asset_administration_shells` | `{ tenantId: 1, updatedAt: -1 }` | Normal | Listado por tenant ordenado por reciente |
| `submodels` | `{ tenantId: 1, shellId: 1, idShort: 1 }` | Único | Unicidad idShort por shell+tenant |
| `submodels` | `{ tenantId: 1, shellId: 1 }` | Normal | Listado de submodelos de un shell |

---

## Errores de Dominio (reutilizar los existentes en `internal/domain/errors.go`)

| Error driver MongoDB | Error de dominio expuesto |
|----------------------|--------------------------|
| `mongo.ErrNoDocuments` | `domain.ErrNotFound` |
| `mongo.IsDuplicateKeyError(err) == true` | `domain.ErrConflict` |
| Cualquier otro error de red/timeout | Error genérico wrapeado con contexto |

> Los tipos del driver (`mongo.WriteException`, etc.) **nunca** deben escapar fuera de `internal/repo/mongo/`.

---

## Paginación

Todas las operaciones de listado usan `limit` y `offset` enteros:

- `limit <= 0` → se aplica el default (100)
- `limit > 500` → se rechaza con error de validación en la capa de usecase (no en el repo)
- `offset < 0` → tratado como 0

Los métodos de listado retornan `([]*T, int64, error)` donde `int64` es el total de documentos que matchean el filtro (sin paginación), necesario para que el cliente calcule la cantidad de páginas.
