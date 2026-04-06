package main

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"ya-music/ui"
	"ya-music/utils"
	"ya-music/ya"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	downloadLogger, err := utils.NewDownloadLogger(utils.DefaultDownloadLogPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize download logger: %v\n", err)
		downloadLogger = utils.NewDiscardDownloadLogger()
	}
	defer downloadLogger.Close()

	if err := downloadLogger.Reset(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to reset download log file: %v\n", err)
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	client := ya.NewClient(utils.NewHttpClientWithLogger(downloadLogger))
	prog := tea.NewProgram(ui.StartUi(client), tea.WithAltScreen())

	go func() {
		sig := <-sigCh
		prog.Send(ui.ShutdownRequestedMsg{
			Reason: "signal_" + strings.ToLower(sig.String()),
		})
	}()

	downloadLogger.Info("application started",
		"log_path", downloadLogger.Path(),
	)

	if os.Getenv("DEBUG") != "" {
		utils.NewLogger("").CleanLogFile()
	}

	if _, err := prog.Run(); err != nil {
		downloadLogger.Error("application terminated with error",
			"error", err,
		)
		os.Exit(1)
	}

	downloadLogger.Info("application stopped")
}
