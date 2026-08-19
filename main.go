package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"sync/atomic"
	"syscall"
	"time"
	"ya-music/batch"
	"ya-music/ui"
	"ya-music/utils"
	"ya-music/ya"
	"ya-music/ya/model"

	tea "charm.land/bubbletea/v2"
)

const defaultOutputDir = "./downloads"

type tuiOptions struct {
	downloadTimeoutSeconds int
	skipCover              bool
}

type downloadOptions struct {
	token                  string
	link                   string
	format                 ya.AudioFormat
	output                 string
	downloadTimeoutSeconds int
	skipCover              bool
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 && args[0] == "download" {
		return runDownload(args[1:], stdout, stderr)
	}
	return runTUI(args, stderr)
}

func runTUI(args []string, stderr io.Writer) int {
	options, exitCode := parseTUIOptions(args, stderr)
	if exitCode >= 0 {
		return exitCode
	}

	if isKnownProblematicTerm(os.Getenv("TERM")) {
		fmt.Fprintln(stderr, problematicTermWarning(os.Getenv("TERM")))
		return 2
	}

	downloadLogger := newDownloadLogger(stderr)
	defer downloadLogger.Close()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	httpClient := utils.NewHttpClientWithLogger(downloadLogger)
	httpClient.SetDownloadTimeout(time.Duration(options.downloadTimeoutSeconds) * time.Second)
	client := ya.NewClient(httpClient)
	downloadOptions := ya.DownloadOptions{SkipCover: options.skipCover}
	prog := tea.NewProgram(ui.StartUi(client, downloadOptions))

	go func() {
		sig := <-sigCh
		prog.Send(ui.ShutdownRequestedMsg{Reason: "signal_" + strings.ToLower(sig.String())})
	}()

	downloadLogger.Info("application started",
		"log_path", downloadLogger.Path(),
		"download_timeout_seconds", options.downloadTimeoutSeconds,
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

func parseTUIOptions(args []string, stderr io.Writer) (tuiOptions, int) {
	options := tuiOptions{}
	flags := flag.NewFlagSet("yamdl", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.IntVar(&options.downloadTimeoutSeconds, "timeout", 0, "download timeout in seconds (0 disables timeout)")
	flags.BoolVar(&options.skipCover, "skip-cover", false, "skip downloading and embedding track cover images")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return tuiOptions{}, 0
		}
		return tuiOptions{}, 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(stderr, "unexpected command; use 'yamdl download --help' for batch downloads")
		return tuiOptions{}, 2
	}
	if options.downloadTimeoutSeconds < 0 {
		fmt.Fprintln(stderr, "timeout must be >= 0 seconds")
		return tuiOptions{}, 2
	}
	return options, -1
}

func runDownload(args []string, stdout, stderr io.Writer) int {
	options, exitCode := parseDownloadOptions(args, stderr)
	if exitCode >= 0 {
		return exitCode
	}

	downloadLogger := newDownloadLogger(stderr)
	defer downloadLogger.Close()
	httpClient := utils.NewHttpClientWithLogger(downloadLogger)
	httpClient.SetDownloadTimeout(time.Duration(options.downloadTimeoutSeconds) * time.Second)
	client := ya.NewClient(httpClient)
	client.SetToken(options.token)

	account, err := client.AccountStatus()
	if err != nil || account.Uid == 0 {
		if err == nil {
			err = errors.New("account has no user ID")
		}
		fmt.Fprintf(stderr, "failed to validate token: %v\n", err)
		return 1
	}

	tracks, err := ui.ResolveSourceTracks(client, options.link)
	if err != nil {
		fmt.Fprintf(stderr, "failed to resolve source: %v\n", err)
		return 1
	}
	if len(tracks) == 0 {
		fmt.Fprintln(stderr, "failed to resolve source: no tracks found")
		return 1
	}
	if err := utils.CreateDirIfNotExists(options.output); err != nil {
		fmt.Fprintf(stderr, "failed to create output directory: %v\n", err)
		return 1
	}
	outputInfo, err := os.Stat(options.output)
	if err != nil {
		fmt.Fprintf(stderr, "failed to inspect output directory: %v\n", err)
		return 1
	}
	if !outputInfo.IsDir() {
		fmt.Fprintln(stderr, "--output must be a directory")
		return 2
	}

	downloadLogger.Info("batch download started",
		"source", utils.SanitizeURL(options.link),
		"total_tracks", len(tracks),
		"format", options.format,
		"output", options.output,
	)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	stop := make(chan struct{})
	defer close(stop)

	interrupt := make(chan struct{})
	go func() {
		select {
		case <-sigCh:
			close(interrupt)
		case <-stop:
		}
	}()

	summary, interrupted := consumeDownloadEvents(
		stdout,
		batch.Run(batch.Config{
			Client:    client,
			Tracks:    tracks,
			OutputDir: options.output,
			Options: ya.DownloadOptions{
				SkipCover:   options.skipCover,
				AudioFormat: options.format,
			},
		}),
		interrupt,
		client.Cancel,
	)

	fmt.Fprintf(stdout, "\nFinished: %d downloaded, %d skipped, %d failed\nOutput: %s\n",
		summary.downloaded,
		summary.skipped,
		summary.failed,
		options.output,
	)
	downloadLogger.Info("batch download finished",
		"downloaded", summary.downloaded,
		"skipped", summary.skipped,
		"failed", summary.failed,
		"interrupted", interrupted,
	)
	if interrupted {
		return 130
	}
	return 0
}

func parseDownloadOptions(args []string, stderr io.Writer) (downloadOptions, int) {
	options := downloadOptions{format: ya.AudioFormatMP3, output: defaultOutputDir}
	flags := flag.NewFlagSet("yamdl download", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.StringVar(&options.token, "token", "", "Yandex Music authentication token (required)")
	flags.StringVar(&options.link, "link", "", "Yandex Music track, album, playlist, or chart URL (required)")
	format := string(options.format)
	flags.StringVar(&format, "format", format, "audio format: mp3 or flac")
	flags.StringVar(&options.output, "output", options.output, "directory for downloaded tracks")
	flags.IntVar(&options.downloadTimeoutSeconds, "timeout", 0, "download timeout in seconds (0 disables timeout)")
	flags.BoolVar(&options.skipCover, "skip-cover", false, "skip downloading and embedding track cover images")
	flags.Usage = func() {
		fmt.Fprintln(stderr, "Usage: yamdl download --token TOKEN --link URL [options]")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return downloadOptions{}, 0
		}
		return downloadOptions{}, 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(stderr, "download does not accept positional arguments")
		return downloadOptions{}, 2
	}
	if strings.TrimSpace(options.token) == "" {
		fmt.Fprintln(stderr, "--token is required")
		return downloadOptions{}, 2
	}
	if strings.TrimSpace(options.link) == "" {
		fmt.Fprintln(stderr, "--link is required")
		return downloadOptions{}, 2
	}
	if options.downloadTimeoutSeconds < 0 {
		fmt.Fprintln(stderr, "timeout must be >= 0 seconds")
		return downloadOptions{}, 2
	}
	options.format = ya.AudioFormat(strings.ToLower(strings.TrimSpace(format)))
	if options.format != ya.AudioFormatMP3 && options.format != ya.AudioFormatFLAC {
		fmt.Fprintln(stderr, "--format must be mp3 or flac")
		return downloadOptions{}, 2
	}
	options.output = strings.TrimSpace(options.output)
	if options.output == "" {
		fmt.Fprintln(stderr, "--output must not be empty")
		return downloadOptions{}, 2
	}
	options.token = strings.TrimSpace(options.token)
	options.link = strings.TrimSpace(options.link)
	return options, -1
}

type batchSummary struct {
	downloaded int
	skipped    int
	failed     int
}

func (s *batchSummary) add(event batch.Event) {
	switch event.Status {
	case batch.StatusDone:
		s.downloaded++
	case batch.StatusSkipped:
		s.skipped++
	case batch.StatusError:
		s.failed++
	}
}

func consumeDownloadEvents(
	stdout io.Writer,
	events <-chan batch.Event,
	interrupt <-chan struct{},
	cancel func(),
) (summary batchSummary, interrupted bool) {
	var interruptedFlag atomic.Bool

	if interrupt != nil && cancel != nil {
		go func() {
			<-interrupt
			interruptedFlag.Store(true)
			cancel()
		}()
	}

	for event := range events {
		fmt.Fprintln(stdout, formatBatchEvent(event))
		summary.add(event)
	}

	return summary, interruptedFlag.Load()
}

func formatBatchEvent(event batch.Event) string {
	label := formatTrackLabel(event.Track)
	switch event.Status {
	case batch.StatusDownloading:
		return fmt.Sprintf("%3d. %s — downloading", event.Index, label)
	case batch.StatusDone:
		return fmt.Sprintf("%3d. %s — done (%s)", event.Index, label, event.Format)
	case batch.StatusSkipped:
		return fmt.Sprintf("%3d. %s — skipped: %s", event.Index, label, event.Reason)
	case batch.StatusError:
		return fmt.Sprintf("%3d. %s — error: %s", event.Index, label, event.Reason)
	default:
		return fmt.Sprintf("%3d. %s — %s", event.Index, label, event.Status)
	}
}

func formatTrackLabel(track model.Track) string {
	title := strings.TrimSpace(track.FullTitle())
	artists := strings.TrimSpace(track.ArtistsString())
	if artists == "" {
		return title
	}
	if title == "" {
		return artists
	}
	return artists + " — " + title
}

func newDownloadLogger(stderr io.Writer) *utils.DownloadLogger {
	downloadLogger, err := utils.NewDownloadLogger(utils.DefaultDownloadLogPath)
	if err != nil {
		fmt.Fprintf(stderr, "failed to initialize download logger: %v\n", err)
		downloadLogger = utils.NewDiscardDownloadLogger()
	}
	if err := downloadLogger.Reset(); err != nil {
		fmt.Fprintf(stderr, "failed to reset download log file: %v\n", err)
	}
	return downloadLogger
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
