package ui

import (
	"bytes"
	"testing"
	"ya-music/utils"
	"ya-music/ya"
	"ya-music/ya/model"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestReset(t *testing.T) {
	m := NewDownloadModel(nil)
	m.downloadOptions = ya.DownloadOptions{SkipCover: true}
	m.AddTracks([]model.Track{
		{Title: "A", Available: true},
		{Title: "B", Available: true},
	})
	m.focusedView = viewQuitButton
	m.hideDuplicates = true
	m.selectedTrackInfo = "x"
	m.isDownloading = true

	m.trackList, _ = m.trackList.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	m.trackList, _ = m.trackList.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	assert.NotEmpty(t, m.trackList.FilterValue())

	m.Reset()

	assert.Equal(t, 0, len(m.tracksProgress))
	assert.False(t, m.isDownloading)
	assert.Equal(t, viewList, m.focusedView)
	assert.False(t, m.hideDuplicates)
	assert.Equal(t, "", m.selectedTrackInfo)
	assert.Equal(t, 0, len(m.trackList.Items()))
	assert.Equal(t, "", m.trackList.FilterValue())
	assert.True(t, m.downloadOptions.SkipCover)
}

func TestNewDownloadModelStoresDownloadOptions(t *testing.T) {
	m := NewDownloadModel(nil, ya.DownloadOptions{SkipCover: true})

	assert.True(t, m.downloadOptions.SkipCover)
}

func TestAddTracks(t *testing.T) {
	m := NewDownloadModel(nil)
	tracks := []model.Track{
		{
			ID:        model.FlexibleID(uuid.New().String()),
			Title:     "Track 1",
			Available: true,
		},
		{
			ID:        model.FlexibleID(uuid.New().String()),
			Title:     "Track 2",
			Available: false,
		},
	}

	m.AddTracks(tracks)

	assert.Equal(t, 2, len(m.tracksProgress))
	assert.Equal(t, TrackStatusReady, m.tracksProgress[0].status)
	assert.Equal(t, TrackStatusNotAvailable, m.tracksProgress[1].status)
	assert.Equal(t, 2, m.tracksTotalCount)
	assert.Equal(t, 1, m.downloadableCount)
}

func TestCycleFocusMovesBetweenListAndActionGroupOnly(t *testing.T) {
	m := NewDownloadModel(nil)

	assert.Equal(t, viewList, m.focusedView)

	m.cycleFocus()
	assert.Equal(t, viewFormatMP3, m.focusedView)

	m.cycleFocus()
	assert.Equal(t, viewList, m.focusedView)

	m.focusedView = viewDownloadButton
	m.lastActionFocus = viewDownloadButton
	m.cycleFocus()
	assert.Equal(t, viewList, m.focusedView)

	m.cycleFocus()
	assert.Equal(t, viewDownloadButton, m.focusedView)
}

func TestCycleFocusSkipsBackWhenDownloading(t *testing.T) {
	m := NewDownloadModel(nil)
	m.isDownloading = true

	m.cycleFocus()
	assert.Equal(t, viewQuitButton, m.focusedView)

	m.cycleFocus()
	assert.Equal(t, viewList, m.focusedView)
}

func TestCycleFocusMovesOffDisabledControlWhenDownloadingStarts(t *testing.T) {
	m := NewDownloadModel(nil)
	m.focusedView = viewBackButton
	m.isDownloading = true

	m.cycleFocus()
	assert.Equal(t, viewList, m.focusedView)
}

func TestToggleAudioFormat(t *testing.T) {
	m := NewDownloadModel(nil)
	m.focusedView = viewFormatFLAC

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	assert.Equal(t, ya.AudioFormatFLAC, updated.downloadOptions.FormatOrDefault())

	updated.focusedView = viewFormatMP3
	updated, _ = updated.Update(tea.KeyMsg{Type: tea.KeySpace})
	assert.Equal(t, ya.AudioFormatMP3, updated.downloadOptions.FormatOrDefault())
}

func TestToggleAudioFormatIsDisabledWhileDownloading(t *testing.T) {
	m := NewDownloadModel(nil, ya.DownloadOptions{AudioFormat: ya.AudioFormatMP3})
	m.focusedView = viewFormatFLAC
	m.isDownloading = true

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	assert.Equal(t, ya.AudioFormatMP3, updated.downloadOptions.FormatOrDefault())
}

func TestRenderFormatToggleShowsSelectedFormat(t *testing.T) {
	m := NewDownloadModel(nil, ya.DownloadOptions{AudioFormat: ya.AudioFormatFLAC})

	assert.Contains(t, renderActionBar(m), "Format")
	assert.Contains(t, renderActionBar(m), "MP3")
	assert.Contains(t, renderActionBar(m), "FLAC")
}

func TestArrowKeysMoveAcrossActionControls(t *testing.T) {
	m := NewDownloadModel(nil)
	m.focusedView = viewFormatMP3

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	assert.Equal(t, viewFormatFLAC, updated.focusedView)

	updated, _ = updated.Update(tea.KeyMsg{Type: tea.KeyRight})
	assert.Equal(t, viewBackButton, updated.focusedView)

	updated, _ = updated.Update(tea.KeyMsg{Type: tea.KeyLeft})
	assert.Equal(t, viewFormatFLAC, updated.focusedView)
}

func TestActionBarActivationUsesEnterAndSpace(t *testing.T) {
	m := NewDownloadModel(nil)
	m.focusedView = viewFormatFLAC

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	assert.Equal(t, ya.AudioFormatFLAC, updated.downloadOptions.FormatOrDefault())

	updated.focusedView = viewFormatMP3
	updated, _ = updated.Update(tea.KeyMsg{Type: tea.KeyEnter})
	assert.Equal(t, ya.AudioFormatMP3, updated.downloadOptions.FormatOrDefault())
}

func TestWindowResizeShrinksTrackListToAvailableHeight(t *testing.T) {
	m := NewDownloadModel(nil)

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	assert.Equal(t, 92, updated.trackList.Width())
	assert.Equal(t, 17, updated.trackList.Height())
	assert.Equal(t, 92, updated.progress.Width)
	assert.Equal(t, 92, updated.help.Width)
}

func TestWindowResizeKeepsMinimumTrackListHeight(t *testing.T) {
	m := NewDownloadModel(nil)

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 30, Height: 12})

	assert.Equal(t, 40, updated.trackList.Width())
	assert.Equal(t, minTrackListHeight, updated.trackList.Height())
}

