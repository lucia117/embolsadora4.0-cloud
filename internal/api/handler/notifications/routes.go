package notifications

import (
	"github.com/gin-gonic/gin"
	appNotifications "github.com/tu-org/embolsadora-api/internal/app/notifications"
)

// RegisterRoutes registra los endpoints de notificaciones bajo el grupo dado.
// ORDEN CRÍTICO: rutas estáticas (/count) deben ir antes que parámetros (/:id).
func RegisterRoutes(g *gin.RouterGroup, service *appNotifications.Service) {
	// Rutas estáticas primero (evita que Gin interprete "count" como :id)
	g.GET("/notifications/count", CountNotifications(service))
	g.GET("/notifications", ListNotifications(service))

	// Rutas con parámetro :id
	g.GET("/notifications/:id", GetNotification(service))
	g.POST("/notifications/:id/ack", AckNotification(service))
	g.POST("/notifications/:id/close", CloseNotification(service))
}
