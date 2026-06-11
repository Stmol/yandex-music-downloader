package ui

import (
	"fmt"

	"ya-music/utils"
	"ya-music/ya"
	"ya-music/ya/model"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type UiState int

const (
	UiStateTokenInput UiState = iota
	UiStateSelectSource
	UiStateDownloading
)

var (
	redForeground     = lipgloss.NewStyle().Foreground(lipgloss.Color("#CC0000"))
	whiteForeground   = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF"))
	greenForeground   = lipgloss.NewStyle().Foreground(lipgloss.Color("#006400"))
	orangeForeground  = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF8C00"))
	grayForeground    = lipgloss.NewStyle().Foreground(lipgloss.Color("#666666"))
	dimGrayForeground = lipgloss.NewStyle().Foreground(lipgloss.Color("#808080"))
	spinnerForeground = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	boldStyle         = lipgloss.NewStyle().Bold(true)
	boldRedStyle      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	spinnerStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
)

// BackToURLMsg is sent when the user chooses to leave the download screen and return to URL input.
type BackToURLMsg struct{}
type ShutdownRequestedMsg struct {
	Reason string
}

type Model struct {
	initState     UiState
	tokenModel    TokenModel
	sourceModel   SourceModel
	downloadModel DownloadModel
}

func (m Model) Init() tea.Cmd {
	return m.tokenModel.Init()
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	utils.NewLogger("").Debug(fmt.Sprintf("Update: %T - %v", msg, msg))

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		if msg.String() == "ctrl+c" {
			return m.handleShutdown("ctrl_c")
		}

	case BackToURLMsg:
		m.initState = UiStateSelectSource
		m.downloadModel.Reset()
		m.sourceModel.Reset()
		return m, m.sourceModel.Init()

	case TokenOkMsg:
		m.initState = UiStateSelectSource
		cmds = append(cmds, m.sourceModel.Init())

	case SourceSubmitMsg:
		m.initState = UiStateDownloading

		var tracks []model.Track
		if msg.Track != nil {
			tracks = append(tracks, *msg.Track)
		} else if msg.Album != nil {
			for _, volume := range msg.Album.Volumes {
				tracks = append(tracks, volume...)
			}
		} else if msg.Playlist != nil {
			for _, trackShort := range msg.Playlist.Tracks {
				tracks = append(tracks, trackShort.Track)
			}
		}
		m.downloadModel.AddTracks(tracks)

		cmds = append(cmds, m.downloadModel.Init())

	case ShutdownRequestedMsg:
		return m.handleShutdown(msg.Reason)
	}

	switch m.initState {
	case UiStateSelectSource:
		newSourceModel, newCmd := m.sourceModel.Update(msg)
		m.sourceModel = newSourceModel
		cmds = append(cmds, newCmd)

	case UiStateDownloading:
		newDownloadModel, newCmd := m.downloadModel.Update(msg)
		m.downloadModel = newDownloadModel
		cmds = append(cmds, newCmd)

	case UiStateTokenInput:
		newTokenModel, newCmd := m.tokenModel.Update(msg)
		m.tokenModel = newTokenModel
		cmds = append(cmds, newCmd)
	}

	return m, tea.Batch(cmds...)
}

func (m Model) View() tea.View {
	var content string

	switch m.initState {
	case UiStateDownloading:
		content = "\n\n" + m.downloadModel.render()
	case UiStateSelectSource:
		content = "\n\n" + marginLeftStyle.Render(m.sourceModel.render())
	case UiStateTokenInput:
		content = "\n\n" + marginLeftStyle.Render(m.tokenModel.render())
	}

	view := tea.NewView(content)
	view.AltScreen = true
	return view
}

func StartUi(client *ya.Client, options ...ya.DownloadOptions) Model {
	return Model{
		initState:     UiStateTokenInput,
		tokenModel:    NewTokenModel(client),
		sourceModel:   NewSourceModel(client),
		downloadModel: NewDownloadModel(client, downloadOptionsOrDefault(options)),
	}
}

func (m Model) handleShutdown(reason string) (tea.Model, tea.Cmd) {
	if m.downloadModel.isDownloading {
		m.downloadModel.requestShutdown(reason, true)
		return m, nil
	}

	downloadLogger(m.downloadModel.client).Info("application quit requested",
		"reason", reason,
		"is_downloading", false,
	)
	return m, tea.Quit
}
