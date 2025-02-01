package ui

import (
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
	"ya-music/ya/model"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	itemStyle         = lipgloss.NewStyle().PaddingLeft(4)
	selectedItemStyle = lipgloss.NewStyle().Background(lipgloss.Color("170"))
	emptyItemStyle    = lipgloss.NewStyle()

	// Unselected styles
	trackNumberStyle = lipgloss.NewStyle().PaddingLeft(4)
	descriptionStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

	// Selected styles
	selectedTrackNumberStyle      = lipgloss.NewStyle().PaddingLeft(2).Background(lipgloss.Color("170"))
	selectedTrackDescriptionStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Background(lipgloss.Color("170"))

	// Status styles for unselected items
	readyStatusStyle         = greenForeground
	duplicateStatusStyle     = grayForeground
	downloadedStatusStyle    = greenForeground
	errorStatusStyle         = redForeground
	notAvailableStatusStyle  = redForeground
	unknownStatusStyle       = dimGrayForeground
	alreadyExistsStatusStyle = greenForeground
)

type TrackStatus int

const (
	TrackStatusReady TrackStatus = iota
	TrackStatusDownloaded
	TrackStatusDownloading
	TrackStatusError
	TrackStatusNotAvailable
	TrackStatusDuplicate
	TrackStatusAlreadyExists
)

func (t TrackStatus) String() string {
	switch t {
	case TrackStatusDuplicate:
		return "Duplicate"
	case TrackStatusDownloading:
		return "Downloading..."
	case TrackStatusDownloaded:
		return "✅"
	case TrackStatusError:
		return "Error"
	case TrackStatusReady:
		return "Ready"
	case TrackStatusNotAvailable:
		return "Not Available"
	case TrackStatusAlreadyExists:
		return "Already Exists"
	default:
		return "Unknown"
	}
}

type ListSelectedItemMsg string
type ListHasFocusMsg struct{}
type ListLostFocusMsg struct{}

type TrackListItem struct {
	uid    string
	track  *model.Track
	status TrackStatus
}

func (t TrackListItem) FilterValue() string {
	return t.track.FullTitle()
}

func (t TrackListItem) Title() string {
	return t.track.FullTitle()
}

func (t TrackListItem) Description() string {
	return t.track.ArtistsString()
}

func (t TrackListItem) Height() int  { return 1 }
func (t TrackListItem) Spacing() int { return 0 }
func (t TrackListItem) Update(msg tea.Msg, m *list.Model) tea.Cmd {
	if m.SelectedItem() == nil {
		return nil
	}

	currItem := m.SelectedItem().(TrackListItem)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k", "down", "j", "left", "right", "l", "h", "g", "G", "end", "home", "pgup", "pgdn":
			return func() tea.Msg {
				return ListSelectedItemMsg(currItem.uid)
			}
		}
	}

	return nil
}

func (t TrackListItem) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	item, ok := listItem.(TrackListItem)
	if !ok {
		return
	}

	title := item.Title()
	desc := item.Description()
	maxLen := 50
	maxDescLen := 10
	dots := "..."

	combined := fmt.Sprintf("%s %s", title, desc)
	if utf8.RuneCountInString(combined) > maxLen {
		availableSpace := maxLen - utf8.RuneCountInString(title) - 1 - utf8.RuneCountInString(dots) // 1 for space
		if availableSpace >= maxDescLen {
			descRunes := []rune(desc)
			desc = string(descRunes[:availableSpace])
			combined = fmt.Sprintf("%s %s%s", title, desc, dots)
		} else {
			titleRunes := []rune(title)
			descRunes := []rune(desc)
			title = string(titleRunes[:maxLen-maxDescLen-1-utf8.RuneCountInString(dots)]) + dots // Add dots to the truncated title
			if utf8.RuneCountInString(desc) > maxDescLen {
				desc = string(descRunes[:maxDescLen-utf8.RuneCountInString(dots)]) + dots
			}
			combined = fmt.Sprintf("%s %s", title, desc)
		}
	}

	titleRunes := []rune(title)
	titlePart := string([]rune(combined)[:len(titleRunes)+1])
	descPart := string([]rune(combined)[len(titleRunes)+1:])
	padding := strings.Repeat(" ", maxLen-utf8.RuneCountInString(combined)+2)
	statusStr := fmt.Sprintf("%-15s", item.status.String())

	switch item.status {
	case TrackStatusDuplicate:
		statusStr = duplicateStatusStyle.Render(statusStr)
	case TrackStatusDownloaded:
		statusStr = downloadedStatusStyle.Render(statusStr)
	case TrackStatusError:
		statusStr = errorStatusStyle.Render(statusStr)
	case TrackStatusNotAvailable:
		statusStr = notAvailableStatusStyle.Render(statusStr)
	case TrackStatusAlreadyExists:
		statusStr = alreadyExistsStatusStyle.Render(statusStr)
	case TrackStatusReady:
		statusStr = readyStatusStyle.Render(statusStr)
	}

	isSelected := index == m.Index()

	trackNumber := fmt.Sprintf("%02d. ", index+1)
	trackNumberStyleToUse := trackNumberStyle
	if isSelected {
		trackNumberStyleToUse = selectedTrackNumberStyle
		trackNumber = "> " + trackNumber
	}

	titleStyleToUse := emptyItemStyle
	paddingStyleToUse := emptyItemStyle
	descStyleToUse := descriptionStyle

	if isSelected {
		titleStyleToUse = selectedItemStyle
		paddingStyleToUse = selectedItemStyle
		descStyleToUse = selectedTrackDescriptionStyle
	}

	if isSelected {
		statusStr = fmt.Sprintf("%-15s", item.status.String())
		statusStr = selectedItemStyle.Render(statusStr)
	}

	str := fmt.Sprintf("%s%s%s%s%s",
		trackNumberStyleToUse.Render(trackNumber),
		titleStyleToUse.Render(titlePart),
		descStyleToUse.Render(descPart),
		paddingStyleToUse.Render(padding),
		statusStr,
	)

	fmt.Fprint(w, str)
}
