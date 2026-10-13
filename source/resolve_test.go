package source

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"ya-music/utils"
	"ya-music/ya/model"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeSourceClient struct {
	track             *model.Track
	trackErr          error
	album             *model.Album
	albumErr          error
	albumsByID        map[string]*model.Album
	directAlbums      []model.Album
	directAlbumsErr   error
	legacyPlaylist    *model.Playlist
	legacyPlaylistErr error
	uuidPlaylist      *model.Playlist
	uuidPlaylistErr   error
	chart             *model.Playlist
	chartErr          error
	albumCallErrs     map[string][]error
	mu                sync.Mutex
}

func (f *fakeSourceClient) TrackInfo(string) (*model.Track, error) {
	return f.track, f.trackErr
}

func (f *fakeSourceClient) AlbumWithTracks(id string) (*model.Album, error) {
	f.mu.Lock()
	if errs := f.albumCallErrs[id]; len(errs) > 0 {
		err := errs[0]
		f.albumCallErrs[id] = errs[1:]
		f.mu.Unlock()
		if err != nil {
			return nil, err
		}
	} else {
		f.mu.Unlock()
	}
	if f.albumsByID != nil {
		if album, ok := f.albumsByID[id]; ok {
			return album, nil
		}
		return nil, fmt.Errorf("album not found")
	}
	return f.album, f.albumErr
}

func (f *fakeSourceClient) ArtistDirectAlbums(string) ([]model.Album, error) {
	return f.directAlbums, f.directAlbumsErr
}

func (f *fakeSourceClient) UsersPlaylist(string, string) (*model.Playlist, error) {
	return f.legacyPlaylist, f.legacyPlaylistErr
}

func (f *fakeSourceClient) PlaylistByUUID(string) (*model.Playlist, error) {
	return f.uuidPlaylist, f.uuidPlaylistErr
}

func (f *fakeSourceClient) Chart(string) (*model.Playlist, error) {
	return f.chart, f.chartErr
}

func TestResolveAlbumFlattensVolumes(t *testing.T) {
	client := &fakeSourceClient{
		album: &model.Album{
			Volumes: [][]model.Track{
				{{ID: model.FlexibleID("1"), Title: "A"}},
				{{ID: model.FlexibleID("2"), Title: "B"}},
			},
		},
	}

	result, err := Resolve(client, "https://music.yandex.ru/album/123")
	require.NoError(t, err)
	require.Len(t, result.Tracks, 2)
	assert.Equal(t, "1", result.Tracks[0].ID.String())
	assert.Equal(t, "2", result.Tracks[1].ID.String())
}

func TestResolveLegacyPlaylistExtractsTracks(t *testing.T) {
	client := &fakeSourceClient{
		legacyPlaylist: &model.Playlist{
			Tracks: []model.TrackShort{
				{Track: model.Track{ID: model.FlexibleID("10"), Title: "One"}},
			},
		},
	}

	result, err := Resolve(client, "https://music.yandex.ru/users/user/playlists/99")
	require.NoError(t, err)
	require.Len(t, result.Tracks, 1)
	assert.Equal(t, "10", result.Tracks[0].ID.String())
}

func TestResolveUUIDPlaylistExtractsTracks(t *testing.T) {
	playlistUUID := uuid.NewString()
	client := &fakeSourceClient{
		uuidPlaylist: &model.Playlist{
			Tracks: []model.TrackShort{
				{Track: model.Track{ID: model.FlexibleID("20"), Title: "Two"}},
			},
		},
	}

	result, err := Resolve(client, "https://music.yandex.ru/playlists/"+playlistUUID)
	require.NoError(t, err)
	require.Len(t, result.Tracks, 1)
	assert.Equal(t, "20", result.Tracks[0].ID.String())
}

