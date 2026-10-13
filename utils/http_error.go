package utils

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"unicode/utf8"
)

// HTTPStatusError is a response with status >= 400. APIError holds the JSON API
// error from the response body when there is one, so the status code stays
// available for retry decisions either way.
type HTTPStatusError struct {
	StatusCode int
	Body       string
	APIError   error
}

func newHTTPStatusError(statusCode int, body []byte) *HTTPStatusError {
	text := strings.TrimSpace(string(body))
	if !utf8.ValidString(text) || !isMostlyPrintable(body) {
		text = ""
	}
	if len(text) > 200 {
		text = text[:200]
	}
	return &HTTPStatusError{StatusCode: statusCode, Body: text}
}

func (e *HTTPStatusError) Error() string {
	if e == nil {
		return "HTTP error"
	}
	if e.APIError != nil {
		return e.APIError.Error()
	}
	if strings.TrimSpace(e.Body) == "" {
		return fmt.Sprintf("HTTP %d", e.StatusCode)
	}
	return fmt.Sprintf("HTTP %d: %s", e.StatusCode, e.Body)
}

// Unwrap exposes the JSON API error, if any.
func (e *HTTPStatusError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.APIError
}

func IsRetryableHTTPError(err error) bool {
	var httpErr *HTTPStatusError
	if !errors.As(err, &httpErr) || httpErr == nil {
		return false
	}
	return httpErr.StatusCode == http.StatusTooManyRequests || httpErr.StatusCode >= 500
}
