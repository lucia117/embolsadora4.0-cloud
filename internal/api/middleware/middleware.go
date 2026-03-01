package middleware

import (
	"github.com/gin-gonic/gin"
)

// TODO: Stub middleware implementations only, no logic.
// JWTAuth validates the JWT token and should populate the request context with the user ID using:
//
//	ctx := platform.WithUserID(c.Request.Context(), userIDFromJWT)
//	c.Request = c.Request.WithContext(ctx)
func JWTAuth() gin.HandlerFunc { return func(c *gin.Context) { /* TODO */ c.Next() } }

// TenantFromJWT extracts the tenant ID from JWT and should populate the request context using:
//
//	ctx := platform.WithTenantID(c.Request.Context(), tenantIDFromJWT)
//	c.Request = c.Request.WithContext(ctx)
func TenantFromJWT() gin.HandlerFunc { return func(c *gin.Context) { /* TODO */ c.Next() } }

func RequestID() gin.HandlerFunc { return func(c *gin.Context) { /* TODO */ c.Next() } }
func Logger() gin.HandlerFunc    { return func(c *gin.Context) { /* TODO */ c.Next() } }
func CORS() gin.HandlerFunc      { return func(c *gin.Context) { /* TODO */ c.Next() } }
