package ui

import (
	"testing"
	"ya-music/utils"
	"ya-music/ya"
	"ya-music/ya/model"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestShutdownRequestedMsgCancelsDownloadsBeforeQuit(t *testing.T) {
	client := ya.NewClient(utils.NewHttpClient())
	model := StartUi(client)
	model.initState = UiStateDownloading
	model.downloadModel.isDownloading = true

	updatedModel, cmd := model.Update(ShutdownRequestedMsg{Reason: "signal_sigterm"})
	updated := updatedModel.(Model)

	assert.True(t, updated.downloadModel.shutdownRequested)
	assert.Nil(t, cmd)
}

func TestShutdownRequestedMsgQuitsImmediatelyWhenIdle(t *testing.T) {
	client := ya.NewClient(utils.NewHttpClient())
	model := StartUi(client)

	_, cmd := model.Update(ShutdownRequestedMsg{Reason: "signal_sigterm"})

	if assert.NotNil(t, cmd) {
		assert.IsType(t, tea.QuitMsg{}, cmd())
	}
}

func TestStartUiPassesDownloadOptions(t *testing.T) {
	client := ya.NewClient(utils.NewHttpClient())
	model := StartUi(client, ya.DownloadOptions{SkipCover: true})

	assert.True(t, model.downloadModel.downloadOptions.SkipCover)
}

func TestSourceSubmitAlbumAddsVolumeTracks(t *testing.T) {
	client := ya.NewClient(utils.NewHttpClient())
	m := StartUi(client)

	updatedModel, _ := m.Update(SourceSubmitMsg{
		Album: &model.Album{
			Volumes: [][]model.Track{
				{
					{ID: model.FlexibleID("2"), Title: "B", Available: true},
					{ID: model.FlexibleID("1"), Title: "A", Available: true},
				},
				{
					{ID: model.FlexibleID("3"), Title: "C", Available: true},
				},
			},
		},
	})
	updated := updatedModel.(Model)

	assert.Equal(t, UiStateDownloading, updated.initState)
	assert.Equal(t, 3, updated.downloadModel.tracksTotalCount)
	assert.Equal(t, 3, updated.downloadModel.downloadableCount)
}
