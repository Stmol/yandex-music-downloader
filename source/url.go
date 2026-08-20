package source

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/google/uuid"
)

var (
	yandexMusicHostPattern = `music\.yandex\.(?:ru|com|kz|by|uz)`
	trackPattern           = regexp.MustCompile(`^(?:https?://)?` + yandexMusicHostPattern + `/album/(?P<albumId>\d+)/track/(?P<trackId>\d+)(?:\?.*)?$`)
	albumPattern           = regexp.MustCompile(`^(?:https?://)?` + yandexMusicHostPattern + `/album/(?P<albumId>\d+)(?:\?.*)?$`)
	playlistPattern        = regexp.MustCompile(`^(?:https?://)?` + yandexMusicHostPattern + `/users/(?P<username>[^/]+)/playlists/(?P<playlistId>\d+)(?:\?.*)?$`)
	playlistUUIDPattern    = regexp.MustCompile(`^(?:https?://)?` + yandexMusicHostPattern + `/playlists/(?P<playlistUuid>(?:[a-z]{2}\.)?[0-9a-fA-F-]{36})(?:\?.*)?$`)
	chartPattern           = regexp.MustCompile(`^(?:https?://)?` + yandexMusicHostPattern + `/chart(?:/(?P<region>[a-z]+))?(?:\?.*)?$`)
)

type Kind int

const (
	KindTrack Kind = iota
	KindAlbum
	KindLegacyPlaylist
	KindPlaylistUUID
	KindChart
)

type Ref struct {
	Kind         Kind
	TrackID      string
	AlbumID      string
	PlaylistID   string
	PlaylistUUID string
	Username     string
	Region       string
}

func Parse(input string) (*Ref, error) {
	ref := parseRef(input)
	if ref == nil {
		return nil, fmt.Errorf("invalid URL")
	}
	return ref, nil
}

func parseRef(input string) *Ref {
	if matches := trackPattern.FindStringSubmatch(input); matches != nil {
		return &Ref{
			Kind:    KindTrack,
			TrackID: matches[2],
		}
	}
	if matches := albumPattern.FindStringSubmatch(input); matches != nil {
		return &Ref{
			Kind:    KindAlbum,
			AlbumID: matches[1],
		}
	}
	if matches := playlistPattern.FindStringSubmatch(input); matches != nil {
		return &Ref{
			Kind:       KindLegacyPlaylist,
			PlaylistID: matches[2],
			Username:   matches[1],
		}
	}
	if matches := playlistUUIDPattern.FindStringSubmatch(input); matches != nil {
		playlistID := matches[1]
		uuidPart := playlistID
		if prefix, rest, found := strings.Cut(playlistID, "."); found {
			if len(prefix) != 2 {
				return nil
			}
			uuidPart = rest
		}
		if _, err := uuid.Parse(uuidPart); err != nil {
			return nil
		}

		return &Ref{
			Kind:         KindPlaylistUUID,
			PlaylistUUID: playlistID,
		}
	}
	if matches := chartPattern.FindStringSubmatch(input); matches != nil {
		return &Ref{
			Kind:   KindChart,
			Region: matches[1],
		}
	}
	return nil
}
