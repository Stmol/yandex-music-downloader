package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"ya-music/batch"
	"ya-music/ya/model"
)

const signalTestTimeout = time.Second

func TestInterruptSignalsRouteFirstAndSecondSIGINTAndSIGTERM(t *testing.T) {
	tests := []struct {
		name   string
		signal os.Signal
	}{
		{name: "SIGINT", signal: syscall.SIGINT},
		{name: "SIGTERM", signal: syscall.SIGTERM},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			interrupts := newInterruptSignals()
			t.Cleanup(interrupts.stop)
			process, err := os.FindProcess(os.Getpid())
			if err != nil {
				t.Fatalf("find current process: %v", err)
			}

			if err := process.Signal(tt.signal); err != nil {
				t.Fatalf("send first %s: %v", tt.name, err)
			}
			waitForSignalStage(t, interrupts.first, "first")
			assertSignalStagePending(t, interrupts.force, "force")

			if err := process.Signal(tt.signal); err != nil {
				t.Fatalf("send second %s: %v", tt.name, err)
			}
			waitForSignalStage(t, interrupts.force, "force")
		})
	}
}

func TestInterruptSignalsStopIsIdempotent(t *testing.T) {
	signalCh := make(chan os.Signal, 2)
	var unregisterCalls atomic.Int32
	interrupts := routeInterruptSignals(signalCh, func() {
		unregisterCalls.Add(1)
	})

	interrupts.stop()
	interrupts.stop()
	if got := unregisterCalls.Load(); got != 1 {
		t.Fatalf("unregister calls = %d, want 1", got)
	}
}

func TestInterruptSignalsFlushKeepsRouterRegisteredAndRunning(t *testing.T) {
	signalCh := make(chan os.Signal, 1)
	var unregisterCalls atomic.Int32
	interrupts := routeInterruptSignals(signalCh, func() {
		unregisterCalls.Add(1)
	})
	t.Cleanup(interrupts.stop)

	interrupts.flush()
	if got := unregisterCalls.Load(); got != 0 {
		t.Fatalf("unregister calls after flush = %d, want 0", got)
	}

	signalCh <- syscall.SIGTERM
	waitForSignalStage(t, interrupts.first, "first signal after flush")
	assertSignalStagePending(t, interrupts.force, "force signal after flush")

	interrupts.stop()
	interrupts.stop()
	if got := unregisterCalls.Load(); got != 1 {
		t.Fatalf("unregister calls after stop = %d, want 1", got)
	}
}

func TestDownloadPreflightInterruptCancelsWaitsAndPrintsOnlyCancellation(t *testing.T) {
	client := &blockingPreflightClient{
		started:   make(chan struct{}),
		cancelled: make(chan struct{}),
		returned:  make(chan struct{}),
	}
	results := startDownloadPreflight(client, "https://music.yandex.ru/album/1")
	if got := cap(results); got != 1 {
		t.Fatalf("preflight result channel capacity = %d, want 1", got)
	}
	waitForSignalStage(t, client.started, "preflight start")

	firstInterrupt := make(chan struct{})
	close(firstInterrupt)
	var stdout bytes.Buffer
	_, interrupted := awaitDownloadPreflight(&stdout, results, firstInterrupt, func() {
		close(client.cancelled)
	})

	if !interrupted {
		t.Fatal("expected interrupted preflight")
	}
	waitForSignalStage(t, client.returned, "preflight return")
	if got, want := stdout.String(), "Interrupted: preflight cancelled\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
}

func TestDownloadPreflightInterruptCancelsSourceResolutionAndWaits(t *testing.T) {
	client := &blockingSourcePreflightClient{
		started:   make(chan struct{}),
		cancelled: make(chan struct{}),
		returned:  make(chan struct{}),
	}
	results := startDownloadPreflight(client, "https://music.yandex.com/album/1/track/2")
	waitForSignalStage(t, client.started, "source resolution start")

	firstInterrupt := make(chan struct{})
	close(firstInterrupt)
	var stdout bytes.Buffer
	_, interrupted := awaitDownloadPreflight(&stdout, results, firstInterrupt, func() {
		close(client.cancelled)
	})

	if !interrupted {
		t.Fatal("expected interrupted preflight")
	}
	waitForSignalStage(t, client.returned, "source resolution return")
	if got, want := stdout.String(), "Interrupted: preflight cancelled\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
}

