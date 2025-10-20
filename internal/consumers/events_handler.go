package consumers

import (
    "log"
    "net/http"

    "github.com/gin-gonic/gin"
)

// TODO: handlers are stubs only; no business logic.
func IngestEvents(c *gin.Context) {
    log.Println("not implemented: IngestEvents")
    c.String(http.StatusNotImplemented, "not implemented")
}
