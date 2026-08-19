package ui

import (
	"fmt"
	"regexp"
	"strings"
	"ya-music/ya"
	"ya-music/ya/model"

	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/google/uuid"
)

var (
	yandexMusicHostPattern = `music\.yandex\.(?:ru|com|kz|by|uz)`
	trackPattern           = regexp.MustCompile(`^(?:https?://)?` + yandexMusicHostPattern + `/album/(?P<albumId>\d+)/track/(?P<trackId>\d+)(?:\?.*)?$`)
	albumPattern           = regexp.MustCompile(`^(?:https?://)?` + yandexMusicHostPattern + `/album/(?P<albumId>\d+)(?:\?.*)?$`)
	playlistPattern        = regexp.MustCompile(`^(?:https?://)?` + yandexMusicHostPattern + `/users/(?P<username>[^/]+)/playlists/(?P<playlistId>\d+)(?:\?.*)?$`)
	playlistUUIDPattern    = regexp.MustCompile(`^(?:https?://)?` + yandexMusicHostPattern + `/playlists/(?P<playlistUuid>(?:[a-z]{2}\.)?[0-9a-fA-F-]{36})(?:\?.*)?$`)
	chartPattern           = regexp.MustCompile(`^(?:https?://)?` + yandexMusicHostPattern + `/chart(?:/(?P<region>[a-z]+))?(?:\?.*)?$`)
)

type sourceURLKind int

const (
	sourceURLTrack sourceURLKind = iota
	sourceURLAlbum
	sourceURLLegacyPlaylist
	sourceURLPlaylistUUID
	sourceURLChart
)

type (
	URLSubmitMsg struct {
		kind         sourceURLKind
		TrackID      string
		AlbumID      string
		PlaylistID   string
		PlaylistUUID string
		Username     string
		Region       string
	}

	SourceSubmitMsg struct {
		Tracks []model.Track
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
	urlInput.SetWidth(minInputWidth)
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

func (m *SourceModel) Resize(width, height int) {
	m.urlInput.SetWidth(responsiveWidth(width, inputHorizontalChrome, minInputWidth))
}

func (m SourceModel) Init() tea.Cmd {
	return textinput.Blink
}

// Reset clears the URL field and errors so the source screen is ready for a new link (e.g. after Back to URL).
func (m *SourceModel) Reset() tea.Cmd {
	m.urlInput.SetValue("")
	m.errorMsg = ""
	m.isProcessing = false
	return m.urlInput.Focus()
}

func (m SourceModel) Update(msg tea.Msg) (SourceModel, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	m.urlInput, cmd = m.urlInput.Update(msg)
	cmds = append(cmds, cmd)

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		if msg.String() == "enter" && !m.isProcessing {
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
	msg := parseSourceURL(input)
	if msg == nil {
		return nil
	}
	return *msg
}

func parseSourceURL(input string) *URLSubmitMsg {
	if matches := trackPattern.FindStringSubmatch(input); matches != nil {
		return &URLSubmitMsg{
			kind:    sourceURLTrack,
			TrackID: matches[2],
		}
	}
	if matches := albumPattern.FindStringSubmatch(input); matches != nil {
		return &URLSubmitMsg{
			kind:    sourceURLAlbum,
			AlbumID: matches[1],
		}
	}
	if matches := playlistPattern.FindStringSubmatch(input); matches != nil {
		return &URLSubmitMsg{
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

		return &URLSubmitMsg{
			kind:         sourceURLPlaylistUUID,
			PlaylistUUID: playlistID,
		}
	}
	if matches := chartPattern.FindStringSubmatch(input); matches != nil {
		return &URLSubmitMsg{
			kind:   sourceURLChart,
			Region: matches[1],
		}
	}
	return nil
}

func (m *SourceModel) handleURL(msg URLSubmitMsg) tea.Cmd {
	return func() tea.Msg {
		tracks, err := resolveSourceTracks(m.client, msg)
		if err != nil {
			return URLHandleErrorMsg(err.Error())
		}
		return SourceSubmitMsg{Tracks: tracks}
	}
}

type sourceClient interface {
	TrackInfo(id string) (*model.Track, error)
	AlbumWithTracks(id string) (*model.Album, error)
	UsersPlaylist(id string, username string) (*model.Playlist, error)
	PlaylistByUUID(id string) (*model.Playlist, error)
	Chart(region string) (*model.Playlist, error)
}

// ResolveSourceTracks turns a supported Yandex Music URL into its tracks.
// Both the terminal UI and the non-interactive command use this resolver.
func ResolveSourceTracks(client sourceClient, input string) ([]model.Track, error) {
	msg := parseSourceURL(strings.TrimSpace(input))
	if msg == nil {
		return nil, fmt.Errorf("invalid URL")
	}
	return resolveSourceTracks(client, *msg)
}

func resolveSourceTracks(client sourceClient, msg URLSubmitMsg) ([]model.Track, error) {
	switch msg.kind {
	case sourceURLTrack:
		track, err := client.TrackInfo(msg.TrackID)
		if err != nil {
			return nil, err
		}
		return []model.Track{*track}, nil
	case sourceURLAlbum:
		album, err := client.AlbumWithTracks(msg.AlbumID)
		if err != nil {
			return nil, err
		}
		var tracks []model.Track
		for _, volume := range album.Volumes {
			tracks = append(tracks, volume...)
		}
		return tracks, nil
	case sourceURLLegacyPlaylist:
		playlist, err := client.UsersPlaylist(msg.PlaylistID, msg.Username)
		if err != nil {
			return nil, err
		}
		return playlistTracks(playlist), nil
	case sourceURLPlaylistUUID:
		playlist, err := client.PlaylistByUUID(msg.PlaylistUUID)
		if err != nil {
			return nil, err
		}
		return playlistTracks(playlist), nil
	case sourceURLChart:
		playlist, err := client.Chart(msg.Region)
		if err != nil {
			return nil, err
		}
		return playlistTracks(playlist), nil
	default:
		return nil, fmt.Errorf("unsupported URL type")
	}
}

func playlistTracks(playlist *model.Playlist) []model.Track {
	tracks := make([]model.Track, 0, len(playlist.Tracks))
	for _, short := range playlist.Tracks {
		tracks = append(tracks, short.Track)
	}
	return tracks
}

func (m SourceModel) View() tea.View {
	return tea.NewView(m.render())
}

func (m SourceModel) render() string {
	s := "What do you want to download?\n\n"
	s += dimGrayForeground.Render("Examples of URL:")
	s += dimGrayForeground.Render("\n- Track: https://music.yandex.ru/album/1231231/track/12312345")
	s += dimGrayForeground.Render("\n- Album: https://music.yandex.ru/album/1231231")
	s += dimGrayForeground.Render("\n- Playlist: https://music.yandex.ru/playlists/4dc94b2f-e96b-2daf-a53c-ce71846901b3")
	s += dimGrayForeground.Render("\n- Legacy playlist: https://music.yandex.ru/users/username/playlists/12312311")
	s += dimGrayForeground.Render("\n- Chart: https://music.yandex.ru/chart (or /chart/world)")
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