func TestConsumeDownloadEventsGracefulInterruptDrainsStartedDownload(t *testing.T) {
	track := model.Track{Title: "Track", Artists: []model.Artist{{Name: "Artist"}}}
	events := make(chan batch.Event, 2)
	firstInterrupt := make(chan struct{})
	forceInterrupt := make(chan struct{})
	stopSchedulingCalled := make(chan struct{}, 1)
	forceCancelCalled := make(chan struct{}, 1)
	resultCh := make(chan consumeResult, 1)
	var stdout bytes.Buffer

	go func() {
		summary, interrupted := consumeDownloadEvents(
			&stdout,
			events,
			firstInterrupt,
			forceInterrupt,
			func() { stopSchedulingCalled <- struct{}{} },
			func() { forceCancelCalled <- struct{}{} },
		)
		resultCh <- consumeResult{summary: summary, interrupted: interrupted}
	}()

	events <- batch.Event{Index: 1, Track: track, Status: batch.StatusDownloading}
	close(firstInterrupt)
	waitForSignalStage(t, stopSchedulingCalled, "stop scheduling callback")
	assertSignalStagePending(t, forceCancelCalled, "force cancel callback")
	events <- batch.Event{Index: 1, Track: track, Status: batch.StatusDone, Format: batch.ContainerMP3}
	close(events)

	result := waitForConsumeResult(t, resultCh)
	if !result.interrupted {
		t.Fatal("expected interrupted batch")
	}
	if result.summary != (batchSummary{downloaded: 1}) {
		t.Fatalf("summary = %#v, want one downloaded", result.summary)
	}
	if got, want := stdout.String(), strings.Join([]string{
		"[downloading] Artist — Track",
		"[done] Artist — Track",
		"Interrupted: remaining tracks stayed queued",
	}, "\n")+"\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
}

func TestConsumeDownloadEventsForceInterruptCancelsOnlyInFlightRequests(t *testing.T) {
	events := make(chan batch.Event, 1)
	firstInterrupt := make(chan struct{})
	forceInterrupt := make(chan struct{})
	stopSchedulingCalled := make(chan struct{}, 1)
	forceCancelCalled := make(chan struct{}, 1)
	resultCh := make(chan consumeResult, 1)

	go func() {
		summary, interrupted := consumeDownloadEvents(
			&bytes.Buffer{},
			events,
			firstInterrupt,
			forceInterrupt,
			func() { stopSchedulingCalled <- struct{}{} },
			func() { forceCancelCalled <- struct{}{} },
		)
		resultCh <- consumeResult{summary: summary, interrupted: interrupted}
	}()

	close(firstInterrupt)
	waitForSignalStage(t, stopSchedulingCalled, "stop scheduling callback")
	assertSignalStagePending(t, forceCancelCalled, "force cancel callback")

	close(forceInterrupt)
	waitForSignalStage(t, forceCancelCalled, "force cancel callback")
	assertSignalStagePending(t, stopSchedulingCalled, "second stop scheduling callback")

	events <- batch.Event{Status: batch.StatusError, Reason: context.Canceled.Error()}
	close(events)
	result := waitForConsumeResult(t, resultCh)
	if !result.interrupted || result.summary != (batchSummary{failed: 1}) {
		t.Fatalf("result = %#v", result)
	}
}

func TestConsumeDownloadEventsFlushesReceivedRouterSignalsBeforeReturning(t *testing.T) {
	signalCh := make(chan os.Signal, 2)
	firstSignalReceived := make(chan struct{})
	releaseRouter := make(chan struct{})
	var releaseOnce sync.Once
	var routed atomic.Int32
	interrupts := routeInterruptSignalsWithBeforeRoute(signalCh, func() {}, func() {
		if routed.Add(1) == 1 {
			close(firstSignalReceived)
			<-releaseRouter
		}
	})
	t.Cleanup(func() {
		releaseOnce.Do(func() { close(releaseRouter) })
		interrupts.stop()
	})

	signalCh <- syscall.SIGINT
	signalCh <- syscall.SIGTERM
	waitForSignalStage(t, firstSignalReceived, "router receive")

	events := make(chan batch.Event)
	close(events)
	flushStarted := make(chan struct{})
	resultCh := make(chan consumeResult, 1)
	var calls []string

	go func() {
		summary, interrupted := consumeDownloadEventsWithFlush(
			&bytes.Buffer{},
			events,
			interrupts.first,
			interrupts.force,
			func() { calls = append(calls, "stop scheduling") },
			func() { calls = append(calls, "cancel in-flight") },
			func() {
				close(flushStarted)
				interrupts.flush()
			},
		)
		resultCh <- consumeResult{summary: summary, interrupted: interrupted}
	}()

	waitForSignalStage(t, flushStarted, "consumer flush")
	assertConsumeResultPending(t, resultCh)
	releaseOnce.Do(func() { close(releaseRouter) })
	result := waitForConsumeResult(t, resultCh)

	if !result.interrupted {
		t.Fatal("expected interrupted batch")
	}
	if got, want := strings.Join(calls, ", "), "stop scheduling, cancel in-flight"; got != want {
		t.Fatalf("callback order = %q, want %q", got, want)
	}
}

