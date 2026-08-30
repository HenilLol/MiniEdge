// Package errors defines the stable error classes used across MiniEdge.
// No internal details (stack traces, paths, credentials) are ever exposed
// to clients; those are logged via the logger package only.
package errors

import (
	"fmt"
	"net/http"
)

// Class is a stable, machine-readable error category returned to callers.
type Class string

const (
	ClassInvalidRequest    Class = "invalid_request"
	ClassRejectedUpstream  Class = "rejected_upstream"
	ClassTimeout           Class = "timeout"
	ClassUnavailable       Class = "upstream_unavailable"
	ClassLimitExceeded     Class = "limit_exceeded"
	ClassRateLimited       Class = "rate_limited"
	ClassUnauthorized      Class = "unauthorized"
	ClassInvalidConfig     Class = "invalid_configuration"
	ClassInvalidSimulation Class = "invalid_simulation"
	ClassInternal          Class = "internal_error"
)

// HTTPStatus maps each error class to its canonical HTTP status code.
func (c Class) HTTPStatus() int {
	switch c {
	case ClassInvalidRequest:
		return http.StatusBadRequest
	case ClassRejectedUpstream:
		return http.StatusBadGateway
	case ClassTimeout:
		return http.StatusGatewayTimeout
	case ClassUnavailable:
		return http.StatusBadGateway
	case ClassLimitExceeded:
		return http.StatusTooManyRequests
	case ClassRateLimited:
		return http.StatusTooManyRequests
	case ClassUnauthorized:
		return http.StatusUnauthorized
	case ClassInvalidConfig:
		return http.StatusUnprocessableEntity
	case ClassInvalidSimulation:
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}

// EdgeError is a bounded, stable error that may be returned to clients.
// It never carries internal details.
type EdgeError struct {
	Class     Class
	PublicMsg string // safe for client consumption
}

func (e *EdgeError) Error() string {
	return fmt.Sprintf("%s: %s", e.Class, e.PublicMsg)
}

// New creates an EdgeError with the given class and a client-safe message.
func New(class Class, publicMsg string) *EdgeError {
	return &EdgeError{Class: class, PublicMsg: publicMsg}
}

// Newf creates an EdgeError with a formatted client-safe message.
// Never include internal details in the format string.
func Newf(class Class, format string, args ...any) *EdgeError {
	return &EdgeError{Class: class, PublicMsg: fmt.Sprintf(format, args...)}
}
