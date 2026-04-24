package ui

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
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
