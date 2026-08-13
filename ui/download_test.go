package ui

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"ya-music/utils"
	"ya-music/ya"
	"ya-music/ya/model"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func keyText(text string) tea.KeyPressMsg {
	return tea.KeyPressMsg(tea.Key{Text: text})
}

func keyCode(code rune) tea.KeyPressMsg {
	return tea.KeyPressMsg(tea.Key{Code: code})
}

func addReadyTracks(m *DownloadModel, count int) {
	tracks := make([]model.Track, count)
	for i := range tracks {
		tracks[i] = model.Track{
			ID:        model.FlexibleID(uuid.NewString()),
			Title:     "Track",
			Available: true,
		}
	}
	m.AddTracks(tracks)
}

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

	m.trackList, _ = m.trackList.Update(keyText("/"))
	m.trackList, _ = m.trackList.Update(keyText("x"))
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

func TestCycleFocusWalksThroughCommandDeck(t *testing.T) {
	m := NewDownloadModel(nil)

	assert.Equal(t, viewList, m.focusedView)

	m.cycleFocus()
	assert.Equal(t, viewFormatMP3, m.focusedView)

	m.cycleFocus()
	assert.Equal(t, viewFormatFLAC, m.focusedView)

	m.cycleFocus()
	assert.Equal(t, viewDownloadButton, m.focusedView)

	m.cycleFocus()
	assert.Equal(t, viewBackButton, m.focusedView)

	m.cycleFocus()
	assert.Equal(t, viewQuitButton, m.focusedView)

	m.cycleFocus()
	assert.Equal(t, viewList, m.focusedView)

	updated, _ := m.Update(keyText("shift+tab"))
	assert.Equal(t, viewQuitButton, updated.focusedView)

	updated, _ = updated.Update(keyText("shift+tab"))
	assert.Equal(t, viewBackButton, updated.focusedView)
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
	assert.Equal(t, viewQuitButton, m.focusedView)
}

func TestToggleAudioFormat(t *testing.T) {
	m := NewDownloadModel(nil)
	m.focusedView = viewFormatFLAC

	updated, _ := m.Update(keyCode(tea.KeyEnter))
	assert.Equal(t, ya.AudioFormatFLAC, updated.downloadOptions.FormatOrDefault())

	updated.focusedView = viewFormatMP3
	updated, _ = updated.Update(keyCode(tea.KeySpace))
	assert.Equal(t, ya.AudioFormatMP3, updated.downloadOptions.FormatOrDefault())
}

func TestToggleAudioFormatIsDisabledWhileDownloading(t *testing.T) {
	m := NewDownloadModel(nil, ya.DownloadOptions{AudioFormat: ya.AudioFormatMP3})
	m.focusedView = viewFormatFLAC
	m.isDownloading = true

	updated, _ := m.Update(keyCode(tea.KeyEnter))

	assert.Equal(t, ya.AudioFormatMP3, updated.downloadOptions.FormatOrDefault())
}

func TestFormatShortcutsSelectFormatWhileIdle(t *testing.T) {
	m := NewDownloadModel(nil)

	updated, _ := m.Update(keyText("2"))
	assert.Equal(t, ya.AudioFormatFLAC, updated.downloadOptions.FormatOrDefault())

	updated, _ = updated.Update(keyText("1"))
	assert.Equal(t, ya.AudioFormatMP3, updated.downloadOptions.FormatOrDefault())
}

func TestFormatShortcutsAreDisabledWhileDownloading(t *testing.T) {
	m := NewDownloadModel(nil, ya.DownloadOptions{AudioFormat: ya.AudioFormatMP3})
	m.isDownloading = true

	updated, _ := m.Update(keyText("2"))
	assert.Equal(t, ya.AudioFormatMP3, updated.downloadOptions.FormatOrDefault())
}

