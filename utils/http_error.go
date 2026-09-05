package utils

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"unicode/utf8"
)

// HTTPStatusError is a response with status >= 400 that has no JSON API error.
type HTTPStatusError struct {
	StatusCode int
	Body       string
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
	if strings.TrimSpace(e.Body) == "" {
		return fmt.Sprintf("HTTP %d", e.StatusCode)
	}
	return fmt.Sprintf("HTTP %d: %s", e.StatusCode, e.Body)
}

func IsRetryableHTTPError(err error) bool {
	var httpErr *HTTPStatusError
	if !errors.As(err, &httpErr) || httpErr == nil {
		return false
	}
	return httpErr.StatusCode == http.StatusTooManyRequests || httpErr.StatusCode >= 500
}
