package ui

import (
	"fmt"
	"regexp"
	"strings"
	"ya-music/ya"
	"ya-music/ya/model"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

var (
	trackPattern    = regexp.MustCompile(`^(?:https?://)?music\.yandex\.ru/album/(?P<albumId>\d+)/track/(?P<trackId>\d+)(?:\?.*)?$`)
	playlistPattern = regexp.MustCompile(`^(?:https?://)?music\.yandex\.ru/users/(?P<username>[^/]+)/playlists/(?P<playlistId>\d+)(?:\?.*)?$`)
)

type (
	URLSubmitTrackMsg struct {
		TrackID string
	}

	URLSubmitPlaylistMsg struct {
		PlaylistID string
		Username   string
	}

	SourceSubmitMsg struct {
		Playlist *model.Playlist
		Track    *model.Track
	}

	URLHandleErrorMsg string
)

type SourceModel struct {
	client       *ya.Client
	urlInput     textinput.Model
	errorMsg     string
	spinner      spinner.Model
	isProcessing bool
}

func NewSourceModel(client *ya.Client) SourceModel {
	urlInput := textinput.New()
	urlInput.Placeholder = "Enter URL"
	urlInput.CharLimit = 256
	urlInput.Width = 128
	urlInput.Focus()

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = spinnerStyle

	return SourceModel{
		client:   client,
		urlInput: urlInput,
		spinner:  s,
	}
}

func (m SourceModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m SourceModel) Update(msg tea.Msg) (SourceModel, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	m.urlInput, cmd = m.urlInput.Update(msg)
	cmds = append(cmds, cmd)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.Type == tea.KeyEnter && !m.isProcessing {
			return m.handleEnterKey()
		}

	case URLSubmitTrackMsg:
		m.isProcessing = true
		cmds = append(cmds, m.handleTrackURL(msg))

	case URLSubmitPlaylistMsg:
		m.isProcessing = true
		cmds = append(cmds, m.handlePlaylistURL(msg))

	case SourceSubmitMsg:
		m.isProcessing = false
		cmds = append(cmds, m.urlInput.Focus())

	case URLHandleErrorMsg:
		m.isProcessing = false
		m.errorMsg = fmt.Sprintf("Failed to get info: %s", string(msg))
		cmds = append(cmds, m.urlInput.Focus())

	case spinner.TickMsg:
		if m.isProcessing {
			m.spinner, cmd = m.spinner.Update(msg)
			cmds = append(cmds, cmd)
		}
	}

	return m, tea.Batch(cmds...)
}

func (m SourceModel) handleEnterKey() (SourceModel, tea.Cmd) {
	input := strings.TrimSpace(m.urlInput.Value())
	if msg := m.parseURL(input); msg != nil {
		m.isProcessing = true
		m.urlInput.Blur()
		return m, tea.Batch(
			func() tea.Msg { return msg },
			m.spinner.Tick,
		)
	}
	m.errorMsg = "Invalid URL"
	return m, nil
}

func (m *SourceModel) parseURL(input string) tea.Msg {
	if matches := trackPattern.FindStringSubmatch(input); matches != nil {
		return URLSubmitTrackMsg{TrackID: matches[2]}
	}
	if matches := playlistPattern.FindStringSubmatch(input); matches != nil {
		return URLSubmitPlaylistMsg{
			PlaylistID: matches[2],
			Username:   matches[1],
		}
	}
	return nil
}

func (m *SourceModel) handleTrackURL(msg URLSubmitTrackMsg) tea.Cmd {
	return func() tea.Msg {
		track, err := m.client.TrackInfo(msg.TrackID)
		if err != nil {
			return URLHandleErrorMsg(err.Error())
		}
		return SourceSubmitMsg{Track: track}
	}
}

func (m *SourceModel) handlePlaylistURL(msg URLSubmitPlaylistMsg) tea.Cmd {
	return func() tea.Msg {
		playlist, err := m.client.UsersPlaylist(msg.PlaylistID, msg.Username)
		if err != nil {
			return URLHandleErrorMsg(err.Error())
		}
		return SourceSubmitMsg{Playlist: playlist}
	}
}

func (m SourceModel) View() string {
	s := "What do you want to download?\n\n"
	s += dimGrayForeground.Render("Examples of URL:")
	s += dimGrayForeground.Render("\n- Track: https://music.yandex.ru/album/1231231/track/12312345")
	s += dimGrayForeground.Render("\n- Playlist: https://music.yandex.ru/users/username/playlists/12312311")
	s += "\n\n"
	s += m.urlInput.View()

	if m.isProcessing {
		s += "\n\n" + m.spinner.View() + " Loading..."
	}

	if m.errorMsg != "" {
		s += "\n\n" + redForeground.Render(m.errorMsg)
	}

	return s
}
