package ui

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"ya-music/ya/model"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/mattn/go-runewidth"
)

var (
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

type TrackListItem struct {
	uid    string
	track  *model.Track
	status TrackStatus
	format string
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
	case tea.KeyPressMsg:
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

	statusPlain := formatStatusColumn(item.statusLabel())
	statusStr := statusPlain

	switch item.status {
	case TrackStatusDuplicate:
		statusStr = duplicateStatusStyle.Render(statusPlain)
	case TrackStatusDownloaded:
		statusStr = downloadedStatusStyle.Render(statusPlain)
	case TrackStatusError:
		statusStr = errorStatusStyle.Render(statusPlain)
	case TrackStatusNotAvailable:
		statusStr = notAvailableStatusStyle.Render(statusPlain)
	case TrackStatusAlreadyExists:
		statusStr = alreadyExistsStatusStyle.Render(statusPlain)
	case TrackStatusReady:
		statusStr = readyStatusStyle.Render(statusPlain)
	}

	isSelected := index == m.Index()

	trackNumber := formatTrackNumber(index, len(m.Items()))
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
		statusStr = selectedItemStyle.Render(statusPlain)
	}

	trackNumberRendered := trackNumberStyleToUse.Render(trackNumber)
	numberWidth := lipgloss.Width(trackNumberRendered)
	statusWidth := trackStatusColumnWidth

	textWidth := m.Width() - numberWidth - statusWidth
	if textWidth < 0 {
		textWidth = 0
	}

	combined := trackText(title, desc)
	displayText := truncateToWidth(combined, textWidth)
	titlePart, descPart := splitTrackText(displayText, title)
	paddingWidth := m.Width() - numberWidth - lipgloss.Width(displayText) - statusWidth
	if paddingWidth < 0 {
		paddingWidth = 0
	}
	padding := strings.Repeat(" ", paddingWidth)

	str := fmt.Sprintf("%s%s%s%s%s",
		trackNumberRendered,
		titleStyleToUse.Render(titlePart),
		descStyleToUse.Render(descPart),
		paddingStyleToUse.Render(padding),
		statusStr,
	)

	fmt.Fprint(w, str)
}

const trackStatusColumnWidth = 15

func formatStatusColumn(label string) string {
	return padToWidth(truncateToWidth(label, trackStatusColumnWidth), trackStatusColumnWidth)
}

func padToWidth(s string, width int) string {
	current := runewidth.StringWidth(s)
	if current >= width {
		return s
	}
	return s + strings.Repeat(" ", width-current)
}

func trackText(title, desc string) string {
	if desc == "" {
		return title
	}
	return fmt.Sprintf("%s %s", title, desc)
}

func splitTrackText(displayText, title string) (string, string) {
	titleWidth := runewidth.StringWidth(title)
	currentWidth := 0

	for i, r := range displayText {
		if currentWidth >= titleWidth && r == ' ' {
			return displayText[:i+1], displayText[i+1:]
		}

		currentWidth += runewidth.RuneWidth(r)
	}

	return displayText, ""
}

func truncateToWidth(s string, width int) string {
	if runewidth.StringWidth(s) <= width {
		return s
	}

	dots := "..."
	dotsWidth := runewidth.StringWidth(dots)
	if width <= dotsWidth {
		return strings.Repeat(".", width)
	}

	var b strings.Builder
	currentWidth := 0
	for _, r := range s {
		runeWidth := runewidth.RuneWidth(r)
		if currentWidth+runeWidth > width-dotsWidth {
			break
		}
		b.WriteRune(r)
		currentWidth += runeWidth
	}
	b.WriteString(dots)
	return b.String()
}

func (t TrackListItem) statusLabel() string {
	if t.status != TrackStatusDownloaded {
		return t.status.String()
	}
	format := strings.TrimSpace(t.format)
	if format == "" {
		format = "MP3"
	}
	return t.status.String() + " " + format
}

func formatTrackNumber(index int, totalItems int) string {
	width := len(strconv.Itoa(totalItems))
	if width <= 2 {
		return fmt.Sprintf("%02d. ", index+1)
	}

	return fmt.Sprintf("%*d. ", width, index+1)
}
