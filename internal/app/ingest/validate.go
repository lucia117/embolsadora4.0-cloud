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
func ValidateEvent(raw json.RawMessage, dev domain.DeviceContext, now time.Time) (domain.Measurement, *domain.EventError) {
	var e rawEvent
	if err := json.Unmarshal(raw, &e); err != nil {
		return domain.Measurement{}, invalid("evento no deserializable: %v", err)
	}

	switch {
	case e.EventID == nil || *e.EventID == "":
		return domain.Measurement{}, invalid("falta el campo requerido eventId")
	case e.MachineID == nil || *e.MachineID == "":
		return domain.Measurement{}, invalid("falta el campo requerido machineId")
	case e.Ts == nil:
		return domain.Measurement{}, invalid("falta el campo requerido ts")
	case e.Kind == nil:
		return domain.Measurement{}, invalid("falta el campo requerido kind")
	case e.SchemaVersion == nil:
		return domain.Measurement{}, invalid("falta el campo requerido schemaVersion")
	case e.Payload == nil:
		return domain.Measurement{}, invalid("falta el campo requerido payload")
	}

	ts, err := time.Parse(time.RFC3339, *e.Ts)
	if err != nil {
		return domain.Measurement{}, invalid("ts no es RFC3339: %q", *e.Ts)
	}

	switch *e.Kind {
	case domain.KindMetric, domain.KindAlarm, domain.KindHeartbeat:
	default:
		return domain.Measurement{}, invalid("kind fuera del enum: %q", *e.Kind)
	}

	schemaVersion, ok := jsonInt(*e.SchemaVersion)
	if !ok || schemaVersion < 1 {
		return domain.Measurement{}, invalid("schemaVersion debe ser un entero >= 1")
	}

	var seq *int64
	if e.Seq != nil {
		n, ok := jsonInt(*e.Seq)
		if !ok || n < 0 {
			return domain.Measurement{}, invalid("seq debe ser un entero >= 0")
		}
		seq = &n
	}

	// A partir de aca el sobre es valido; lo que sigue son rechazos de
	// VALIDATION_FAILED, que el Edge tambien manda a DEAD pero que significan
	// otra cosa: el dato esta bien formado y aun asi no corresponde.

	// Un Pi comprometido no puede escribir en nombre de otra maquina: el
	// machineId del body solo se acepta si coincide con el device de la key.
	if *e.MachineID != dev.MachineID {
		return domain.Measurement{}, failed("machineId %q no corresponde al device de la API key", *e.MachineID)
	}
	if schemaVersion > domain.MaxSchemaVersion {
		return domain.Measurement{}, failed("schemaVersion %d no soportada (maxima: %d)", schemaVersion, domain.MaxSchemaVersion)
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
	}, nil
}
