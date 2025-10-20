package middleware

import (
	"github.com/gin-gonic/gin"
)

// TODO: Stub middleware implementations only, no logic.
func APIKeyAuth() gin.HandlerFunc   { return func(c *gin.Context) { /* TODO */ c.Next() } }
func RateLimit() gin.HandlerFunc    { return func(c *gin.Context) { /* TODO */ c.Next() } }
func Idempotency() gin.HandlerFunc  { return func(c *gin.Context) { /* TODO */ c.Next() } }
func NoCORS() gin.HandlerFunc       { return func(c *gin.Context) { /* TODO */ c.Next() } }
func Timeout() gin.HandlerFunc      { return func(c *gin.Context) { /* TODO */ c.Next() } }