func TestConsumeDownloadEventsPreservesSummaryWhileDrainingAfterInterrupt(t *testing.T) {
	events := make(chan batch.Event, 4)
	events <- batch.Event{Status: batch.StatusDone}
	firstInterrupt := make(chan struct{})
	close(firstInterrupt)
	events <- batch.Event{Status: batch.StatusSkipped, Reason: batch.SkipDuplicate}
	events <- batch.Event{Status: batch.StatusError, Reason: "cancelled"}
	close(events)

	var stdout bytes.Buffer
	summary, interrupted := consumeDownloadEvents(
		&stdout,
		events,
		firstInterrupt,
		nil,
		func() {},
		nil,
	)

	if !interrupted {
		t.Fatal("expected interrupted batch")
	}
	if want := (batchSummary{downloaded: 1, skipped: 1, failed: 1}); summary != want {
		t.Fatalf("summary = %#v, want %#v", summary, want)
	}
}

type consumeResult struct {
	summary     batchSummary
	interrupted bool
}

type blockingPreflightClient struct {
	started   chan struct{}
	cancelled chan struct{}
	returned  chan struct{}
}

func (c *blockingPreflightClient) AccountStatus() (*model.Account, error) {
	close(c.started)
	<-c.cancelled
	close(c.returned)
	return nil, errors.New("request cancelled")
}

func (*blockingPreflightClient) TrackInfo(string) (*model.Track, error) {
	panic("TrackInfo must not be called after account cancellation")
}

func (*blockingPreflightClient) AlbumWithTracks(string) (*model.Album, error) {
	panic("AlbumWithTracks must not be called after account cancellation")
}

func (*blockingPreflightClient) UsersPlaylist(string, string) (*model.Playlist, error) {
	panic("UsersPlaylist must not be called after account cancellation")
}

func (*blockingPreflightClient) PlaylistByUUID(string) (*model.Playlist, error) {
	panic("PlaylistByUUID must not be called after account cancellation")
}

func (*blockingPreflightClient) Chart(string) (*model.Playlist, error) {
	panic("Chart must not be called after account cancellation")
}

type blockingSourcePreflightClient struct {
	started   chan struct{}
	cancelled chan struct{}
	returned  chan struct{}
}

func (*blockingSourcePreflightClient) AccountStatus() (*model.Account, error) {
	return &model.Account{Uid: 1}, nil
}

func (c *blockingSourcePreflightClient) TrackInfo(string) (*model.Track, error) {
	close(c.started)
	<-c.cancelled
	close(c.returned)
	return nil, errors.New("request cancelled")
}

func (*blockingSourcePreflightClient) AlbumWithTracks(string) (*model.Album, error) {
	panic("AlbumWithTracks must not be called for a track URL")
}

func (*blockingSourcePreflightClient) UsersPlaylist(string, string) (*model.Playlist, error) {
	panic("UsersPlaylist must not be called for a track URL")
}

func (*blockingSourcePreflightClient) PlaylistByUUID(string) (*model.Playlist, error) {
	panic("PlaylistByUUID must not be called for a track URL")
}

func (*blockingSourcePreflightClient) Chart(string) (*model.Playlist, error) {
	panic("Chart must not be called for a track URL")
}

func waitForSignalStage(t *testing.T, stage <-chan struct{}, name string) {
	t.Helper()
	select {
	case <-stage:
	case <-time.After(signalTestTimeout):
		t.Fatalf("timed out waiting for %s", name)
	}
}

func assertSignalStagePending(t *testing.T, stage <-chan struct{}, name string) {
	t.Helper()
	select {
	case <-stage:
		t.Fatalf("%s happened too early", name)
	default:
	}
}

func waitForConsumeResult(t *testing.T, results <-chan consumeResult) consumeResult {
	t.Helper()
	select {
	case result := <-results:
		return result
	case <-time.After(signalTestTimeout):
		t.Fatal("timed out waiting for event consumer")
		return consumeResult{}
	}
}

func assertConsumeResultPending(t *testing.T, results <-chan consumeResult) {
	t.Helper()
	select {
	case result := <-results:
		t.Fatalf("event consumer returned before router flush: %#v", result)
	default:
	}
}
