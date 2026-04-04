package api

import (
	"context"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	createTenant "github.com/tu-org/embolsadora-api/internal/api/handler/tenants/create_tenant"
	deleteTenant "github.com/tu-org/embolsadora-api/internal/api/handler/tenants/delete_tenant"
	getAllTenants "github.com/tu-org/embolsadora-api/internal/api/handler/tenants/get_all_tenants"
	getTenant "github.com/tu-org/embolsadora-api/internal/api/handler/tenants/get_tenant"
	updateTenant "github.com/tu-org/embolsadora-api/internal/api/handler/tenants/update_tenant"
	assignUserRole "github.com/tu-org/embolsadora-api/internal/api/handler/user_roles/assign_user_role"
	listUserRoles "github.com/tu-org/embolsadora-api/internal/api/handler/user_roles/list_user_roles"
	revokeUserRole "github.com/tu-org/embolsadora-api/internal/api/handler/user_roles/revoke_user_role"
	updateUserRole "github.com/tu-org/embolsadora-api/internal/api/handler/user_roles/update_user_role"
	bulkAssignUserRole "github.com/tu-org/embolsadora-api/internal/api/handler/user_roles/bulk_assign_user_roles"
	getUserRoles "github.com/tu-org/embolsadora-api/internal/api/handler/user_roles/get_user_roles"
	appUsers "github.com/tu-org/embolsadora-api/internal/app/users"
	userhandlers "github.com/tu-org/embolsadora-api/internal/api/handler/users"
	usersRepo "github.com/tu-org/embolsadora-api/internal/repo/pg/users"
	"github.com/tu-org/embolsadora-api/internal/api/middleware"
	ucCreateTenant "github.com/tu-org/embolsadora-api/internal/api/usecases/tenants/create_tenant"
	ucDeleteTenant "github.com/tu-org/embolsadora-api/internal/api/usecases/tenants/delete_tenant"
	ucGetAllTenants "github.com/tu-org/embolsadora-api/internal/api/usecases/tenants/get_all_tenants"
	ucGetTenant "github.com/tu-org/embolsadora-api/internal/api/usecases/tenants/get_tenant"
	ucUpdateTenant "github.com/tu-org/embolsadora-api/internal/api/usecases/tenants/update_tenant"
	ucAssignUserRole "github.com/tu-org/embolsadora-api/internal/api/usecases/user_roles/assign_user_role"
	ucListUserRoles "github.com/tu-org/embolsadora-api/internal/api/usecases/user_roles/list_user_roles"
	ucRevokeUserRole "github.com/tu-org/embolsadora-api/internal/api/usecases/user_roles/revoke_user_role"
	ucUpdateUserRole "github.com/tu-org/embolsadora-api/internal/api/usecases/user_roles/update_user_role"
	ucBulkAssignUserRole "github.com/tu-org/embolsadora-api/internal/api/usecases/user_roles/bulk_assign_user_roles"
	ucGetUserRoles "github.com/tu-org/embolsadora-api/internal/api/usecases/user_roles/get_user_roles"
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
	Logger       *zap.Logger                    // New: for user management
	UserRepo     usersRepo.Repository           // New: for user management
}

// TODO: fill in configuration as needed.
type Config struct{}

// RegisterAdminRoutes wires API surface routes under the provided group (e.g., /api/v1).
func RegisterAdminRoutes(g *gin.RouterGroup, deps Deps, cfg Config) {
	// Users - All /users routes share a single group with tenant middleware to avoid
	// wildcard conflicts between groups (e.g., /users/:id vs /users/:id/roles).
	userService := appUsers.NewService(deps.UserRepo, deps.UserRoleRepo, deps.Logger)
	uh := userhandlers.NewHandler(userService, deps.Logger)

	getUserRolesUseCase := ucGetUserRoles.NewUseCase(deps.UserRoleRepo)
	getUserRolesHandler := getUserRoles.NewGetUserRolesHandler(getUserRolesUseCase)

	userRoutes := g.Group("")
	userRoutes.Use(middleware.ExtractTenantID())

	// Literal routes MUST be registered before wildcard routes to avoid Gin conflicts
	userRoutes.GET("/users/pending", middleware.RequireRole("admin"), uh.ListPendingUsers)

	// Read operations (no RBAC required)
	userRoutes.GET("/users", uh.ListUsers)
	userRoutes.GET("/users/:id", uh.GetUser)
	userRoutes.GET("/users/:id/roles", getUserRolesHandler.Handle)

	// Write operations (admin only)
	userRoutes.POST("/users", middleware.RequireRole("admin"), uh.CreateUser)
	userRoutes.PATCH("/users/:id", middleware.RequireRole("admin"), uh.UpdateUser)
	userRoutes.PATCH("/users/:id/status", middleware.RequireRole("admin"), uh.UpdateUserStatus)
	userRoutes.DELETE("/users/:id", middleware.RequireRole("admin"), uh.DeleteUser)

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

	bulkAssignUserRoleUseCase := ucBulkAssignUserRole.NewUseCase(deps.UserRoleRepo)
	bulkAssignUserRoleHandler := bulkAssignUserRole.NewBulkAssignUserRolesHandler(bulkAssignUserRoleUseCase)
	g.POST("/user-roles/bulk", bulkAssignUserRoleHandler.Handle)

	updateUserRoleUseCase := ucUpdateUserRole.NewUseCase(deps.UserRoleRepo)
	updateUserRoleHandler := updateUserRole.NewUpdateUserRoleHandler(updateUserRoleUseCase)
	g.PUT("/user-roles/:id", updateUserRoleHandler.Handle)

	revokeUserRoleUseCase := ucRevokeUserRole.NewUseCase(deps.UserRoleRepo)
	revokeUserRoleHandler := revokeUserRole.NewRevokeUserRoleHandler(revokeUserRoleUseCase)
	g.DELETE("/user-roles/:id", revokeUserRoleHandler.Handle)
}
