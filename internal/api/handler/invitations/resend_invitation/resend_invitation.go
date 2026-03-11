package resend_invitation

import (
	"errors"
	"net/http"

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

func (h *Handler) Handle(c *gin.Context) {
	id := c.Param("id")

	if err := h.uc.ResendInvitation(c.Request.Context(), id); err != nil {
		switch {
		case errors.Is(err, domain.ErrNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "invitation not found"})
		case errors.Is(err, domain.ErrInvitationNotPending):
			c.JSON(http.StatusConflict, gin.H{"error": "invitation is not in pending status"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "invitation resent"})
}
