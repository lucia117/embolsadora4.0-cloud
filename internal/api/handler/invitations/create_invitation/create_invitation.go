package create_invitation

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tu-org/embolsadora-api/internal/api/handler/invitations/create_invitation/models"
	"github.com/tu-org/embolsadora-api/internal/api/usecases"
	"github.com/tu-org/embolsadora-api/internal/domain"
)

type Handler struct {
	uc *usecases.InvitationUsecase
}

func NewHandler(uc *usecases.InvitationUsecase) *Handler {
	return &Handler{uc: uc}
}

func (h *Handler) Handle(c *gin.Context) {
	var req models.CreateInvitationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	inv, err := h.uc.CreateInvitation(c.Request.Context(), req.Email, req.RoleID)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrInvitationAlreadyPending):
			c.JSON(http.StatusConflict, gin.H{"error": "invitation already pending for this email"})
		case errors.Is(err, domain.ErrInvitationRateLimitExceeded):
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "invitation rate limit exceeded"})
		case errors.Is(err, domain.ErrForbidden):
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		}
		return
	}

	resp := models.InvitationResponse{
		ID:        inv.ID,
		TenantID:  inv.TenantID,
		Email:     inv.Email,
		RoleID:    inv.RoleID,
		Status:    string(inv.Status),
		InvitedBy: inv.InvitedBy,
		CreatedAt: inv.CreatedAt,
		ExpiresAt: inv.ExpiresAt,
	}
	c.JSON(http.StatusCreated, resp)
}
