package ya

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"ya-music/utils"
	"ya-music/ya/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildCoverURLUsesTrackCover(t *testing.T) {
	track := model.Track{
		CoverURI: "avatars.yandex.net/get-music-content/123/%%",
	}

	assert.Equal(t, "https://avatars.yandex.net/get-music-content/123/400x400", buildCoverURL(track))
}

func TestBuildCoverURLFallsBackToAlbumCover(t *testing.T) {
	track := model.Track{
		Albums: []model.Album{
			{CoverURI: "avatars.yandex.net/get-music-content/album/%%"},
		},
	}

	assert.Equal(t, "https://avatars.yandex.net/get-music-content/album/400x400", buildCoverURL(track))
}

func TestBuildCoverURLPreservesAbsoluteURL(t *testing.T) {
	track := model.Track{
		CoverURI: "https://example.test/cover/%%",
	}

	assert.Equal(t, "https://example.test/cover/400x400", buildCoverURL(track))
}

func TestBuildCoverURLReturnsEmptyWithoutCover(t *testing.T) {
	assert.Empty(t, buildCoverURL(model.Track{}))
}

func TestDownloadCoverFailureIsReturnedForCallerToIgnore(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := NewClient(utils.NewHttpClient())
	track := model.Track{
		ID:       model.FlexibleID("1"),
		Title:    "No Cover",
		CoverURI: server.URL + "/%%",
	}

	ch := client.startCoverDownload(track, filepath.Join(t.TempDir(), "track.mp3"), DownloadOptions{})
	result := <-ch

	assert.Empty(t, result.filename)
	assert.Error(t, result.err)
}

func TestDownloadCoverReplacesStaleCoverFile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("fresh cover"))
	}))
	defer server.Close()

	client := NewClient(utils.NewHttpClient())
	audioPath := filepath.Join(t.TempDir(), "track.mp3")
	coverPath := buildCoverFilename(audioPath)
	require.NoError(t, os.WriteFile(coverPath, []byte("stale cover"), 0644))

	track := model.Track{
		ID:       model.FlexibleID("1"),
		Title:    "Fresh Cover",
		CoverURI: server.URL + "/%%",
	}

	ch := client.startCoverDownload(track, audioPath, DownloadOptions{})
	result := <-ch

	require.NoError(t, result.err)
	assert.Equal(t, coverPath, result.filename)
	data, err := os.ReadFile(coverPath)
	require.NoError(t, err)
	assert.Equal(t, "fresh cover", string(data))
}

func TestDownloadCoverSkipReturnsNoError(t *testing.T) {
	client := NewClient(utils.NewHttpClient())
	track := model.Track{
		ID:       model.FlexibleID("1"),
		Title:    "Skip Cover",
		CoverURI: "https://example.test/%%",
	}

	ch := client.startCoverDownload(track, filepath.Join(t.TempDir(), "track.mp3"), DownloadOptions{SkipCover: true})
	result := <-ch

	assert.Empty(t, result.filename)
	require.NoError(t, result.err)
}
