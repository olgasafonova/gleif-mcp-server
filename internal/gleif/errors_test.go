package gleif

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// TestAPIErrorError verifies the Error() string format with and without a
// status code.
func TestAPIErrorError(t *testing.T) {
	tests := []struct {
		name string
		err  *APIError
		want string
	}{
		{
			name: "with status code",
			err:  &APIError{Code: ErrCodeNotFound, Message: "missing", StatusCode: 404},
			want: "not_found (HTTP 404): missing",
		},
		{
			name: "without status code",
			err:  &APIError{Code: ErrCodeTimeout, Message: "slow"},
			want: "timeout: slow",
		},
		{
			name: "zero status code omits HTTP prefix",
			err:  &APIError{Code: ErrCodeNetworkError, Message: "down", StatusCode: 0},
			want: "network_error: down",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.want {
				t.Errorf("Error() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestAPIErrorIs verifies code-based comparison through errors.Is.
func TestAPIErrorIs(t *testing.T) {
	notFound := &APIError{Code: ErrCodeNotFound, Message: "a"}
	otherNotFound := &APIError{Code: ErrCodeNotFound, Message: "different message"}
	rateLimited := &APIError{Code: ErrCodeRateLimited}

	if !errors.Is(notFound, otherNotFound) {
		t.Error("errors.Is should match on equal Code regardless of message")
	}
	if errors.Is(notFound, rateLimited) {
		t.Error("errors.Is should not match on differing Code")
	}
	if errors.Is(notFound, errors.New("plain")) {
		t.Error("errors.Is should not match a non-APIError target")
	}
}

// TestNewNotFoundError verifies the not-found constructor's shape.
func TestNewNotFoundError(t *testing.T) {
	err := NewNotFoundError("LEI XYZ")
	if err.Code != ErrCodeNotFound {
		t.Errorf("Code = %q, want %q", err.Code, ErrCodeNotFound)
	}
	if err.StatusCode != 404 {
		t.Errorf("StatusCode = %d, want 404", err.StatusCode)
	}
	if err.Retryable {
		t.Error("not-found should not be retryable")
	}
	if !strings.Contains(err.Message, "LEI XYZ") {
		t.Errorf("Message %q should contain the resource name", err.Message)
	}
}

// TestNewInvalidFormatError verifies the invalid-format constructor embeds the
// field, reason, and structured details.
func TestNewInvalidFormatError(t *testing.T) {
	err := NewInvalidFormatError("LEI", "must be 20 chars")
	if err.Code != ErrCodeInvalidFormat {
		t.Errorf("Code = %q, want %q", err.Code, ErrCodeInvalidFormat)
	}
	if err.Retryable {
		t.Error("invalid-format should not be retryable")
	}
	if !strings.Contains(err.Message, "LEI") || !strings.Contains(err.Message, "must be 20 chars") {
		t.Errorf("Message %q should contain field and reason", err.Message)
	}
	if !strings.Contains(err.Message, feedbackURL) {
		t.Errorf("Message %q should contain the feedback URL", err.Message)
	}

	details, ok := err.Details.(map[string]string)
	if !ok {
		t.Fatalf("Details is %T, want map[string]string", err.Details)
	}
	if details["field"] != "LEI" || details["reason"] != "must be 20 chars" {
		t.Errorf("Details = %v, want field/reason populated", details)
	}
}

// TestNewRateLimitError verifies the rate-limit constructor.
func TestNewRateLimitError(t *testing.T) {
	err := NewRateLimitError(30)
	if err.Code != ErrCodeRateLimited {
		t.Errorf("Code = %q, want %q", err.Code, ErrCodeRateLimited)
	}
	if err.StatusCode != 429 {
		t.Errorf("StatusCode = %d, want 429", err.StatusCode)
	}
	if !err.Retryable {
		t.Error("rate-limit should be retryable")
	}
	details, ok := err.Details.(map[string]int)
	if !ok {
		t.Fatalf("Details is %T, want map[string]int", err.Details)
	}
	if details["retryAfterSeconds"] != 30 {
		t.Errorf("retryAfterSeconds = %d, want 30", details["retryAfterSeconds"])
	}
}

// TestNewTimeoutError verifies the timeout constructor.
func TestNewTimeoutError(t *testing.T) {
	err := NewTimeoutError()
	if err.Code != ErrCodeTimeout {
		t.Errorf("Code = %q, want %q", err.Code, ErrCodeTimeout)
	}
	if !err.Retryable {
		t.Error("timeout should be retryable")
	}
}

// TestNewServerError verifies server-error retryability depends on the status
// code and that the body argument is dropped (HG-2 sanitization).
func TestNewServerError(t *testing.T) {
	tests := []struct {
		name          string
		statusCode    int
		wantRetryable bool
	}{
		{"500 retryable", 500, true},
		{"502 retryable", 502, true},
		{"503 retryable", 503, true},
		{"400 not retryable", 400, false},
		{"499 not retryable", 499, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			secretBody := "internal stack trace SECRET-TOKEN-123"
			err := NewServerError(tt.statusCode, secretBody)
			if err.Code != ErrCodeServerError {
				t.Errorf("Code = %q, want %q", err.Code, ErrCodeServerError)
			}
			if err.StatusCode != tt.statusCode {
				t.Errorf("StatusCode = %d, want %d", err.StatusCode, tt.statusCode)
			}
			if err.Retryable != tt.wantRetryable {
				t.Errorf("Retryable = %v, want %v", err.Retryable, tt.wantRetryable)
			}
			if strings.Contains(err.Message, secretBody) || strings.Contains(err.Error(), "SECRET-TOKEN") {
				t.Error("server error must not leak the raw response body")
			}
		})
	}
}

// TestNewNetworkError verifies the network-error constructor wraps the cause
// message and is retryable.
func TestNewNetworkError(t *testing.T) {
	cause := errors.New("connection refused")
	err := NewNetworkError(cause)
	if err.Code != ErrCodeNetworkError {
		t.Errorf("Code = %q, want %q", err.Code, ErrCodeNetworkError)
	}
	if !err.Retryable {
		t.Error("network error should be retryable")
	}
	if !strings.Contains(err.Message, "connection refused") {
		t.Errorf("Message %q should contain the cause", err.Message)
	}
}

// TestIsRetryable verifies retryability detection through wrapped errors.
func TestIsRetryable(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"retryable rate limit", NewRateLimitError(10), true},
		{"retryable timeout", NewTimeoutError(), true},
		{"non-retryable not found", NewNotFoundError("x"), false},
		{"non-retryable invalid format", NewInvalidFormatError("LEI", "bad"), false},
		{"wrapped retryable", fmt.Errorf("context: %w", NewTimeoutError()), true},
		{"plain error", errors.New("boom"), false},
		{"nil error", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsRetryable(tt.err); got != tt.want {
				t.Errorf("IsRetryable(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}
