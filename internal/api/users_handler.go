package api

import (
    "log"
    "net/http"

    "github.com/gin-gonic/gin"
)

// TODO: handlers are stubs only; no business logic.
func ListUsers(c *gin.Context) {
    log.Println("not implemented: ListUsers")
    c.String(http.StatusNotImplemented, "not implemented")
}

func CreateUser(c *gin.Context) {
    log.Println("not implemented: CreateUser")
    c.String(http.StatusNotImplemented, "not implemented")
}
