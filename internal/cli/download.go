package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
	"ya-music/internal/batch"
	"ya-music/source"
	"ya-music/utils"
	"ya-music/ya"
	"ya-music/ya/model"
)

func newLoggedClient(timeoutSeconds int, stderr io.Writer) (*utils.DownloadLogger, *ya.Client) {
	downloadLogger, err := utils.NewDownloadLogger(utils.DefaultDownloadLogPath)
	if err != nil {
		fmt.Fprintf(stderr, "failed to initialize download logger: %v\n", err)
		downloadLogger = utils.NewDiscardDownloadLogger()
	}
	if err := downloadLogger.Reset(); err != nil {
		fmt.Fprintf(stderr, "failed to reset download log file: %v\n", err)
	}
	httpClient := utils.NewHttpClientWithLogger(downloadLogger)
	httpClient.SetDownloadTimeout(time.Duration(timeoutSeconds) * time.Second)
	return downloadLogger, ya.NewClient(httpClient)
}

func runDownload(args []string, stdout, stderr io.Writer) int {
	parsed := parseDownloadOptions(args, stderr)
	if !parsed.proceed {
		return parsed.exitCode
	}
	options := parsed.options

	downloadLogger, client := newLoggedClient(options.timeoutSeconds, stderr)
	defer downloadLogger.Close()
	client.SetToken(options.token)

	interrupts := newInterruptSignals()
	defer interrupts.stop()

	preflight, interrupted := awaitDownloadPreflight(
		stdout,
		startDownloadPreflight(client, options.link),
		interrupts.first,
		client.Cancel,
	)
	if interrupted {
		return 130
	}
	if preflight.err != nil {
		fmt.Fprintln(stderr, preflight.err)
		return 1
	}
	tracks := preflight.tracks

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

	batchContext, cancelBatch := context.WithCancel(context.Background())
	defer cancelBatch()

	summary, interrupted := consumeDownloadEventsWithFlush(
		stdout,
		batch.Run(batch.Config{
			Client:    client,
			Tracks:    tracks,
			OutputDir: options.output,
			Context:   batchContext,
			Options: ya.DownloadOptions{
				SkipCover:   options.skipCover,
				AudioFormat: options.format,
			},
		}),
		interrupts.first,
		interrupts.force,
		cancelBatch,
		client.Cancel,
		interrupts.flush,
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

type downloadPreflightClient interface {
	source.Client
	AccountStatus() (*model.Account, error)
}

type downloadPreflightResult struct {
	tracks []model.Track
	err    error
}

func startDownloadPreflight(client downloadPreflightClient, link string) <-chan downloadPreflightResult {
	results := make(chan downloadPreflightResult, 1)
	go func() {
		defer close(results)
		results <- runDownloadPreflight(client, link)
	}()
	return results
}

func runDownloadPreflight(client downloadPreflightClient, link string) downloadPreflightResult {
	account, err := client.AccountStatus()
	if err != nil || account == nil || account.Uid == 0 {
		if err == nil {
			err = errors.New("account has no user ID")
		}
		return downloadPreflightResult{err: fmt.Errorf("failed to validate token: %w", err)}
	}

	tracks, err := source.Resolve(client, link)
	if err != nil {
		return downloadPreflightResult{err: fmt.Errorf("failed to resolve source: %w", err)}
	}
	if len(tracks) == 0 {
		return downloadPreflightResult{err: errors.New("failed to resolve source: no tracks found")}
	}
	return downloadPreflightResult{tracks: tracks}
}

func awaitDownloadPreflight(
	stdout io.Writer,
	results <-chan downloadPreflightResult,
	firstInterrupt <-chan struct{},
	cancelInFlight func(),
) (downloadPreflightResult, bool) {
	cancelAndWait := func() (downloadPreflightResult, bool) {
		if cancelInFlight != nil {
			cancelInFlight()
		}
		result := <-results
		fmt.Fprintln(stdout, "Interrupted: preflight cancelled")
		return result, true
	}

	select {
	case <-firstInterrupt:
		return cancelAndWait()
	default:
	}

	select {
	case <-firstInterrupt:
		return cancelAndWait()
	case result := <-results:
		select {
		case <-firstInterrupt:
			if cancelInFlight != nil {
				cancelInFlight()
			}
			fmt.Fprintln(stdout, "Interrupted: preflight cancelled")
			return result, true
		default:
			return result, false
		}
	}
}

func consumeDownloadEvents(
	stdout io.Writer,
	events <-chan batch.Event,
	firstInterrupt <-chan struct{},
	forceInterrupt <-chan struct{},
	stopScheduling func(),
	cancelInFlight func(),
) (batchSummary, bool) {
	return consumeDownloadEventsWithFlush(
		stdout,
		events,
		firstInterrupt,
		forceInterrupt,
		stopScheduling,
		cancelInFlight,
		nil,
	)
}

func consumeDownloadEventsWithFlush(
	stdout io.Writer,
	events <-chan batch.Event,
	firstInterrupt <-chan struct{},
	forceInterrupt <-chan struct{},
	stopScheduling func(),
	cancelInFlight func(),
	flushInterrupts func(),
) (summary batchSummary, interrupted bool) {
	activeForceInterrupt := (<-chan struct{})(nil)
	if firstInterrupt == nil {
		activeForceInterrupt = forceInterrupt
	}

	emit := func(event batch.Event) {
		fmt.Fprintln(stdout, formatBatchEvent(event))
		summary.add(event)
	}
	handleFirstInterrupt := func() {
		interrupted = true
		firstInterrupt = nil
		activeForceInterrupt = forceInterrupt
		if stopScheduling != nil {
			stopScheduling()
		}
	}
	handleForceInterrupt := func() {
		interrupted = true
		forceInterrupt = nil
		activeForceInterrupt = nil
		if cancelInFlight != nil {
			cancelInFlight()
		}
	}

	for events != nil {
		select {
		case <-firstInterrupt:
			handleFirstInterrupt()
		case <-activeForceInterrupt:
			handleForceInterrupt()
		case event, ok := <-events:
			if !ok {
				events = nil
				continue
			}
			emit(event)
		}
	}
	if flushInterrupts != nil {
		flushInterrupts()
	}

	if firstInterrupt != nil {
		select {
		case <-firstInterrupt:
			handleFirstInterrupt()
		default:
		}
	}
	if activeForceInterrupt != nil {
		select {
		case <-activeForceInterrupt:
			handleForceInterrupt()
		default:
		}
	}

	if interrupted {
		fmt.Fprintln(stdout, "Interrupted: remaining tracks stayed queued")
	}
	return summary, interrupted
}

func formatBatchEvent(event batch.Event) string {
	status := string(event.Status)
	if event.Status == batch.StatusSkipped {
		if reason := strings.TrimSpace(event.Reason); reason != "" {
			status = reason
		}
	}
	return fmt.Sprintf("[%s] %s", status, event.Track.DisplayLabel())
}
