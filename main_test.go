package main

import (
	"bytes"
	"strings"
	"testing"
	"ya-music/batch"
	"ya-music/ya"
	"ya-music/ya/model"
)

func TestIsKnownProblematicTerm(t *testing.T) {
	tests := []struct {
		name string
		term string
		want bool
	}{
		{name: "xterm", term: "xterm", want: true},
		{name: "xterm 256 color", term: "xterm-256color", want: false},
		{name: "screen 256 color", term: "screen-256color", want: false},
		{name: "tmux 256 color", term: "tmux-256color", want: false},
		{name: "empty", term: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isKnownProblematicTerm(tt.term); got != tt.want {
				t.Fatalf("isKnownProblematicTerm(%q) = %v, want %v", tt.term, got, tt.want)
			}
		})
	}
}

func TestParseDownloadOptionsAcceptsCurrentDownloadFlags(t *testing.T) {
	var stderr bytes.Buffer
	options, exitCode := parseDownloadOptions([]string{
		"--token", "  abc123  ",
		"--link", "  https://music.yandex.ru/album/123  ",
		"--format", "FLAC",
		"--output", "./music",
		"--timeout", "180",
		"--skip-cover",
	}, &stderr)

	if exitCode != -1 {
		t.Fatalf("exit code = %d, stderr: %s", exitCode, stderr.String())
	}
	if options.token != "abc123" || options.link != "https://music.yandex.ru/album/123" {
		t.Fatalf("unexpected required options: %#v", options)
	}
	if options.format != ya.AudioFormatFLAC || options.output != "./music" || options.downloadTimeoutSeconds != 180 || !options.skipCover {
		t.Fatalf("unexpected parsed options: %#v", options)
	}
}

func TestParseDownloadOptionsRejectsInvalidInput(t *testing.T) {
	tests := [][]string{
		{"--link", "https://music.yandex.ru/album/123"},
		{"--token", "abc123"},
		{"--token", "abc123", "--link", "https://music.yandex.ru/album/123", "--format", "aac"},
		{"--token", "abc123", "--link", "https://music.yandex.ru/album/123", "--timeout", "-1"},
	}

	for _, args := range tests {
		var stderr bytes.Buffer
		_, exitCode := parseDownloadOptions(args, &stderr)
		if exitCode != 2 {
			t.Fatalf("args %q: exit code = %d, stderr: %s", args, exitCode, stderr.String())
		}
	}
}

func TestFormatBatchEventAndSummary(t *testing.T) {
	track := model.Track{
		Title:   "Track",
		Artists: []model.Artist{{Name: "Artist"}},
	}
	event := batch.Event{Index: 2, Track: track, Status: batch.StatusDone, Format: "m4a"}
	if got, want := formatBatchEvent(event), "  2. Artist — Track — done (m4a)"; got != want {
		t.Fatalf("formatBatchEvent() = %q, want %q", got, want)
	}

	summary := batchSummary{}
	summary.add(event)
	summary.add(batch.Event{Status: batch.StatusSkipped})
	summary.add(batch.Event{Status: batch.StatusError})
	if summary != (batchSummary{downloaded: 1, skipped: 1, failed: 1}) {
		t.Fatalf("unexpected summary: %#v", summary)
	}
}

func TestProblematicTermWarning(t *testing.T) {
	warning := problematicTermWarning("xterm")

	for _, want := range []string{
		"TERM=xterm",
		"Colors, selected row highlighting, and focus/navigation",
		"export TERM=xterm-256color",
	} {
		if !strings.Contains(warning, want) {
			t.Fatalf("warning does not contain %q:\n%s", want, warning)
		}
	}
}

func TestConsumeDownloadEventsDrainsAllEventsWithoutInterrupt(t *testing.T) {
	events := make(chan batch.Event, 3)
	events <- batch.Event{Index: 1, Status: batch.StatusDone}
	events <- batch.Event{Index: 2, Status: batch.StatusSkipped}
	events <- batch.Event{Index: 3, Status: batch.StatusError}
	close(events)

	var stdout bytes.Buffer
	cancelCalled := false
	summary, interrupted := consumeDownloadEvents(&stdout, events, make(chan struct{}), func() {
		cancelCalled = true
	})

	if interrupted {
		t.Fatal("expected interrupted = false")
	}
	if cancelCalled {
		t.Fatal("cancel should not be called without interrupt")
	}
	if summary != (batchSummary{downloaded: 1, skipped: 1, failed: 1}) {
		t.Fatalf("unexpected summary: %#v", summary)
	}
	if strings.Count(stdout.String(), "\n") != 3 {
		t.Fatalf("expected 3 event lines, got:\n%s", stdout.String())
	}
}

func TestConsumeDownloadEventsCallsCancelAndDrainsOnInterrupt(t *testing.T) {
	events := make(chan batch.Event)
	interrupt := make(chan struct{})

	go func() {
		events <- batch.Event{Index: 1, Status: batch.StatusDownloading}
		close(interrupt)
		events <- batch.Event{Index: 1, Status: batch.StatusDone}
		events <- batch.Event{Index: 2, Status: batch.StatusError}
		close(events)
	}()

	var stdout bytes.Buffer
	cancelCalled := false
	summary, interrupted := consumeDownloadEvents(&stdout, events, interrupt, func() {
		cancelCalled = true
	})

	if !interrupted {
		t.Fatal("expected interrupted = true")
	}
	if !cancelCalled {
		t.Fatal("cancel should be called on interrupt")
	}
	if summary.downloaded != 1 || summary.failed != 1 {
		t.Fatalf("unexpected summary: %#v", summary)
	}
}
