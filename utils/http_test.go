package utils

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewHttpClient(t *testing.T) {
	client := NewHttpClient()

	assert.NotNil(t, client.httpClient)
	assert.Zero(t, client.httpClient.Timeout)
	assert.Equal(t, DefaultRequestTimeout, client.requestTimeout)
	assert.Zero(t, client.downloadTimeout)
	assert.Equal(t, UserAgent, client.headers["User-Agent"])
	assert.Equal(t, "application/json", client.headers["Content-Type"])
}

func TestSetToken(t *testing.T) {
	client := NewHttpClient()
	token := "test-token"
	client.SetToken(token)

	expected := "OAuth " + token
	assert.Equal(t, expected, client.headers["Authorization"])
}

func TestSetDownloadTimeout(t *testing.T) {
	client := NewHttpClient()

	client.SetDownloadTimeout(42 * time.Second)
	assert.Equal(t, 42*time.Second, client.downloadTimeout)

	client.SetDownloadTimeout(-1 * time.Second)
	assert.Zero(t, client.downloadTimeout)
}

func TestGet(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		w.Write([]byte(`{"result": "success"}`))
	}))
	defer server.Close()

	client := NewHttpClient()
	resp, err := client.Get(server.URL)

	assert.NoError(t, err)
	assert.Equal(t, `{"result": "success"}`, string(resp))
}

func TestGetUsesConfiguredRequestTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"result":"success"}`))
	}))
	defer server.Close()

	client := NewHttpClient()
	client.requestTimeout = 50 * time.Millisecond

	_, err := client.Get(server.URL)

	assert.Error(t, err)
	assert.ErrorContains(t, err, "context deadline exceeded")
}

func TestGetWithContextLogsRequestAndRedactsAuthorization(t *testing.T) {
	var logs bytes.Buffer
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		w.Write([]byte(`{"result": "success"}`))
	}))
	defer server.Close()

	logger := NewDownloadLoggerForWriter(&logs)
	client := NewHttpClientWithLogger(logger)
	client.SetToken("secret-token")

	trackCtx := TrackLogContext{
		ID:    "123",
		Title: "Test Track",
		URL:   "https://music.yandex.ru/track/123",
	}

	resp, err := client.GetWithContext(RequestLogContext{
		Track:     trackCtx,
		Stage:     "download_info",
		Operation: "fetch_download_info",
	}, server.URL)

	assert.NoError(t, err)
	assert.Equal(t, `{"result": "success"}`, string(resp))
	assert.Contains(t, logs.String(), "http request started")
	assert.Contains(t, logs.String(), "http request finished")
	assert.Contains(t, logs.String(), "track_title=\"Test Track\"")
	assert.Contains(t, logs.String(), "stage=download_info")
	assert.NotContains(t, logs.String(), "secret-token")
	assert.Contains(t, logs.String(), "***")
}

func TestPost(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		w.Write([]byte(`{"result": "success"}`))
	}))
	defer server.Close()

	client := NewHttpClient()
	resp, err := client.Post(server.URL, []byte(`{"test": "data"}`))

	assert.NoError(t, err)
	assert.Equal(t, `{"result": "success"}`, string(resp))
}

func TestDownloadFile(t *testing.T) {
	content := "test file content"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		w.Write([]byte(content))
	}))
	defer server.Close()

	client := NewHttpClient()
	tempFile := "test_download.tmp"
	defer os.Remove(tempFile)

	err := client.DownloadFile(server.URL, tempFile)
	assert.NoError(t, err)

	data, err := os.ReadFile(tempFile)
	assert.NoError(t, err)
	assert.Equal(t, content, string(data))
}

func TestDownloadFileError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := NewHttpClient()
	tempFile := "test_download_error.tmp"
	defer os.Remove(tempFile)

	err := client.DownloadFile(server.URL, tempFile)
	assert.Error(t, err)
}

func TestDownloadFileWithContextLogsBadStatus(t *testing.T) {
	var logs bytes.Buffer
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error":"missing"}`))
	}))
	defer server.Close()

	logger := NewDownloadLoggerForWriter(&logs)
	client := NewHttpClientWithLogger(logger)
	tempFile := "test_download_error_with_logs.tmp"
	defer os.Remove(tempFile)

	err := client.DownloadFileWithContext(RequestLogContext{
		Track: TrackLogContext{
			ID:    "123",
			Title: "Test Track",
			URL:   "https://music.yandex.ru/track/123",
		},
		Stage:     "download_file",
		Operation: "download_mp3",
	}, server.URL, tempFile)

	assert.Error(t, err)
	assert.Contains(t, logs.String(), "download request finished with bad status")
	assert.Contains(t, logs.String(), "status_code=404")
	assert.Contains(t, logs.String(), "track_url=https://music.yandex.ru/track/123")
}

