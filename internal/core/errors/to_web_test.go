package errors

import (
	stderrors "errors"
	"net/http"
	"testing"
)

func TestToWebMappings(t *testing.T) {
	tests := []struct {
		name        string
		err         error
		wantStatus  int
		wantMessage string
	}{
		{"bad request", NewBadRequest("invalid input parameter"), http.StatusBadRequest, "invalid input parameter"},
		{"unauthorized", NewUnauthorized("authentication required"), http.StatusUnauthorized, "authentication required"},
		{"forbidden", NewForbidden("access denied"), http.StatusForbidden, "access denied"},
		{"not found", NewNotFound("resource not found"), http.StatusNotFound, "resource not found"},
		{"too many", NewTooManyRequests("rate limit exceeded"), http.StatusTooManyRequests, "rate limit exceeded"},
		{"internal", NewInternalServerError("internal error"), http.StatusInternalServerError, "internal error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			webErr, ok := ToWeb(tt.err).(*WebError)
			if !ok {
				t.Fatalf("expected *WebError, got %T", ToWeb(tt.err))
			}
			if webErr.Status != tt.wantStatus {
				t.Fatalf("status mismatch: %d", webErr.Status)
			}
			if webErr.Message != tt.wantMessage {
				t.Fatalf("message mismatch: %s", webErr.Message)
			}
		})
	}
}

func TestToWebUnknownDefaultsToInternal(t *testing.T) {
	webErr, ok := ToWeb(stderrors.New("boom")).(*WebError)
	if !ok {
		t.Fatalf("expected *WebError, got %T", ToWeb(stderrors.New("boom")))
	}
	if webErr.Status != http.StatusInternalServerError {
		t.Fatalf("status mismatch: %d", webErr.Status)
	}
	if webErr.Message != "internal error" {
		t.Fatalf("message mismatch: %s", webErr.Message)
	}
}
