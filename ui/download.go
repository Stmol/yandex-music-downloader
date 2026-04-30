package ui

import (
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"runtime/debug"
	"sort"
	"strings"
	"sync"
	"ya-music/utils"
	"ya-music/ya"
	"ya-music/ya/model"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/google/uuid"
)

// Global constants.
const (
	outputDir              = "./downloads" // Root directory for downloads.
	maxConcurrentDownloads = 3             // Maximum number of concurrent downloads.
	defaultDownloadWidth   = 80
	defaultTrackListHeight = 18
	minTrackListHeight     = 6
)

// Global style variables.
var (
	marginLeftStyle     = lipgloss.NewStyle().MarginLeft(2)
	baseTrackListStyle  = lipgloss.NewStyle().PaddingRight(3)
	borderStyle         = lipgloss.RoundedBorder()
	actionBarStyle      = lipgloss.NewStyle().MarginLeft(2)
	actionBarFocusStyle = lipgloss.NewStyle().Margin(1, 0, 0, 0).Border(borderStyle).Padding(0, 1)
	actionBarBlurStyle  = lipgloss.NewStyle().Margin(1, 0, 0, 1).Padding(1, 1, 0, 1)
	controlBaseStyle    = lipgloss.NewStyle().MarginRight(1)
	controlFocusStyle   = controlBaseStyle.Foreground(lipgloss.Color("#FFFFFF")).Background(lipgloss.Color("#4A0549")).Bold(true)
	controlActiveStyle  = controlBaseStyle.Foreground(lipgloss.Color("#006400")).Bold(true)
	controlDimStyle     = controlBaseStyle.Foreground(lipgloss.Color("#808080"))
)

// Focusable represents which view element is currently focused.
type focusable int

// UI view constants.
const (
	viewList focusable = iota
	viewFormatMP3
	viewFormatFLAC
	viewBackButton
	viewDownloadButton
	viewQuitButton
)

var actionFocusOrder = []focusable{
	viewFormatMP3,
	viewFormatFLAC,
	viewBackButton,
	viewDownloadButton,
	viewQuitButton,
}

var downloadKeys = downloadKeyMap{
	Next: key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("tab", "next"),
	),
	Prev: key.NewBinding(
		key.WithKeys("shift+tab"),
		key.WithHelp("shift+tab", "prev"),
	),
	Left: key.NewBinding(
		key.WithKeys("left", "h"),
		key.WithHelp("left/right", "move controls"),
	),
	Right: key.NewBinding(
		key.WithKeys("right", "l"),
		key.WithHelp("", ""),
	),
	Activate: key.NewBinding(
		key.WithKeys("enter", " "),
		key.WithHelp("enter/space", "activate"),
	),
	FocusList: key.NewBinding(
		key.WithKeys("esc", "up"),
		key.WithHelp("esc/up", "tracks"),
	),
	Duplicates: key.NewBinding(
		key.WithKeys("t", "T"),
		key.WithHelp("t", "duplicates"),
	),
}

type downloadKeyMap struct {
	Next       key.Binding
	Prev       key.Binding
	Left       key.Binding
	Right      key.Binding
	Activate   key.Binding
	FocusList  key.Binding
	Duplicates key.Binding
}

func (k downloadKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Next, k.Left, k.Activate, k.FocusList, k.Duplicates}
}

func (k downloadKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Next, k.Prev},
		{k.Left, k.Activate},
		{k.FocusList, k.Duplicates},
	}
}

// TrackProgress represents the download progress and state of a track.
type TrackProgress struct {
	uid      string
	track    *model.Track
	status   TrackStatus
	errMsg   string
	filename string
	format   string
}

type DownloadStartMsg struct{}
type DownloadEndMsg struct{}
type DownloadProgressUpdateMsg struct {
	downloaded bool
}

