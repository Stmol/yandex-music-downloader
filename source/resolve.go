package source

import (
	"fmt"
	"strings"

	"ya-music/ya/model"
)

type Client interface {
	TrackInfo(id string) (*model.Track, error)
	AlbumWithTracks(id string) (*model.Album, error)
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
	case KindLegacyPlaylist:
		playlist, err := client.UsersPlaylist(ref.PlaylistID, ref.Username)
		if err != nil {
			return nil, err
		}
		return playlist.TracksList(), nil
	case KindPlaylistUUID:
		playlist, err := client.PlaylistByUUID(ref.PlaylistUUID)
		if err != nil {
			return nil, err
		}
		return playlist.TracksList(), nil
	case KindChart:
		playlist, err := client.Chart(ref.Region)
		if err != nil {
			return nil, err
		}
		return playlist.TracksList(), nil
	default:
		return nil, fmt.Errorf("unsupported URL type")
	}
}
