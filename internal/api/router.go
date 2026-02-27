package api

import (
	"context"

	"github.com/gin-gonic/gin"
	createTask "github.com/tu-org/embolsadora-api/internal/api/handler/tasks/create_task"
	deleteTask "github.com/tu-org/embolsadora-api/internal/api/handler/tasks/delete_task"
	getTask "github.com/tu-org/embolsadora-api/internal/api/handler/tasks/get_task"
	getTasks "github.com/tu-org/embolsadora-api/internal/api/handler/tasks/get_tasks"
	updateTask "github.com/tu-org/embolsadora-api/internal/api/handler/tasks/update_task"
	createTenant "github.com/tu-org/embolsadora-api/internal/api/handler/tenants/create_tenant"
	deleteTenant "github.com/tu-org/embolsadora-api/internal/api/handler/tenants/delete_tenant"
	getAllTenants "github.com/tu-org/embolsadora-api/internal/api/handler/tenants/get_all_tenants"
	getTenant "github.com/tu-org/embolsadora-api/internal/api/handler/tenants/get_tenant"
	updateTenant "github.com/tu-org/embolsadora-api/internal/api/handler/tenants/update_tenant"
	userhandlers "github.com/tu-org/embolsadora-api/internal/api/handler/users"
	"github.com/tu-org/embolsadora-api/internal/api/usecases/tasks"
	ucCreateTenant "github.com/tu-org/embolsadora-api/internal/api/usecases/tenants/create_tenant"
	ucDeleteTenant "github.com/tu-org/embolsadora-api/internal/api/usecases/tenants/delete_tenant"
	ucGetAllTenants "github.com/tu-org/embolsadora-api/internal/api/usecases/tenants/get_all_tenants"
	ucGetTenant "github.com/tu-org/embolsadora-api/internal/api/usecases/tenants/get_tenant"
	ucUpdateTenant "github.com/tu-org/embolsadora-api/internal/api/usecases/tenants/update_tenant"
	"github.com/tu-org/embolsadora-api/internal/repo/pg/tenants"
	"github.com/tu-org/embolsadora-api/internal/security"
)

// Deps contiene las dependencias necesarias para los handlers
type Deps struct {
	JWTVerifier security.Verifier
	RBACCan     func(ctx context.Context, perm string) error
	TaskService tasks.Service
	TenantRepo  tenants.TenantRepository
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

	// Tasks
	getTasksHandler := getTasks.NewGetTasksHandler(deps.TaskService)
	getTaskHandler := getTask.NewGetTaskHandler(deps.TaskService)
	createTaskHandler := createTask.NewCreateTaskHandler(deps.TaskService)
	updateTaskHandler := updateTask.NewUpdateTaskHandler(deps.TaskService)
	deleteTaskHandler := deleteTask.NewDeleteTaskHandler(deps.TaskService)
	g.GET("/tasks", getTasksHandler.GetTasks)
	g.POST("/tasks", createTaskHandler.CreateTask)
	g.GET("/tasks/:id", getTaskHandler.GetTask)
	g.PUT("/tasks/:id", updateTaskHandler.UpdateTask)
	g.DELETE("/tasks/:id", deleteTaskHandler.DeleteTask)
}
