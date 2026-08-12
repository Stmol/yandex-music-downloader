package ui

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"ya-music/utils"
	"ya-music/ya"
	"ya-music/ya/model"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/progress"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/google/uuid"
)

// Global constants.
const (
	outputDir                = "./downloads" // Root directory for downloads.
	maxConcurrentDownloads   = 3             // Maximum number of concurrent downloads.
	defaultTrackListHeight   = 18
	minTrackListHeight       = 6
	downloadHorizontalChrome = 5
)

// Global style variables.
var (
	marginLeftStyle     = lipgloss.NewStyle().MarginLeft(2)
	baseTrackListStyle  = lipgloss.NewStyle().PaddingRight(3)
	borderStyle         = lipgloss.RoundedBorder()
	actionBarFocusStyle = lipgloss.NewStyle().Margin(1, 0, 0, 0).Border(borderStyle).Padding(0, 1)
	actionBarBlurStyle  = lipgloss.NewStyle().Margin(1, 0, 0, 1).Padding(1, 1)
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
		key.WithHelp("left/right", "move horizontally"),
	),
	Right: key.NewBinding(
		key.WithKeys("right", "l"),
		key.WithHelp("", ""),
	),
	Up: key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("up/down", "move vertically"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("", ""),
	),
	Activate: key.NewBinding(
		key.WithKeys("enter", "space"),
		key.WithHelp("enter/space", "activate"),
	),
	FocusList: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "tracks"),
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
	Up         key.Binding
	Down       key.Binding
	Activate   key.Binding
	FocusList  key.Binding
	Duplicates key.Binding
}

type DownloadEndMsg struct{}
type downloadSessionStartedMsg struct {
	events <-chan DownloadSessionEvent
}

type DownloadProgressUpdateMsg struct {
	progress  TrackProgress
	completed bool
}