func TestRenderFormatToggleShowsSelectedFormat(t *testing.T) {
	m := NewDownloadModel(nil, ya.DownloadOptions{AudioFormat: ya.AudioFormatFLAC})

	actionBar := renderActionBar(m)
	assert.Contains(t, actionBar, "FORMAT")
	assert.Contains(t, actionBar, "○ MP3")
	assert.Contains(t, actionBar, "● FLAC")
	assert.NotContains(t, actionBar, "[ MP3 ]")
}

func TestFocusedFormatDoesNotLeakNestedANSI(t *testing.T) {
	m := NewDownloadModel(nil, ya.DownloadOptions{AudioFormat: ya.AudioFormatFLAC})
	m.trackList.SetWidth(95)
	m.focusedView = viewFormatFLAC

	actionBar := renderActionBar(m)
	stripped := ansi.Strip(actionBar)

	assert.Contains(t, stripped, "2 ● FLAC")
	assert.NotContains(t, stripped, "[1;38;2;213;183;208m")
}

func TestFocusedControlUsesHighlightWithoutUnderline(t *testing.T) {
	rendered := renderControl("q Quit", true, false, true)

	assert.Contains(t, ansi.Strip(rendered), "q Quit")
	assert.NotContains(t, rendered, "\x1b[4m")
	assert.Contains(t, rendered, "38;2;213;183;208")
}

func TestArrowKeysMoveAcrossActionControls(t *testing.T) {
	m := NewDownloadModel(nil)
	m.focusedView = viewFormatMP3

	updated, _ := m.Update(keyCode(tea.KeyRight))
	assert.Equal(t, viewFormatFLAC, updated.focusedView)

	updated, _ = updated.Update(keyCode(tea.KeyRight))
	assert.Equal(t, viewDownloadButton, updated.focusedView)

	updated, _ = updated.Update(keyCode(tea.KeyLeft))
	assert.Equal(t, viewFormatFLAC, updated.focusedView)
}

func TestArrowKeysMoveBetweenActionRows(t *testing.T) {
	m := NewDownloadModel(nil)
	m.focusedView = viewFormatMP3

	updated, _ := m.Update(keyCode(tea.KeyDown))
	assert.Equal(t, viewDownloadButton, updated.focusedView)

	updated, _ = updated.Update(keyCode(tea.KeyUp))
	assert.Equal(t, viewFormatMP3, updated.focusedView)

	updated.focusedView = viewFormatFLAC
	updated, _ = updated.Update(keyCode(tea.KeyDown))
	assert.Equal(t, viewDownloadButton, updated.focusedView)

	updated.focusedView = viewQuitButton
	updated, _ = updated.Update(keyCode(tea.KeyUp))
	assert.Equal(t, viewFormatFLAC, updated.focusedView)
}

func TestArrowDownFallsBackToQuitWhenOnlyQuitEnabled(t *testing.T) {
	m := NewDownloadModel(nil)
	m.isDownloading = true
	m.focusedView = viewFormatMP3

	updated, _ := m.Update(keyCode(tea.KeyDown))
	assert.Equal(t, viewQuitButton, updated.focusedView)
}

func TestActionBarActivationUsesEnterAndSpace(t *testing.T) {
	m := NewDownloadModel(nil)
	m.focusedView = viewFormatFLAC

	updated, _ := m.Update(keyCode(tea.KeySpace))
	assert.Equal(t, ya.AudioFormatFLAC, updated.downloadOptions.FormatOrDefault())

	updated.focusedView = viewFormatMP3
	updated, _ = updated.Update(keyCode(tea.KeyEnter))
	assert.Equal(t, ya.AudioFormatMP3, updated.downloadOptions.FormatOrDefault())
}

func TestQuitShortcutUsesVisibleQuitBehavior(t *testing.T) {
	idle := NewDownloadModel(nil)
	_, idleCmd := idle.Update(keyText("q"))
	if assert.NotNil(t, idleCmd) {
		assert.IsType(t, tea.QuitMsg{}, idleCmd())
	}

	active := NewDownloadModel(ya.NewClient(utils.NewHttpClient()))
	active.isDownloading = true
	updated, activeCmd := active.Update(keyText("q"))
	assert.True(t, updated.shutdownRequested)
	assert.False(t, updated.quitAfterCancel)
	assert.Nil(t, activeCmd)
}

