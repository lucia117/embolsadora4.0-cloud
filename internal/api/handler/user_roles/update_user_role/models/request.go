package models

import (
	"github.com/gin-gonic/gin"
)

// UpdateUserRoleRequest holds the validated fields from the request body.
type UpdateUserRoleRequest struct {
	RoleID string `json:"roleId" binding:"required"`
}

// Parse binds and validates the JSON body.
func Parse(c *gin.Context) (UpdateUserRoleRequest, error) {
	var req UpdateUserRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"success": false, "error": err.Error()})
		return UpdateUserRoleRequest{}, err
	}
	return req, nil
}