func TestDownloadFileCancelRemovesTempFile(t *testing.T) {
	serverStarted := make(chan struct{})
	serverCanFinish := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("partial"))
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}

		close(serverStarted)
		<-serverCanFinish
		_, _ = w.Write([]byte(" content"))
	}))
	defer server.Close()

	client := NewHttpClient()
	targetFile := filepath.Join(t.TempDir(), "track.mp3")

	errCh := make(chan error, 1)
	go func() {
		errCh <- client.DownloadFile(server.URL, targetFile)
	}()

	<-serverStarted
	client.Cancel()
	close(serverCanFinish)

	err := <-errCh
	assert.Error(t, err)
	_, statErr := os.Stat(targetFile)
	assert.True(t, os.IsNotExist(statErr))

	tempFiles, globErr := filepath.Glob(targetFile + ".*.part")
	assert.NoError(t, globErr)
	assert.Empty(t, tempFiles)
}

func TestDownloadFileWithConfiguredTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("partial"))
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}

		time.Sleep(100 * time.Millisecond)
		_, _ = w.Write([]byte(" content"))
	}))
	defer server.Close()

	client := NewHttpClient()
	client.SetDownloadTimeout(50 * time.Millisecond)
	targetFile := filepath.Join(t.TempDir(), "track.mp3")

	err := client.DownloadFile(server.URL, targetFile)

	assert.Error(t, err)
	assert.ErrorContains(t, err, "context deadline exceeded")

	_, statErr := os.Stat(targetFile)
	assert.True(t, os.IsNotExist(statErr))
}

func TestDownloadFileDoesNotUseRequestTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		time.Sleep(75 * time.Millisecond)
		_, _ = w.Write([]byte("full content"))
	}))
	defer server.Close()

	client := NewHttpClient()
	client.requestTimeout = 25 * time.Millisecond
	targetFile := filepath.Join(t.TempDir(), "track.mp3")

	err := client.DownloadFile(server.URL, targetFile)

	assert.NoError(t, err)

	data, readErr := os.ReadFile(targetFile)
	assert.NoError(t, readErr)
	assert.Equal(t, "full content", string(data))
}

func TestDownloadFileWithContextRedactsSignedURL(t *testing.T) {
	var logs bytes.Buffer
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	}))
	defer server.Close()

	logger := NewDownloadLoggerForWriter(&logs)
	client := NewHttpClientWithLogger(logger)
	tempFile := filepath.Join(t.TempDir(), "test_download_url_redaction.tmp")

	signedURL := server.URL + "/get-mp3/signed-part/ts-value/path/to/file.mp3?token=secret"
	err := client.DownloadFileWithContext(RequestLogContext{}, signedURL, tempFile)

	assert.NoError(t, err)
	assert.Contains(t, logs.String(), "url="+SanitizeURL(signedURL))
	assert.NotContains(t, logs.String(), "token=secret")
	assert.NotContains(t, logs.String(), "/signed-part/ts-value/path/to/file.mp3")
}
