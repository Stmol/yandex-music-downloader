package ui

import (
	"fmt"
	"strings"
	"ya-music/source"
	"ya-music/ya"
	"ya-music/ya/model"

	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
)

type (
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

	case source.Ref:
		m.isProcessing = true
		cmds = append(cmds, m.handleURL(&msg))

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
	ref, err := source.Parse(input)
	if err != nil {
		m.errorMsg = "Invalid URL"
		return m, nil
	}
	m.isProcessing = true
	m.urlInput.Blur()
	return m, tea.Batch(
		func() tea.Msg { return *ref },
		m.spinner.Tick,
	)
}

func (m *SourceModel) handleURL(ref *source.Ref) tea.Cmd {
	return func() tea.Msg {
		tracks, err := source.ResolveRef(m.client, ref)
		if err != nil {
			return URLHandleErrorMsg(err.Error())
		}
		return SourceSubmitMsg{Tracks: tracks}
	}
}

func (m SourceModel) View() tea.View {
	return tea.NewView(m.render())
}

func (m SourceModel) render() string {
	s := renderScreenHeader("SOURCE", "Paste a Yandex Music link") + "\n\n"
	s += sectionLabelStyle.Render("SUPPORTED SOURCES") + "\n"
	s += dimGrayForeground.Render("Track  ·  Album  ·  Playlist  ·  Chart") + "\n\n"
	s += sectionLabelStyle.Render("URL EXAMPLES") + "\n"
	s += dimGrayForeground.Render(strings.Join([]string{
		"· Track: https://music.yandex.ru/album/1231231/track/12312345",
		"· Album: https://music.yandex.ru/album/1231231",
		"· Playlist: https://music.yandex.ru/playlists/4dc94b2f-e96b-2daf-a53c-ce71846901b3",
		"· Legacy playlist: https://music.yandex.ru/users/username/playlists/12312311",
		"· Chart: https://music.yandex.ru/chart (or /chart/world)",
	}, "\n")) + "\n\n"
	s += m.urlInput.View()

	if m.isProcessing {
		s += "\n\n" + m.spinner.View() + " Loading..."
	}

	if m.errorMsg != "" {
		s += "\n\n" + redForeground.Render(m.errorMsg)
	}

	return s
}
