package ui

import (
	"testing"
	"ya-music/utils"
	"ya-music/ya"
	"ya-music/ya/model"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
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
	assert.True(t, updated.downloadModel.quitAfterCancel)
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

func TestWindowSizeMsgIsStoredAndAppliedToChildModels(t *testing.T) {
	m := StartUi(nil)
	m.initState = UiStateDownloading

	updatedModel, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	updated := updatedModel.(Model)

	assert.Equal(t, 120, updated.windowWidth)
	assert.Equal(t, 40, updated.windowHeight)
	assert.Equal(t, 116, updated.tokenModel.inputField.Width())
	assert.Equal(t, 116, updated.sourceModel.urlInput.Width())
	assert.Equal(t, 115, updated.downloadModel.trackList.Width())
	assert.Equal(t, 115, updated.downloadModel.progress.Width())
	assert.Equal(t, 38, updated.downloadModel.windowHeight)
}

func TestTokenOkAppliesStoredWindowSizeToSourceModel(t *testing.T) {
	m := StartUi(nil)
	m.windowWidth = 100
	m.windowHeight = 30

	updatedModel, _ := m.Update(TokenOkMsg{})
	updated := updatedModel.(Model)

	assert.Equal(t, UiStateSelectSource, updated.initState)
	assert.Equal(t, 96, updated.sourceModel.urlInput.Width())
}

func TestSourceSubmitAppliesStoredWindowSizeToDownloadModel(t *testing.T) {
	m := StartUi(nil)
	m.initState = UiStateSelectSource
	m.windowWidth = 100
	m.windowHeight = 30

	updatedModel, _ := m.Update(SourceSubmitMsg{
		Tracks: []model.Track{{
			ID:        model.FlexibleID("1"),
			Title:     "Track",
			Available: true,
		}},
	})
	updated := updatedModel.(Model)

	assert.Equal(t, UiStateDownloading, updated.initState)
	assert.Equal(t, 95, updated.downloadModel.trackList.Width())
	assert.Equal(t, 15, updated.downloadModel.trackList.Height())
	assert.Equal(t, 95, updated.downloadModel.progress.Width())
}

func TestDownloadViewFitsInsideStoredWindowHeight(t *testing.T) {
	m := StartUi(nil)

	updatedModel, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	updated := updatedModel.(Model)
	updated.initState = UiStateSelectSource

	tracks := make([]model.Track, 40)
	for i := range tracks {
		tracks[i] = model.Track{
			ID:        model.FlexibleID("1"),
			Title:     "Track",
			Available: true,
		}
	}

	updatedModel, _ = updated.Update(SourceSubmitMsg{
		Tracks: tracks,
	})
	updated = updatedModel.(Model)

	assert.LessOrEqual(t, lipgloss.Height(updated.View().Content), 30)
}

func TestBackToURLAppliesStoredWindowSizeToSourceModel(t *testing.T) {
	m := StartUi(nil)
	m.initState = UiStateDownloading
	m.windowWidth = 100
	m.windowHeight = 30

	updatedModel, _ := m.Update(BackToURLMsg{})
	updated := updatedModel.(Model)

	assert.Equal(t, UiStateSelectSource, updated.initState)
	assert.Equal(t, 96, updated.sourceModel.urlInput.Width())
}

func TestSourceSubmitAddsTracks(t *testing.T) {
	client := ya.NewClient(utils.NewHttpClient())
	m := StartUi(client)

	updatedModel, _ := m.Update(SourceSubmitMsg{
		Tracks: []model.Track{
			{ID: model.FlexibleID("2"), Title: "B", Available: true},
			{ID: model.FlexibleID("1"), Title: "A", Available: true},
			{ID: model.FlexibleID("3"), Title: "C", Available: true},
		},
	})
	updated := updatedModel.(Model)

	assert.Equal(t, UiStateDownloading, updated.initState)
	assert.Equal(t, 3, updated.downloadModel.tracksTotalCount)
	assert.Equal(t, 3, updated.downloadModel.downloadableCount)
}
