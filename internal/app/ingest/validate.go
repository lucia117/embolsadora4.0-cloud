// Package ingest implementa el caso de uso de ingesta de mediciones.
package ingest

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	domain "github.com/tu-org/embolsadora-api/internal/domain/ingest"
)

// rawEvent es el sobre tal como llega. Todos los campos son punteros para poder
// distinguir "ausente" de "presente con el valor cero": sin esa distincion, un
// schemaVersion faltante y un schemaVersion 0 serian el mismo caso.
//
// Los numericos son json.RawMessage y no int: si fueran int, un valor de tipo
// equivocado abortaria el decode del evento entero con un mensaje generico, y
// perderiamos la posibilidad de decir cual campo estaba mal.
type rawEvent struct {
	EventID       *string          `json:"eventId"`
	MachineID     *string          `json:"machineId"`
	Ts            *string          `json:"ts"`
	Seq           *json.RawMessage `json:"seq"`
	Kind          *string          `json:"kind"`
	SchemaVersion *json.RawMessage `json:"schemaVersion"`
	Payload       *map[string]any  `json:"payload"`
}

// jsonInt parsea un entero JSON de forma estricta.
//
// El chequeo del primer byte no es paranoia: encoding/json acepta el STRING
// "1" dentro de un json.Number sin devolver error, asi que un
// `"schemaVersion": "1"` pasaria por valido si confiaramos en el decoder.
// Tambien rechaza 1.5 y 1e3, que el contrato no admite.
func jsonInt(raw json.RawMessage) (int64, bool) {
	s := strings.TrimSpace(string(raw))
	if s == "" || s[0] == '"' {
		return 0, false
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, false
	}
	return n, true
}

func invalid(format string, args ...any) *domain.EventError {
	return &domain.EventError{Code: domain.CodeInvalidSchema, Message: fmt.Sprintf(format, args...)}
}

func failed(format string, args ...any) *domain.EventError {
	return &domain.EventError{Code: domain.CodeValidationFailed, Message: fmt.Sprintf(format, args...)}
}

// ValidateEvent valida el SOBRE de un evento y lo convierte en Measurement.
//
// El contenido de `payload` no se valida nunca (D-8): se copia tal cual. Si
// validaramos el payload, cualquier cambio del catalogo AAS del Edge produciria
// INVALID_SCHEMA y el Edge mandaria mediciones reales a DEAD, sin reintento.
//
// El campo Index del EventError devuelto queda en 0: lo setea el llamador, que
// es el unico que conoce la posicion en el array original (invariante I-3).
//
// El tercer valor devuelto es domain.SkewReasonNone salvo que el evento haya
// sido aceptado con un kind fuera de Kinds o un schemaVersion por encima de
// MaxSchemaVersion (ver los comentarios de esas dos constantes en
// internal/domain/ingest/measurement.go): esos dos casos ya NO se rechazan,
// pero el llamador necesita saber que pasaron para dejar constancia
// observable (metrica + log), que es lo que sostiene que "aceptar en
// silencio" no sea lo mismo que "aceptar y que se note".
func ValidateEvent(raw json.RawMessage, dev domain.DeviceContext, now time.Time) (domain.Measurement, *domain.EventError, domain.SkewReason) {
	var e rawEvent
	if err := json.Unmarshal(raw, &e); err != nil {
		return domain.Measurement{}, invalid("evento no deserializable: %v", err), domain.SkewReasonNone
	}

	switch {
	case e.EventID == nil || *e.EventID == "":
		return domain.Measurement{}, invalid("falta el campo requerido eventId"), domain.SkewReasonNone
	case e.MachineID == nil || *e.MachineID == "":
		return domain.Measurement{}, invalid("falta el campo requerido machineId"), domain.SkewReasonNone
	case e.Ts == nil:
		return domain.Measurement{}, invalid("falta el campo requerido ts"), domain.SkewReasonNone
	case e.Kind == nil:
		return domain.Measurement{}, invalid("falta el campo requerido kind"), domain.SkewReasonNone
	case e.SchemaVersion == nil:
		return domain.Measurement{}, invalid("falta el campo requerido schemaVersion"), domain.SkewReasonNone
	case e.Payload == nil:
		return domain.Measurement{}, invalid("falta el campo requerido payload"), domain.SkewReasonNone
	}

	ts, err := time.Parse(time.RFC3339, *e.Ts)
	if err != nil {
		return domain.Measurement{}, invalid("ts no es RFC3339: %q", *e.Ts), domain.SkewReasonNone
	}

	// kind fuera del enum: sobre bien formado, version que este build todavia
	// no conoce. Se acepta (ver comentario de Kinds); skew queda marcado para
	// el llamador.
	skew := domain.SkewReasonNone
	switch *e.Kind {
	case domain.KindMetric, domain.KindAlarm, domain.KindHeartbeat:
	default:
		skew = domain.SkewReasonUnknownKind
	}

	schemaVersion, ok := jsonInt(*e.SchemaVersion)
	if !ok || schemaVersion < 1 {
		return domain.Measurement{}, invalid("schemaVersion debe ser un entero >= 1"), domain.SkewReasonNone
	}

	var seq *int64
	if e.Seq != nil {
		n, ok := jsonInt(*e.Seq)
		if !ok || n < 0 {
			return domain.Measurement{}, invalid("seq debe ser un entero >= 0"), domain.SkewReasonNone
		}
		seq = &n
	}

	// A partir de aca el sobre es valido; lo que sigue (machineId) es el unico
	// rechazo de VALIDATION_FAILED que queda: el dato esta bien formado y aun
	// asi no corresponde a este device.

	// Un Pi comprometido no puede escribir en nombre de otra maquina: el
	// machineId del body solo se acepta si coincide con el de la key.
	if *e.MachineID != dev.MachineID {
		return domain.Measurement{}, failed("machineId %q no corresponde al device de la API key", *e.MachineID), domain.SkewReasonNone
	}

	// schemaVersion por encima de la maxima conocida: igual que kind
	// desconocido, se acepta (ver comentario de MaxSchemaVersion). Si el kind
	// YA disparo skew, esa es la razon que se reporta: son mutuamente
	// excluyentes en la practica (el Edge no manda las dos cosas a la vez) y
	// el llamador solo necesita una etiqueta por evento.
	if schemaVersion > domain.MaxSchemaVersion && skew == domain.SkewReasonNone {
		skew = domain.SkewReasonSchemaVersionAhead
	}

	return domain.Measurement{
		EventID: *e.EventID,
		// tenantId y deviceId salen SIEMPRE de la API key resuelta (D-10).
		TenantID:      dev.TenantID,
		DeviceID:      dev.DeviceID,
		MachineID:     *e.MachineID,
		Ts:            ts.UTC(),
		Seq:           seq,
		Kind:          *e.Kind,
		SchemaVersion: int(schemaVersion),
		Payload:       *e.Payload,
		ReceivedAt:    now,
	}, nil, skew
}
