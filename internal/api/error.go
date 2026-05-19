package api

import (
	"errors"
	"fmt"
	"time"

	"github.com/fguillen/dun-cli/internal/api/gen"
)

// Error is the typed form of the backend's
// `{ "error": { "code", "message", "retry_after?" } }` envelope.
//
// UI code switches on Code (a stable string identifier per §
// game-design.md error_envelope) rather than on HTTPStatus, since the
// underlying union response types from ogen do not expose the status
// code. RequestID is the X-Request-Id captured by the transport — empty
// when the upstream response carried no such header.
type Error struct {
	Code       string
	Message    string
	RequestID  string
	HTTPStatus int
}

// Error implements the error interface.
func (e *Error) Error() string {
	if e == nil {
		return "<nil api.Error>"
	}
	if e.RequestID != "" {
		return fmt.Sprintf("api: %s: %s (request_id=%s)", e.Code, e.Message, e.RequestID)
	}
	return fmt.Sprintf("api: %s: %s", e.Code, e.Message)
}

// RateLimitError wraps Error when the response envelope set `retry_after`.
// It exists as a distinct type so callers can `errors.As` against it
// without re-parsing the Code field.
type RateLimitError struct {
	Err        *Error
	RetryAfter time.Duration
}

// Error implements the error interface.
func (e *RateLimitError) Error() string {
	if e == nil || e.Err == nil {
		return "<nil api.RateLimitError>"
	}
	return fmt.Sprintf("%s (retry after %s)", e.Err.Error(), e.RetryAfter)
}

// Unwrap exposes the underlying *Error so errors.As works for both the
// rate-limit and plain envelope cases.
func (e *RateLimitError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// AsError extracts an *Error from any error returned by this package,
// whether it is wrapped in a *RateLimitError or not. Returns nil if the
// error is not an api-typed error.
func AsError(err error) *Error {
	var apiErr *Error
	if errors.As(err, &apiErr) {
		return apiErr
	}
	return nil
}

// fromEnvelope builds the typed error from a generated ErrorEnvelope plus
// the X-Request-Id captured by the transport.
func fromEnvelope(env *gen.ErrorEnvelope, requestID string) error {
	if env == nil {
		return nil
	}
	base := &Error{
		Code:      string(env.Error.Code),
		Message:   env.Error.Message,
		RequestID: requestID,
	}
	if ra, ok := env.Error.RetryAfter.Get(); ok {
		return &RateLimitError{
			Err:        base,
			RetryAfter: time.Duration(ra) * time.Second,
		}
	}
	return base
}
