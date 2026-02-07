package errors

import (
	"encoding/json"
	"fmt"
	"net/http"
)

func ExampleNewWebError() {
	err := NewWebError(http.StatusBadRequest, "invalid input parameter")
	data, _ := json.Marshal(err)
	fmt.Println(string(data))
	// Output: {"status":400,"error":"bad_request","message":"invalid input parameter"}
}
