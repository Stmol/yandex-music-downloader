package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
	"ya-music/ui"
	"ya-music/utils"
	"ya-music/ya"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	var downloadTimeoutSeconds int
	flag.IntVar(&downloadTimeoutSeconds, "timeout", 0, "download timeout in seconds (0 disables timeout)")
	flag.Parse()

	if downloadTimeoutSeconds < 0 {
		fmt.Fprintln(os.Stderr, "timeout must be >= 0 seconds")
		os.Exit(2)
	}

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

	httpClient := utils.NewHttpClientWithLogger(downloadLogger)
	httpClient.SetDownloadTimeout(time.Duration(downloadTimeoutSeconds) * time.Second)

	client := ya.NewClient(httpClient)
	prog := tea.NewProgram(ui.StartUi(client), tea.WithAltScreen())

	go func() {
		sig := <-sigCh
		prog.Send(ui.ShutdownRequestedMsg{
			Reason: "signal_" + strings.ToLower(sig.String()),
		})
	}()

	downloadLogger.Info("application started",
		"log_path", downloadLogger.Path(),
		"download_timeout_seconds", downloadTimeoutSeconds,
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
