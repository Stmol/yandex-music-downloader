package source

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"ya-music/utils"
	"ya-music/ya/model"
)

const (
	artistAlbumFetchConcurrency = 2
	artistAlbumFetchAttempts    = 4
)

var albumFetchRetryBackoff = defaultAlbumFetchRetryBackoff

func defaultAlbumFetchRetryBackoff(attempt int) time.Duration {
	return time.Duration(250*(1<<attempt)) * time.Millisecond
}

type Client interface {
	TrackInfo(id string) (*model.Track, error)
	AlbumWithTracks(id string) (*model.Album, error)
	ArtistDirectAlbums(id string) ([]model.Album, error)
	UsersPlaylist(id string, username string) (*model.Playlist, error)
	PlaylistByUUID(id string) (*model.Playlist, error)
	Chart(region string) (*model.Playlist, error)
}

func Resolve(client Client, input string) ([]model.Track, error) {
	ref, err := Parse(strings.TrimSpace(input))
	if err != nil {
		return nil, err
	}
	return ResolveRef(client, ref)
}

func ResolveRef(client Client, ref *Ref) ([]model.Track, error) {
	return resolveRef(client, ref)
}

func resolvePlaylist(load func() (*model.Playlist, error)) ([]model.Track, error) {
	playlist, err := load()
	if err != nil {
		return nil, err
	}
	return playlist.TracksList(), nil
}

func resolveRef(client Client, ref *Ref) ([]model.Track, error) {
	switch ref.Kind {
	case KindTrack:
		track, err := client.TrackInfo(ref.TrackID)
		if err != nil {
			return nil, err
		}
		return []model.Track{*track}, nil
	case KindAlbum:
		album, err := client.AlbumWithTracks(ref.AlbumID)
		if err != nil {
			return nil, err
		}
		var tracks []model.Track
		for _, volume := range album.Volumes {
			tracks = append(tracks, volume...)
		}
		return tracks, nil
	case KindArtist:
		return resolveArtist(client, ref.ArtistID)
	case KindLegacyPlaylist:
		return resolvePlaylist(func() (*model.Playlist, error) {
			return client.UsersPlaylist(ref.PlaylistID, ref.Username)
		})
	case KindPlaylistUUID:
		return resolvePlaylist(func() (*model.Playlist, error) {
			return client.PlaylistByUUID(ref.PlaylistUUID)
		})
	case KindChart:
		return resolvePlaylist(func() (*model.Playlist, error) {
			return client.Chart(ref.Region)
		})
	default:
		return nil, fmt.Errorf("unsupported URL type")
	}
}

// resolveArtist returns the tracks of the artist's own albums and singles.
func resolveArtist(client Client, artistID string) ([]model.Track, error) {
	albums, err := client.ArtistDirectAlbums(artistID)
	if err != nil {
		return nil, err
	}
	return collectArtistAlbumTracks(client, albums)
}

// collectArtistAlbumTracks loads every album with a small concurrency limit.
// An album that still fails after retries is skipped; the error is returned
// only when no album could be loaded.
func collectArtistAlbumTracks(client Client, albums []model.Album) ([]model.Track, error) {
	if len(albums) == 0 {
		return nil, nil
	}

	results := make([][]model.Track, len(albums))
	errs := make([]error, len(albums))
	sem := make(chan struct{}, artistAlbumFetchConcurrency)
	var wg sync.WaitGroup

	for i, album := range albums {
		wg.Add(1)
		go func(i int, album model.Album) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			full, err := fetchAlbumWithTracks(client, album.ID.String())
			if err != nil {
				errs[i] = err
				return
			}
			parent := *full
			if parent.Year == 0 {
				parent.Year = album.Year
			}
			if strings.TrimSpace(parent.Title) == "" {
				parent.Title = album.Title
			}
			if len(parent.Artists) == 0 {
				parent.Artists = album.Artists
			}
			results[i] = attachAlbumMeta(flattenVolumes(full), parent)
		}(i, album)
	}
	wg.Wait()

	var tracks []model.Track
	var firstErr error
	for i, err := range errs {
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		tracks = append(tracks, results[i]...)
	}
	if len(tracks) == 0 && firstErr != nil {
		return nil, firstErr
	}
	return tracks, nil
}

// fetchAlbumWithTracks retries rate-limited (429) and server (5xx) responses with backoff.
func fetchAlbumWithTracks(client Client, albumID string) (*model.Album, error) {
	var lastErr error
	for attempt := 0; attempt < artistAlbumFetchAttempts; attempt++ {
		album, err := client.AlbumWithTracks(albumID)
		if err == nil {
			return album, nil
		}
		lastErr = err
		if !utils.IsRetryableHTTPError(err) || attempt == artistAlbumFetchAttempts-1 {
			break
		}
		if delay := albumFetchRetryBackoff(attempt); delay > 0 {
			time.Sleep(delay)
		}
	}
	return nil, lastErr
}

func flattenVolumes(album *model.Album) []model.Track {
	if album == nil {
		return nil
	}
	var tracks []model.Track
	for _, volume := range album.Volumes {
		tracks = append(tracks, volume...)
	}
	return tracks
}

// attachAlbumMeta fills missing album metadata of each track from the release it is downloaded from.
func attachAlbumMeta(tracks []model.Track, album model.Album) []model.Track {
	for i := range tracks {
		tracks[i].Albums = mergeTrackAlbum(tracks[i].Albums, album)
	}
	return tracks
}

func mergeTrackAlbum(existing []model.Album, parent model.Album) []model.Album {
	meta := parent
	meta.Volumes = nil
	if len(existing) == 0 {
		return []model.Album{meta}
	}

	current := existing[0]
	if strings.TrimSpace(current.Title) == "" {
		current.Title = parent.Title
	}
	if current.Year == 0 {
		current.Year = parent.Year
	}
	if len(current.Artists) == 0 {
		current.Artists = parent.Artists
	}
	if current.ID.String() == "" {
		current.ID = parent.ID
	}
	existing[0] = current
	return existing
}