type DownloadModel struct {
	// External dependencies.
	client          *ya.Client
	downloadOptions ya.DownloadOptions

	// UI components.
	spinner   spinner.Model
	progress  progress.Model
	trackList list.Model

	// Download progress channel and tracking.
	sessionEvents  <-chan DownloadSessionEvent
	tracksProgress []*TrackProgress

	// Counters.
	tracksTotalCount  int
	downloadedCount   int
	downloadableCount int
	errorCount        int

	// UI state.
	isDownloading     bool
	shutdownRequested bool
	quitAfterCancel   bool
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
		progress.WithDefaultBlend(),
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

	return DownloadModel{
		client:            client,
		downloadOptions:   downloadOptionsOrDefault(options),
		spinner:           sp,
		progress:          p,
		trackList:         l,
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
	m.sessionEvents = nil
	m.tracksTotalCount = 0
	m.downloadedCount = 0
	m.downloadableCount = 0
	m.errorCount = 0
	m.isDownloading = false
	m.shutdownRequested = false
	m.quitAfterCancel = false
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

func (m *DownloadModel) Update(msg tea.Msg) (DownloadModel, tea.Cmd) {
	var cmds []tea.Cmd
	var cmd tea.Cmd

	if m.focusedView == viewList {
		m.trackList, cmd = m.trackList.Update(msg)
		cmds = append(cmds, cmd)
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.Resize(msg.Width, msg.Height)

	case tea.KeyPressMsg:
		previousFocus := m.focusedView
		switch {
		case key.Matches(msg, downloadKeys.Activate):
			*m, cmd = m.activateFocusedControl()

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

		case key.Matches(msg, downloadKeys.Down):
			if m.focusedView != viewList {
				m.focusDownAction()
			}

		case key.Matches(msg, downloadKeys.Up):
			if m.focusedView != viewList {
				m.focusUpAction()
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
		if previousFocus != m.focusedView {
			m.resizeToWindow()
		}

	case downloadSessionStartedMsg:
		m.sessionEvents = msg.events
		m.updateTrackList()
		cmd = nextDownloadSessionEvent(m.sessionEvents)

	case DownloadProgressUpdateMsg:
		if m.applyProgress(msg.progress) {
			if msg.completed {
				m.downloadedCount++
			}
			m.errorCount = countStatus(m.tracksProgress, TrackStatusError)
			m.updateTrackList()
		}
		cmd = nextDownloadSessionEvent(m.sessionEvents)

	case DownloadEndMsg:
		m.isDownloading = false
		if m.shutdownRequested {
			m.normalizeCanceledTracks()
			m.shutdownRequested = false
			if m.client != nil {
				m.client.ResetCancel()
			}
		}
		m.sessionEvents = nil
		if m.quitAfterCancel {
			m.quitAfterCancel = false
			return *m, tea.Quit
		}

	case ListSelectedItemMsg, list.FilterMatchesMsg:
		if item, ok := m.trackList.SelectedItem().(TrackListItem); ok {
			m.selectedTrackInfo = m.getTrackInfo(item.uid)
		}
	}

	cmds = append(cmds, cmd)
	return *m, tea.Batch(cmds...)
}

func (m DownloadModel) View() tea.View {
	return tea.NewView(m.render())
}

func (m *DownloadModel) Resize(width, height int) {
	m.windowWidth = width
	m.windowHeight = height
	m.resizeToWindow()
}

func (m DownloadModel) render() string {
	viewStr := m.headerBlock()
	viewStr += "\n" + m.renderTrackList()
	viewStr += "\n" + marginLeftStyle.Render(m.renderProgress())
	viewStr += renderActionBar(m)
	return viewStr
}

func (m *DownloadModel) resizeToWindow() {
	if m.windowWidth <= 0 || m.windowHeight <= 0 {
		return
	}

	contentWidth := responsiveWidth(m.windowWidth, downloadHorizontalChrome, 40)

	m.progress.SetWidth(contentWidth)
	m.trackList.SetWidth(contentWidth)
	m.trackList.SetHeight(m.availableTrackListHeight())
}

func (m DownloadModel) availableTrackListHeight() int {
	if m.windowHeight <= 0 {
		return defaultTrackListHeight
	}

	height := m.windowHeight - m.fixedDownloadHeight()
	if height < minTrackListHeight {
		return minTrackListHeight
	}
	return height
}

func (m DownloadModel) fixedDownloadHeight() int {
	return lipgloss.Height(m.headerBlock()) +
		1 +
		m.trackListStyle().GetVerticalFrameSize() +
		1 +
		lipgloss.Height(marginLeftStyle.Render(m.renderProgress())) +
		lipgloss.Height(renderActionBar(m))
}

func (m DownloadModel) headerBlock() string {
	header := renderHeader(m.downloadedCount, m.tracksTotalCount, m.downloadableCount, m.errorCount)
	return marginLeftStyle.Render(header) + "\n" + marginLeftStyle.Render(m.selectedTrackInfo)
}

func (m DownloadModel) trackListStyle() lipgloss.Style {
	trackListStyle := baseTrackListStyle
	if m.focusedView == viewList {
		return trackListStyle.Border(borderStyle)
	}
	return trackListStyle.Margin(1)
}

func (m DownloadModel) renderTrackList() string {
	content := m.trackList.View()
	if missingRows := m.trackList.Height() - lipgloss.Height(content); missingRows > 0 {
		content += strings.Repeat("\n ", missingRows)
	}
	return m.trackListStyle().Render(content)
}

func (m *DownloadModel) cycleFocus() {
	m.focusNext()
}

func (m DownloadModel) startDownloadSession() tea.Cmd {
	progress := make([]TrackProgress, 0, len(m.tracksProgress))
	for _, item := range m.tracksProgress {
		progress = append(progress, *item)
	}
	client := m.client
	logger := downloadLogger(client)
	options := m.downloadOptions

	return func() tea.Msg {
		session := NewDownloadSession(client, logger, options, outputDir)
		return downloadSessionStartedMsg{events: session.Run(progress)}
	}
}

func (m *DownloadModel) applyProgress(progress TrackProgress) bool {
	for _, current := range m.tracksProgress {
		if current.uid == progress.uid {
			*current = progress
			return true
		}
	}
	return false
}

func (m *DownloadModel) resetState() {
	for _, tp := range m.tracksProgress {
		switch tp.status {
		case TrackStatusDownloaded, TrackStatusAlreadyExists, TrackStatusDuplicate, TrackStatusNotAvailable:
			continue
		default:
			tp.status = TrackStatusReady
			tp.format = ""
		}
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
	return m.progress.ViewAs(percent)
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

func (m *DownloadModel) activateFocusedControl() (DownloadModel, tea.Cmd) {
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
			return *m, func() tea.Msg { return BackToURLMsg{} }
		}

	case viewDownloadButton:
		if m.isDownloading {
			return *m, nil
		}
		m.isDownloading = true
		m.resetState()
		m.focusedView = viewList

		utils.CreateDirIfNotExists(outputDir)
		return *m, m.startDownloadSession()

	case viewQuitButton:
		if m.isDownloading {
			m.requestShutdown("quit_button", false)
			return *m, nil
		}

		downloadLogger(m.client).Info("application quit requested",
			"reason", "quit_button",
			"is_downloading", false,
		)
		return *m, tea.Quit
	}

	return *m, nil
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

func (m *DownloadModel) focusDownAction() {
	m.focusVerticalAction(verticalTargetsBelow(m.focusedView))
}

func (m *DownloadModel) focusUpAction() {
	m.focusVerticalAction(verticalTargetsAbove(m.focusedView))
}

func (m *DownloadModel) focusVerticalAction(targets []focusable) {
	for _, target := range targets {
		if m.controlEnabled(target) {
			m.focusedView = target
			m.lastActionFocus = target
			return
		}
	}
}

func verticalTargetsBelow(control focusable) []focusable {
	switch control {
	case viewFormatMP3:
		return []focusable{viewBackButton, viewDownloadButton, viewQuitButton}
	case viewFormatFLAC:
		return []focusable{viewDownloadButton, viewQuitButton, viewBackButton}
	default:
		return nil
	}
}

func verticalTargetsAbove(control focusable) []focusable {
	switch control {
	case viewBackButton:
		return []focusable{viewFormatMP3, viewFormatFLAC}
	case viewDownloadButton, viewQuitButton:
		return []focusable{viewFormatFLAC, viewFormatMP3}
	default:
		return nil
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
	content := formatRow + "\n" + actionRow
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

func nextDownloadSessionEvent(events <-chan DownloadSessionEvent) tea.Cmd {
	return func() tea.Msg {
		event, ok := <-events
		if !ok {
			return DownloadEndMsg{}
		}
		return DownloadProgressUpdateMsg{
			progress:  event.Progress,
			completed: event.Completed,
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
	case ".m4a", ".mp4":
		return "M4A"
	default:
		return "MP3"
	}
}

func (m *DownloadModel) requestShutdown(reason string, quitAfterCancel bool) {
	if m.shutdownRequested {
		return
	}

	m.shutdownRequested = true
	m.quitAfterCancel = quitAfterCancel
	downloadLogger(m.client).Info("application quit requested",
		"reason", reason,
		"is_downloading", m.isDownloading,
	)

	if m.client != nil {
		m.client.Cancel()
	}
}

func (m *DownloadModel) normalizeCanceledTracks() {
	for _, tp := range m.tracksProgress {
		if tp.status == TrackStatusDownloaded || tp.status == TrackStatusAlreadyExists {
			continue
		}

		if tp.status == TrackStatusDownloading || isCanceledError(tp.errMsg) {
			tp.status = TrackStatusReady
			tp.errMsg = ""
			tp.filename = ""
			tp.format = ""
		}
	}

	m.downloadedCount = countStatus(m.tracksProgress, TrackStatusDownloaded)
	m.errorCount = countStatus(m.tracksProgress, TrackStatusError)
	m.downloadableCount = countStatus(m.tracksProgress, TrackStatusReady)
	m.tracksTotalCount = len(m.tracksProgress)
	m.updateTrackList()
}

func isCanceledError(errMsg string) bool {
	errMsg = strings.ToLower(strings.TrimSpace(errMsg))
	if errMsg == "" {
		return false
	}

	return strings.Contains(errMsg, "context canceled") ||
		strings.Contains(errMsg, "operation was canceled") ||
		strings.Contains(errMsg, "request canceled")
}
