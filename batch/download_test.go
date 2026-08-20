package batch

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"ya-music/ya"
	"ya-music/ya/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeClient struct {
	results map[string]fakeResult
}

type fakeResult struct {
	filename string
	err      error
}

func (c fakeClient) DownloadTrackWithOptions(track model.Track, _ string, _ ya.DownloadOptions) (string, error) {
	result := c.results[track.ID.String()]
	return result.filename, result.err
}

func TestRunReportsTrackLifecycleAndContinuesAfterErrors(t *testing.T) {
	events := collect(Run(Config{
		Client: fakeClient{results: map[string]fakeResult{
			"1": {filename: "first.mp3"},
			"2": {err: errors.New("access denied")},
		}},
		Tracks: []model.Track{
			{ID: "1", Title: "First", Available: true},
			{ID: "2", Title: "Second", Available: true},
		},
		Concurrency: 1,
	}))

	require.Len(t, events, 4)
	assert.Equal(t, StatusDownloading, findEvent(t, events, 1, StatusDownloading).Status)
	done := findEvent(t, events, 1, StatusDone)
	assert.Equal(t, ContainerMP3, done.Format)
	assert.Equal(t, StatusDownloading, findEvent(t, events, 2, StatusDownloading).Status)
	failed := findEvent(t, events, 2, StatusError)
	assert.Equal(t, "access denied", failed.Reason)
}

func TestRunSkipsUnavailableDuplicatesAndExistingFiles(t *testing.T) {
	events := collect(Run(Config{
		Client: fakeClient{results: map[string]fakeResult{
			"1": {filename: "first.mp3", err: ya.ErrTrackAlreadyExists},
		}},
		Tracks: []model.Track{
			{ID: "1", Title: "First", Available: true},
			{ID: "1", Title: "First", Available: true},
			{ID: "3", Title: "Hidden", Available: false},
		},
		Concurrency: 1,
	}))

	require.Len(t, events, 4)
	assert.Equal(t, StatusDownloading, findEvent(t, events, 1, StatusDownloading).Status)
	assert.Equal(t, string(SkipDuplicate), findEvent(t, events, 2, StatusSkipped).Reason)
	assert.Equal(t, string(SkipUnavailable), findEvent(t, events, 3, StatusSkipped).Reason)
	assert.Equal(t, string(SkipAlreadyExists), findEvent(t, events, 1, StatusSkipped).Reason)
}

type blockingClient struct {
	started chan string
	release chan struct{}
	mu      sync.Mutex
	called  []string
}

func (c *blockingClient) DownloadTrackWithOptions(track model.Track, _ string, _ ya.DownloadOptions) (string, error) {
	c.mu.Lock()
	c.called = append(c.called, track.ID.String())
	c.mu.Unlock()
	c.started <- track.ID.String()
	<-c.release
	return track.ID.String() + ".mp3", nil
}

func (c *blockingClient) calls() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), c.called...)
}

func TestRunStopsSchedulingTracksWhenContextIsCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	client := &blockingClient{
		started: make(chan string, 1),
		release: make(chan struct{}),
	}

	events := Run(Config{
		Client: client,
		Tracks: []model.Track{
			{ID: "1", Title: "First", Available: true},
			{ID: "2", Title: "Second", Available: true},
		},
		Concurrency: 1,
		Context:     ctx,
	})

	first := <-events
	assert.Equal(t, Event{Index: 1, Track: model.Track{ID: "1", Title: "First", Available: true}, Status: StatusDownloading}, first)
	assert.Equal(t, "1", <-client.started)

	cancel()
	close(client.release)

	remaining := collect(events)
	assert.Len(t, remaining, 1)
	assert.Equal(t, StatusDone, remaining[0].Status)
	assert.Equal(t, 1, remaining[0].Index)
	assert.Equal(t, []string{"1"}, client.calls())
}

type panicClient struct {
	results map[string]fakeResult
}

func (c panicClient) DownloadTrackWithOptions(track model.Track, _ string, _ ya.DownloadOptions) (string, error) {
	if track.ID.String() == "1" {
		panic("boom")
	}
	result := c.results[track.ID.String()]
	return result.filename, result.err
}

func TestRunRecoversWorkerPanicAndContinues(t *testing.T) {
	events := collect(Run(Config{
		Client: panicClient{results: map[string]fakeResult{
			"2": {filename: "second.mp3"},
		}},
		Tracks: []model.Track{
			{ID: "1", Title: "First", Available: true},
			{ID: "2", Title: "Second", Available: true},
		},
		Concurrency: 1,
	}))

	require.Len(t, events, 4)
	panicEvent := findEvent(t, events, 1, StatusError)
	assert.True(t, strings.HasPrefix(panicEvent.Reason, "panic: "))
	done := findEvent(t, events, 2, StatusDone)
	assert.Equal(t, ContainerMP3, done.Format)
}

func collect(events <-chan Event) []Event {
	var result []Event
	for event := range events {
		result = append(result, event)
	}
	return result
}

func findEvent(t *testing.T, events []Event, index int, status Status) Event {
	t.Helper()
	for _, event := range events {
		if event.Index == index && event.Status == status {
			return event
		}
	}
	t.Fatalf("event with index %d and status %q not found in %#v", index, status, events)
	return Event{}
}