func TestResetState(t *testing.T) {
	m := NewDownloadModel(nil)
	m.tracksProgress = []*TrackProgress{
		{status: TrackStatusDownloaded},
		{status: TrackStatusError},
		{status: TrackStatusDuplicate},
		{status: TrackStatusNotAvailable},
	}

	m.resetState()

	assert.Equal(t, TrackStatusReady, m.tracksProgress[0].status)
	assert.Equal(t, TrackStatusReady, m.tracksProgress[1].status)
	assert.Equal(t, TrackStatusDuplicate, m.tracksProgress[2].status)
	assert.Equal(t, TrackStatusNotAvailable, m.tracksProgress[3].status)
	assert.Equal(t, 4, m.tracksTotalCount)
	assert.Equal(t, 2, m.downloadableCount)
}

func TestUpdateTrackList(t *testing.T) {
	m := NewDownloadModel(nil)
	m.tracksProgress = []*TrackProgress{
		{status: TrackStatusReady},
		{status: TrackStatusDuplicate},
	}

	m.updateTrackList()
	assert.Equal(t, 2, len(m.trackList.Items()))

	m.hideDuplicates = true
	m.updateTrackList()
	assert.Equal(t, 1, len(m.trackList.Items()))
}

func TestGetTrackInfo(t *testing.T) {
	m := NewDownloadModel(nil)
	track := &model.Track{
		Title: "Test Track",
	}

	uid := uuid.New().String()
	m.tracksProgress = []*TrackProgress{
		{
			uid:      uid,
			track:    track,
			filename: "test.mp3",
		},
	}

	info := m.getTrackInfo(uid)
	assert.Equal(t, "Downloaded: test.mp3", info)

	// Test error message
	m.tracksProgress[0].errMsg = "Download failed"
	info = m.getTrackInfo(uid)
	assert.Equal(t, "Download failed", info)
}

func TestDownloadFormatFromFilename(t *testing.T) {
	assert.Equal(t, "FLAC", downloadFormatFromFilename("Artist - Song.flac"))
	assert.Equal(t, "FLAC", downloadFormatFromFilename("Artist - Song.FLAC"))
	assert.Equal(t, "MP3", downloadFormatFromFilename("Artist - Song.mp3"))
	assert.Equal(t, "MP3", downloadFormatFromFilename(""))
}

