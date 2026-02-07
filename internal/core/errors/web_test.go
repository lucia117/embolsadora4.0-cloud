package errors

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestWebErrorMarshalJSON(t *testing.T) {
	err := NewWebError(http.StatusBadRequest, "invalid input parameter")
	data, marshalErr := json.Marshal(err)
	if marshalErr != nil {
		t.Fatalf("marshal error: %v", marshalErr)
	}

	var payload map[string]any
	if unmarshalErr := json.Unmarshal(data, &payload); unmarshalErr != nil {
		t.Fatalf("unmarshal error: %v", unmarshalErr)
	}

	if payload["status"] != float64(http.StatusBadRequest) {
		t.Fatalf("status mismatch: %v", payload["status"])
	}
	if payload["error"] != "bad_request" {
		t.Fatalf("error mismatch: %v", payload["error"])
	}
	if payload["message"] != "invalid input parameter" {
		t.Fatalf("message mismatch: %v", payload["message"])
	}
}

func TestWebErrorCodeAutoFromStatus(t *testing.T) {
	err := &WebError{Status: http.StatusNotFound, Message: "resource not found"}
	data, marshalErr := json.Marshal(err)
	if marshalErr != nil {
		t.Fatalf("marshal error: %v", marshalErr)
	}

	var payload map[string]any
	if unmarshalErr := json.Unmarshal(data, &payload); unmarshalErr != nil {
		t.Fatalf("unmarshal error: %v", unmarshalErr)
	}

	if payload["error"] != "not_found" {
		t.Fatalf("error mismatch: %v", payload["error"])
	}
}

func TestWebErrorStatusCode(t *testing.T) {
	err := NewWebError(http.StatusUnauthorized, "authentication required")
	if err.StatusCode() != http.StatusUnauthorized {
		t.Fatalf("status code mismatch: %d", err.StatusCode())
	}
}
