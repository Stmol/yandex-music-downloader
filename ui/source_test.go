package ui

import (
	"errors"
	"testing"

	"ya-music/ya/model"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSourceParseURLTrack(t *testing.T) {
	m := SourceModel{}

	msg := m.parseURL("https://music.yandex.com/album/1231231/track/12312345?utm_source=web")

	assert.Equal(t, URLSubmitMsg{
		kind:    sourceURLTrack,
		TrackID: "12312345",
	}, msg)
}

func TestSourceParseURLAlbum(t *testing.T) {
	m := SourceModel{}

	msg := m.parseURL("https://music.yandex.ru/album/5942930?utm_source=web&utm_medium=copy_link")

	assert.Equal(t, URLSubmitMsg{
		kind:    sourceURLAlbum,
		AlbumID: "5942930",
	}, msg)
}

func TestSourceParseURLAlbumWithoutQuery(t *testing.T) {
	m := SourceModel{}

	msg := m.parseURL("https://music.yandex.com/album/5942930")

	assert.Equal(t, URLSubmitMsg{
		kind:    sourceURLAlbum,
		AlbumID: "5942930",
	}, msg)
}

func TestSourceParseURLLegacyPlaylist(t *testing.T) {
	m := SourceModel{}

	msg := m.parseURL("https://music.yandex.ru/users/username/playlists/12312311?utm_source=web")

	assert.Equal(t, URLSubmitMsg{
		kind:       sourceURLLegacyPlaylist,
		PlaylistID: "12312311",
		Username:   "username",
	}, msg)
}

func TestSourceParseURLPlaylistUUID(t *testing.T) {
	m := SourceModel{}

	playlistUUID := uuid.NewString()
	msg := m.parseURL("https://music.yandex.ru/playlists/" + playlistUUID + "?utm_source=web&utm_medium=copy_link")

	assert.Equal(t, URLSubmitMsg{
		kind:         sourceURLPlaylistUUID,
		PlaylistUUID: playlistUUID,
	}, msg)
}

func TestSourceParseURLPlaylistUUIDWithLikesPrefix(t *testing.T) {
	m := SourceModel{}

	playlistUUID := uuid.NewString()
	msg := m.parseURL("https://music.yandex.ru/playlists/lk." + playlistUUID + "?utm_source=web&utm_medium=copy_link")

	assert.Equal(t, URLSubmitMsg{
		kind:         sourceURLPlaylistUUID,
		PlaylistUUID: "lk." + playlistUUID,
	}, msg)
}

func TestSourceParseURLPlaylistUUIDWithGenericPrefix(t *testing.T) {
	m := SourceModel{}

	playlistUUID := uuid.NewString()
	msg := m.parseURL("https://music.yandex.ru/playlists/ps." + playlistUUID + "?utm_source=web&utm_medium=copy_link")

	assert.Equal(t, URLSubmitMsg{
		kind:         sourceURLPlaylistUUID,
		PlaylistUUID: "ps." + playlistUUID,
	}, msg)
}

func TestSourceParseURLChart(t *testing.T) {
	m := SourceModel{}

	msg := m.parseURL("https://music.yandex.ru/chart?utm_source=web")

	assert.Equal(t, URLSubmitMsg{
		kind: sourceURLChart,
	}, msg)
}

func TestSourceParseURLChartWithRegion(t *testing.T) {
	m := SourceModel{}

	msg := m.parseURL("https://music.yandex.com/chart/world")

	assert.Equal(t, URLSubmitMsg{
		kind:   sourceURLChart,
		Region: "world",
	}, msg)
}

func TestSourceParseURLInvalid(t *testing.T) {
	m := SourceModel{}

	msg := m.parseURL("https://music.yandex.ru/playlists/not-a-uuid")

	assert.Nil(t, msg)
}

func TestSourceParseURLRejectsMalformedPlaylistUUIDWithGenericPrefix(t *testing.T) {
	m := SourceModel{}

	msg := m.parseURL("https://music.yandex.ru/playlists/p." + uuid.NewString())

	assert.Nil(t, msg)
}

func TestSourceParseURLRejectsMalformedPlaylistUUIDWithLikesPrefix(t *testing.T) {
	m := SourceModel{}

	msg := m.parseURL("https://music.yandex.ru/playlists/lk.------------------------------------")

	assert.Nil(t, msg)
}

func TestSourceParseURLRejectsMalformedPlaylistUUID(t *testing.T) {
	m := SourceModel{}

	msg := m.parseURL("https://music.yandex.ru/playlists/------------------------------------")

	assert.Nil(t, msg)
}

func TestSourceResizeExpandsURLInputToWindowWidth(t *testing.T) {
	m := NewSourceModel(nil)

	m.Resize(120, 40)

	assert.Equal(t, 116, m.urlInput.Width())
}

func TestSourceResizeKeepsMinimumURLInputWidth(t *testing.T) {
	m := NewSourceModel(nil)

	m.Resize(10, 40)

	assert.Equal(t, minInputWidth, m.urlInput.Width())
}

type fakeSourceClient struct {
	track              *model.Track
	trackErr           error
	album              *model.Album
	albumErr           error
	legacyPlaylist     *model.Playlist
	legacyPlaylistErr  error
	uuidPlaylist       *model.Playlist
	uuidPlaylistErr    error
	chart              *model.Playlist
	chartErr           error
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

func TestResolveSourceTracksAlbumFlattensVolumes(t *testing.T) {
	client := fakeSourceClient{
		album: &model.Album{
			Volumes: [][]model.Track{
				{{ID: model.FlexibleID("1"), Title: "A"}},
				{{ID: model.FlexibleID("2"), Title: "B"}},
			},
		},
	}

	tracks, err := ResolveSourceTracks(client, "https://music.yandex.ru/album/123")
	require.NoError(t, err)
	require.Len(t, tracks, 2)
	assert.Equal(t, "1", tracks[0].ID.String())
	assert.Equal(t, "2", tracks[1].ID.String())
}

func TestResolveSourceTracksLegacyPlaylistExtractsTracks(t *testing.T) {
	client := fakeSourceClient{
		legacyPlaylist: &model.Playlist{
			Tracks: []model.TrackShort{
				{Track: model.Track{ID: model.FlexibleID("10"), Title: "One"}},
			},
		},
	}

	tracks, err := ResolveSourceTracks(client, "https://music.yandex.ru/users/user/playlists/99")
	require.NoError(t, err)
	require.Len(t, tracks, 1)
	assert.Equal(t, "10", tracks[0].ID.String())
}

func TestResolveSourceTracksUUIDPlaylistExtractsTracks(t *testing.T) {
	playlistUUID := uuid.NewString()
	client := fakeSourceClient{
		uuidPlaylist: &model.Playlist{
			Tracks: []model.TrackShort{
				{Track: model.Track{ID: model.FlexibleID("20"), Title: "Two"}},
			},
		},
	}

	tracks, err := ResolveSourceTracks(client, "https://music.yandex.ru/playlists/"+playlistUUID)
	require.NoError(t, err)
	require.Len(t, tracks, 1)
	assert.Equal(t, "20", tracks[0].ID.String())
}

func TestResolveSourceTracksChartExtractsTracks(t *testing.T) {
	client := fakeSourceClient{
		chart: &model.Playlist{
			Tracks: []model.TrackShort{
				{Track: model.Track{ID: model.FlexibleID("30"), Title: "Chart hit"}},
			},
		},
	}

	tracks, err := ResolveSourceTracks(client, "https://music.yandex.ru/chart/world")
	require.NoError(t, err)
	require.Len(t, tracks, 1)
	assert.Equal(t, "30", tracks[0].ID.String())
}

func TestResolveSourceTracksRejectsInvalidURL(t *testing.T) {
	_, err := ResolveSourceTracks(fakeSourceClient{}, "not-a-url")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid URL")
}

func TestResolveSourceTracksReturnsEmptyAlbumWithoutError(t *testing.T) {
	client := fakeSourceClient{album: &model.Album{}}

	tracks, err := ResolveSourceTracks(client, "https://music.yandex.ru/album/123")
	require.NoError(t, err)
	assert.Empty(t, tracks)
}

func TestResolveSourceTracksReturnsEmptyPlaylistWithoutError(t *testing.T) {
	client := fakeSourceClient{legacyPlaylist: &model.Playlist{}}

	tracks, err := ResolveSourceTracks(client, "https://music.yandex.ru/users/user/playlists/99")
	require.NoError(t, err)
	assert.Empty(t, tracks)
}

func TestResolveSourceTracksPropagatesClientErrors(t *testing.T) {
	client := fakeSourceClient{albumErr: errors.New("access denied")}

	_, err := ResolveSourceTracks(client, "https://music.yandex.ru/album/123")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "access denied")
}
