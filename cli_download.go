package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
	"ya-music/batch"
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

	account, err := client.AccountStatus()
	if err != nil || account.Uid == 0 {
		if err == nil {
			err = errors.New("account has no user ID")
		}
		fmt.Fprintf(stderr, "failed to validate token: %v\n", err)
		return 1
	}

	tracks, err := source.Resolve(client, options.link)
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

	signalStop := make(chan struct{})
	defer close(signalStop)

	interrupt := make(chan struct{})
	go func() {
		select {
		case <-sigCh:
			close(interrupt)
		case <-signalStop:
		}
	}()
	batchContext, cancelBatch := context.WithCancel(context.Background())
	defer cancelBatch()

	summary, interrupted := consumeDownloadEvents(
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
		interrupt,
		func() { interruptBatch(cancelBatch) },
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

func interruptBatch(cancelBatch context.CancelFunc) {
	cancelBatch()
}

func consumeDownloadEvents(
	stdout io.Writer,
	events <-chan batch.Event,
	interrupt <-chan struct{},
	cancel func(),
) (summary batchSummary, interrupted bool) {
	emit := func(event batch.Event) {
		fmt.Fprintln(stdout, formatBatchEvent(event))
		summary.add(event)
	}

	for events != nil {
		if interrupt == nil {
			event, ok := <-events
			if !ok {
				break
			}
			emit(event)
			continue
		}

		select {
		case <-interrupt:
			interrupted = true
			interrupt = nil
			if cancel != nil {
				cancel()
			}
		case event, ok := <-events:
			if !ok {
				select {
				case <-interrupt:
					interrupted = true
					if cancel != nil {
						cancel()
					}
				default:
				}
				events = nil
				continue
			}
			emit(event)
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

func formatTrackLabel(track model.Track) string {
	return track.DisplayLabel()
}
