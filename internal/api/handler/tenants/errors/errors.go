package tenantserrors

// ErrorResponse is the standard error shape for all tenant handlers.
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Status  int    `json:"status"`
}
