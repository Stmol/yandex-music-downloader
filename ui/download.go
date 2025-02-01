package ui

import (
	"fmt"
	"slices"
	"sort"
	"strings"
	"sync"
	"ya-music/utils"
	"ya-music/ya"
	"ya-music/ya/model"

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
)

// Global style variables.
var (
	marginLeftStyle    = lipgloss.NewStyle().MarginLeft(2)
	baseTrackListStyle = lipgloss.NewStyle().PaddingRight(3)
	borderStyle        = lipgloss.RoundedBorder()
	buttonBaseStyle    = lipgloss.NewStyle().Margin(0, 1)
	focusedButtonStyle = buttonBaseStyle.Background(lipgloss.Color("#4A0549")).Foreground(lipgloss.Color("#FFFFFF"))
)

// Focusable represents which view element is currently focused.
type focusable int

// UI view constants.
const (
	viewList focusable = iota
	viewDownloadButton
	viewQuitButton
)

// TrackProgress represents the download progress and state of a track.
type TrackProgress struct {
	uid      string
	track    *model.Track
	status   TrackStatus
	errMsg   string
	filename string
}

type DownloadStartMsg struct{}
type DownloadEndMsg struct{}
type DownloadProgressUpdateMsg struct {
	downloaded bool
}

type DownloadModel struct {
	// External dependencies.
	client *ya.Client

	// UI components.
	spinner   spinner.Model
	progress  progress.Model
	trackList list.Model

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
	focusedView       focusable
	selectedTrackInfo string
	hideDuplicates    bool
}

func NewDownloadModel(client *ya.Client) DownloadModel {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = spinnerStyle

	p := progress.New(
		progress.WithDefaultGradient(),
		progress.WithWidth(75),
		progress.WithoutPercentage(),
	)

	l := list.New([]list.Item{}, TrackListItem{}, 60, 30)
	l.DisableQuitKeybindings()
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.AdditionalShortHelpKeys = func() []key.Binding {
		return []key.Binding{
			key.NewBinding(key.WithKeys("t"), key.WithHelp("t", "Toggle duplicates")),
		}
	}
	l.AdditionalFullHelpKeys = func() []key.Binding {
		return []key.Binding{
			key.NewBinding(key.WithKeys("t", "T"), key.WithHelp("t / T", "Toggle duplicates")),
		}
	}

	return DownloadModel{
		client:            client,
		spinner:           sp,
		progress:          p,
		trackList:         l,
		tracksProgress:    []*TrackProgress{},
		focusedView:       viewList,
		hideDuplicates:    false,
		selectedTrackInfo: "",
	}
}

func (m DownloadModel) Init() tea.Cmd {
	return nil
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
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			if m.focusedView == viewQuitButton {
				return m, tea.Quit
			}
			if m.isDownloading || m.focusedView == viewList {
				break
			}
			m.isDownloading = true
			m.resetState()
			m.focusedView = viewList
			m.tpUpdateCh = make(chan TrackProgress)

			utils.CreateDirIfNotExists(outputDir)

			cmd = m.downloadTracks(m.tpUpdateCh, m.tracksProgress)

		case "tab":
			m.cycleFocus()

		case "t", "T":
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
	viewStr := marginLeftStyle.Render(header + m.renderProgress())
	viewStr += "\n" + marginLeftStyle.Render(m.selectedTrackInfo) + "\n"

	trackListStyle := baseTrackListStyle
	if m.focusedView == viewList {
		trackListStyle = trackListStyle.Border(borderStyle)
	} else {
		trackListStyle = trackListStyle.Margin(1)
	}
	viewStr += trackListStyle.Render(m.trackList.View()) + "\n\n"
	viewStr += renderButtons(m)
	return viewStr
}

func (m *DownloadModel) cycleFocus() {
	switch m.focusedView {
	case viewList:
		m.focusedView = viewDownloadButton
	case viewDownloadButton:
		m.focusedView = viewQuitButton
	case viewQuitButton:
		m.focusedView = viewList
	}
}

func (m *DownloadModel) downloadTracks(updCh chan TrackProgress, progressList []*TrackProgress) tea.Cmd {
	return func() tea.Msg {
		var wg sync.WaitGroup
		sem := make(chan struct{}, maxConcurrentDownloads)

		skipStatuses := []TrackStatus{
			TrackStatusDownloading,
			TrackStatusDuplicate,
			TrackStatusNotAvailable,
		}

		for _, tp := range progressList {
			if slices.Contains(skipStatuses, tp.status) {
				continue
			}

			wg.Add(1)
			go m.downloadTrack(tp, &wg, sem, updCh)
		}

		go func() {
			wg.Wait()
			close(updCh)
		}()

		return DownloadStartMsg{}
	}
}

func (m *DownloadModel) downloadTrack(tp *TrackProgress, wg *sync.WaitGroup, sem chan struct{}, updCh chan TrackProgress) {
	defer wg.Done()
	sem <- struct{}{}
	defer func() { <-sem }()

	tp.status = TrackStatusDownloading
	updCh <- *tp

	filePath, err := m.client.DownloadTrack(*tp.track, outputDir)
	if err != nil {
		tp.status = TrackStatusError
		tp.errMsg = err.Error()
		tp.filename = filePath

		if filePath != "" {
			tp.status = TrackStatusAlreadyExists
		}
	} else {
		tp.status = TrackStatusDownloaded
		tp.filename = filePath
		tp.errMsg = ""
	}

	updCh <- *tp
}

func (m *DownloadModel) resetState() {
	for _, tp := range m.tracksProgress {
		if tp.status == TrackStatusDuplicate || tp.status == TrackStatusNotAvailable {
			continue
		}
		tp.status = TrackStatusReady
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
	return fmt.Sprintf("Total tracks: %d\nTo download: %d\nCompleted: %d\nErrors: %d\n\n",
		total, downloadable, completed, errors)
}

func renderButtons(m DownloadModel) string {
	downloadBtnStyle := buttonBaseStyle
	quitBtnStyle := buttonBaseStyle

	if m.focusedView == viewDownloadButton {
		downloadBtnStyle = focusedButtonStyle
	}
	if m.focusedView == viewQuitButton {
		quitBtnStyle = focusedButtonStyle
	}

	return downloadBtnStyle.Render("[  Download  ]") + "  " + quitBtnStyle.Render("[  Quit  ]")
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
