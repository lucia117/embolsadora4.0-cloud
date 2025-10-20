package api

import (
    "log"
    "net/http"

    "github.com/gin-gonic/gin"
)

// TODO: handlers are stubs only; no business logic.
func ListTenants(c *gin.Context) {
    log.Println("not implemented: ListTenants")
    c.String(http.StatusNotImplemented, "not implemented")
}

func CreateTenant(c *gin.Context) {
    log.Println("not implemented: CreateTenant")
    c.String(http.StatusNotImplemented, "not implemented")
}
