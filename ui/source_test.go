package ui

import (
	"testing"

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

	msg := m.parseURL("https://music.yandex.ru/playlists/4dc94b2f-e96b-2daf-a53c-ce71846901b3?utm_source=web&utm_medium=copy_link")

	assert.Equal(t, URLSubmitMsg{
		kind:         sourceURLPlaylistUUID,
		PlaylistUUID: "4dc94b2f-e96b-2daf-a53c-ce71846901b3",
	}, msg)
}

func TestSourceParseURLInvalid(t *testing.T) {
	m := SourceModel{}

	msg := m.parseURL("https://music.yandex.ru/playlists/not-a-uuid")

	assert.Nil(t, msg)
}

func TestSourceParseURLRejectsMalformedPlaylistUUID(t *testing.T) {
	m := SourceModel{}

	msg := m.parseURL("https://music.yandex.ru/playlists/------------------------------------")

	assert.Nil(t, msg)
}
