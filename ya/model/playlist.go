package model

import "time"

type Playlist struct {
	Available          bool         `json:"available"`
	Collective         bool         `json:"collective"`
	Cover              Cover        `json:"cover"`
	Created            time.Time    `json:"created"`
	DurationMs         int          `json:"durationMs"`
	HasTrailer         bool         `json:"hasTrailer"`
	IsBanner           bool         `json:"isBanner"`
	IsPremiere         bool         `json:"isPremiere"`
	Kind               int          `json:"kind"`
	LastOwnerPlaylists []Playlist   `json:"lastOwnerPlaylists"`
	LikesCount         int          `json:"likesCount"`
	Modified           time.Time    `json:"modified"`
	OGImage            string       `json:"ogImage"`
	Owner              User         `json:"owner"`
	Pager              Pager        `json:"pager"`
	PlaylistUUID       string       `json:"playlistUuid"`
	Revision           int          `json:"revision"`
	Snapshot           int          `json:"snapshot"`
	Tags               []string     `json:"tags"`
	Title              string       `json:"title"`
	TrackCount         int          `json:"trackCount"`
	Tracks             []TrackShort `json:"tracks"`
	UID                int          `json:"uid"`
	Visibility         string       `json:"visibility"`
}
