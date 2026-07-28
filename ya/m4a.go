package ya

import (
	"fmt"
	"strings"

	"github.com/tommyo123/mtag"
)

const (
	maxM4ATaggingFileSize = 2 << 30
	m4aSourceURLField     = "----:com.yandex-music-downloader:source-url"
)

var m4aTaggingFileSizeLimit int64 = maxM4ATaggingFileSize

type m4aTagInput struct {
	Title       string
	Artist      string
	Album       string
	AlbumArtist string
	Genre       string
	Year        int
	Track       int
	TrackTotal  int
	Disc        int
	DiscTotal   int
	SourceURL   string
	CoverMIME   string
	CoverData   []byte
}

func writeM4ATags(path string, tags m4aTagInput) error {
	file, err := mtag.Open(path, mtag.WithMaxFileSize(m4aTaggingFileSizeLimit))
	if err != nil {
		return fmt.Errorf("open M4A metadata: %w", err)
	}
	defer func() {
		_ = file.Close()
	}()

	if file.Container() != mtag.ContainerMP4 {
		return fmt.Errorf("unsupported M4A container: %s", file.Container())
	}

	if value := strings.TrimSpace(tags.Title); value != "" {
		file.SetTitle(value)
	}
	if value := strings.TrimSpace(tags.Artist); value != "" {
		file.SetArtist(value)
	}
	if value := strings.TrimSpace(tags.AlbumArtist); value != "" {
		file.SetAlbumArtist(value)
	}
	if value := strings.TrimSpace(tags.Album); value != "" {
		file.SetAlbum(value)
	}
	if value := strings.TrimSpace(tags.Genre); value != "" {
		file.SetGenre(value)
	}
	if tags.Year > 0 {
		file.SetYear(tags.Year)
	}
	if tags.Track > 0 || tags.TrackTotal > 0 {
		file.SetTrack(tags.Track, tags.TrackTotal)
	}
	if tags.Disc > 0 || tags.DiscTotal > 0 {
		file.SetDisc(tags.Disc, tags.DiscTotal)
	}
	if value := strings.TrimSpace(tags.SourceURL); value != "" {
		file.SetCustomValues(m4aSourceURLField, value)
	}
	if strings.TrimSpace(tags.CoverMIME) != "" && len(tags.CoverData) > 0 {
		file.SetCoverArt(tags.CoverMIME, tags.CoverData)
	}

	if err := file.Save(); err != nil {
		return fmt.Errorf("save M4A metadata: %w", err)
	}
	return nil
}
