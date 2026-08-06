// Package dto contiene los tipos de transporte del contrato con el Edge.
package dto

import (
	"encoding/json"

	"github.com/tu-org/embolsadora-api/internal/domain/ingest"
)

// BatchEventsRequest es el body del POST /api/v1/consumers/events.
//
// Events es []json.RawMessage y no []Event a proposito: si fuera una lista de
// structs tipados, UN evento con un tipo equivocado —un ts numerico, por
// ejemplo— haria fallar el Unmarshal del body entero. Eso obligaria a devolver
// 400, y el Edge mandaria los hasta 1000 eventos del batch a DEAD por culpa de
// uno solo (invariante I-2). Difiriendo el decode a cada elemento, el evento
// roto se rechaza solo, con su indice.
type BatchEventsRequest struct {
	Events []json.RawMessage `json:"events"`
}

// BatchEventsResponse es la respuesta 200.
//
// El contrato pide {"data": {...}} pelado. NO lleva el envelope
// {"success": true, "data": ...} que usa el resto de los handlers del repo:
// el parser del Edge esta congelado contra esta forma.
type BatchEventsResponse struct {
	Data ingest.Result `json:"data"`
}
