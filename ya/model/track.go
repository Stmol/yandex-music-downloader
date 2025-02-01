package model

import (
	"encoding/json"
	"fmt"
	"strings"
)

type Track struct {
	Available         bool          `json:"available"`
	Artists           []Artist      `json:"artists"`
	Albums            []interface{} `json:"albums"`
	CanPublish        bool          `json:"canPublish"`
	CoverURI          string        `json:"coverUri,omitempty"`
	DesiredVisibility string        `json:"desiredVisibility"`
	DurationMs        int           `json:"durationMs"`
	Filename          string        `json:"filename"`
	ID                json.Number   `json:"id"`
	MetaData          MetaData      `json:"metaData,omitempty"`
	Title             string        `json:"title"`
	Version           string        `json:"version"`
}

func (t Track) FullTitle() string {
	return strings.TrimSpace(fmt.Sprintf("%s %s", t.Title, t.Version))
}

func (t Track) ArtistsString() string {
	artists := make([]string, len(t.Artists))
	for i, artist := range t.Artists {
		artists[i] = artist.Name
	}
	return strings.Join(artists, ", ")
}