func TestEnterOnTrackListDoesNotActivateAction(t *testing.T) {
	m := NewDownloadModel(nil)
	addReadyTracks(&m, 1)

	updated, cmd := m.Update(keyCode(tea.KeyEnter))
	assert.False(t, updated.isDownloading)
	assert.Equal(t, viewList, updated.focusedView)
	assert.Nil(t, cmd)
}

func TestEnterWhileFilteringOnlyAppliesTheFilter(t *testing.T) {
	m := NewDownloadModel(nil)
	addReadyTracks(&m, 1)

	updated, _ := m.Update(keyText("/"))
	assert.Equal(t, list.Filtering, updated.trackList.FilterState())
	updated, _ = updated.Update(keyText("x"))

	updated, _ = updated.Update(keyCode(tea.KeyEnter))
	assert.False(t, updated.isDownloading)
	assert.Equal(t, list.FilterApplied, updated.trackList.FilterState())
}

func TestDisabledPrimaryShortcutsKeepFocusOnTrackList(t *testing.T) {
	m := NewDownloadModel(nil)
	m.isDownloading = true

	for _, shortcut := range []string{"D", "b"} {
		updated, _ := m.Update(keyText(shortcut))
		assert.Equal(t, viewList, updated.focusedView)
	}

	updated, _ := m.Update(keyCode(tea.KeyEnter))
	assert.Equal(t, viewList, updated.focusedView)
}

func TestWindowResizeShrinksTrackListToAvailableHeight(t *testing.T) {
	m := NewDownloadModel(nil)

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	assert.Equal(t, 95, updated.trackList.Width())
	assert.Equal(t, 17, updated.trackList.Height())
	assert.Equal(t, 95, updated.progress.Width())
}

func TestWindowResizeKeepsRenderedDownloadWithinWindowHeight(t *testing.T) {
	m := NewDownloadModel(nil)
	addReadyTracks(&m, 40)

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	assert.LessOrEqual(t, lipgloss.Height(updated.render()), 30)
}

func TestWindowResizeRendersTrackListAcrossWindowWidth(t *testing.T) {
	m := NewDownloadModel(nil)
	addReadyTracks(&m, 40)

	m.tracksProgress[0].status = TrackStatusAlreadyExists
	m.tracksProgress[1].status = TrackStatusDownloaded
	m.tracksProgress[1].format = "FLAC"
	m.tracksProgress[2].status = TrackStatusDownloaded
	m.tracksProgress[2].format = "MP3"
	m.updateTrackList()

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	assert.Equal(t, 100, lipgloss.Width(updated.renderTrackList()))
}

func TestWindowResizeKeepsRoomForFocusedActionBar(t *testing.T) {
	m := NewDownloadModel(nil)
	addReadyTracks(&m, 40)
	m.focusedView = viewDownloadButton

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	assert.LessOrEqual(t, lipgloss.Height(updated.render()), 30)
}

func TestTrackListHeightStaysConstantAcrossStatuses(t *testing.T) {
	m := NewDownloadModel(nil)
	addReadyTracks(&m, 9)
	m.isDownloading = true
	m.Resize(100, 30)

	expectedHeight := m.trackList.Height()
	updates := []struct {
		status    TrackStatus
		completed bool
		format    string
	}{
		{status: TrackStatusDownloading},
		{status: TrackStatusDownloaded, completed: true, format: "MP3"},
		{status: TrackStatusAlreadyExists, completed: true},
	}

	for i, update := range updates {
		updated, _ := m.Update(DownloadProgressUpdateMsg{
			progress: TrackProgress{
				uid:    m.tracksProgress[i].uid,
				status: update.status,
				format: update.format,
			},
			completed: update.completed,
		})
		m = updated

		assert.Equal(t, expectedHeight, m.trackList.Height(), update.status.String())
		assert.Equal(t, m.availableTrackListHeight(), m.trackList.Height(), update.status.String())
	}
}

