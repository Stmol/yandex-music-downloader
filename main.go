package main

import (
	"os"
	"ya-music/ui"
	"ya-music/utils"
	"ya-music/ya"

	tea "github.com/charmbracelet/bubbletea"
)

var (
	client *ya.Client
	prog   *tea.Program
)

func init() {
	client = ya.NewClient(utils.NewHttpClient())
	ui := ui.StartUi(client)
	prog = tea.NewProgram(ui, tea.WithAltScreen())
}

func main() {
	if os.Getenv("DEBUG") != "" {
		utils.NewLogger("").CleanLogFile()
	}

	if _, err := prog.Run(); err != nil {
		os.Exit(1)
	}
}
