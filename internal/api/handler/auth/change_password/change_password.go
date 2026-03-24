package change_password

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tu-org/embolsadora-api/internal/api/usecases"
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "password_change_required cleared"})
}