func TestStartingDownloadReflowsTrackList(t *testing.T) {
	m := NewDownloadModel(nil)
	addReadyTracks(&m, 1)
	m.Resize(100, 30)
	m.trackList.SetHeight(1)
	m.focusedView = viewDownloadButton

	updated, _ := m.activateFocusedControl()

	assert.True(t, updated.isDownloading)
	assert.Equal(t, updated.availableTrackListHeight(), updated.trackList.Height())
}

func TestActionBarRendersCommandHelp(t *testing.T) {
	m := NewDownloadModel(nil)
	m.trackList.SetWidth(200)
	m.focusedView = viewDownloadButton

	actionBar := renderActionBar(m)
	plain := ansi.Strip(actionBar)

	assert.Contains(t, plain, "tab tracks")
	assert.Contains(t, plain, "esc tracks")
	assert.Contains(t, plain, "select action")
	assert.Contains(t, plain, "D Download all")
	assert.Contains(t, plain, "more help")
	assert.NotContains(t, plain, "select/download")
	assert.NotContains(t, plain, "[ Download all ]")

	m.focusedView = viewList
	assert.Contains(t, ansi.Strip(renderActionBar(m)), "duplicates")
}

func TestDownloadAllShortcutUsesUppercaseD(t *testing.T) {
	m := NewDownloadModel(nil)
	m.trackList.SetWidth(200)

	assert.Equal(t, []string{"D"}, downloadKeys.Download.Keys())
	assert.Contains(t, ansi.Strip(renderActionBar(m)), "D Download all")

	updated, cmd := m.Update(keyText("D"))
	assert.True(t, updated.isDownloading)
	assert.NotNil(t, cmd)

	idle := NewDownloadModel(nil)
	idle, _ = idle.Update(keyText("d"))
	assert.False(t, idle.isDownloading)
}

func TestHelpToggleExpandsAndCollapsesCommandHelp(t *testing.T) {
	m := NewDownloadModel(nil)
	compactHeight := lipgloss.Height(renderActionBar(m))

	updated, _ := m.Update(keyText("?"))
	assert.True(t, updated.help.ShowAll)
	assert.Greater(t, lipgloss.Height(renderActionBar(updated)), compactHeight)

	updated, _ = updated.Update(keyText("?"))
	assert.False(t, updated.help.ShowAll)
}

func TestActionBarFitsNarrowTerminalWidth(t *testing.T) {
	m := NewDownloadModel(nil)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 50, Height: 18})
	queueWidth := updated.trackList.Width() + updated.trackListStyle().GetHorizontalFrameSize()

	for _, line := range strings.Split(ansi.Strip(renderActionBar(updated)), "\n") {
		assert.LessOrEqual(t, lipgloss.Width(line), queueWidth)
	}
}

func TestActionBarMatchesQueueWidthAndKeepsQuitTogether(t *testing.T) {
	m := NewDownloadModel(nil)
	addReadyTracks(&m, 1)
	m.trackList.SetWidth(95)

	actionBar := renderActionBar(m)
	queueWidth := lipgloss.Width(m.renderTrackList())

	assert.Equal(t, queueWidth, lipgloss.Width(actionBar))
	for _, line := range strings.Split(ansi.Strip(actionBar), "\n") {
		assert.LessOrEqual(t, lipgloss.Width(line), queueWidth)
	}

	for _, line := range strings.Split(ansi.Strip(actionBar), "\n") {
		if strings.Contains(line, "ACTIONS") {
			assert.Contains(t, line, "q Quit")
		}
	}
}

func TestActionBarUsesTheSameFullFrameAsTheTrackList(t *testing.T) {
	m := NewDownloadModel(nil)
	addReadyTracks(&m, 1)
	m.trackList.SetWidth(95)

	actionBar := ansi.Strip(renderActionBar(m))
	for _, edge := range []string{
		borderStyle.TopLeft,
		borderStyle.TopRight,
		borderStyle.BottomLeft,
		borderStyle.BottomRight,
		borderStyle.Left,
		borderStyle.Right,
	} {
		assert.Contains(t, actionBar, edge)
	}
}

