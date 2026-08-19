package batch

import (
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

type Event struct {
	Index  int
	Track  model.Track
	Status Status
	Format string
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
}

// Run downloads all eligible tracks and reports immutable lifecycle events.
func Run(config Config) <-chan Event {
	events := make(chan Event)
	concurrency := config.Concurrency
	if concurrency <= 0 {
		concurrency = DefaultConcurrency
	}

	go func() {
		defer close(events)

		seenIDs := make(map[string]struct{}, len(config.Tracks))
		seenNames := make(map[string]struct{}, len(config.Tracks))
		sem := make(chan struct{}, concurrency)
		var workers sync.WaitGroup

		for position, track := range config.Tracks {
			event := Event{Index: position + 1, Track: track}
			if !track.Available {
				event.Status = StatusSkipped
				event.Reason = "unavailable"
				events <- event
				continue
			}

			id := track.ID.String()
			name := track.FullTitle() + " - " + track.ArtistsString()
			if _, exists := seenIDs[id]; exists {
				event.Status = StatusSkipped
				event.Reason = "duplicate"
				events <- event
				continue
			}
			if _, exists := seenNames[name]; exists {
				event.Status = StatusSkipped
				event.Reason = "duplicate"
				events <- event
				continue
			}
			seenIDs[id] = struct{}{}
			seenNames[name] = struct{}{}

			workers.Add(1)
			go func(event Event) {
				defer workers.Done()
				defer func() {
					if recovered := recover(); recovered != nil {
						event.Status = StatusError
						event.Reason = fmt.Sprintf("panic: %v", recovered)
						events <- event
					}
				}()

				sem <- struct{}{}
				defer func() { <-sem }()

				event.Status = StatusDownloading
				events <- event

				filename, err := config.Client.DownloadTrackWithOptions(event.Track, config.OutputDir, config.Options)
				if errors.Is(err, ya.ErrTrackAlreadyExists) {
					event.Status = StatusSkipped
					event.Reason = "already exists"
					event.Format = formatFromFilename(filename)
				} else if err != nil {
					event.Status = StatusError
					event.Reason = err.Error()
				} else {
					event.Status = StatusDone
					event.Format = formatFromFilename(filename)
				}
				events <- event
			}(event)
		}

		workers.Wait()
	}()

	return events
}

func formatFromFilename(filename string) string {
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".flac":
		return "flac"
	case ".m4a", ".mp4":
		return "m4a"
	default:
		return "mp3"
	}
}