type DownloadModel struct {
	// External dependencies.
	client          *ya.Client
	downloadOptions ya.DownloadOptions

	// UI components.
	spinner   spinner.Model
	progress  progress.Model
	trackList list.Model
	help      help.Model

	// Download progress channels and tracking.
	tpUpdateCh     chan TrackProgress
	tracksProgress []*TrackProgress

	// Counters.
	tracksTotalCount  int
	downloadedCount   int
	downloadableCount int
	errorCount        int

	// UI state.
	isDownloading     bool
	shutdownRequested bool
	focusedView       focusable
	lastActionFocus   focusable
	selectedTrackInfo string
	hideDuplicates    bool
	windowWidth       int
	windowHeight      int
}

func NewDownloadModel(client *ya.Client, options ...ya.DownloadOptions) DownloadModel {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = spinnerStyle

	p := progress.New(
		progress.WithDefaultGradient(),
		progress.WithWidth(75),
		progress.WithoutPercentage(),
	)

	l := list.New([]list.Item{}, TrackListItem{}, 60, defaultTrackListHeight)
	l.DisableQuitKeybindings()
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.AdditionalShortHelpKeys = func() []key.Binding {
		return []key.Binding{
			downloadKeys.Duplicates,
		}
	}
	l.AdditionalFullHelpKeys = func() []key.Binding {
		return []key.Binding{
			downloadKeys.Duplicates,
		}
	}

	h := help.New()
	h.Width = defaultDownloadWidth

	return DownloadModel{
		client:            client,
		downloadOptions:   downloadOptionsOrDefault(options),
		spinner:           sp,
		progress:          p,
		trackList:         l,
		help:              h,
		tracksProgress:    []*TrackProgress{},
		focusedView:       viewList,
		lastActionFocus:   viewFormatMP3,
		hideDuplicates:    false,
		shutdownRequested: false,
		selectedTrackInfo: "",
	}
}

func downloadOptionsOrDefault(options []ya.DownloadOptions) ya.DownloadOptions {
	if len(options) == 0 {
		return ya.DownloadOptions{}
	}
	return options[0]
}

func (m DownloadModel) Init() tea.Cmd {
	return nil
}

func (m *DownloadModel) Reset() {
	m.tracksProgress = nil
	m.tpUpdateCh = nil
	m.tracksTotalCount = 0
	m.downloadedCount = 0
	m.downloadableCount = 0
	m.errorCount = 0
	m.isDownloading = false
	m.shutdownRequested = false
	m.focusedView = viewList
	m.lastActionFocus = viewFormatMP3
	m.selectedTrackInfo = ""
	m.hideDuplicates = false
	m.trackList.ResetFilter()
	m.trackList.ResetSelected()
	m.trackList.SetItems(nil)
}

func (m *DownloadModel) AddTracks(tracks []model.Track) {
	for _, track := range tracks {
		status := TrackStatusReady
		if !track.Available {
			status = TrackStatusNotAvailable
		}

		progress := &TrackProgress{
			uid:    uuid.New().String(),
			track:  &track,
			status: status,
		}

		m.tracksProgress = append(m.tracksProgress, progress)
	}

	findDuplicates(m.tracksProgress)
	sortTracksByTitle(m.tracksProgress)

	m.updateTrackList()
	m.tracksTotalCount = len(m.tracksProgress)
	m.downloadableCount = countStatus(m.tracksProgress, TrackStatusReady)

	if item, ok := m.trackList.SelectedItem().(TrackListItem); ok {
		m.selectedTrackInfo = m.getTrackInfo(item.uid)
	}
}

