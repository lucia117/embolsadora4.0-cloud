package roles

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tu-org/embolsadora-api/internal/api/handler/roles/dto"
	appRoles "github.com/tu-org/embolsadora-api/internal/app/roles"
)

func GetRole(service *appRoles.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")

		role, err := service.GetRole(c.Request.Context(), id)
		if err != nil {
			HandleError(c, err)
			return
		}

		c.JSON(http.StatusOK, dto.FromDomain(role))
	}
}
