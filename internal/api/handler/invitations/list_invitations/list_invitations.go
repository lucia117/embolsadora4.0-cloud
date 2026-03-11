package list_invitations

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tu-org/embolsadora-api/internal/api/usecases"
)

type Handler struct {
	uc *usecases.InvitationUsecase
}

func NewHandler(uc *usecases.InvitationUsecase) *Handler {
	return &Handler{uc: uc}
}

type invitationItem struct {
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
	statusFilter := c.Query("status")
	var statusPtr *string
	if statusFilter != "" {
		statusPtr = &statusFilter
	}

	list, err := h.uc.ListInvitations(c.Request.Context(), statusPtr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	items := make([]invitationItem, 0, len(list))
	for _, inv := range list {
		items = append(items, invitationItem{
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

	c.JSON(http.StatusOK, gin.H{"invitations": items})
}
