// Command genfixture emite un batch de 108 eventos con la misma forma que los
// que manda el Pi, para usar como fixture de los tests de contrato.
//
// Es determinista: correrlo dos veces produce byte por byte el mismo archivo,
// asi que el fixture se puede versionar y los diffs son significativos.
//
//	go run ./scripts/genfixture > internal/consumers/testdata/last-batch.json
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"time"
)

const machineID = "EMB-DEV-001"

type event struct {
	EventID       string         `json:"eventId"`
	MachineID     string         `json:"machineId"`
	Ts            string         `json:"ts"`
	Seq           *int64         `json:"seq,omitempty"`
	Kind          string         `json:"kind"`
	SchemaVersion int            `json:"schemaVersion"`
	Payload       map[string]any `json:"payload"`
}

// computeEventID replica embolsadora-edge/internal/app/forwarder/eventid.go:
// sha256(machineId|aasPath|ts_nano).
func computeEventID(machineID, aasPath string, ts time.Time) string {
	h := sha256.New()
	h.Write([]byte(machineID))
	h.Write([]byte("|"))
	h.Write([]byte(aasPath))
	h.Write([]byte("|"))
	h.Write([]byte(fmt.Sprintf("%d", ts.UnixNano())))
	return hex.EncodeToString(h.Sum(nil))
}

func main() {
	props := []struct {
		path, unit, valueType string
		value                 any
	}{
		{"Operativos/Pesada/peso", "kg", "xs:float", 1.0},
		{"Operativos/Pesada/contador", "count", "xs:int", 100.0},
		{"Operativos/Sellado/temperatura", "degC", "xs:float", 82.5},
		{"Alarmas/estado", "bool", "xs:boolean", false},
	}

	base := time.Date(2026, 7, 31, 1, 6, 37, 147166000, time.UTC)
	events := make([]event, 0, 108)

	// 27 instantes x 4 propiedades = 108 eventos. Los 4 de un mismo instante
	// comparten ts y se distinguen por seq: es exactamente lo que hace el
	// historian, que escribe varias propiedades en el mismo tick.
	for tick := 0; tick < 27; tick++ {
		ts := base.Add(time.Duration(tick) * time.Second)
		for i, p := range props {
			seq := int64(i)
			events = append(events, event{
				EventID:       computeEventID(machineID, p.path, ts),
				MachineID:     machineID,
				Ts:            ts.Format(time.RFC3339Nano),
				Seq:           &seq,
				Kind:          "metric",
				SchemaVersion: 1,
				Payload: map[string]any{
					"aasPath":   p.path,
					"value":     p.value,
					"unit":      p.unit,
					"valueType": p.valueType,
				},
			})
		}
	}

	out, err := json.MarshalIndent(map[string]any{"events": events}, "", "  ")
	if err != nil {
		panic(err)
	}
	os.Stdout.Write(append(out, '\n'))
}
