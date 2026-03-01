package models

import "github.com/tu-org/embolsadora-api/internal/api/usecases/user_roles/bulk_assign_user_roles"

// AssignmentSummary is a compact representation of a single UTR in the bulk response.
type AssignmentSummary struct {
	ID     string `json:"id"`
	UserID string `json:"userId"`
	RoleID string `json:"roleId"`
	Status string `json:"status"`
}

// BulkAssignResponse is the JSON shape returned for a bulk assignment operation.
type BulkAssignResponse struct {
	Assigned    int                  `json:"assigned"`
	Failed      int                  `json:"failed"`
	Assignments []AssignmentSummary `json:"assignments"`
}

// FromDomain converts a BulkAssignResult to a BulkAssignResponse.
func FromDomain(result *bulk_assign_user_roles.BulkAssignResult) *BulkAssignResponse {
	assignments := make([]AssignmentSummary, 0, len(result.Assignments))
	for _, utr := range result.Assignments {
		roleID := ""
		if utr.RoleID != nil {
			roleID = *utr.RoleID
		}
		summary := AssignmentSummary{
			ID:     utr.ID.String(),
			UserID: utr.UserID.String(),
			RoleID: roleID,
			Status: string(utr.Status),
		}
		assignments = append(assignments, summary)
	}
	return &BulkAssignResponse{
		Assigned:    result.Assigned,
		Failed:      result.Failed,
		Assignments: assignments,
	}
}
