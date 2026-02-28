package models

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// AssignUserRoleRequest holds the parsed and validated fields from the request body.
type AssignUserRoleRequest struct {
	UserID   uuid.UUID
	TenantID uuid.UUID
	RoleID   string
}

// Parse binds and validates the JSON body, parses UUIDs, and returns the request.
func Parse(c *gin.Context) (AssignUserRoleRequest, error) {
	var body struct {
		UserID   string `json:"userId"   binding:"required"`
		TenantID string `json:"tenantId" binding:"required"`
		RoleID   string `json:"roleId"   binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(400, gin.H{"success": false, "error": err.Error()})
		return AssignUserRoleRequest{}, err
	}

	userID, err := uuid.Parse(body.UserID)
	if err != nil {
		c.JSON(400, gin.H{"success": false, "error": "invalid userId: must be a UUID"})
		return AssignUserRoleRequest{}, err
	}

	tenantID, err := uuid.Parse(body.TenantID)
	if err != nil {
		c.JSON(400, gin.H{"success": false, "error": "invalid tenantId: must be a UUID"})
		return AssignUserRoleRequest{}, err
	}

	return AssignUserRoleRequest{
		UserID:   userID,
		TenantID: tenantID,
		RoleID:   body.RoleID,
	}, nil
}
