package models

import "github.com/tu-org/embolsadora-api/internal/domain"

// RevokeResponse is the JSON shape returned after a successful revoke.
// The Pact contract specifies only id and status in the response body.
type RevokeResponse struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

// FromDomain converts a domain.UserTenantRole to a RevokeResponse.
func FromDomain(utr *domain.UserTenantRole) RevokeResponse {
	return RevokeResponse{
		ID:     utr.ID.String(),
		Status: string(utr.Status),
	}
}
