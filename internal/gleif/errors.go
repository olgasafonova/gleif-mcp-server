package gleif

import (
	"errors"
	"fmt"
)

// feedbackURL is the pre-filled issue link shown on persistent errors.
const feedbackURL = "https://github.com/olgasafonova/gleif-mcp-server/issues/new?template=bug_report.yml"

// Error codes for API errors.
const (
	ErrCodeNotFound      = "not_found"
	ErrCodeInvalidFormat = "invalid_format"
	ErrCodeRateLimited   = "rate_limited"
	ErrCodeTimeout       = "timeout"
	ErrCodeServerError   = "server_error"
	ErrCodeNetworkError  = "network_error"
	ErrCodeValidation    = "validation_error"
)

// Common errors.
var (
	ErrNotFound       = errors.New("resource not found")
	ErrRateLimited    = errors.New("rate limit exceeded")
	ErrInvalidLEI     = errors.New("invalid LEI format")
	ErrInvalidBIC     = errors.New("invalid BIC format")
	ErrInvalidISIN    = errors.New("invalid ISIN format")
	ErrInvalidCountry = errors.New("invalid country code")
)

// APIError represents a structured API error.
type APIError struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	Details    any    `json:"details,omitempty"`
	StatusCode int    `json:"statusCode,omitempty"`
	Retryable  bool   `json:"retryable"`
}

func (e *APIError) Error() string {
	if e.StatusCode > 0 {
		return fmt.Sprintf("%s (HTTP %d): %s", e.Code, e.StatusCode, e.Message)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Is implements error comparison.
func (e *APIError) Is(target error) bool {
	if t, ok := target.(*APIError); ok {
		return e.Code == t.Code
	}
	return false
}

// NewNotFoundError creates a not found error.
func NewNotFoundError(resource string) *APIError {
	return &APIError{
		Code:       ErrCodeNotFound,
		Message:    fmt.Sprintf("%s not found in GLEIF database", resource),
		StatusCode: 404,
		Retryable:  false,
	}
}

// NewInvalidFormatError creates a validation error.
func NewInvalidFormatError(field, reason string) *APIError {
	return &APIError{
		Code:      ErrCodeInvalidFormat,
		Message:   fmt.Sprintf("invalid %s format: %s. Still stuck? "+feedbackURL, field, reason),
		Details:   map[string]string{"field": field, "reason": reason},
		Retryable: false,
	}
}

// NewRateLimitError creates a rate limit error.
func NewRateLimitError(retryAfter int) *APIError {
	return &APIError{
		Code:       ErrCodeRateLimited,
		Message:    "GLEIF API rate limit exceeded (60 requests/minute). Please retry later.",
		Details:    map[string]int{"retryAfterSeconds": retryAfter},
		StatusCode: 429,
		Retryable:  true,
	}
}

// NewTimeoutError creates a timeout error.
func NewTimeoutError() *APIError {
	return &APIError{
		Code:      ErrCodeTimeout,
		Message:   "request timed out - GLEIF API may be slow or unavailable",
		Retryable: true,
	}
}

// NewServerError creates a server error. The body argument is deliberately
// ignored at this layer — we never propagate raw response bodies into the
// MCP caller's error structure (HG-2). Operators who need to inspect body
// content during incidents should add request-level debug logging in the
// HTTP client; the MCP error path is sanitized at the boundary.
//
// The body parameter remains in the signature for backward compatibility
// with callers that may still pass it, but it is dropped on the floor.
func NewServerError(statusCode int, _ string) *APIError {
	return &APIError{
		Code:       ErrCodeServerError,
		Message:    "GLEIF API returned an error",
		StatusCode: statusCode,
		Retryable:  statusCode >= 500,
	}
}

// NewNetworkError creates a network error.
func NewNetworkError(err error) *APIError {
	return &APIError{
		Code:      ErrCodeNetworkError,
		Message:   fmt.Sprintf("network error: %v", err),
		Retryable: true,
	}
}

// IsRetryable checks if an error can be retried.
func IsRetryable(err error) bool {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.Retryable
	}
	return false
}
