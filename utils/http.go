package utils

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"ya-music/ya/model"
)

const (
	UserAgent              = "Yandex-Music-API"
	DefaultTimeout         = 10 * time.Second
	DefaultDownloadTimeout = 2 * time.Minute
	DefaultBufferSize      = 1024 * 1024
)

type HttpClient struct {
	httpClient *http.Client
	headers    map[string]string
	logger     *DownloadLogger
	ctx        context.Context
	cancel     context.CancelFunc
}

func NewHttpClient() *HttpClient {
	return NewHttpClientWithLogger(nil)
}

func NewHttpClientWithLogger(logger *DownloadLogger) *HttpClient {
	if logger == nil {
		logger = NewDiscardDownloadLogger()
	}

	ctx, cancel := context.WithCancel(context.Background())

	client := &HttpClient{
		httpClient: &http.Client{Timeout: DefaultTimeout},
		headers:    make(map[string]string),
		logger:     logger,
		ctx:        ctx,
		cancel:     cancel,
	}

	client.headers["User-Agent"] = UserAgent
	client.headers["Content-Type"] = "application/json"

	return client
}

func (c *HttpClient) Logger() *DownloadLogger {
	return c.logger
}

func (c *HttpClient) Cancel() {
	if c == nil || c.cancel == nil {
		return
	}

	c.cancel()
}

func (c *HttpClient) SetToken(token string) {
	c.headers["Authorization"] = fmt.Sprintf("OAuth %s", token)
}

func (c *HttpClient) Get(url string) ([]byte, error) {
	return c.GetWithContext(RequestLogContext{}, url)
}

func (c *HttpClient) GetWithContext(reqCtx RequestLogContext, url string) ([]byte, error) {
	return c.sendRequest(reqCtx, http.MethodGet, url, nil)
}

func (c *HttpClient) Post(url string, data []byte) ([]byte, error) {
	return c.PostWithContext(RequestLogContext{}, url, data)
}

func (c *HttpClient) PostWithContext(reqCtx RequestLogContext, url string, data []byte) ([]byte, error) {
	return c.sendRequest(reqCtx, http.MethodPost, url, data)
}

func (c *HttpClient) sendRequest(reqCtx RequestLogContext, method, url string, data []byte) ([]byte, error) {
	req, err := c.createRequest(method, url, data)
	if err != nil {
		c.logRequest(slog.LevelError, reqCtx, "http request create failed", method, url,
			"error", err,
		)
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	startedAt := time.Now()
	c.logRequest(slog.LevelInfo, reqCtx, "http request started", method, url,
		"headers", SanitizeHeaders(req.Header),
		"request_bytes", len(data),
	)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.logRequest(slog.LevelError, reqCtx, "http request failed", method, url,
			"duration_ms", time.Since(startedAt).Milliseconds(),
			"error", err,
		)
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		c.logRequest(slog.LevelError, reqCtx, "http response read failed", method, url,
			"status_code", resp.StatusCode,
			"duration_ms", time.Since(startedAt).Milliseconds(),
			"error", err,
		)
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	logLevel := slog.LevelInfo
	if resp.StatusCode >= http.StatusBadRequest {
		logLevel = slog.LevelError
	}

	c.logRequest(logLevel, reqCtx, "http request finished", method, url,
		"status_code", resp.StatusCode,
		"duration_ms", time.Since(startedAt).Milliseconds(),
		"response_bytes", len(body),
		"response_preview", responsePreview(resp.Header.Get("Content-Type"), body),
	)

	var errorResp model.ErrorResponse
	if err := json.Unmarshal(body, &errorResp); err == nil && errorResp.IsError() {
		c.logRequest(slog.LevelError, reqCtx, "api returned error payload", method, url,
			"api_error", errorResp.APIError.Name,
			"api_message", errorResp.APIError.Message,
		)
		return nil, &errorResp
	}

	return body, nil
}

func (c *HttpClient) createRequest(method, url string, data []byte) (*http.Request, error) {
	req, err := http.NewRequestWithContext(c.baseContext(), method, url, bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}

	for key, value := range c.headers {
		req.Header.Set(key, value)
	}

	return req, nil
}

func (c *HttpClient) DownloadFile(url, filepath string) error {
	return c.DownloadFileWithContext(RequestLogContext{}, url, filepath)
}

func (c *HttpClient) DownloadFileWithContext(reqCtx RequestLogContext, url, filepath string) error {
	ctx, cancel := context.WithTimeout(c.baseContext(), DefaultDownloadTimeout)
	defer cancel()

	req, err := c.createDownloadRequest(ctx, url)
	if err != nil {
		c.logRequest(slog.LevelError, reqCtx, "download request create failed", http.MethodGet, url,
			"destination", filepath,
			"error", err,
		)
		return err
	}

	startedAt := time.Now()
	c.logRequest(slog.LevelInfo, reqCtx, "download request started", http.MethodGet, url,
		"destination", filepath,
		"headers", SanitizeHeaders(req.Header),
	)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.logRequest(slog.LevelError, reqCtx, "download request failed", http.MethodGet, url,
			"destination", filepath,
			"duration_ms", time.Since(startedAt).Milliseconds(),
			"error", err,
		)
		return fmt.Errorf("failed to download file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		c.logRequest(slog.LevelError, reqCtx, "download request finished with bad status", http.MethodGet, url,
			"destination", filepath,
			"status_code", resp.StatusCode,
			"duration_ms", time.Since(startedAt).Milliseconds(),
			"response_bytes", len(body),
			"response_preview", responsePreview(resp.Header.Get("Content-Type"), body),
		)
		return fmt.Errorf("failed to download file: status code %d", resp.StatusCode)
	}

	written, err := c.saveResponseToFile(resp.Body, filepath)
	if err != nil {
		c.logRequest(slog.LevelError, reqCtx, "download file save failed", http.MethodGet, url,
			"destination", filepath,
			"status_code", resp.StatusCode,
			"duration_ms", time.Since(startedAt).Milliseconds(),
			"error", err,
		)
		return err
	}

	c.logRequest(slog.LevelInfo, reqCtx, "download request finished", http.MethodGet, url,
		"destination", filepath,
		"status_code", resp.StatusCode,
		"duration_ms", time.Since(startedAt).Milliseconds(),
		"response_bytes", written,
	)

	return nil
}

