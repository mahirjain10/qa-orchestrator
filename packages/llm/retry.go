package llm

import (
	"fmt"
	"time"
)

type RetryConfig struct {
	MaxRetries   int
	InitialDelay time.Duration
	MaxDelay     time.Duration
	Multiplier   float64
}

var DefaultRetryConfig = RetryConfig{
	MaxRetries:   3,
	InitialDelay: 500 * time.Millisecond,
	MaxDelay:     10 * time.Second,
	Multiplier:   2.0,
}

type RetryableError struct {
	Err        error
	Retryable  bool
	RetryAfter time.Duration
}

func (e *RetryableError) Error() string {
	return e.Err.Error()
}

func (e *RetryableError) Unwrap() error {
	return e.Err
}

func IsRetryable(err error) bool {
	re, ok := err.(*RetryableError)
	if !ok {
		return false
	}
	return re.Retryable
}

func IsRateLimit(err error) bool {
	re, ok := err.(*RetryableError)
	if !ok {
		return false
	}
	return re.RetryAfter > 0
}

func (c *RetryConfig) CalculateDelay(attempt int) time.Duration {
	delay := c.InitialDelay
	for i := 0; i < attempt; i++ {
		delay = time.Duration(float64(delay) * c.Multiplier)
		if delay > c.MaxDelay {
			delay = c.MaxDelay
			break
		}
	}
	return delay
}

func ShouldRetry(err error, attempt int, maxRetries int) (bool, time.Duration) {
	if attempt >= maxRetries {
		return false, 0
	}

	re, ok := err.(*RetryableError)
	if !ok {
		return false, 0
	}

	if !re.Retryable {
		return false, 0
	}

	delay := re.RetryAfter
	if delay == 0 {
		delay = DefaultRetryConfig.CalculateDelay(attempt)
	}

	return true, delay
}

func NewRetryableError(err error, retryable bool, retryAfter ...time.Duration) *RetryableError {
	re := &RetryableError{
		Err:       err,
		Retryable: retryable,
	}
	if len(retryAfter) > 0 {
		re.RetryAfter = retryAfter[0]
	}
	return re
}

func NewNonRetryableError(err error) *RetryableError {
	return &RetryableError{
		Err:       err,
		Retryable: false,
	}
}

type NetworkError string

func (e NetworkError) Error() string {
	return string(e)
}

func IsNetworkError(err error) bool {
	_, ok := err.(NetworkError)
	return ok
}

type APIError struct {
	StatusCode int
	Message    string
	Type       string
	Code       string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("API error (status=%d): %s", e.StatusCode, e.Message)
}

func IsAPIError(err error) bool {
	_, ok := err.(*APIError)
	return ok
}

func IsRetryableStatusCode(code int) bool {
	switch code {
	case 429, 500, 502, 503, 504:
		return true
	default:
		return false
	}
}
