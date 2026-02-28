package api

import (
	"context"

	"github.com/gin-gonic/gin"
	createTenant "github.com/tu-org/embolsadora-api/internal/api/handler/tenants/create_tenant"
	deleteTenant "github.com/tu-org/embolsadora-api/internal/api/handler/tenants/delete_tenant"
	getAllTenants "github.com/tu-org/embolsadora-api/internal/api/handler/tenants/get_all_tenants"
	getTenant "github.com/tu-org/embolsadora-api/internal/api/handler/tenants/get_tenant"
	updateTenant "github.com/tu-org/embolsadora-api/internal/api/handler/tenants/update_tenant"
	assignUserRole "github.com/tu-org/embolsadora-api/internal/api/handler/user_roles/assign_user_role"
	listUserRoles "github.com/tu-org/embolsadora-api/internal/api/handler/user_roles/list_user_roles"
	revokeUserRole "github.com/tu-org/embolsadora-api/internal/api/handler/user_roles/revoke_user_role"
	updateUserRole "github.com/tu-org/embolsadora-api/internal/api/handler/user_roles/update_user_role"
	userhandlers "github.com/tu-org/embolsadora-api/internal/api/handler/users"
	ucCreateTenant "github.com/tu-org/embolsadora-api/internal/api/usecases/tenants/create_tenant"
	ucDeleteTenant "github.com/tu-org/embolsadora-api/internal/api/usecases/tenants/delete_tenant"
	ucGetAllTenants "github.com/tu-org/embolsadora-api/internal/api/usecases/tenants/get_all_tenants"
	ucGetTenant "github.com/tu-org/embolsadora-api/internal/api/usecases/tenants/get_tenant"
	ucUpdateTenant "github.com/tu-org/embolsadora-api/internal/api/usecases/tenants/update_tenant"
	ucAssignUserRole "github.com/tu-org/embolsadora-api/internal/api/usecases/user_roles/assign_user_role"
	ucListUserRoles "github.com/tu-org/embolsadora-api/internal/api/usecases/user_roles/list_user_roles"
	ucRevokeUserRole "github.com/tu-org/embolsadora-api/internal/api/usecases/user_roles/revoke_user_role"
	ucUpdateUserRole "github.com/tu-org/embolsadora-api/internal/api/usecases/user_roles/update_user_role"
	"github.com/tu-org/embolsadora-api/internal/repo/pg/tenants"
	userRolesRepo "github.com/tu-org/embolsadora-api/internal/repo/pg/user_roles"
	"github.com/tu-org/embolsadora-api/internal/security"
)

// Deps contiene las dependencias necesarias para los handlers
type Deps struct {
	JWTVerifier  security.Verifier
	RBACCan      func(ctx context.Context, perm string) error
	TenantRepo   tenants.TenantRepository
	UserRoleRepo userRolesRepo.UserRoleRepository
}

// TODO: fill in configuration as needed.
type Config struct{}

// RegisterAdminRoutes wires API surface routes under the provided group (e.g., /api/v1).
func RegisterAdminRoutes(g *gin.RouterGroup, deps Deps, cfg Config) {
	// Users
	uh := userhandlers.NewUserHandler()
	g.GET("/users", uh.ListUsers)
	g.POST("/users", uh.CreateUser)
	g.GET("/users/:id", uh.GetUser)
	g.PUT("/users/:id", uh.UpdateUser)
	g.DELETE("/users/:id", uh.DeleteUser)

	// User profile
	g.GET("/profile", uh.GetProfile)
	g.PUT("/password", uh.UpdatePassword)

	// Machines
	g.GET("/machines", ListMachines)
	g.POST("/machines", CreateMachine)

	// Tenants
	getAllTenantsUseCase := ucGetAllTenants.NewUseCase(deps.TenantRepo)
	getTenantUseCase := ucGetTenant.NewUseCase(deps.TenantRepo)
	createTenantUseCase := ucCreateTenant.NewUseCase(deps.TenantRepo)
	updateTenantUseCase := ucUpdateTenant.NewUseCase(deps.TenantRepo)
	deleteTenantUseCase := ucDeleteTenant.NewUseCase(deps.TenantRepo)

	getAllTenantsHandler := getAllTenants.NewGetAllTenantsHandler(getAllTenantsUseCase)
	getTenantHandler := getTenant.NewGetTenantHandler(getTenantUseCase)
	createTenantHandler := createTenant.NewCreateTenantHandler(createTenantUseCase)
	updateTenantHandler := updateTenant.NewUpdateTenantHandler(updateTenantUseCase)
	deleteTenantHandler := deleteTenant.NewDeleteTenantHandler(deleteTenantUseCase)

	g.GET("/tenants", getAllTenantsHandler.GetAllTenants)
	g.POST("/tenants", createTenantHandler.CreateTenant)
	g.GET("/tenants/:id", getTenantHandler.GetTenant)
	g.PATCH("/tenants/:id", updateTenantHandler.UpdateTenant)
	g.DELETE("/tenants/:id", deleteTenantHandler.DeleteTenant)

	// User Roles
	assignUserRoleUseCase := ucAssignUserRole.NewUseCase(deps.UserRoleRepo)
	assignUserRoleHandler := assignUserRole.NewAssignUserRoleHandler(assignUserRoleUseCase)
	g.POST("/user-roles", assignUserRoleHandler.Handle)

	listUserRolesUseCase := ucListUserRoles.NewUseCase(deps.UserRoleRepo)
	listUserRolesHandler := listUserRoles.NewListUserRolesHandler(listUserRolesUseCase)
	g.GET("/user-roles", listUserRolesHandler.Handle)

	updateUserRoleUseCase := ucUpdateUserRole.NewUseCase(deps.UserRoleRepo)
	updateUserRoleHandler := updateUserRole.NewUpdateUserRoleHandler(updateUserRoleUseCase)
	g.PUT("/user-roles/:id", updateUserRoleHandler.Handle)

	revokeUserRoleUseCase := ucRevokeUserRole.NewUseCase(deps.UserRoleRepo)
	revokeUserRoleHandler := revokeUserRole.NewRevokeUserRoleHandler(revokeUserRoleUseCase)
	g.DELETE("/user-roles/:id", revokeUserRoleHandler.Handle)
}