func TestCountStatus(t *testing.T) {
	tracks := []*TrackProgress{
		{status: TrackStatusReady},
		{status: TrackStatusReady},
		{status: TrackStatusError},
		{status: TrackStatusDownloaded},
	}

	assert.Equal(t, 2, countStatus(tracks, TrackStatusReady))
	assert.Equal(t, 1, countStatus(tracks, TrackStatusError))
	assert.Equal(t, 1, countStatus(tracks, TrackStatusDownloaded))
	assert.Equal(t, 0, countStatus(tracks, TrackStatusDuplicate))
}

func TestRenderHeader(t *testing.T) {
	header := renderHeader(5, 10, 8, 2)
	assert.Contains(t, header, "Total tracks:")
	assert.Contains(t, header, "10")
	assert.Contains(t, header, "To download:")
	assert.Contains(t, header, "8")
	assert.Contains(t, header, "Completed:")
	assert.Contains(t, header, "5")
	assert.Contains(t, header, "Errors:")
	assert.Contains(t, header, "2")
	assert.NotContains(t, header, "\nTo download")
}

func TestSortTracksByTitle(t *testing.T) {
	tracks := []*TrackProgress{
		{track: &model.Track{Title: "C"}},
		{track: &model.Track{Title: "A"}},
		{track: &model.Track{Title: "B"}},
	}

	sortTracksByTitle(tracks)

	assert.Equal(t, "A", tracks[0].track.Title)
	assert.Equal(t, "B", tracks[1].track.Title)
	assert.Equal(t, "C", tracks[2].track.Title)
}

func TestFindDuplicates(t *testing.T) {
	id1 := uuid.New()
	tracks := []*TrackProgress{
		{track: &model.Track{ID: model.FlexibleID(id1.String()), Title: "Same"}},
		{track: &model.Track{ID: model.FlexibleID(id1.String()), Title: "Same"}},
		{track: &model.Track{ID: model.FlexibleID(uuid.New().String()), Title: "Same"}},
		{track: &model.Track{ID: model.FlexibleID(uuid.New().String()), Title: "Unique"}},
	}

	findDuplicates(tracks)

	assert.Equal(t, TrackStatusReady, tracks[0].status)
	assert.Equal(t, TrackStatusDuplicate, tracks[1].status)
	assert.Equal(t, TrackStatusDuplicate, tracks[2].status)
	assert.Equal(t, TrackStatusReady, tracks[3].status)
}

func TestDownloadTracksLogsSkippedReasons(t *testing.T) {
	var logs bytes.Buffer
	logger := utils.NewDownloadLoggerForWriter(&logs)
	client := ya.NewClient(utils.NewHttpClientWithLogger(logger))
	m := NewDownloadModel(client)
	updCh := make(chan TrackProgress)

	progressList := []*TrackProgress{
		{
			track:  &model.Track{ID: model.FlexibleID("1"), Title: "Duplicate"},
			status: TrackStatusDuplicate,
		},
		{
			track:  &model.Track{ID: model.FlexibleID("2"), Title: "Unavailable"},
			status: TrackStatusNotAvailable,
		},
	}

	msg := m.downloadTracks(updCh, progressList)()
	assert.IsType(t, DownloadStartMsg{}, msg)

	assert.Contains(t, logs.String(), "download session started")
	assert.Contains(t, logs.String(), "reason=duplicate")
	assert.Contains(t, logs.String(), "reason=not_available")
	assert.Contains(t, logs.String(), "track_title=Duplicate")
	assert.Contains(t, logs.String(), "track_title=Unavailable")
}

func TestQuitButtonCancelsActiveDownloads(t *testing.T) {
	client := ya.NewClient(utils.NewHttpClient())
	m := NewDownloadModel(client)
	m.isDownloading = true
	m.focusedView = viewQuitButton

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	assert.True(t, updated.shutdownRequested)
	assert.Nil(t, cmd)
}

func TestDownloadEndQuitsAfterShutdownRequest(t *testing.T) {
	m := NewDownloadModel(nil)
	m.isDownloading = true
	m.shutdownRequested = true

	updated, cmd := m.Update(DownloadEndMsg{})

	assert.False(t, updated.isDownloading)
	if assert.NotNil(t, cmd) {
		assert.IsType(t, tea.QuitMsg{}, cmd())
	}
}