func (m DownloadModel) Update(msg tea.Msg) (DownloadModel, tea.Cmd) {
	var cmds []tea.Cmd
	var cmd tea.Cmd

	if m.focusedView == viewList {
		m.trackList, cmd = m.trackList.Update(msg)
		cmds = append(cmds, cmd)
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.windowWidth = msg.Width
		m.windowHeight = msg.Height
		m.resizeToWindow()

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, downloadKeys.Activate):
			m, cmd = m.activateFocusedControl()

		case key.Matches(msg, downloadKeys.Next):
			m.focusNext()

		case key.Matches(msg, downloadKeys.Prev):
			m.focusPrevious()

		case key.Matches(msg, downloadKeys.Right):
			if m.focusedView != viewList {
				m.focusNextAction()
			}

		case key.Matches(msg, downloadKeys.Left):
			if m.focusedView != viewList {
				m.focusPreviousAction()
			}

		case key.Matches(msg, downloadKeys.FocusList):
			if m.focusedView != viewList {
				m.focusedView = viewList
			}

		case key.Matches(msg, downloadKeys.Duplicates):
			if m.focusedView == viewList {
				m.hideDuplicates = !m.hideDuplicates
				m.updateTrackList()
			}
		}

	case DownloadStartMsg:
		m.updateTrackList()
		cmd = handleDownloadsProgress(m.tpUpdateCh)

	case DownloadProgressUpdateMsg:
		if msg.downloaded {
			m.downloadedCount++
		}
		m.errorCount = countStatus(m.tracksProgress, TrackStatusError)
		m.updateTrackList()
		cmd = handleDownloadsProgress(m.tpUpdateCh)

	case DownloadEndMsg:
		m.isDownloading = false
		if m.shutdownRequested {
			return m, tea.Quit
		}

	case ListSelectedItemMsg, list.FilterMatchesMsg:
		if item, ok := m.trackList.SelectedItem().(TrackListItem); ok {
			m.selectedTrackInfo = m.getTrackInfo(item.uid)
		}
	}

	cmds = append(cmds, cmd)
	return m, tea.Batch(cmds...)
}

func (m DownloadModel) View() string {
	header := renderHeader(m.downloadedCount, m.tracksTotalCount, m.downloadableCount, m.errorCount)
	viewStr := marginLeftStyle.Render(header)
	viewStr += "\n" + marginLeftStyle.Render(m.selectedTrackInfo) + "\n"

	trackListStyle := baseTrackListStyle
	if m.focusedView == viewList {
		trackListStyle = trackListStyle.Border(borderStyle)
	} else {
		trackListStyle = trackListStyle.Margin(1)
	}
	viewStr += trackListStyle.Render(m.trackList.View()) + "\n"
	viewStr += marginLeftStyle.Render(m.renderProgress())
	viewStr += renderActionBar(m)
	return viewStr
}

func (m *DownloadModel) resizeToWindow() {
	if m.windowWidth <= 0 || m.windowHeight <= 0 {
		return
	}

	contentWidth := m.windowWidth - 8
	if contentWidth < 40 {
		contentWidth = 40
	}

	m.progress.Width = contentWidth
	m.help.Width = contentWidth
	m.trackList.SetWidth(contentWidth)
	m.trackList.SetHeight(m.availableTrackListHeight())
}

func (m DownloadModel) availableTrackListHeight() int {
	reservedRows := 13
	if m.windowHeight <= 0 {
		return defaultTrackListHeight
	}

	height := m.windowHeight - reservedRows
	if height < minTrackListHeight {
		return minTrackListHeight
	}
	return height
}

func (m *DownloadModel) cycleFocus() {
	m.focusNext()
}

func (m *DownloadModel) downloadTracks(updCh chan TrackProgress, progressList []*TrackProgress) tea.Cmd {
	return func() tea.Msg {
		var wg sync.WaitGroup
		sem := make(chan struct{}, maxConcurrentDownloads)

		logger := downloadLogger(m.client)
		logger.Info("download session started",
			"total_tracks", len(progressList),
			"max_concurrent_downloads", maxConcurrentDownloads,
			"format", m.downloadOptions.FormatOrDefault(),
		)

		for _, tp := range progressList {
			if reason, shouldSkip := skipDownloadReason(tp.status); shouldSkip {
				logger.LogTrack(slog.LevelInfo, utils.NewTrackLogContext(*tp.track), "skipped",
					"stage", "queue",
					"reason", reason,
				)
				continue
			}

			logger.LogTrack(slog.LevelInfo, utils.NewTrackLogContext(*tp.track), "queued",
				"stage", "queue",
			)
			wg.Add(1)
			go m.downloadTrack(tp, &wg, sem, updCh)
		}

		go func() {
			wg.Wait()
			logger.Info("download session finished")
			close(updCh)
		}()

		return DownloadStartMsg{}
	}
}

