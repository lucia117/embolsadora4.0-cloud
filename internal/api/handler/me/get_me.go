package me

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tu-org/embolsadora-api/internal/api/usecases"
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, resp)
}
