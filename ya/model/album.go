package model

type Album struct {
	ID            FlexibleID    `json:"id"`
	Title         string        `json:"title"`
	TrackCount    int           `json:"trackCount,omitempty"`
	Available     bool          `json:"available"`
	CoverURI      string        `json:"coverUri,omitempty"`
	Genre         string        `json:"genre,omitempty"`
	Year          int           `json:"year,omitempty"`
	ReleaseDate   string        `json:"releaseDate,omitempty"`
	TrackPosition TrackPosition `json:"trackPosition,omitempty"`
	Volumes       [][]Track     `json:"volumes,omitempty"`
}

type TrackPosition struct {
	Volume int `json:"volume,omitempty"`
	Index  int `json:"index,omitempty"`
}