func (m *DownloadModel) downloadTrack(tp *TrackProgress, wg *sync.WaitGroup, sem chan struct{}, updCh chan TrackProgress) {
	defer wg.Done()
	logger := downloadLogger(m.client)
	trackCtx := utils.NewTrackLogContext(*tp.track)

	defer func() {
		if r := recover(); r != nil {
			tp.status = TrackStatusError
			tp.errMsg = fmt.Sprintf("panic: %v", r)
			logger.LogTrack(slog.LevelError, trackCtx, "panic recovered",
				"stage", "download_track",
				"error", fmt.Sprintf("%v", r),
				"stack", string(debug.Stack()),
			)
			updCh <- *tp
		}
	}()

	sem <- struct{}{}
	defer func() { <-sem }()

	tp.status = TrackStatusDownloading
	logger.LogTrack(slog.LevelInfo, trackCtx, "worker started",
		"stage", "download_track",
	)
	updCh <- *tp

	filePath, err := m.client.DownloadTrackWithOptions(*tp.track, outputDir, m.downloadOptions)
	if err != nil {
		tp.status = TrackStatusError
		tp.errMsg = err.Error()
		tp.filename = filePath

		if errors.Is(err, ya.ErrTrackAlreadyExists) {
			tp.status = TrackStatusAlreadyExists
			tp.format = downloadFormatFromFilename(filePath)
			logger.LogTrack(slog.LevelInfo, trackCtx, "worker skipped",
				"stage", "download_track",
				"status", tp.status.String(),
				"filename", filePath,
				"reason", "already_exists",
			)
			updCh <- *tp
			return
		}

		logger.LogTrack(slog.LevelError, trackCtx, "worker finished with error",
			"stage", "download_track",
			"status", tp.status.String(),
			"filename", filePath,
			"error", err,
		)
	} else {
		tp.status = TrackStatusDownloaded
		tp.filename = filePath
		tp.format = downloadFormatFromFilename(filePath)
		tp.errMsg = ""

		logger.LogTrack(slog.LevelInfo, trackCtx, "worker finished",
			"stage", "download_track",
			"status", tp.status.String(),
			"filename", filePath,
		)
	}

	updCh <- *tp
}

func (m *DownloadModel) resetState() {
	for _, tp := range m.tracksProgress {
		if tp.status == TrackStatusDuplicate || tp.status == TrackStatusNotAvailable {
			continue
		}
		tp.status = TrackStatusReady
		tp.format = ""
	}

	m.downloadedCount = countStatus(m.tracksProgress, TrackStatusDownloaded)
	m.errorCount = countStatus(m.tracksProgress, TrackStatusError)
	m.downloadableCount = countStatus(m.tracksProgress, TrackStatusReady)
	m.tracksTotalCount = len(m.tracksProgress)

	m.updateTrackList()
}

func (m *DownloadModel) updateTrackList() {
	items := make([]list.Item, 0, len(m.tracksProgress))
	for _, tp := range m.tracksProgress {
		if m.hideDuplicates && tp.status == TrackStatusDuplicate {
			continue
		}
		items = append(items, TrackListItem{
			uid:    tp.uid,
			track:  tp.track,
			status: tp.status,
			format: tp.format,
		})
	}
	m.trackList.SetItems(items)
}

func (m *DownloadModel) getTrackInfo(uid string) string {
	var info string
	for _, tp := range m.tracksProgress {
		if tp.uid == uid {
			info = fmt.Sprintf("%s - %s", tp.track.FullTitle(), tp.track.ArtistsString())
			if tp.filename != "" {
				info = fmt.Sprintf("Downloaded: %s", tp.filename)
			}
			if tp.errMsg != "" {
				info = tp.errMsg
			}
			break
		}
	}

	if len(info) > 70 {
		info = info[:67] + "..."
	}
	return strings.TrimSpace(info)
}

