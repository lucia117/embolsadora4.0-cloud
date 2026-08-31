package ingest

import "context"

// Repository persiste mediciones.
//
// La idempotencia es responsabilidad de la implementacion —un indice unico
// compuesto (tenantId, eventId), no solo sobre eventId: eventId no lleva el
// tenant adentro, y dos tenants distintos pueden generarlo igual de forma
// legitima— y no del service. Resolverla en la aplicacion (leer, decidir,
// escribir) tendria una condicion de carrera entre dos batches concurrentes con
// el mismo evento; la base no la tiene.
type Repository interface {
	// InsertMany inserta el lote completo sin abortar ante el primer fallo y
	// reporta el desenlace de cada documento.
	//
	// Devuelve error SOLO si la operacion fallo entera (Mongo inalcanzable,
	// timeout): ese caso se traduce a HTTP 500 y el Edge reintenta. Los fallos
	// por documento van en el InsertReport, nunca en el error.
	InsertMany(ctx context.Context, docs []Measurement) (InsertReport, error)

	// Ping verifica la conectividad. Lo usa el health check.
	Ping(ctx context.Context) error
}

// InsertReport describe el desenlace de un InsertMany parcialmente exitoso.
//
// Los indices de los tres mapas son posiciones dentro del slice `docs` que se
// le paso a InsertMany, NO del array `events` del request. Traducirlos es
// responsabilidad del service, que es quien conoce esa correspondencia
// (invariante I-3).
type InsertReport struct {
	// Duplicated: indices que Mongo rechazo por violar el indice unico (E11000).
	Duplicated map[int]struct{}

	// Failed: indices que fallaron por cualquier otra causa RECUPERABLE, con su
	// mensaje. Se reportan como STORAGE_UNAVAILABLE para que el Edge reintente.
	Failed map[int]string

	// Invalid: indices que NUNCA van a poder persistirse, por mas que se
	// reintenten — hoy, documentos que ni siquiera se pueden serializar a BSON
	// (p. ej. una clave de `payload` con un byte NUL, que `payload` acepta
	// porque no se valida nunca, D-8).
	//
	// Van separados de Failed porque el codigo que el service les asigna es el
	// opuesto: STORAGE_UNAVAILABLE le dice al Edge "reintenta" y estos eventos
	// reintentarian para siempre sin avanzar jamas, bloqueando el outbox. Son
	// INVALID_SCHEMA: el problema esta en el dato, no en la infraestructura.
	Invalid map[int]string
}

// Inserted devuelve cuantos documentos de un lote de tamano `total` entraron.
func (r InsertReport) Inserted(total int) int {
	return total - len(r.Duplicated) - len(r.Failed) - len(r.Invalid)
}
