// Package ingest contiene el modelo de dominio de la ingesta de mediciones.
// No conoce Mongo, ni Gin, ni Postgres.
package ingest

import "time"

// Codigos del contrato con el Edge. Los valores son literales fijos: el Edge
// decide con ellos si un evento se ACKea, se reintenta o se manda a DEAD
// (embolsadora-edge/internal/app/forwarder/errors.go:96-110). Cambiar uno de
// estos strings, o usarlo con otra semantica, es perder datos.
const (
	// CodeDuplicate: el eventId ya existia. El Edge lo ACKea — es el resultado
	// esperado de un reintento y no es un error.
	CodeDuplicate = "DUPLICATE"

	// CodeInvalidSchema: el sobre del evento esta mal formado. El Edge lo manda
	// a DEAD y no lo reintenta NUNCA MAS. Solo para errores realmente
	// irrecuperables del dato.
	CodeInvalidSchema = "INVALID_SCHEMA"

	// CodeValidationFailed: el sobre es valido pero el evento no corresponde.
	// El Edge lo manda a DEAD.
	CodeValidationFailed = "VALIDATION_FAILED"

	// CodeStorageUnavailable: no se pudo escribir por una causa de
	// infraestructura. No esta en la lista de codigos terminales del Edge, asi
	// que cae en el default: reintentar con backoff. Es el codigo que sostiene
	// el invariante I-1.
	CodeStorageUnavailable = "STORAGE_UNAVAILABLE"
)

// Kinds admitidos por el contrato.
const (
	KindMetric    = "metric"
	KindAlarm     = "alarm"
	KindHeartbeat = "heartbeat"
)

// MaxSchemaVersion es la version de sobre mas alta que este cloud entiende.
// Un evento con una version mayor se rechaza con VALIDATION_FAILED, no con
// INVALID_SCHEMA: el sobre esta bien formado, simplemente es de un futuro que
// esta build no conoce.
const MaxSchemaVersion = 1

// DeviceContext es la identidad ya resuelta por la capa de auth. El service la
// recibe en vez de leerla del body: tenantId y deviceId nunca salen del
// request (D-10), y machineId solo se acepta si coincide con el de aca.
type DeviceContext struct {
	TenantID  string
	DeviceID  string
	MachineID string
}

// Measurement es el documento que se persiste, uno por evento aceptado.
type Measurement struct {
	EventID       string         `bson:"eventId"`
	TenantID      string         `bson:"tenantId"`
	DeviceID      string         `bson:"deviceId"`
	MachineID     string         `bson:"machineId"`
	Ts            time.Time      `bson:"ts"`
	Seq           *int64         `bson:"seq,omitempty"`
	Kind          string         `bson:"kind"`
	SchemaVersion int            `bson:"schemaVersion"`
	Payload       map[string]any `bson:"payload"`
	ReceivedAt    time.Time      `bson:"receivedAt"`
}

// EventError es un elemento de errors[] en la respuesta.
type EventError struct {
	// Index es la posicion, con base 0, en el array `events` del request
	// ORIGINAL — no en ningun array filtrado (invariante I-3). El Edge usa este
	// numero para elegir que fila de su outbox marcar como DEAD: un desfasaje
	// mata eventos sanos y da por buenos los rotos.
	Index   int    `json:"index"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Result es el contenido de `data` en la respuesta 200.
type Result struct {
	Accepted int          `json:"accepted"`
	Rejected int          `json:"rejected"`
	Errors   []EventError `json:"errors,omitempty"`
}
