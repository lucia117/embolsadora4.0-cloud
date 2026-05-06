package change_password

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tu-org/embolsadora-api/internal/api/usecases"
	"github.com/tu-org/embolsadora-api/internal/domain"
)

type Handler struct {
	uc *usecases.PasswordUsecase
}

func NewHandler(uc *usecases.PasswordUsecase) *Handler {
	return &Handler{uc: uc}
}

// Handle clears the password_change_required flag for the authenticated user.
// Called from the frontend Supabase callback after the user completes password change.
func (h *Handler) Handle(c *gin.Context) {
	if err := h.uc.ClearPasswordChangeRequired(c.Request.Context()); err != nil {
		if errors.Is(err, domain.ErrForbidden) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "password_change_required cleared"})
}
