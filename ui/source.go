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
	"github.com/google/uuid"
)

var (
	yandexMusicHostPattern = `music\.yandex\.(?:ru|com|kz|by|uz)`
	trackPattern           = regexp.MustCompile(`^(?:https?://)?` + yandexMusicHostPattern + `/album/(?P<albumId>\d+)/track/(?P<trackId>\d+)(?:\?.*)?$`)
	albumPattern           = regexp.MustCompile(`^(?:https?://)?` + yandexMusicHostPattern + `/album/(?P<albumId>\d+)(?:\?.*)?$`)
	playlistPattern        = regexp.MustCompile(`^(?:https?://)?` + yandexMusicHostPattern + `/users/(?P<username>[^/]+)/playlists/(?P<playlistId>\d+)(?:\?.*)?$`)
	playlistUUIDPattern    = regexp.MustCompile(`^(?:https?://)?` + yandexMusicHostPattern + `/playlists/(?P<playlistUuid>(?:[a-z]{2}\.)?[0-9a-fA-F-]{36})(?:\?.*)?$`)
)

type sourceURLKind int

const (
	sourceURLTrack sourceURLKind = iota
	sourceURLAlbum
	sourceURLLegacyPlaylist
	sourceURLPlaylistUUID
)

type (
	URLSubmitMsg struct {
		kind         sourceURLKind
		TrackID      string
		AlbumID      string
		PlaylistID   string
		PlaylistUUID string
		Username     string
	}

	SourceSubmitMsg struct {
		Playlist *model.Playlist
		Album    *model.Album
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

// Reset clears the URL field and errors so the source screen is ready for a new link (e.g. after Back to URL).
func (m *SourceModel) Reset() {
	m.urlInput.SetValue("")
	m.errorMsg = ""
	m.isProcessing = false
	m.urlInput.Focus()
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

	case URLSubmitMsg:
		m.isProcessing = true
		cmds = append(cmds, m.handleURL(msg))

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
		return URLSubmitMsg{
			kind:    sourceURLTrack,
			TrackID: matches[2],
		}
	}
	if matches := albumPattern.FindStringSubmatch(input); matches != nil {
		return URLSubmitMsg{
			kind:    sourceURLAlbum,
			AlbumID: matches[1],
		}
	}
	if matches := playlistPattern.FindStringSubmatch(input); matches != nil {
		return URLSubmitMsg{
			kind:       sourceURLLegacyPlaylist,
			PlaylistID: matches[2],
			Username:   matches[1],
		}
	}
	if matches := playlistUUIDPattern.FindStringSubmatch(input); matches != nil {
		playlistID := matches[1]
		uuidPart := playlistID
		if prefix, rest, found := strings.Cut(playlistID, "."); found {
			if len(prefix) != 2 {
				return nil
			}
			uuidPart = rest
		}
		if _, err := uuid.Parse(uuidPart); err != nil {
			return nil
		}

		return URLSubmitMsg{
			kind:         sourceURLPlaylistUUID,
			PlaylistUUID: playlistID,
		}
	}
	return nil
}

func (m *SourceModel) handleURL(msg URLSubmitMsg) tea.Cmd {
	return func() tea.Msg {
		switch msg.kind {
		case sourceURLTrack:
			track, err := m.client.TrackInfo(msg.TrackID)
			if err != nil {
				return URLHandleErrorMsg(err.Error())
			}
			return SourceSubmitMsg{Track: track}
		case sourceURLAlbum:
			album, err := m.client.AlbumWithTracks(msg.AlbumID)
			if err != nil {
				return URLHandleErrorMsg(err.Error())
			}
			return SourceSubmitMsg{Album: album}
		case sourceURLLegacyPlaylist, sourceURLPlaylistUUID:
			playlist, err := m.fetchPlaylist(msg)
			if err != nil {
				return URLHandleErrorMsg(err.Error())
			}
			return SourceSubmitMsg{Playlist: playlist}
		default:
			return URLHandleErrorMsg("unsupported url type")
		}
	}
}

func (m *SourceModel) fetchPlaylist(msg URLSubmitMsg) (*model.Playlist, error) {
	switch msg.kind {
	case sourceURLLegacyPlaylist:
		return m.client.UsersPlaylist(msg.PlaylistID, msg.Username)
	case sourceURLPlaylistUUID:
		return m.client.PlaylistByUUID(msg.PlaylistUUID)
	default:
		return nil, fmt.Errorf("unsupported playlist url type")
	}
}

func (m SourceModel) View() string {
	s := "What do you want to download?\n\n"
	s += dimGrayForeground.Render("Examples of URL:")
	s += dimGrayForeground.Render("\n- Track: https://music.yandex.ru/album/1231231/track/12312345")
	s += dimGrayForeground.Render("\n- Album: https://music.yandex.ru/album/1231231")
	s += dimGrayForeground.Render("\n- Playlist: https://music.yandex.ru/playlists/4dc94b2f-e96b-2daf-a53c-ce71846901b3")
	s += dimGrayForeground.Render("\n- Legacy playlist: https://music.yandex.ru/users/username/playlists/12312311")
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