func TestResolveChartExtractsTracks(t *testing.T) {
	client := &fakeSourceClient{
		chart: &model.Playlist{
			Tracks: []model.TrackShort{
				{Track: model.Track{ID: model.FlexibleID("30"), Title: "Chart hit"}},
			},
		},
	}

	result, err := Resolve(client, "https://music.yandex.ru/chart/world")
	require.NoError(t, err)
	require.Len(t, result.Tracks, 1)
	assert.Equal(t, "30", result.Tracks[0].ID.String())
}

func TestResolvePlaylistLoadsTracks(t *testing.T) {
	tracks, err := resolvePlaylist(func() (*model.Playlist, error) {
		return &model.Playlist{
			Tracks: []model.TrackShort{
				{Track: model.Track{ID: model.FlexibleID("40"), Title: "Loaded"}},
			},
		}, nil
	})
	require.NoError(t, err)
	require.Len(t, tracks, 1)
	assert.Equal(t, "40", tracks[0].ID.String())
}

func TestResolvePlaylistPropagatesLoadError(t *testing.T) {
	wantErr := errors.New("load failed")
	tracks, err := resolvePlaylist(func() (*model.Playlist, error) {
		return nil, wantErr
	})
	assert.Empty(t, tracks)
	assert.ErrorIs(t, err, wantErr)
}

func TestResolveRejectsInvalidURL(t *testing.T) {
	_, err := Resolve(&fakeSourceClient{}, "not-a-url")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid URL")
}

func TestResolveReturnsEmptyAlbumWithoutError(t *testing.T) {
	client := &fakeSourceClient{album: &model.Album{}}

	result, err := Resolve(client, "https://music.yandex.ru/album/123")
	require.NoError(t, err)
	assert.Empty(t, result.Tracks)
}

func TestResolveReturnsEmptyPlaylistWithoutError(t *testing.T) {
	client := &fakeSourceClient{legacyPlaylist: &model.Playlist{}}

	result, err := Resolve(client, "https://music.yandex.ru/users/user/playlists/99")
	require.NoError(t, err)
	assert.Empty(t, result.Tracks)
}

func TestResolvePropagatesClientErrors(t *testing.T) {
	client := &fakeSourceClient{albumErr: errors.New("access denied")}

	_, err := Resolve(client, "https://music.yandex.ru/album/123")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "access denied")
}

func TestResolveArtistFlattensAlbums(t *testing.T) {
	client := &fakeSourceClient{
		directAlbums: []model.Album{
			{ID: model.FlexibleID("1"), Title: "Arrival", Year: 1976},
			{ID: model.FlexibleID("2"), Title: "SOS", Year: 1975},
		},
		albumsByID: map[string]*model.Album{
			"1": {
				ID:    model.FlexibleID("1"),
				Title: "Arrival",
				Year:  1976,
				Volumes: [][]model.Track{
					{{ID: model.FlexibleID("10"), Title: "Dancing Queen"}},
				},
			},
			"2": {
				ID:    model.FlexibleID("2"),
				Title: "SOS",
				Volumes: [][]model.Track{
					{{ID: model.FlexibleID("20"), Title: "SOS", Albums: []model.Album{{ID: model.FlexibleID("2"), Title: "SOS"}}}},
				},
			},
		},
	}

	result, err := Resolve(client, "https://music.yandex.ru/artist/170660")
	require.NoError(t, err)
	require.Len(t, result.Tracks, 2)
	assert.Equal(t, "10", result.Tracks[0].ID.String())
	assert.Equal(t, 1976, result.Tracks[0].Albums[0].Year)
	assert.Equal(t, "Arrival", result.Tracks[0].Albums[0].Title)
	assert.Equal(t, "20", result.Tracks[1].ID.String())
	assert.Equal(t, 1975, result.Tracks[1].Albums[0].Year)
}

func TestResolveArtistReturnsEmptyDiscographyWithoutError(t *testing.T) {
	client := &fakeSourceClient{
		directAlbums: []model.Album{},
	}

	result, err := Resolve(client, "https://music.yandex.ru/artist/170660")
	require.NoError(t, err)
	assert.Empty(t, result.Tracks)
}

