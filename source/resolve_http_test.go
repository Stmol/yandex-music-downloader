package source

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
	"ya-music/utils"
	"ya-music/ya"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeMusicAPI serves the artist and album endpoints used by artist resolution.
// albumFailures lists the HTTP statuses returned, in order, before an album
// request succeeds; each failure carries a JSON API error body.
type fakeMusicAPI struct {
	mu            sync.Mutex
	albumFailures map[string][]int
	albumCalls    map[string]int
}

func (api *fakeMusicAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	switch {
	case strings.HasSuffix(r.URL.Path, "/direct-albums"):
		_, _ = fmt.Fprint(w, `{"result":{"albums":[{"id":1,"title":"Arrival","year":1976},{"id":2,"title":"SOS","year":1975}],"pager":{"total":2,"page":0,"perPage":50}}}`)
	case strings.HasPrefix(r.URL.Path, "/albums/"):
		id := strings.Split(strings.TrimPrefix(r.URL.Path, "/albums/"), "/")[0]
		api.mu.Lock()
		api.albumCalls[id]++
		var status int
		if failures := api.albumFailures[id]; len(failures) > 0 {
			status = failures[0]
			api.albumFailures[id] = failures[1:]
		}
		api.mu.Unlock()
		if status != 0 {
			w.WriteHeader(status)
			_, _ = fmt.Fprint(w, `{"error":{"name":"temporary-error","message":"Try again later"}}`)
			return
		}
		_, _ = fmt.Fprintf(w, `{"result":{"id":%s,"title":"Album %s","volumes":[[{"id":"%s0","title":"Track %s","available":true}]]}}`, id, id, id, id)
	default:
		http.NotFound(w, r)
	}
}

// rewriteToServer sends every request of the real API client to the test server.
type rewriteToServer struct{ host string }

func (t rewriteToServer) RoundTrip(req *http.Request) (*http.Response, error) {
	cloned := req.Clone(req.Context())
	cloned.URL.Scheme = "http"
	cloned.URL.Host = t.host
	cloned.Host = t.host
	return http.DefaultTransport.RoundTrip(cloned)
}

func newResolverClient(t *testing.T, api *fakeMusicAPI) *ya.Client {
	t.Helper()
	server := httptest.NewServer(api)
	t.Cleanup(server.Close)
	httpClient := utils.NewHttpClient()
	httpClient.SetTransport(rewriteToServer{host: server.Listener.Addr().String()})
	return ya.NewClient(httpClient)
}

func withoutAlbumRetryBackoff(t *testing.T) {
	t.Helper()
	prev := albumFetchRetryBackoff
	albumFetchRetryBackoff = func(int) time.Duration { return 0 }
	t.Cleanup(func() { albumFetchRetryBackoff = prev })
}

func TestResolveArtistRetriesJSONRateLimitAndServerErrors(t *testing.T) {
	withoutAlbumRetryBackoff(t)
	api := &fakeMusicAPI{
		albumFailures: map[string][]int{
			"1": {http.StatusTooManyRequests},
			"2": {http.StatusServiceUnavailable},
		},
		albumCalls: map[string]int{},
	}

	result, err := Resolve(newResolverClient(t, api), "https://music.yandex.ru/artist/170660")

	require.NoError(t, err)
	require.Len(t, result.Tracks, 2)
	assert.ElementsMatch(t, []string{"10", "20"}, []string{result.Tracks[0].ID.String(), result.Tracks[1].ID.String()})
	assert.Equal(t, 2, api.albumCalls["1"])
	assert.Equal(t, 2, api.albumCalls["2"])
}

func TestResolveArtistReportsAlbumThatExhaustsRetries(t *testing.T) {
	withoutAlbumRetryBackoff(t)
	unavailable := http.StatusServiceUnavailable
	api := &fakeMusicAPI{
		albumFailures: map[string][]int{
			"2": {unavailable, unavailable, unavailable, unavailable},
		},
		albumCalls: map[string]int{},
	}

	result, err := Resolve(newResolverClient(t, api), "https://music.yandex.ru/artist/170660")

	require.NoError(t, err)
	require.Len(t, result.Tracks, 1)
	assert.Equal(t, "10", result.Tracks[0].ID.String())
	require.Len(t, result.SkippedAlbums, 1)
	skipped := result.SkippedAlbums[0]
	assert.Equal(t, "2", skipped.ID)
	assert.Equal(t, "SOS", skipped.Title)
	assert.EqualError(t, skipped.Err, "Try again later")
	assert.True(t, utils.IsRetryableHTTPError(skipped.Err))
	assert.Equal(t, artistAlbumFetchAttempts, api.albumCalls["2"])
}

func TestResolveArtistEscapesSkippedAlbumIDWithoutTitle(t *testing.T) {
	withoutAlbumRetryBackoff(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/direct-albums"):
			_, _ = fmt.Fprint(w, `{"result":{"albums":[{"id":1,"title":"Arrival"},{"id":"2\u001b[2J"}],"pager":{"total":2,"page":0,"perPage":50}}}`)
		case strings.HasPrefix(r.URL.Path, "/albums/1/"):
			_, _ = fmt.Fprint(w, `{"result":{"id":1,"title":"Arrival","volumes":[[{"id":"10","title":"Track","available":true}]]}}`)
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = fmt.Fprint(w, `{"error":{"name":"not-found","message":"Album not found"}}`)
		}
	}))
	t.Cleanup(server.Close)
	httpClient := utils.NewHttpClient()
	httpClient.SetTransport(rewriteToServer{host: server.Listener.Addr().String()})

	result, err := Resolve(ya.NewClient(httpClient), "https://music.yandex.ru/artist/170660")

	require.NoError(t, err)
	require.Len(t, result.SkippedAlbums, 1)
	assert.Equal(t, "2\x1b[2J", result.SkippedAlbums[0].ID)
	label := result.SkippedAlbums[0].Label()
	assert.NotContains(t, label, "\x1b")
	assert.Equal(t, `#2\x1b[2J`, label)
}
