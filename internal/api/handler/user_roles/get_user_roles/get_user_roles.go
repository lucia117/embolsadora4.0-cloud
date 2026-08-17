package get_user_roles

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tu-org/embolsadora-api/internal/api/handler/user_roles/get_user_roles/models"
	ucGetUserRoles "github.com/tu-org/embolsadora-api/internal/api/usecases/user_roles/get_user_roles"
	"github.com/tu-org/embolsadora-api/internal/platform"
	"github.com/tu-org/embolsadora-api/internal/security"
)

// Handler handles GET /api/v1/users/:id/roles requests.
type Handler struct {
	useCase ucGetUserRoles.UseCase
}

// NewGetUserRolesHandler creates a new Handler.
func NewGetUserRolesHandler(useCase ucGetUserRoles.UseCase) *Handler {
	return &Handler{useCase: useCase}
}

// Handle retrieves the role assignments for a user.
//
// La ruta está detrás de RBACCheck("perm_users_view") (router.go), el mismo permiso que
// GET /user-roles: es la misma información, indexada por usuario en vez de por tenant.
// Acá se resuelven los dos ejes de alcance que la consulta necesita, con el patrón de
// la rama — se deciden en el borde y viajan como parámetros explícitos:
//   - CrossTenant: solo los roles cross-tenant ven las membresías del usuario en otros
//     tenants. Para el resto la respuesta se acota al tenant del request.
//   - IncludeGlobal: solo super_admin ve las membresías a roles globales.
func (h *Handler) Handle(c *gin.Context) {
	userID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid id: must be a UUID"})
		return
	}

	ctx := c.Request.Context()
	tenantID, err := uuid.Parse(platform.TenantID(ctx))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "unauthenticated"})
		return
	}

	results, err := h.useCase.Execute(ctx, ucGetUserRoles.Query{
		UserID:        userID,
		TenantID:      tenantID,
		CrossTenant:   security.IsCrossTenantRole(ctx),
		IncludeGlobal: security.CanSeePlatformInternals(ctx),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "internal server error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": models.FromDomain(results)})
}