func (m DownloadModel) renderProgress() string {
	var percent float64
	if m.downloadableCount > 0 {
		percent = float64(m.downloadedCount) / float64(m.downloadableCount)
	}
	return m.progress.ViewAs(percent) + "\n"
}

func countStatus(tracks []*TrackProgress, status TrackStatus) int {
	count := 0
	for _, tp := range tracks {
		if tp.status == status {
			count++
		}
	}
	return count
}

func renderHeader(completed, total, downloadable, errors int) string {
	return strings.Join([]string{
		renderCounter("Total tracks", total),
		renderCounter("To download", downloadable),
		renderCounter("Completed", completed),
		renderCounter("Errors", errors),
	}, "  ") + "\n"
}

func renderCounter(label string, value int) string {
	return dimGrayForeground.Render(label+":") + " " + fmt.Sprintf("%d", value)
}

func (m DownloadModel) activateFocusedControl() (DownloadModel, tea.Cmd) {
	switch m.focusedView {
	case viewFormatMP3:
		if !m.isDownloading {
			m.downloadOptions.AudioFormat = ya.AudioFormatMP3
		}

	case viewFormatFLAC:
		if !m.isDownloading {
			m.downloadOptions.AudioFormat = ya.AudioFormatFLAC
		}

	case viewBackButton:
		if !m.isDownloading {
			return m, func() tea.Msg { return BackToURLMsg{} }
		}

	case viewDownloadButton:
		if m.isDownloading {
			return m, nil
		}
		m.isDownloading = true
		m.resetState()
		m.focusedView = viewList
		m.tpUpdateCh = make(chan TrackProgress)

		utils.CreateDirIfNotExists(outputDir)
		return m, m.downloadTracks(m.tpUpdateCh, m.tracksProgress)

	case viewQuitButton:
		if m.isDownloading {
			m.requestShutdown("quit_button")
			return m, nil
		}

		downloadLogger(m.client).Info("application quit requested",
			"reason", "quit_button",
			"is_downloading", false,
		)
		return m, tea.Quit
	}

	return m, nil
}

func (m *DownloadModel) focusNext() {
	if m.focusedView == viewList {
		m.focusFirstAction()
		return
	}
	m.focusedView = viewList
}

func (m *DownloadModel) focusPrevious() {
	if m.focusedView == viewList {
		m.focusFirstAction()
		return
	}
	m.focusedView = viewList
}

func (m *DownloadModel) focusFirstAction() {
	if m.controlEnabled(m.lastActionFocus) {
		m.focusedView = m.lastActionFocus
		return
	}
	m.focusedView = firstEnabledAction(*m)
}

func (m *DownloadModel) focusNextAction() {
	index := actionIndex(m.focusedView)
	for offset := 1; offset <= len(actionFocusOrder); offset++ {
		next := actionFocusOrder[(index+offset+len(actionFocusOrder))%len(actionFocusOrder)]
		if m.controlEnabled(next) {
			m.focusedView = next
			m.lastActionFocus = next
			return
		}
	}
}

func (m *DownloadModel) focusPreviousAction() {
	index := actionIndex(m.focusedView)
	for offset := 1; offset <= len(actionFocusOrder); offset++ {
		previous := actionFocusOrder[(index-offset+len(actionFocusOrder))%len(actionFocusOrder)]
		if m.controlEnabled(previous) {
			m.focusedView = previous
			m.lastActionFocus = previous
			return
		}
	}
}

func firstEnabledAction(m DownloadModel) focusable {
	for _, control := range actionFocusOrder {
		if m.controlEnabled(control) {
			return control
		}
	}
	return viewList
}

func actionIndex(view focusable) int {
	for i, control := range actionFocusOrder {
		if control == view {
			return i
		}
	}
	return -1
}

func (m DownloadModel) controlEnabled(control focusable) bool {
	switch control {
	case viewFormatMP3, viewFormatFLAC, viewBackButton, viewDownloadButton:
		return !m.isDownloading
	case viewQuitButton:
		return true
	default:
		return false
	}
}

