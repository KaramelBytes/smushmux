package ai

import (
	"fmt"
	"time"
)

// AuthError indicates authentication/authorization failures (401/403).
type AuthError struct{ *APIError }

func (e *AuthError) Error() string {
	return fmt.Sprintf("authentication failed: %s", e.APIError.Error())
}

// RateLimitError indicates 429 responses and may include a Retry-After.
type RateLimitError struct {
	*APIError
	RetryAfter time.Duration
}

func (e *RateLimitError) Error() string {
	if e.RetryAfter > 0 {
		return fmt.Sprintf("rate limited: wait about %ds before retrying: %s", int(e.RetryAfter.Seconds()), e.APIError.Error())
	}
	return fmt.Sprintf("rate limited: %s", e.APIError.Error())
}

// ModelNotFoundError indicates the requested model is not available.
type ModelNotFoundError struct{ *APIError }

func (e *ModelNotFoundError) Error() string {
	return fmt.Sprintf("model not found: %s", e.APIError.Error())
}

// BadRequestError indicates a 4xx request problem (e.g., 400 validation).
type BadRequestError struct{ *APIError }

func (e *BadRequestError) Error() string { return fmt.Sprintf("bad request: %s", e.APIError.Error()) }

// QuotaExceededError indicates billing/quota problems.
type QuotaExceededError struct{ *APIError }

func (e *QuotaExceededError) Error() string {
	return fmt.Sprintf("quota exceeded: %s", e.APIError.Error())
}

// ServerError indicates 5xx errors from the provider.
type ServerError struct{ *APIError }

func (e *ServerError) Error() string { return fmt.Sprintf("provider error: %s", e.APIError.Error()) }

// UnreachableError indicates the target runtime is not reachable (e.g., local Ollama down).
type UnreachableError struct {
	Host string
	Err  error
}

func (e *UnreachableError) Error() string {
	if e == nil {
		return "unreachable"
	}
	if e.Host != "" {
		return fmt.Sprintf("endpoint unreachable at %s: %v", e.Host, e.Err)
	}
	return fmt.Sprintf("endpoint unreachable: %v", e.Err)
}
