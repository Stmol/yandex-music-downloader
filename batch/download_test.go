package batch

import (
	"errors"
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
	assert.Equal(t, "mp3", done.Format)
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
	assert.Equal(t, "duplicate", findEvent(t, events, 2, StatusSkipped).Reason)
	assert.Equal(t, "unavailable", findEvent(t, events, 3, StatusSkipped).Reason)
	assert.Equal(t, "already exists", findEvent(t, events, 1, StatusSkipped).Reason)
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
