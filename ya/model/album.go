package model

import (
	"fmt"
	"strings"
)

type Album struct {
	Title         string        `json:"title"`
	CoverURI      string        `json:"coverUri"`
	Genre         string        `json:"genre"`
	Year          int           `json:"year"`
	TrackCount    int           `json:"trackCount"`
	TrackPosition TrackPosition `json:"trackPosition"`
}

type TrackPosition struct {
	Index int `json:"index"`
}

func (a Album) Cover(size int) string {
	return "https://" + strings.ReplaceAll(a.CoverURI, `%%`, fmt.Sprintf("%dx%d", size, size))
}

func (a Album) Index() int {
	return a.TrackPosition.Index
}
