package utils

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
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
}

func NewHttpClient() *HttpClient {
	client := &HttpClient{
		httpClient: &http.Client{Timeout: DefaultTimeout},
		headers:    make(map[string]string),
	}

	client.headers["User-Agent"] = UserAgent
	client.headers["Content-Type"] = "application/json"

	return client
}

func (c *HttpClient) SetToken(token string) {
	c.headers["Authorization"] = fmt.Sprintf("OAuth %s", token)
}

func (c *HttpClient) Get(url string) ([]byte, error) {
	return c.sendRequest(http.MethodGet, url, nil)
}

func (c *HttpClient) Post(url string, data []byte) ([]byte, error) {
	return c.sendRequest(http.MethodPost, url, data)
}

func (c *HttpClient) sendRequest(method, url string, data []byte) ([]byte, error) {
	req, err := c.createRequest(method, url, data)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var errorResp model.ErrorResponse
	if err := json.Unmarshal(body, &errorResp); err == nil && errorResp.IsError() {
		return nil, &errorResp
	}

	return body, nil
}

func (c *HttpClient) createRequest(method, url string, data []byte) (*http.Request, error) {
	req, err := http.NewRequest(method, url, bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}

	for key, value := range c.headers {
		req.Header.Set(key, value)
	}

	return req, nil
}

func (c *HttpClient) DownloadFile(url, filepath string) error {
	ctx, cancel := context.WithTimeout(context.Background(), DefaultDownloadTimeout)
	defer cancel()

	req, err := c.createDownloadRequest(ctx, url)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download file: status code %d", resp.StatusCode)
	}

	return c.saveResponseToFile(resp.Body, filepath)
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

func (c *HttpClient) saveResponseToFile(body io.Reader, filepath string) error {
	out, err := os.Create(filepath)
	if err != nil {
		return fmt.Errorf("error creating file: %w", err)
	}
	defer out.Close()

	buf := make([]byte, DefaultBufferSize)
	_, err = io.CopyBuffer(out, body, buf)
	if err != nil {
		return fmt.Errorf("error writing to file: %w", err)
	}

	return nil
}
