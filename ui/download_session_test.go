package ui

import (
	"bytes"
	"context"
	"io"
	"testing"
	"time"
	"ya-music/utils"
	"ya-music/ya"
	"ya-music/ya/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeDownloadClient struct {
	filename string
	err      error
}

func (f *fakeDownloadClient) DownloadTrackWithOptions(
	model.Track,
	string,
	ya.DownloadOptions,
) (string, error) {
	return f.filename, f.err
}

func TestDownloadSessionEmitsSnapshotsWithoutMutatingInput(t *testing.T) {
	input := []TrackProgress{{
		uid: "track-1", track: &model.Track{ID: model.FlexibleID("1"), Title: "Song"}, status: TrackStatusReady,
	}}
	session := NewDownloadSession(
		&fakeDownloadClient{filename: "downloads/Song.mp3"},
		utils.NewDownloadLoggerForWriter(io.Discard),
		ya.DownloadOptions{},
		t.TempDir(),
	)

	var events []DownloadSessionEvent
	for event := range session.Run(input) {
		events = append(events, event)
	}

	require.Len(t, events, 2)
	assert.Equal(t, TrackStatusReady, input[0].status)
	assert.Equal(t, TrackStatusDownloading, events[0].Progress.status)
	assert.Equal(t, TrackStatusDownloaded, events[1].Progress.status)
	assert.True(t, events[1].Completed)
}

func TestDownloadSessionTurnsClientErrorIntoCompletedErrorEvent(t *testing.T) {
	session := NewDownloadSession(
		&fakeDownloadClient{err: context.Canceled},
		utils.NewDownloadLoggerForWriter(io.Discard),
		ya.DownloadOptions{},
		t.TempDir(),
	)

	var final DownloadSessionEvent
	for event := range session.Run([]TrackProgress{{
		uid: "track-1", track: &model.Track{ID: model.FlexibleID("1"), Title: "Song"}, status: TrackStatusReady,
	}}) {
		if event.Completed {
			final = event
		}
	}

	assert.Equal(t, TrackStatusError, final.Progress.status)
	assert.Contains(t, final.Progress.errMsg, "context canceled")
}

type blockingDownloadClient struct {
	started chan struct{}
	release chan struct{}
	path    string
}

func (c *blockingDownloadClient) DownloadTrackWithOptions(
	model.Track,
	string,
	ya.DownloadOptions,
) (string, error) {
	close(c.started)
	<-c.release
	return c.path, nil
}

func TestDownloadSessionClosesOnlyAfterWorkersFinish(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	client := &blockingDownloadClient{started: started, release: release, path: "song.mp3"}
	session := NewDownloadSession(client, utils.NewDiscardDownloadLogger(), ya.DownloadOptions{}, t.TempDir())
	events := session.Run([]TrackProgress{{
		uid: "track-1", track: &model.Track{ID: model.FlexibleID("1"), Title: "Song"}, status: TrackStatusReady,
	}})

	select {
	case event, ok := <-events:
		if !ok {
			t.Fatal("session closed before worker started")
		}
		if event.Progress.status != TrackStatusDownloading {
			t.Fatalf("first event status = %v, want Downloading", event.Progress.status)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for Downloading event")
	}

	<-started
	select {
	case _, ok := <-events:
		if !ok {
			t.Fatal("session closed before worker release")
		}
	case <-time.After(100 * time.Millisecond):
	}

	close(release)
	for range events {
	}
}

func TestDownloadSessionLogsSkippedTracks(t *testing.T) {
	var logs bytes.Buffer
	session := NewDownloadSession(
		&fakeDownloadClient{},
		utils.NewDownloadLoggerForWriter(&logs),
		ya.DownloadOptions{},
		t.TempDir(),
	)

	for range session.Run([]TrackProgress{
		{uid: "duplicate", track: &model.Track{ID: model.FlexibleID("1"), Title: "Duplicate"}, status: TrackStatusDuplicate},
		{uid: "unavailable", track: &model.Track{ID: model.FlexibleID("2"), Title: "Unavailable"}, status: TrackStatusNotAvailable},
	}) {
	}

	assert.Contains(t, logs.String(), "reason=duplicate")
	assert.Contains(t, logs.String(), "reason=not_available")
	assert.Contains(t, logs.String(), "download session finished")
}

type panicDownloadClient struct{}

func (panicDownloadClient) DownloadTrackWithOptions(
	model.Track,
	string,
	ya.DownloadOptions,
) (string, error) {
	panic("download failure")
}

func TestDownloadSessionConvertsWorkerPanicToCompletedErrorEvent(t *testing.T) {
	session := NewDownloadSession(
		panicDownloadClient{},
		utils.NewDiscardDownloadLogger(),
		ya.DownloadOptions{},
		t.TempDir(),
	)

	var events []DownloadSessionEvent
	for event := range session.Run([]TrackProgress{{
		uid: "track-1", track: &model.Track{ID: model.FlexibleID("1"), Title: "Song"}, status: TrackStatusReady,
	}}) {
		events = append(events, event)
	}

	require.Len(t, events, 2)
	assert.Equal(t, TrackStatusError, events[1].Progress.status)
	assert.True(t, events[1].Completed)
	assert.Contains(t, events[1].Progress.errMsg, "download failure")
}
