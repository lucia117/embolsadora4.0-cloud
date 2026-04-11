package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tu-org/embolsadora-api/internal/security"
)

// APIKeyAuth validates the X-API-Key header using the provided lookup.
// If lookup is nil, the middleware passes through (no validation).
func APIKeyAuth(lookup security.APIKeyLookup) gin.HandlerFunc {
	return func(c *gin.Context) {
		if lookup == nil {
			c.Next()
			return
		}
		key := c.GetHeader("X-API-Key")
		if key == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing X-API-Key header"})
			return
		}
		_, _, _, ok := lookup.Lookup(key)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid API key"})
			return
		}
		c.Next()
	}
}

func RateLimit() gin.HandlerFunc   { return func(c *gin.Context) { /* TODO */ c.Next() } }
func Idempotency() gin.HandlerFunc { return func(c *gin.Context) { /* TODO */ c.Next() } }
func NoCORS() gin.HandlerFunc      { return func(c *gin.Context) { /* TODO */ c.Next() } }
func Timeout() gin.HandlerFunc     { return func(c *gin.Context) { /* TODO */ c.Next() } }
