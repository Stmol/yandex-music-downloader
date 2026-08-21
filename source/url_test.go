package source

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestParseTrackURL(t *testing.T) {
	ref, err := Parse("https://music.yandex.com/album/1231231/track/12312345?utm_source=web")
	requireNoError(t, err)
	assert.Equal(t, &Ref{
		Kind:    KindTrack,
		TrackID: "12312345",
	}, ref)
}

func TestParseAlbumURL(t *testing.T) {
	ref, err := Parse("https://music.yandex.ru/album/5942930?utm_source=web&utm_medium=copy_link")
	requireNoError(t, err)
	assert.Equal(t, &Ref{Kind: KindAlbum, AlbumID: "5942930"}, ref)
}

func TestParseAlbumURLWithoutQuery(t *testing.T) {
	ref, err := Parse("https://music.yandex.com/album/5942930")
	requireNoError(t, err)
	assert.Equal(t, &Ref{Kind: KindAlbum, AlbumID: "5942930"}, ref)
}

func TestParseLegacyPlaylistURL(t *testing.T) {
	ref, err := Parse("https://music.yandex.ru/users/username/playlists/12312311?utm_source=web")
	requireNoError(t, err)
	assert.Equal(t, &Ref{
		Kind:       KindLegacyPlaylist,
		PlaylistID: "12312311",
		Username:   "username",
	}, ref)
}

func TestParsePlaylistUUIDURL(t *testing.T) {
	playlistUUID := uuid.NewString()
	ref, err := Parse("https://music.yandex.ru/playlists/" + playlistUUID + "?utm_source=web&utm_medium=copy_link")
	requireNoError(t, err)
	assert.Equal(t, &Ref{Kind: KindPlaylistUUID, PlaylistUUID: playlistUUID}, ref)
}

func TestParsePlaylistUUIDWithLikesPrefix(t *testing.T) {
	playlistUUID := uuid.NewString()
	ref, err := Parse("https://music.yandex.ru/playlists/lk." + playlistUUID + "?utm_source=web&utm_medium=copy_link")
	requireNoError(t, err)
	assert.Equal(t, &Ref{Kind: KindPlaylistUUID, PlaylistUUID: "lk." + playlistUUID}, ref)
}

func TestParsePlaylistUUIDWithGenericPrefix(t *testing.T) {
	playlistUUID := uuid.NewString()
	ref, err := Parse("https://music.yandex.ru/playlists/ps." + playlistUUID + "?utm_source=web&utm_medium=copy_link")
	requireNoError(t, err)
	assert.Equal(t, &Ref{Kind: KindPlaylistUUID, PlaylistUUID: "ps." + playlistUUID}, ref)
}

func TestParseChartURL(t *testing.T) {
	ref, err := Parse("https://music.yandex.ru/chart?utm_source=web")
	requireNoError(t, err)
	assert.Equal(t, &Ref{Kind: KindChart}, ref)
}

func TestParseChartURLWithRegion(t *testing.T) {
	ref, err := Parse("https://music.yandex.com/chart/world")
	requireNoError(t, err)
	assert.Equal(t, &Ref{Kind: KindChart, Region: "world"}, ref)
}

func TestParseInvalidURL(t *testing.T) {
	_, err := Parse("https://music.yandex.ru/playlists/not-a-uuid")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid URL")
}

func TestParseRejectsMalformedPlaylistUUIDWithGenericPrefix(t *testing.T) {
	_, err := Parse("https://music.yandex.ru/playlists/p." + uuid.NewString())
	assert.Error(t, err)
}

func TestParseRejectsMalformedPlaylistUUIDWithLikesPrefix(t *testing.T) {
	_, err := Parse("https://music.yandex.ru/playlists/lk.------------------------------------")
	assert.Error(t, err)
}

func TestParseRejectsMalformedPlaylistUUID(t *testing.T) {
	_, err := Parse("https://music.yandex.ru/playlists/------------------------------------")
	assert.Error(t, err)
}

func requireNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
