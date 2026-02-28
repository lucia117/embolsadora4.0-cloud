package models

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// BulkAssignUserRolesRequest holds the parsed and validated fields from the request body.
type BulkAssignUserRolesRequest struct {
	UserIDs   []uuid.UUID
	TenantID  uuid.UUID
	RoleID    string
}

// Parse binds and validates the JSON body, parses UUIDs, and returns the request.
func Parse(c *gin.Context) (BulkAssignUserRolesRequest, error) {
	var body struct {
		UserIDs  []string `json:"userIds"  binding:"required,min=1"`
		TenantID string   `json:"tenantId" binding:"required"`
		RoleID   string   `json:"roleId"   binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(400, gin.H{"success": false, "error": err.Error()})
		return BulkAssignUserRolesRequest{}, err
	}

	// Parse userIds
	userIDs := make([]uuid.UUID, 0, len(body.UserIDs))
	for _, idStr := range body.UserIDs {
		id, err := uuid.Parse(idStr)
		if err != nil {
			c.JSON(400, gin.H{"success": false, "error": "invalid userId in userIds: must be a UUID"})
			return BulkAssignUserRolesRequest{}, err
		}
		userIDs = append(userIDs, id)
	}

	tenantID, err := uuid.Parse(body.TenantID)
	if err != nil {
		c.JSON(400, gin.H{"success": false, "error": "invalid tenantId: must be a UUID"})
		return BulkAssignUserRolesRequest{}, err
	}

	return BulkAssignUserRolesRequest{
		UserIDs:   userIDs,
		TenantID:  tenantID,
		RoleID:    body.RoleID,
	}, nil
}
