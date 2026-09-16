package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

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

func TestSourceHandleEnterKeyRejectsInvalidURL(t *testing.T) {
	m := NewSourceModel(nil)
	m.urlInput.SetValue("not-a-url")

	updated, cmd := m.handleEnterKey()

	assert.Equal(t, "Invalid URL", updated.errorMsg)
	assert.False(t, updated.isProcessing)
	assert.Nil(t, cmd)
}

func TestSourceHandleEnterKeyAcceptsValidAlbumURL(t *testing.T) {
	m := NewSourceModel(nil)
	m.urlInput.SetValue("https://music.yandex.ru/album/5942930")

	updated, cmd := m.handleEnterKey()

	assert.Empty(t, updated.errorMsg)
	assert.True(t, updated.isProcessing)
	assert.NotNil(t, cmd)
}

func TestSourceRenderIncludesURLExamples(t *testing.T) {
	m := NewSourceModel(nil)

	rendered := m.render()

	for _, example := range []string{
		"· Track: https://music.yandex.ru/album/1231231/track/12312345",
		"· Album: https://music.yandex.ru/album/1231231",
		"· Playlist: https://music.yandex.ru/playlists/4dc94b2f-e96b-2daf-a53c-ce71846901b3",
		"· Legacy playlist: https://music.yandex.ru/users/username/playlists/12312311",
		"· Chart: https://music.yandex.ru/chart (or /chart/world)",
	} {
		assert.Contains(t, rendered, example)
	}
}