func TestTabFocusDoesNotChangeTrackListHeight(t *testing.T) {
	m := NewDownloadModel(nil)
	addReadyTracks(&m, 40)

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	listHeight := updated.trackList.Height()

	updated, _ = updated.Update(keyCode(tea.KeyTab))

	assert.NotEqual(t, viewList, updated.focusedView)
	assert.Equal(t, listHeight, updated.trackList.Height())
	assert.LessOrEqual(t, lipgloss.Height(updated.render()), 30)

	updated, _ = updated.Update(keyCode(tea.KeyTab))
	assert.Equal(t, viewList, updated.focusedView)
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
		{status: TrackStatusAlreadyExists},
		{status: TrackStatusError},
		{status: TrackStatusDuplicate},
		{status: TrackStatusNotAvailable},
	}

	m.resetState()

	assert.Equal(t, TrackStatusReady, m.tracksProgress[0].status)
	assert.Equal(t, TrackStatusReady, m.tracksProgress[1].status)
	assert.Equal(t, TrackStatusReady, m.tracksProgress[2].status)
	assert.Equal(t, TrackStatusDuplicate, m.tracksProgress[3].status)
	assert.Equal(t, TrackStatusNotAvailable, m.tracksProgress[4].status)
	assert.Equal(t, 5, m.tracksTotalCount)
	assert.Equal(t, 3, m.downloadableCount)
	assert.Equal(t, 0, m.downloadedCount)
	assert.Equal(t, 0, m.sessionCompletedCount)
}

func TestSessionProgressExcludesPriorDownloads(t *testing.T) {
	m := NewDownloadModel(nil)
	for i := 0; i < 5; i++ {
		m.tracksProgress = append(m.tracksProgress, &TrackProgress{
			uid:    fmt.Sprintf("downloaded-%d", i),
			status: TrackStatusDownloaded,
		})
	}
	for i := 0; i < 5; i++ {
		m.tracksProgress = append(m.tracksProgress, &TrackProgress{
			uid:    fmt.Sprintf("ready-%d", i),
			status: TrackStatusReady,
		})
	}

	m.resetState()

	assert.Equal(t, 10, m.downloadableCount)
	assert.InDelta(t, 0.0, m.sessionProgress(), 0.0001)

	for i := 0; i < 5; i++ {
		updated, _ := m.Update(DownloadProgressUpdateMsg{
			progress:  TrackProgress{uid: fmt.Sprintf("downloaded-%d", i), status: TrackStatusDownloaded},
			completed: true,
		})
		m = updated
	}
	for i := 0; i < 5; i++ {
		updated, _ := m.Update(DownloadProgressUpdateMsg{
			progress:  TrackProgress{uid: fmt.Sprintf("ready-%d", i), status: TrackStatusDownloaded},
			completed: true,
		})
		m = updated
	}

	assert.InDelta(t, 1.0, m.sessionProgress(), 0.0001)
}

func TestStartDownloadSessionRequeuesCompletedTracksAfterReset(t *testing.T) {
	m := NewDownloadModel(nil)
	m.tracksProgress = []*TrackProgress{
		{uid: "downloaded", track: &model.Track{ID: model.FlexibleID("1"), Title: "Downloaded"}, status: TrackStatusDownloaded},
		{uid: "exists", track: &model.Track{ID: model.FlexibleID("2"), Title: "Exists"}, status: TrackStatusAlreadyExists},
		{uid: "ready", track: &model.Track{ID: model.FlexibleID("3"), Title: "Ready"}, status: TrackStatusReady},
	}

	m.resetState()
	session := NewDownloadSession(
		&fakeDownloadClient{filename: "song.mp3"},
		utils.NewDiscardDownloadLogger(),
		ya.DownloadOptions{},
		t.TempDir(),
	)

	progress := make([]TrackProgress, 0, len(m.tracksProgress))
	for _, item := range m.tracksProgress {
		progress = append(progress, *item)
	}

	eventCount := 0
	for event := range session.Run(progress) {
		eventCount++
		updated, _ := m.Update(DownloadProgressUpdateMsg{
			progress:  event.Progress,
			completed: event.Completed,
		})
		m = updated
	}

	assert.Equal(t, 6, eventCount)
	assert.Equal(t, TrackStatusDownloaded, m.tracksProgress[0].status)
	assert.Equal(t, TrackStatusDownloaded, m.tracksProgress[1].status)
	assert.Equal(t, TrackStatusDownloaded, m.tracksProgress[2].status)
	assert.Equal(t, "song.mp3", m.tracksProgress[2].filename)
}

