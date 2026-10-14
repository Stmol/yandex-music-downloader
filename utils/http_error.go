package utils

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"unicode"
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
	text = SanitizeTerminalText(text)
	if runes := []rune(text); len(runes) > 200 {
		text = string(runes[:200])
	}
	return &HTTPStatusError{StatusCode: statusCode, Body: text}
}

func (e *HTTPStatusError) Error() string {
	if e == nil {
		return "HTTP error"
	}
	if e.APIError != nil {
		return SanitizeTerminalText(e.APIError.Error())
	}
	body := SanitizeTerminalText(strings.TrimSpace(e.Body))
	if body == "" {
		return fmt.Sprintf("HTTP %d", e.StatusCode)
	}
	return fmt.Sprintf("HTTP %d: %s", e.StatusCode, body)
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

// sanitizeTerminalText makes response text safe to print to a terminal:
// control characters such as ESC are shown as escaped hex instead of being
// interpreted, and line breaks and tabs become spaces.
func SanitizeTerminalText(text string) string {
	var b strings.Builder
	for _, r := range text {
		switch {
		case r == '\n' || r == '\r' || r == '\t':
			b.WriteByte(' ')
		case unicode.IsControl(r):
			fmt.Fprintf(&b, "\\x%02x", r)
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}
