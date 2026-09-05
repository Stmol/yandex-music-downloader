package layout

import (
	"path/filepath"
	"strconv"
	"strings"
	"ya-music/internal/config"
	"ya-music/utils"
	"ya-music/ya/model"
)

// Options defines how the destination directory is derived from a track.
type Options struct {
	Layout       config.Layout
	FolderArtist string
}

// TrackOutputDir builds the path under root according to the layout rules.
func TrackOutputDir(root string, track model.Track, opts Options) string {
	parts := []string{root}
	if opts.Layout.ByArtist {
		if name := sanitizeSegment(artistFolderName(track, opts.FolderArtist)); name != "" {
			parts = append(parts, name)
		}
	}
	if opts.Layout.ByAlbum {
		if name := sanitizeSegment(albumFolderName(track, opts.Layout.AlbumDirMask)); name != "" {
			parts = append(parts, name)
		}
	}
	return filepath.Join(parts...)
}

func artistFolderName(track model.Track, folderArtist string) string {
	if name := strings.TrimSpace(folderArtist); name != "" {
		return name
	}
	if len(track.Albums) > 0 && len(track.Albums[0].Artists) > 0 {
		if name := strings.TrimSpace(track.Albums[0].Artists[0].Name); name != "" {
			return name
		}
	}
	if len(track.Artists) > 0 {
		if name := strings.TrimSpace(track.Artists[0].Name); name != "" {
			return name
		}
	}
	return strings.TrimSpace(track.ArtistsString())
}

func albumFolderName(track model.Track, mask string) string {
	if len(track.Albums) == 0 {
		return ""
	}
	album := track.Albums[0]
	title := strings.TrimSpace(album.Title)
	if title == "" {
		title = strings.TrimSpace(album.ID.String())
	}
	if title == "" && album.Year == 0 {
		return ""
	}
	if strings.TrimSpace(mask) == "" {
		mask = "{title}"
	}
	return ApplyAlbumDirMask(mask, album.Year, title)
}

// ApplyAlbumDirMask fills the release year and title into the folder mask.
func ApplyAlbumDirMask(mask string, year int, title string) string {
	yearStr := ""
	if year > 0 {
		yearStr = strconv.Itoa(year)
	}
	result := strings.ReplaceAll(mask, "{year}", yearStr)
	result = strings.ReplaceAll(result, "{title}", strings.TrimSpace(title))
	result = strings.Join(strings.Fields(result), " ")
	return strings.Trim(result, " -")
}

func sanitizeSegment(name string) string {
	return strings.TrimSpace(utils.SanitizeFilename(strings.TrimSpace(name)))
}
