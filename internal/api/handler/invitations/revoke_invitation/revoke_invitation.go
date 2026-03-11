package revoke_invitation

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tu-org/embolsadora-api/internal/api/usecases"
	"github.com/tu-org/embolsadora-api/internal/domain"
)

type Handler struct {
	uc *usecases.InvitationUsecase
}

func NewHandler(uc *usecases.InvitationUsecase) *Handler {
	return &Handler{uc: uc}
}

type revokedResponse struct {
	ID        string    `json:"id"`
	TenantID  string    `json:"tenant_id"`
	Email     string    `json:"email"`
	RoleID    string    `json:"role_id"`
	Status    string    `json:"status"`
	InvitedBy string    `json:"invited_by"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

func (h *Handler) Handle(c *gin.Context) {
	id := c.Param("id")

	inv, err := h.uc.RevokeInvitation(c.Request.Context(), id)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "invitation not found in this tenant"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		}
		return
	}

	c.JSON(http.StatusOK, revokedResponse{
		ID:        inv.ID,
		TenantID:  inv.TenantID,
		Email:     inv.Email,
		RoleID:    inv.RoleID,
		Status:    string(inv.Status),
		InvitedBy: inv.InvitedBy,
		CreatedAt: inv.CreatedAt,
		ExpiresAt: inv.ExpiresAt,
	})
}