func TestResetStateRequeuesAllDownloadedTracksForNewSession(t *testing.T) {
	m := NewDownloadModel(nil)
	for i := 0; i < 3; i++ {
		m.tracksProgress = append(m.tracksProgress, &TrackProgress{
			uid:    fmt.Sprintf("track-%d", i),
			track:  &model.Track{ID: model.FlexibleID(fmt.Sprintf("%d", i)), Title: fmt.Sprintf("Song %d", i)},
			status: TrackStatusDownloaded,
		})
	}

	m.resetState()
	assert.Equal(t, 3, m.downloadableCount)

	session := NewDownloadSession(
		&fakeDownloadClient{filename: "song.mp3"},
		utils.NewDiscardDownloadLogger(),
		ya.DownloadOptions{},
		t.TempDir(),
	)

	progress := make([]TrackProgress, 0, len(m.tracksProgress))
	for _, item := range m.tracksProgress {
		progress = append(progress, *item)
	}

	eventCount := 0
	for range session.Run(progress) {
		eventCount++
	}

	assert.Equal(t, 6, eventCount)
}

func TestSkipDownloadReason(t *testing.T) {
	testCases := []struct {
		status TrackStatus
		reason string
	}{
		{TrackStatusDownloading, "already_downloading"},
		{TrackStatusDuplicate, "duplicate"},
		{TrackStatusNotAvailable, "not_available"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.reason, func(t *testing.T) {
			reason, shouldSkip := skipDownloadReason(testCase.status)
			assert.True(t, shouldSkip)
			assert.Equal(t, testCase.reason, reason)
		})
	}

	for _, status := range []TrackStatus{TrackStatusDownloaded, TrackStatusAlreadyExists, TrackStatusReady, TrackStatusError} {
		t.Run(status.String(), func(t *testing.T) {
			_, shouldSkip := skipDownloadReason(status)
			assert.False(t, shouldSkip)
		})
	}
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
	assert.Equal(t, "M4A", downloadFormatFromFilename("Artist - Song.m4a"))
	assert.Equal(t, "M4A", downloadFormatFromFilename("Artist - Song.MP4"))
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

func TestDownloadProgressUpdateAppliesWorkerSnapshot(t *testing.T) {
	m := NewDownloadModel(nil)
	track := &model.Track{ID: model.FlexibleID("1"), Title: "Song"}
	m.tracksProgress = []*TrackProgress{{uid: "track-1", track: track, status: TrackStatusReady}}
	m.tracksTotalCount = 1
	m.downloadableCount = 1

	updated, _ := m.Update(DownloadProgressUpdateMsg{
		progress:  TrackProgress{uid: "track-1", track: track, status: TrackStatusDownloaded, filename: "song.mp3", format: "MP3"},
		completed: true,
	})

	assert.Equal(t, TrackStatusDownloaded, updated.tracksProgress[0].status)
	assert.Equal(t, "song.mp3", updated.tracksProgress[0].filename)
	assert.Equal(t, 1, updated.downloadedCount)
}

func TestDownloadProgressUpdateIgnoresUnknownWorkerSnapshot(t *testing.T) {
	m := NewDownloadModel(nil)
	track := &model.Track{ID: model.FlexibleID("1"), Title: "Song"}
	m.tracksProgress = []*TrackProgress{{uid: "track-1", track: track, status: TrackStatusReady}}
	m.downloadedCount = 3

	updated, _ := m.Update(DownloadProgressUpdateMsg{
		progress:  TrackProgress{uid: "unknown", track: track, status: TrackStatusDownloaded, filename: "song.mp3", format: "MP3"},
		completed: true,
	})

	assert.Equal(t, 3, updated.downloadedCount)
	assert.Equal(t, []*TrackProgress{{uid: "track-1", track: track, status: TrackStatusReady}}, updated.tracksProgress)
}

func TestDownloadSessionLogsSkippedReasons(t *testing.T) {
	var logs bytes.Buffer
	logger := utils.NewDownloadLoggerForWriter(&logs)
	client := ya.NewClient(utils.NewHttpClientWithLogger(logger))

	progressList := []TrackProgress{
		{
			track:  &model.Track{ID: model.FlexibleID("1"), Title: "Duplicate"},
			status: TrackStatusDuplicate,
		},
		{
			track:  &model.Track{ID: model.FlexibleID("2"), Title: "Unavailable"},
			status: TrackStatusNotAvailable,
		},
	}

	session := NewDownloadSession(client, logger, ya.DownloadOptions{}, outputDir)
	for range session.Run(progressList) {
	}

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

	updated, cmd := m.Update(keyCode(tea.KeyEnter))

	assert.True(t, updated.shutdownRequested)
	assert.False(t, updated.quitAfterCancel)
	assert.Nil(t, cmd)
}

func TestDownloadEndDoesNotQuitAfterCancelButtonRequest(t *testing.T) {
	m := NewDownloadModel(nil)
	m.isDownloading = true
	m.shutdownRequested = true

	updated, cmd := m.Update(DownloadEndMsg{})

	assert.False(t, updated.isDownloading)
	assert.False(t, updated.shutdownRequested)
	assert.Nil(t, cmd)
}

func TestDownloadEndReflowsTrackList(t *testing.T) {
	m := NewDownloadModel(nil)
	m.isDownloading = true
	m.Resize(100, 30)
	m.trackList.SetHeight(1)

	updated, _ := m.Update(DownloadEndMsg{})

	assert.False(t, updated.isDownloading)
	assert.Equal(t, updated.availableTrackListHeight(), updated.trackList.Height())
}

func TestDownloadEndRestoresCanceledTracksToReady(t *testing.T) {
	m := NewDownloadModel(nil)
	m.isDownloading = true
	m.shutdownRequested = true
	m.sessionCompletedCount = 4
	m.tracksProgress = []*TrackProgress{
		{status: TrackStatusDownloaded, filename: "done.mp3"},
		{status: TrackStatusAlreadyExists, filename: "exists.mp3"},
		{status: TrackStatusError, errMsg: "context canceled"},
		{status: TrackStatusDownloading},
	}

	updated, _ := m.Update(DownloadEndMsg{})

	assert.Equal(t, TrackStatusDownloaded, updated.tracksProgress[0].status)
	assert.Equal(t, TrackStatusAlreadyExists, updated.tracksProgress[1].status)
	assert.Equal(t, TrackStatusReady, updated.tracksProgress[2].status)
	assert.Empty(t, updated.tracksProgress[2].errMsg)
	assert.Equal(t, TrackStatusReady, updated.tracksProgress[3].status)
	assert.Equal(t, 2, updated.downloadedCount)
	assert.Equal(t, 2, updated.downloadableCount)
	assert.Equal(t, 0, updated.sessionCompletedCount)
	assert.InDelta(t, 0.0, updated.sessionProgress(), 0.0001)
	assert.Equal(t, 0, updated.errorCount)
}

func TestDownloadEndQuitsAfterShutdownRequest(t *testing.T) {
	m := NewDownloadModel(nil)
	m.isDownloading = true
	m.shutdownRequested = true
	m.quitAfterCancel = true

	updated, cmd := m.Update(DownloadEndMsg{})

	assert.False(t, updated.isDownloading)
	if assert.NotNil(t, cmd) {
		assert.IsType(t, tea.QuitMsg{}, cmd())
	}
}
