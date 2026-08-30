package model

import (
	"fmt"
)

// ErrorCategory identifies conceptual gateway error categories.
type ErrorCategory string

const (
	ErrCodeRouteNotFound      ErrorCategory = "route_not_found"
	ErrCodeServiceUnavailable ErrorCategory = "service_unavailable"
	ErrCodeUpstreamTimeout    ErrorCategory = "upstream_timeout"
	ErrCodeBadGateway         ErrorCategory = "bad_gateway"
	ErrCodeRateLimitExceeded  ErrorCategory = "rate_limit_exceeded"
	ErrCodeSimulationActive   ErrorCategory = "simulation_active"
	ErrCodeInternalError      ErrorCategory = "internal_error"
)

// GatewayError represents a backend/gateway error suitable for eventual HTTP mapping.
type GatewayError struct {
	Code       ErrorCategory `json:"code"`
	Message    string        `json:"message"`
	HTTPStatus int           `json:"http_status,omitempty"`
}

func (e *GatewayError) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("[%s] %s", e.Code, e.Message)
	}
	return e.Message
}
