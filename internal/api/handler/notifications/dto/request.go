package dto

// ListNotificationsParams contiene los filtros y parámetros de paginación del listado.
type ListNotificationsParams struct {
	Status   string `form:"status"`
	Severity string `form:"severity"`
	Limit    int    `form:"limit,default=20"`
	Offset   int    `form:"offset,default=0"`
}
