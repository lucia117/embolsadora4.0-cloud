package httperr

import (
	"net/http"

	"github.com/gin-gonic/gin"
	apperrors "github.com/tu-org/embolsadora-api/internal/core/errors"
)

// WriteError renders a standardized WebError payload for handlers.
func WriteError(c *gin.Context, err error) {
	webErr, ok := apperrors.ToWeb(err).(*apperrors.WebError)
	if !ok || webErr == nil {
		webErr = apperrors.NewWebError(http.StatusInternalServerError, "internal error")
	}

	c.JSON(webErr.StatusCode(), webErr)
}
