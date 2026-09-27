package saltapp

import (
	"fmt"
	"strings"

	api "github.com/0000F8/saltapp-go/internal/saltapp"
)

// APIError is returned when Salt's API responds with a structured error body
// (the OpenAPI document's Error `{"error": "..."}` or Errors
// `{"errors": ["...", ...]}` schema). Use errors.As to recover the HTTP
// status code and the server's own message(s):
//
//	var apiErr *saltapp.APIError
//	if errors.As(err, &apiErr) && apiErr.StatusCode == 422 { ... }
type APIError struct {
	StatusCode int
	Messages   []string
}

func (e *APIError) Error() string {
	if len(e.Messages) == 0 {
		return fmt.Sprintf("saltapp: unexpected status %d", e.StatusCode)
	}
	return fmt.Sprintf("saltapp: %d: %s", e.StatusCode, strings.Join(e.Messages, "; "))
}

func singleAPIError(status int, e *api.Error) *APIError {
	if e == nil {
		return &APIError{StatusCode: status}
	}
	return &APIError{StatusCode: status, Messages: []string{e.Error}}
}

func pluralAPIError(status int, e *api.Errors) *APIError {
	if e == nil {
		return &APIError{StatusCode: status}
	}
	return &APIError{StatusCode: status, Messages: e.Errors}
}

// unexpectedStatus builds an APIError for a status code the caller didn't
// model as a typed field (a new response shape, a proxy error page, ...).
func unexpectedStatus(statusCode int, body []byte) *APIError {
	msg := strings.TrimSpace(string(body))
	if len(msg) > 300 {
		msg = msg[:300] + "..."
	}
	if msg == "" {
		return &APIError{StatusCode: statusCode}
	}
	return &APIError{StatusCode: statusCode, Messages: []string{msg}}
}
