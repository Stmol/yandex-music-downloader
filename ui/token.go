package ui

import (
	"fmt"
	"ya-music/utils"
	"ya-music/ya"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type TokenModel struct {
	client          *ya.Client      // Yandex client for token validation
	inputField      textinput.Model // Text input field for token entry
	loadingSpinner  spinner.Model   // Spinner for loading indication
	errorMessage    string          // Error message for invalid token
	currentToken    string          // Currently entered or read token
	isCheckingToken bool            // Flag indicating token validation in progress
	confirmSave     bool            // Flag to confirm token saving
	tokenFromFile   bool            // Flag indicating token was read from file
	displayInput    bool            // Flag to display the input field
}

type (
	TokenCorrectMsg      string
	TokenIncorrectMsg    string
	TokenOkMsg           struct{}
	TokenReadFromFileMsg struct {
		Token string
		Err   error
	}
)

func NewTokenModel(client *ya.Client) TokenModel {
	input := textinput.New()
	input.Placeholder = "Enter your token..."
	input.Width = 50
	input.Focus()

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = spinnerStyle

	return TokenModel{
		client:          client,
		inputField:      input,
		loadingSpinner:  sp,
		displayInput:    false,
		isCheckingToken: true,
	}
}

func (m TokenModel) Init() tea.Cmd {
	return tea.Batch(
		m.loadingSpinner.Tick,
		m.readTokenFromFile(),
	)
}

func (m TokenModel) Update(msg tea.Msg) (TokenModel, tea.Cmd) {
	var (
		cmd      tea.Cmd
		fieldCmd tea.Cmd
	)

	m.inputField, fieldCmd = m.inputField.Update(msg)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		m, cmd = m.handleKeyMsg(msg)
	case TokenIncorrectMsg:
		m, cmd = m.handleTokenIncorrectMsg(msg)
	case TokenCorrectMsg:
		m, cmd = m.handleTokenCorrectMsg(msg)
	case TokenReadFromFileMsg:
		m, cmd = m.handleTokenReadFromFileMsg(msg)
	case spinner.TickMsg:
		m, cmd = m.handleSpinnerTickMsg(msg)
	}

	return m, tea.Batch(
		cmd,
		fieldCmd,
	)
}

func (m TokenModel) View() string {
	switch {
	case m.isCheckingToken:
		return fmt.Sprintf("%s Checking token...", m.loadingSpinner.View())
	case m.confirmSave:
		return m.renderSaveConfirmation()
	case m.displayInput:
		return m.renderInputField()
	default:
		return ""
	}
}

func (m TokenModel) checkToken(token string) tea.Cmd {
	return func() tea.Msg {
		m.client.SetToken(token)
		acc, err := m.client.AccountStatus()
		if err != nil || acc.Uid == 0 {
			m.client.SetToken("")
			return TokenIncorrectMsg("Invalid token")
		}
		return TokenCorrectMsg(token)
	}
}

func (m TokenModel) saveTokenToFile(token string) tea.Cmd {
	return func() tea.Msg {
		if err := utils.SaveTokenToFile(token); err != nil {
			return TokenIncorrectMsg("Failed to save token")
		}
		return TokenOkMsg{}
	}
}

func (m TokenModel) readTokenFromFile() tea.Cmd {
	return func() tea.Msg {
		token, err := utils.ReadTokenFromFile()
		return TokenReadFromFileMsg{
			Token: token,
			Err:   err,
		}
	}
}

func (m TokenModel) handleKeyMsg(msg tea.KeyMsg) (TokenModel, tea.Cmd) {
	if m.confirmSave {
		switch msg.String() {
		case "y", "Y":
			return m, m.saveTokenToFile(m.currentToken)
		case "n", "N":
			m.confirmSave = false
			return m, func() tea.Msg { return TokenOkMsg{} }
		}
	} else if msg.Type == tea.KeyEnter && !m.isCheckingToken {
		m.isCheckingToken = true
		m.tokenFromFile = false
		m.inputField.Blur()

		if m.inputField.Value() == "" {
			return m, func() tea.Msg { return TokenCorrectMsg("") }
		}

		return m, tea.Batch(
			m.loadingSpinner.Tick,
			m.checkToken(m.inputField.Value()),
		)
	}
	return m, nil
}

func (m TokenModel) handleTokenIncorrectMsg(msg TokenIncorrectMsg) (TokenModel, tea.Cmd) {
	m.isCheckingToken = false
	m.errorMessage = string(msg)
	if m.tokenFromFile {
		m.errorMessage = "Token from file is invalid"
	}
	m.displayInput = true
	return m, m.inputField.Focus()
}

func (m TokenModel) handleTokenCorrectMsg(msg TokenCorrectMsg) (TokenModel, tea.Cmd) {
	m.errorMessage = ""
	m.isCheckingToken = false
	m.currentToken = string(msg)

	if m.tokenFromFile || m.currentToken == "" {
		return m, func() tea.Msg { return TokenOkMsg{} }
	}

	m.confirmSave = true
	return m, nil
}

func (m TokenModel) handleTokenReadFromFileMsg(msg TokenReadFromFileMsg) (TokenModel, tea.Cmd) {
	if msg.Err != nil {
		m.isCheckingToken = false
		m.displayInput = true
		return m, m.inputField.Focus()
	}

	m.tokenFromFile = true
	m.currentToken = msg.Token
	return m, tea.Batch(
		m.loadingSpinner.Tick,
		m.checkToken(msg.Token),
	)
}

func (m TokenModel) handleSpinnerTickMsg(msg spinner.TickMsg) (TokenModel, tea.Cmd) {
	if m.isCheckingToken {
		var spinnerCmd tea.Cmd
		m.loadingSpinner, spinnerCmd = m.loadingSpinner.Update(msg)
		return m, spinnerCmd
	}
	return m, nil
}

func (m TokenModel) renderSaveConfirmation() string {
	fileName := boldStyle.Render(utils.TokenFileName)
	yesOption := boldRedStyle.Render("Y")
	noOption := boldRedStyle.Render("N")

	token := m.currentToken
	tokenLen := len(token)
	tokenDisplay := token
	if tokenLen > 10 {
		tokenDisplay = token[:5] + "....." + token[tokenLen-5:]
	}
	tokenDisplay = boldStyle.Render(tokenDisplay)

	return fmt.Sprintf("\n\nValid token: %s\n\nSave token to %s for future use? (%s)es/(%s)o",
		tokenDisplay, fileName, yesOption, noOption)
}

func (m TokenModel) renderInputField() string {
	s := "Please enter your Yandex Music OAuth token:\n\n"
	s += m.inputField.View()

	if m.errorMessage != "" {
		s += "\n\n" + redForeground.Render(m.errorMessage)
	}

	s += "\n\n" + dimGrayForeground.Render("You can leave it empty but some features may not work")

	return s
}
