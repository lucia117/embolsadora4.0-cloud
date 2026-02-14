package api

import (
	"context"

	"github.com/gin-gonic/gin"
	cratetask "github.com/tu-org/embolsadora-api/internal/api/handler/tasks/crate_task"
	deletetask "github.com/tu-org/embolsadora-api/internal/api/handler/tasks/delete_task"
	gettask "github.com/tu-org/embolsadora-api/internal/api/handler/tasks/get_task"
	gettasks "github.com/tu-org/embolsadora-api/internal/api/handler/tasks/get_tasks"
	updatetask "github.com/tu-org/embolsadora-api/internal/api/handler/tasks/update_task"
	createtenant "github.com/tu-org/embolsadora-api/internal/api/handler/tenants/create_tenant"
	deletetenant "github.com/tu-org/embolsadora-api/internal/api/handler/tenants/delete_tenant"
	gettenant "github.com/tu-org/embolsadora-api/internal/api/handler/tenants/get_tenant"
	gettenants "github.com/tu-org/embolsadora-api/internal/api/handler/tenants/get_tenants"
	updatetenant "github.com/tu-org/embolsadora-api/internal/api/handler/tenants/update_tenant"
	userhandlers "github.com/tu-org/embolsadora-api/internal/api/handler/users"
	"github.com/tu-org/embolsadora-api/internal/api/usecases/tasks"
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
	getTenantsHandler := gettenants.NewGetTenantsHandler()
	createTenantHandler := createtenant.NewCreateTenantHandler()
	getTenantHandler := gettenant.NewGetTenantHandler(deps.TenantRepo)
	updateTenantHandler := updatetenant.NewUpdateTenantHandler()
	deleteTenantHandler := deletetenant.NewDeleteTenantHandler()
	g.GET("/tenants", getTenantsHandler.GetTenants)
	g.POST("/tenants", createTenantHandler.CreateTenant)
	g.GET("/tenants/:id", getTenantHandler.GetTenant)
	g.PUT("/tenants/:id", updateTenantHandler.UpdateTenant)
	g.DELETE("/tenants/:id", deleteTenantHandler.DeleteTenant)

	// Tasks
	getTasksHandler := gettasks.NewGetTasksHandler(deps.TaskService)
	getTaskHandler := gettask.NewGetTaskHandler(deps.TaskService)
	createTaskHandler := cratetask.NewCreateTaskHandler(deps.TaskService)
	updateTaskHandler := updatetask.NewUpdateTaskHandler(deps.TaskService)
	deleteTaskHandler := deletetask.NewDeleteTaskHandler(deps.TaskService)
	g.GET("/tasks", getTasksHandler.GetTasks)
	g.POST("/tasks", createTaskHandler.CreateTask)
	g.GET("/tasks/:id", getTaskHandler.GetTask)
	g.PUT("/tasks/:id", updateTaskHandler.UpdateTask)
	g.DELETE("/tasks/:id", deleteTaskHandler.DeleteTask)
}
