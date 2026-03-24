package me

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tu-org/embolsadora-api/internal/api/usecases"
	"github.com/tu-org/embolsadora-api/internal/domain"
)

// Handler handles GET /api/v1/me.
type Handler struct {
	uc *usecases.MeUsecase
}

func NewHandler(uc *usecases.MeUsecase) *Handler {
	return &Handler{uc: uc}
}

func (h *Handler) Handle(c *gin.Context) {
	resp, err := h.uc.GetMe(c.Request.Context())
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrForbidden):
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		case errors.Is(err, domain.ErrAccountSuspended):
			c.JSON(http.StatusForbidden, gin.H{"error": "account suspended"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		}
		return
	}
	c.JSON(http.StatusOK, resp)
}