func (c *HttpClient) createDownloadRequest(ctx context.Context, url string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	for key, value := range c.headers {
		req.Header.Set(key, value)
	}

	return req, nil
}

func (c *HttpClient) saveResponseToFile(body io.Reader, filepath string) (int64, error) {
	out, err := c.createTempDownloadFile(filepath)
	if err != nil {
		return 0, fmt.Errorf("error creating file: %w", err)
	}
	tempPath := out.Name()
	cleanupTempFile := true
	defer func() {
		if out != nil {
			_ = out.Close()
		}
		if cleanupTempFile {
			_ = os.Remove(tempPath)
		}
	}()

	buf := make([]byte, DefaultBufferSize)
	written, err := io.CopyBuffer(out, body, buf)
	if err != nil {
		return written, fmt.Errorf("error writing to file: %w", err)
	}

	if err := out.Close(); err != nil {
		return written, fmt.Errorf("error closing temp file: %w", err)
	}
	out = nil

	if err := os.Rename(tempPath, filepath); err != nil {
		return written, fmt.Errorf("error renaming temp file: %w", err)
	}

	cleanupTempFile = false
	return written, nil
}

func (c *HttpClient) baseContext() context.Context {
	if c == nil || c.ctx == nil {
		return context.Background()
	}

	return c.ctx
}

func (c *HttpClient) logRequest(level slog.Level, reqCtx RequestLogContext, msg, method, rawURL string, args ...any) {
	attrs := []any{
		"method", method,
		"url", SanitizeURL(rawURL),
	}
	attrs = append(attrs, args...)
	c.logger.LogRequest(level, reqCtx, msg, attrs...)
}

func (c *HttpClient) createTempDownloadFile(destinationPath string) (*os.File, error) {
	dir := filepath.Dir(destinationPath)
	pattern := filepath.Base(destinationPath) + ".*.part"
	return os.CreateTemp(dir, pattern)
}

func responsePreview(contentType string, body []byte) string {
	if len(body) == 0 {
		return ""
	}

	contentType = strings.ToLower(contentType)
	if !strings.Contains(contentType, "json") && !strings.Contains(contentType, "xml") && !strings.Contains(contentType, "text") {
		return ""
	}

	const maxPreviewBytes = 512
	preview := strings.TrimSpace(string(body))
	if len(preview) <= maxPreviewBytes {
		return preview
	}

	return preview[:maxPreviewBytes] + "...(truncated)"
}
