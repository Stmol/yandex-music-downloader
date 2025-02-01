package utils

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewHttpClient(t *testing.T) {
	client := NewHttpClient()

	assert.NotNil(t, client.httpClient)
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
