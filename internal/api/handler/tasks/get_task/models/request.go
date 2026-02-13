package models

import (
	"net/http"

	"github.com/google/uuid"
)

type Request struct {
	ID       string `path:"id" validate:"required,uuid4"`
	TenantID string `header:"X-Tenant-Id" validate:"required,uuid4"`
}

func Parse(r *http.Request) (Request, error) {

	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		return Request{}, err
	}

	return Request{
		ID:       id.String(),
		TenantID: r.Header.Get("X-Tenant-Id"),
	}, nil
}
