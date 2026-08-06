package edge_devices

import (
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/tu-org/embolsadora-api/internal/api/handler/edge_devices/dto"
	appedge "github.com/tu-org/embolsadora-api/internal/app/edge_devices"
	domainapikeys "github.com/tu-org/embolsadora-api/internal/domain/apikeys"
	edgeerrors "github.com/tu-org/embolsadora-api/internal/domain/edge_devices"
	"github.com/tu-org/embolsadora-api/internal/platform"
)

func timeNow() time.Time { return time.Now().UTC() }

func toAPIKeyResponse(k *domainapikeys.APIKey) dto.APIKeyResponse {
	return dto.APIKeyResponse{
		ID:         k.ID.String(),
		KeyID:      k.KeyID,
		Name:       k.Name,
		CreatedAt:  k.CreatedAt,
		ExpiresAt:  k.ExpiresAt,
		RevokedAt:  k.RevokedAt,
		LastUsedAt: k.LastUsedAt,
		Active:     k.IsActive(timeNow()),
	}
}

// CreateAPIKey genera una API key para el device.
//
// El secreto viaja en esta respuesta y en ninguna otra: no se persiste, asi que
// no hay forma de recuperarlo despues.
func CreateAPIKey(service *appedge.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := platform.TenantUUID(c.Request.Context())
		if tenantID == nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "tenant ID not found"})
			return
		}
		deviceID, err := uuid.Parse(c.Param("deviceId"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "deviceId invalido"})
			return
		}

		var req dto.CreateAPIKeyRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			// El body es opcional: una key sin nombre ni vencimiento es valida,
			// y un body vacio (io.EOF) es exactamente ese caso. Pero cualquier
			// OTRO error de decode -JSON invalido, un expiresAt que no parsea
			// como fecha, etc.- es un body MAL FORMADO, no un body ausente:
			// colapsar los dos en el mismo default silenciaba el error y
			// devolvia una key sin nombre y SIN vencimiento con 201, con el
			// secreto en claro, como si el request hubiera sido el que el
			// operador penso que mandaba.
			if !errors.Is(err, io.EOF) {
				c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "body invalido"})
				return
			}
			req = dto.CreateAPIKeyRequest{}
		}

		key, plaintext, err := service.CreateAPIKey(
			c.Request.Context(), *tenantID, deviceID, req.Name, req.ExpiresAt, platform.UserID(c.Request.Context()))
		if err != nil {
			if errors.Is(err, edgeerrors.ErrDeviceNotFound) {
				c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "device no encontrado"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "no se pudo crear la api key"})
			return
		}

		c.JSON(http.StatusCreated, gin.H{"success": true, "data": dto.CreateAPIKeyResponse{
			APIKeyResponse: toAPIKeyResponse(key),
			Key:            plaintext,
		}})
	}
}

// ListAPIKeys devuelve las keys del device, sin secretos.
func ListAPIKeys(service *appedge.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := platform.TenantUUID(c.Request.Context())
		if tenantID == nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "tenant ID not found"})
			return
		}
		deviceID, err := uuid.Parse(c.Param("deviceId"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "deviceId invalido"})
			return
		}

		keys, err := service.ListAPIKeys(c.Request.Context(), *tenantID, deviceID)
		if err != nil {
			if errors.Is(err, edgeerrors.ErrDeviceNotFound) {
				c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "device no encontrado"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "no se pudieron listar las api keys"})
			return
		}

		out := make([]dto.APIKeyResponse, 0, len(keys))
		for _, k := range keys {
			out = append(out, toAPIKeyResponse(k))
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "data": out})
	}
}

// RevokeAPIKey revoca una key. Es idempotente.
func RevokeAPIKey(service *appedge.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := platform.TenantUUID(c.Request.Context())
		if tenantID == nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "tenant ID not found"})
			return
		}
		deviceID, err := uuid.Parse(c.Param("deviceId"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "deviceId invalido"})
			return
		}
		keyPK, err := uuid.Parse(c.Param("keyId"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "keyId invalido"})
			return
		}

		if err := service.RevokeAPIKey(c.Request.Context(), *tenantID, deviceID, keyPK); err != nil {
			if errors.Is(err, domainapikeys.ErrKeyNotFound) || errors.Is(err, edgeerrors.ErrDeviceNotFound) {
				c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "api key no encontrada"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "no se pudo revocar la api key"})
			return
		}
		c.Status(http.StatusNoContent)
	}
}
