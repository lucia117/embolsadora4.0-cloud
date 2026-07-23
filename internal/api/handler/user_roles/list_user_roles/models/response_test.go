package models_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tu-org/embolsadora-api/internal/api/handler/user_roles/list_user_roles/models"
	"github.com/tu-org/embolsadora-api/internal/domain"
)

func TestFromDomain_MapsRoleNameAndUser(t *testing.T) {
	now := time.Now()
	roleID := "operario"
	firstName := "Join"
	lastName := "Fixture"

	utr := domain.UserTenantRoleDetail{
		UserTenantRole: domain.UserTenantRole{
			ID:        uuid.New(),
			UserID:    uuid.New(),
			TenantID:  uuid.New(),
			RoleID:    &roleID,
			Status:    domain.UserRoleStatusActive,
			CreatedAt: now,
			UpdatedAt: now,
		},
		RoleName:      "Operario",
		UserEmail:     "join-fix-test@example.com",
		UserFirstName: &firstName,
		UserLastName:  &lastName,
	}

	result := models.FromDomain([]domain.UserTenantRoleDetail{utr})

	require.Len(t, result, 1)
	resp := result[0]
	assert.Equal(t, "Operario", resp.RoleName)
	require.NotNil(t, resp.User)
	assert.Equal(t, "join-fix-test@example.com", resp.User.Email)
	require.NotNil(t, resp.User.FirstName)
	assert.Equal(t, "Join", *resp.User.FirstName)
	require.NotNil(t, resp.User.LastName)
	assert.Equal(t, "Fixture", *resp.User.LastName)
	assert.Nil(t, resp.User.Name)
}

func TestFromDomain_PendingAssignmentHasEmptyRoleName(t *testing.T) {
	now := time.Now()
	utr := domain.UserTenantRoleDetail{
		UserTenantRole: domain.UserTenantRole{
			ID:        uuid.New(),
			UserID:    uuid.New(),
			TenantID:  uuid.New(),
			RoleID:    nil,
			Status:    domain.UserRoleStatusPending,
			CreatedAt: now,
			UpdatedAt: now,
		},
		RoleName:  "",
		UserEmail: "pending@example.com",
	}

	result := models.FromDomain([]domain.UserTenantRoleDetail{utr})

	require.Len(t, result, 1)
	assert.Equal(t, "", result[0].RoleName)
	assert.Nil(t, result[0].RoleID)
	require.NotNil(t, result[0].User)
	assert.Equal(t, "pending@example.com", result[0].User.Email)
}
