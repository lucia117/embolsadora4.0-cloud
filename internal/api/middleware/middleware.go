package middleware

import (
	"github.com/gin-gonic/gin"
)

// TODO: Stub middleware implementations only, no logic.
func JWTAuth() gin.HandlerFunc      { return func(c *gin.Context) { /* TODO */ c.Next() } }
func TenantFromJWT() gin.HandlerFunc { return func(c *gin.Context) { /* TODO */ c.Next() } }
func RequestID() gin.HandlerFunc    { return func(c *gin.Context) { /* TODO */ c.Next() } }
func Logger() gin.HandlerFunc       { return func(c *gin.Context) { /* TODO */ c.Next() } }
func CORS() gin.HandlerFunc         { return func(c *gin.Context) { /* TODO */ c.Next() } }
