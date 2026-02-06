package api

import (
	"context"

	"github.com/gin-gonic/gin"
	taskhandlers "github.com/tu-org/embolsadora-api/internal/api/handler/tasks"
	tenanthandlers "github.com/tu-org/embolsadora-api/internal/api/handler/tenants"
	userhandlers "github.com/tu-org/embolsadora-api/internal/api/handler/users"
	"github.com/tu-org/embolsadora-api/internal/api/usecases/tasks"
	"github.com/tu-org/embolsadora-api/internal/security"
)

// Deps contiene las dependencias necesarias para los handlers
type Deps struct {
	JWTVerifier security.Verifier
	RBACCan     func(ctx context.Context, perm string) error
	TaskService tasks.Service
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
	th := tenanthandlers.NewTenantHandler()
	g.GET("/tenants", th.ListTenants)
	g.POST("/tenants", th.CreateTenant)
	g.GET("/tenants/:id", th.GetTenant)
	g.PUT("/tenants/:id", th.UpdateTenant)
	g.DELETE("/tenants/:id", th.DeleteTenant)

	// Tasks
	taskHandler := taskhandlers.NewTaskHandler(deps.TaskService)
	g.GET("/tasks", taskHandler.ListTasks)
	g.POST("/tasks", taskHandler.CreateTask)
	g.GET("/tasks/:id", taskHandler.GetTask)
	g.PUT("/tasks/:id", taskHandler.UpdateTask)
	g.DELETE("/tasks/:id", taskHandler.DeleteTask)
}
