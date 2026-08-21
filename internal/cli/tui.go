package cli

import (
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"ya-music/ui"
	"ya-music/utils"
	"ya-music/ya"

	tea "charm.land/bubbletea/v2"
)

func runTUI(args []string, stderr io.Writer) int {
	parsed := parseTUIOptions(args, stderr)
	if !parsed.proceed {
		return parsed.exitCode
	}
	options := parsed.options

	if isKnownProblematicTerm(os.Getenv("TERM")) {
		fmt.Fprintln(stderr, problematicTermWarning(os.Getenv("TERM")))
		return 2
	}

	downloadLogger, client := newLoggedClient(options.timeoutSeconds, stderr)
	defer downloadLogger.Close()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	downloadOptions := ya.DownloadOptions{SkipCover: options.skipCover}
	prog := tea.NewProgram(ui.StartUi(client, downloadOptions))

	go func() {
		sig := <-sigCh
		prog.Send(ui.ShutdownRequestedMsg{Reason: "signal_" + strings.ToLower(sig.String())})
	}()

	downloadLogger.Info("application started",
		"log_path", downloadLogger.Path(),
		"download_timeout_seconds", options.timeoutSeconds,
		"skip_cover", options.skipCover,
	)

	if os.Getenv("DEBUG") != "" {
		utils.NewLogger("").CleanLogFile()
	}

	if _, err := prog.Run(); err != nil {
		downloadLogger.Error("application terminated with error", "error", err)
		return 1
	}

	downloadLogger.Info("application stopped")
	return 0
}

func isKnownProblematicTerm(term string) bool {
	return term == "xterm"
}

func problematicTermWarning(term string) string {
	return fmt.Sprintf(`Warning: TERM=%s is known to break this terminal UI.

Colors, selected row highlighting, and focus/navigation may not render correctly.
Run this before starting yamdl again:

  export TERM=xterm-256color
`, term)
}
