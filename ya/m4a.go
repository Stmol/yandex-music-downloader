package ya

import (
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"ya-music/ya/model"

	"github.com/gcottom/mp4meta"
)

func writeM4ATags(filename string, track model.Track, coverPath string) error {
	input, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer input.Close()

	tag, err := mp4meta.ReadMP4(input)
	if err != nil {
		return err
	}
	tag.ClearAllTags()

	tag.SetTitle(strings.TrimSpace(track.FullTitle()))
	tag.SetArtist(strings.TrimSpace(track.ArtistsString()))
	tag.SetAlbumArtist(strings.TrimSpace(track.ArtistsString()))

	if album := firstAlbum(track); album != nil {
		tag.SetAlbum(strings.TrimSpace(album.Title))
		tag.SetGenre(strings.TrimSpace(album.Genre))
		if album.TrackPosition.Index > 0 {
			tag.SetTrackNumber(album.TrackPosition.Index)
		}
		if album.TrackCount > 0 {
			tag.SetTrackTotal(album.TrackCount)
		}
		if album.TrackPosition.Volume > 0 {
			tag.SetDiscNumber(album.TrackPosition.Volume)
		}
	}

	if year := trackYear(track); year != "" {
		if parsedYear, err := strconv.Atoi(year); err == nil {
			tag.SetYear(parsedYear)
		}
	}
	if trackURL := yandexTrackURL(track); trackURL != "" {
		tag.SetComments(trackURL)
	}
	if cover, ok := readM4ACoverImage(coverPath); ok {
		tag.SetCoverArt(cover)
	}
	if _, err := input.Seek(0, io.SeekStart); err != nil {
		return err
	}

	tempFile, err := os.CreateTemp(filepath.Dir(filename), filepath.Base(filename)+".*.tagged")
	if err != nil {
		return err
	}
	tempFilename := tempFile.Name()
	cleanup := true
	defer func() {
		_ = tempFile.Close()
		if cleanup {
			_ = os.Remove(tempFilename)
		}
	}()

	if err := tag.Save(tempFile); err != nil {
		return err
	}
	if err := tempFile.Close(); err != nil {
		return err
	}
	if err := input.Close(); err != nil {
		return err
	}

	if err := os.Rename(tempFilename, filename); err != nil {
		return fmt.Errorf("error replacing tagged m4a file: %w", err)
	}
	cleanup = false
	return nil
}

func readM4ACoverImage(coverPath string) (*image.Image, bool) {
	if strings.TrimSpace(coverPath) == "" {
		return nil, false
	}

	file, err := os.Open(coverPath)
	if err != nil {
		return nil, false
	}
	defer file.Close()

	img, _, err := image.Decode(file)
	if err != nil {
		return nil, false
	}
	return &img, true
}
