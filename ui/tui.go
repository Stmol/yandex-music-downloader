package ui

import (
	"fmt"

	"ya-music/utils"
	"ya-music/ya"
	"ya-music/ya/model"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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
	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC {
			return m, tea.Quit
		}

	case TokenOkMsg:
		m.initState = UiStateSelectSource
		cmds = append(cmds, m.sourceModel.Init())

	case SourceSubmitMsg:
		m.initState = UiStateDownloading

		var tracks []model.Track
		if msg.Track != nil {
			tracks = append(tracks, *msg.Track)
		} else if msg.Playlist != nil {
			for _, trackShort := range msg.Playlist.Tracks {
				tracks = append(tracks, trackShort.Track)
			}
		}
		m.downloadModel.AddTracks(tracks)

		cmds = append(cmds, m.downloadModel.Init())
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

func (m Model) View() string {
	switch m.initState {
	case UiStateDownloading:
		return "\n\n" + m.downloadModel.View()
	case UiStateSelectSource:
		return "\n\n" + marginLeftStyle.Render(m.sourceModel.View())
	case UiStateTokenInput:
		return "\n\n" + marginLeftStyle.Render(m.tokenModel.View())
	}

	return ""
}

func StartUi(client *ya.Client) Model {
	return Model{
		initState:     UiStateTokenInput,
		tokenModel:    NewTokenModel(client),
		sourceModel:   NewSourceModel(client),
		downloadModel: NewDownloadModel(client),
	}
}
