package alarm_rules

import (
	"github.com/gin-gonic/gin"
	appAlarmRules "github.com/tu-org/embolsadora-api/internal/app/alarm_rules"
)

// RegisterRoutes registra los endpoints de alarm rules.
// readGroup:  sin RBAC adicional (GET /alarm-rules, GET /alarm-rules/:id).
// writeGroup: con RBACCheck("users:write") aplicado externamente (POST, PATCH, DELETE).
func RegisterRoutes(readGroup, writeGroup *gin.RouterGroup, service *appAlarmRules.Service) {
	readGroup.GET("/alarm-rules", ListAlarmRules(service))
	readGroup.GET("/alarm-rules/:id", GetAlarmRule(service))

	writeGroup.POST("/alarm-rules", CreateAlarmRule(service))
	writeGroup.PATCH("/alarm-rules/:id", UpdateAlarmRule(service))
	writeGroup.DELETE("/alarm-rules/:id", DeleteAlarmRule(service))
}
