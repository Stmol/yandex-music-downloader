package batch

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"ya-music/ya"
	"ya-music/ya/model"
)

const DefaultConcurrency = 3

type Status string

const (
	StatusDownloading Status = "downloading"
	StatusDone        Status = "done"
	StatusSkipped     Status = "skipped"
	StatusError       Status = "error"
)

type Container string

const (
	ContainerMP3  Container = "mp3"
	ContainerFLAC Container = "flac"
	ContainerM4A  Container = "m4a"
)

const (
	SkipUnavailable   = "unavailable"
	SkipDuplicate     = "duplicate"
	SkipAlreadyExists = "already exists"
)

type Event struct {
	Index  int
	Track  model.Track
	Status Status
	Format Container
	Reason string
}

type trackDownloader interface {
	DownloadTrackWithOptions(model.Track, string, ya.DownloadOptions) (string, error)
}

type Config struct {
	Client      trackDownloader
	Tracks      []model.Track
	OutputDir   string
	Options     ya.DownloadOptions
	Concurrency int
	Context     context.Context
}

// Run downloads all eligible tracks and reports immutable lifecycle events.
func Run(config Config) <-chan Event {
	events := make(chan Event)
	ctx := config.Context
	if ctx == nil {
		ctx = context.Background()
	}
	concurrency := config.Concurrency
	if concurrency <= 0 {
		concurrency = DefaultConcurrency
	}

	go func() {
		defer close(events)

		seenIDs := make(map[string]struct{}, len(config.Tracks))
		suffixes := filenameSuffixes(config.Tracks)
		sem := make(chan struct{}, concurrency)
		var workers sync.WaitGroup

		for position, track := range config.Tracks {
			if ctx.Err() != nil {
				break
			}

			event := Event{Index: position + 1, Track: track}
			if !track.Available {
				event.Status = StatusSkipped
				event.Reason = SkipUnavailable
				events <- event
				continue
			}

			id := track.ID.String()
			if id != "" {
				if _, exists := seenIDs[id]; exists {
					event.Status = StatusSkipped
					event.Reason = SkipDuplicate
					events <- event
					continue
				}
				seenIDs[id] = struct{}{}
			}

			acquired := false
			select {
			case sem <- struct{}{}:
				acquired = true
			case <-ctx.Done():
			}
			if !acquired {
				break
			}

			options := config.Options
			options.FilenameSuffix = suffixes[position]

			workers.Add(1)
			go func(event Event, options ya.DownloadOptions) {
				defer workers.Done()
				defer func() { <-sem }()
				defer func() {
					if recovered := recover(); recovered != nil {
						event.Status = StatusError
						event.Reason = fmt.Sprintf("panic: %v", recovered)
						events <- event
					}
				}()

				if ctx.Err() != nil {
					return
				}

				event.Status = StatusDownloading
				events <- event

				filename, err := config.Client.DownloadTrackWithOptions(event.Track, config.OutputDir, options)
				if errors.Is(err, ya.ErrTrackAlreadyExists) {
					event.Status = StatusSkipped
					event.Reason = SkipAlreadyExists
					event.Format = formatFromFilename(filename)
				} else if err != nil {
					event.Status = StatusError
					event.Reason = err.Error()
				} else {
					event.Status = StatusDone
					event.Format = formatFromFilename(filename)
				}
				events <- event
			}(event, options)
		}

		workers.Wait()
	}()

	return events
}

func filenameSuffixes(tracks []model.Track) []string {
	idsByFilenameKey := make(map[string]map[string]struct{}, len(tracks))
	for _, track := range tracks {
		key := ya.TrackFilenameKey(track)
		if idsByFilenameKey[key] == nil {
			idsByFilenameKey[key] = make(map[string]struct{})
		}
		idsByFilenameKey[key][track.ID.String()] = struct{}{}
	}

	suffixes := make([]string, len(tracks))
	for position, track := range tracks {
		if len(idsByFilenameKey[ya.TrackFilenameKey(track)]) > 1 {
			suffixes[position] = fmt.Sprintf("[%s]", track.ID.String())
		}
	}

	return suffixes
}

func formatFromFilename(filename string) Container {
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".flac":
		return ContainerFLAC
	case ".m4a", ".mp4":
		return ContainerM4A
	default:
		return ContainerMP3
	}
}