func renderActionBar(m DownloadModel) string {
	formatControls := lipgloss.JoinHorizontal(lipgloss.Center,
		renderFormatSegment(m, viewFormatMP3, ya.AudioFormatMP3, "MP3"),
		renderFormatSegment(m, viewFormatFLAC, ya.AudioFormatFLAC, "FLAC"),
	)
	actionControls := lipgloss.JoinHorizontal(lipgloss.Center,
		renderActionControl(m, viewBackButton, "Back"),
		renderActionControl(m, viewDownloadButton, "Download all"),
		renderActionControl(m, viewQuitButton, quitControlLabel(m)),
	)

	formatRow := dimGrayForeground.Render("Format ") + formatControls
	actionRow := dimGrayForeground.Render("Actions") + " " + actionControls
	content := formatRow + "\n" + actionRow + "\n\n" + m.help.View(downloadKeys)
	if m.focusedView == viewList {
		return actionBarBlurStyle.Render(content)
	}
	return actionBarFocusStyle.Render(content)
}

func renderFormatSegment(m DownloadModel, control focusable, format ya.AudioFormat, label string) string {
	focused := m.focusedView == control
	active := m.downloadOptions.FormatOrDefault() == format
	enabled := m.controlEnabled(control)
	return renderControl(label, focused, active, enabled)
}

func renderActionControl(m DownloadModel, control focusable, label string) string {
	return renderControl(label, m.focusedView == control, false, m.controlEnabled(control))
}

func renderControl(label string, focused bool, active bool, enabled bool) string {
	text := fmt.Sprintf("[ %s ]", label)
	style := controlBaseStyle
	switch {
	case !enabled:
		style = controlDimStyle
	case focused:
		style = controlFocusStyle
	case active:
		style = controlActiveStyle
	}
	return style.Render(text)
}

func quitControlLabel(m DownloadModel) string {
	if m.shutdownRequested && m.isDownloading {
		return "Cancelling..."
	}
	if m.isDownloading {
		return "Cancel"
	}
	return "Quit"
}

func handleDownloadsProgress(updCh chan TrackProgress) tea.Cmd {
	return func() tea.Msg {
		track, ok := <-updCh
		if !ok {
			return DownloadEndMsg{}
		}
		return DownloadProgressUpdateMsg{
			downloaded: track.status == TrackStatusDownloaded ||
				track.status == TrackStatusAlreadyExists ||
				track.status == TrackStatusError,
		}
	}
}

func sortTracksByTitle(tracks []*TrackProgress) {
	sort.Slice(tracks, func(i, j int) bool {
		return tracks[i].track.FullTitle() < tracks[j].track.FullTitle()
	})
}

func findDuplicates(tracks []*TrackProgress) {
	seen := make(map[string]struct{}, len(tracks)*2)

	for _, tp := range tracks {
		idKey := tp.track.ID.String()
		nameKey := tp.track.FullTitle() + " - " + tp.track.ArtistsString()

		if _, exists := seen[idKey]; exists {
			tp.status = TrackStatusDuplicate
			continue
		}
		if _, exists := seen[nameKey]; exists {
			tp.status = TrackStatusDuplicate
			continue
		}

		seen[idKey] = struct{}{}
		seen[nameKey] = struct{}{}
	}
}

func skipDownloadReason(status TrackStatus) (string, bool) {
	switch status {
	case TrackStatusDownloading:
		return "already_downloading", true
	case TrackStatusDuplicate:
		return "duplicate", true
	case TrackStatusNotAvailable:
		return "not_available", true
	case TrackStatusAlreadyExists:
		return "already_exists", true
	default:
		return "", false
	}
}

func downloadFormatFromFilename(filename string) string {
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".flac":
		return "FLAC"
	default:
		return "MP3"
	}
}

func (m *DownloadModel) requestShutdown(reason string) {
	if m.shutdownRequested {
		return
	}

	m.shutdownRequested = true
	downloadLogger(m.client).Info("application quit requested",
		"reason", reason,
		"is_downloading", m.isDownloading,
	)

	if m.client != nil {
		m.client.Cancel()
	}
}
