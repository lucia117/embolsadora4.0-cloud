package api

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

// TODO: handlers are stubs only; no business logic.
func ListMachines(c *gin.Context) {
	log.Println("not implemented: ListMachines")
	c.String(http.StatusNotImplemented, "not implemented")
}

func CreateMachine(c *gin.Context) {
	log.Println("not implemented: CreateMachine")
	c.String(http.StatusNotImplemented, "not implemented")
}