func TestResolveArtistPropagatesAlbumLoadError(t *testing.T) {
	client := &fakeSourceClient{
		directAlbums: []model.Album{{ID: model.FlexibleID("1")}},
		albumsByID:   map[string]*model.Album{},
	}

	_, err := Resolve(client, "https://music.yandex.ru/artist/170660")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "album not found")
}

func TestResolveArtistRetriesRetryableAlbumError(t *testing.T) {
	prev := albumFetchRetryBackoff
	albumFetchRetryBackoff = func(int) time.Duration { return 0 }
	t.Cleanup(func() { albumFetchRetryBackoff = prev })

	client := &fakeSourceClient{
		directAlbums: []model.Album{{ID: model.FlexibleID("1"), Title: "Arrival", Year: 1976}},
		albumsByID: map[string]*model.Album{
			"1": {
				ID:      model.FlexibleID("1"),
				Title:   "Arrival",
				Year:    1976,
				Volumes: [][]model.Track{{{ID: model.FlexibleID("10"), Title: "Dancing Queen"}}},
			},
		},
		albumCallErrs: map[string][]error{
			"1": {&utils.HTTPStatusError{StatusCode: 429, Body: "Captcha required"}},
		},
	}

	result, err := Resolve(client, "https://music.yandex.ru/artist/170660")
	require.NoError(t, err)
	require.Len(t, result.Tracks, 1)
	assert.Equal(t, "10", result.Tracks[0].ID.String())
}

func TestResolveArtistSkipsAlbumThatKeepsFailing(t *testing.T) {
	prev := albumFetchRetryBackoff
	albumFetchRetryBackoff = func(int) time.Duration { return 0 }
	t.Cleanup(func() { albumFetchRetryBackoff = prev })

	client := &fakeSourceClient{
		directAlbums: []model.Album{
			{ID: model.FlexibleID("1"), Title: "Arrival", Year: 1976},
			{ID: model.FlexibleID("2"), Title: "SOS", Year: 1975},
		},
		albumsByID: map[string]*model.Album{
			"2": {
				ID:      model.FlexibleID("2"),
				Title:   "SOS",
				Year:    1975,
				Volumes: [][]model.Track{{{ID: model.FlexibleID("20"), Title: "SOS"}}},
			},
		},
		albumCallErrs: map[string][]error{
			"1": {
				&utils.HTTPStatusError{StatusCode: 429, Body: "Captcha required"},
				&utils.HTTPStatusError{StatusCode: 429, Body: "Captcha required"},
				&utils.HTTPStatusError{StatusCode: 429, Body: "Captcha required"},
				&utils.HTTPStatusError{StatusCode: 429, Body: "Captcha required"},
			},
		},
	}

	result, err := Resolve(client, "https://music.yandex.ru/artist/170660")
	require.NoError(t, err)
	require.Len(t, result.Tracks, 1)
	assert.Equal(t, "20", result.Tracks[0].ID.String())
	require.Len(t, result.SkippedAlbums, 1)
	assert.Equal(t, "1", result.SkippedAlbums[0].ID)
	assert.Equal(t, "Arrival", result.SkippedAlbums[0].Title)
	assert.ErrorContains(t, result.SkippedAlbums[0].Err, "HTTP 429")
}

func TestSkippedAlbumLabelQuotesTitleOrFallsBackToID(t *testing.T) {
	assert.Equal(t, `"Arrival"`, SkippedAlbum{ID: "1", Title: "Arrival"}.Label())
	assert.Equal(t, `"Bad\x1b[2J"`, SkippedAlbum{ID: "2", Title: "Bad\x1b[2J"}.Label())
	assert.Equal(t, "#3", SkippedAlbum{ID: "3", Title: "  "}.Label())
}
