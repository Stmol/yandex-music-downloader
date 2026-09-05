package layout

import (
	"fmt"
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
	return applyMask(mask, [][2]string{
		{"{year}", yearStr},
		{"{title}", strings.TrimSpace(title)},
	})
}

// TrackFilenameBase builds a sanitized file name from the mask.
func TrackFilenameBase(track model.Track, mask string) string {
	name := ApplyTrackFileMask(mask, track)
	if name == "" {
		name = strings.TrimSpace(track.ID.String())
	}
	return sanitizeSegment(name)
}

// ApplyTrackFileMask fills the track number, year, album, title and artist into the file mask.
func ApplyTrackFileMask(mask string, track model.Track) string {
	if strings.TrimSpace(mask) == "" {
		mask = config.DefaultTrackMask
	}

	n := ""
	nn := ""
	year := ""
	albumTitle := ""
	if len(track.Albums) > 0 {
		album := track.Albums[0]
		if album.TrackPosition.Index > 0 {
			n = strconv.Itoa(album.TrackPosition.Index)
			nn = fmtTrackNumber(album.TrackPosition.Index)
		}
		if album.Year > 0 {
			year = strconv.Itoa(album.Year)
		}
		albumTitle = strings.TrimSpace(album.Title)
	}

	// Replace {nn} before {n}, otherwise "{nn}" would become "{5n}".
	return applyMask(mask, [][2]string{
		{"{nn}", nn},
		{"{n}", n},
		{"{year}", year},
		{"{album}", albumTitle},
		{"{title}", strings.TrimSpace(track.FullTitle())},
		{"{artist}", strings.TrimSpace(track.ArtistsString())},
	})
}

func fmtTrackNumber(index int) string {
	return fmt.Sprintf("%02d", index)
}

func applyMask(mask string, replacements [][2]string) string {
	result := mask
	for _, pair := range replacements {
		result = strings.ReplaceAll(result, pair[0], pair[1])
	}
	result = strings.Join(strings.Fields(result), " ")
	return strings.Trim(result, " -")
}

func sanitizeSegment(name string) string {
	return strings.TrimSpace(utils.SanitizeFilename(strings.TrimSpace(name)))
}
