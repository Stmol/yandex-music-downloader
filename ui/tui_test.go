package ui

import (
	"testing"
	"ya-music/utils"
	"ya-music/ya"

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
	model := StartUi(client, ya.DownloadOptions{SkipCover: true, AudioFormat: ya.AudioFormatFLAC})

	assert.True(t, model.downloadModel.downloadOptions.SkipCover)
	assert.Equal(t, ya.AudioFormatFLAC, model.downloadModel.downloadOptions.FormatOrDefault())
}
