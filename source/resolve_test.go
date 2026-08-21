package source

import (
	"errors"
	"testing"

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
	legacyPlaylist    *model.Playlist
	legacyPlaylistErr error
	uuidPlaylist      *model.Playlist
	uuidPlaylistErr   error
	chart             *model.Playlist
	chartErr          error
}

func (f fakeSourceClient) TrackInfo(string) (*model.Track, error) {
	return f.track, f.trackErr
}

func (f fakeSourceClient) AlbumWithTracks(string) (*model.Album, error) {
	return f.album, f.albumErr
}

func (f fakeSourceClient) UsersPlaylist(string, string) (*model.Playlist, error) {
	return f.legacyPlaylist, f.legacyPlaylistErr
}

func (f fakeSourceClient) PlaylistByUUID(string) (*model.Playlist, error) {
	return f.uuidPlaylist, f.uuidPlaylistErr
}

func (f fakeSourceClient) Chart(string) (*model.Playlist, error) {
	return f.chart, f.chartErr
}

func TestResolveAlbumFlattensVolumes(t *testing.T) {
	client := fakeSourceClient{
		album: &model.Album{
			Volumes: [][]model.Track{
				{{ID: model.FlexibleID("1"), Title: "A"}},
				{{ID: model.FlexibleID("2"), Title: "B"}},
			},
		},
	}

	tracks, err := Resolve(client, "https://music.yandex.ru/album/123")
	require.NoError(t, err)
	require.Len(t, tracks, 2)
	assert.Equal(t, "1", tracks[0].ID.String())
	assert.Equal(t, "2", tracks[1].ID.String())
}

func TestResolveLegacyPlaylistExtractsTracks(t *testing.T) {
	client := fakeSourceClient{
		legacyPlaylist: &model.Playlist{
			Tracks: []model.TrackShort{
				{Track: model.Track{ID: model.FlexibleID("10"), Title: "One"}},
			},
		},
	}

	tracks, err := Resolve(client, "https://music.yandex.ru/users/user/playlists/99")
	require.NoError(t, err)
	require.Len(t, tracks, 1)
	assert.Equal(t, "10", tracks[0].ID.String())
}

func TestResolveUUIDPlaylistExtractsTracks(t *testing.T) {
	playlistUUID := uuid.NewString()
	client := fakeSourceClient{
		uuidPlaylist: &model.Playlist{
			Tracks: []model.TrackShort{
				{Track: model.Track{ID: model.FlexibleID("20"), Title: "Two"}},
			},
		},
	}

	tracks, err := Resolve(client, "https://music.yandex.ru/playlists/"+playlistUUID)
	require.NoError(t, err)
	require.Len(t, tracks, 1)
	assert.Equal(t, "20", tracks[0].ID.String())
}

func TestResolveChartExtractsTracks(t *testing.T) {
	client := fakeSourceClient{
		chart: &model.Playlist{
			Tracks: []model.TrackShort{
				{Track: model.Track{ID: model.FlexibleID("30"), Title: "Chart hit"}},
			},
		},
	}

	tracks, err := Resolve(client, "https://music.yandex.ru/chart/world")
	require.NoError(t, err)
	require.Len(t, tracks, 1)
	assert.Equal(t, "30", tracks[0].ID.String())
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
	assert.Nil(t, tracks)
	assert.ErrorIs(t, err, wantErr)
}

func TestResolveRejectsInvalidURL(t *testing.T) {
	_, err := Resolve(fakeSourceClient{}, "not-a-url")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid URL")
}

func TestResolveReturnsEmptyAlbumWithoutError(t *testing.T) {
	client := fakeSourceClient{album: &model.Album{}}

	tracks, err := Resolve(client, "https://music.yandex.ru/album/123")
	require.NoError(t, err)
	assert.Empty(t, tracks)
}

func TestResolveReturnsEmptyPlaylistWithoutError(t *testing.T) {
	client := fakeSourceClient{legacyPlaylist: &model.Playlist{}}

	tracks, err := Resolve(client, "https://music.yandex.ru/users/user/playlists/99")
	require.NoError(t, err)
	assert.Empty(t, tracks)
}

func TestResolvePropagatesClientErrors(t *testing.T) {
	client := fakeSourceClient{albumErr: errors.New("access denied")}

	_, err := Resolve(client, "https://music.yandex.ru/album/123")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "access denied")
}
