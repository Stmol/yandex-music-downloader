package ya

import (
	"net/http"
	"os"
	"strconv"
	"strings"
	"ya-music/ya/model"

	"github.com/bogem/id3v2/v2"
)

const yandexTrackOwnerIdentifier = "music.yandex.ru"

func writeID3Tags(filename string, track model.Track, coverPath string) error {
	tag, err := id3v2.Open(filename, id3v2.Options{Parse: true})
	if err != nil {
		return err
	}
	defer tag.Close()

	tag.SetVersion(4)
	tag.SetDefaultEncoding(id3v2.EncodingUTF8)

	title := strings.TrimSpace(track.FullTitle())
	if title != "" {
		tag.SetTitle(title)
	}

	artist := strings.TrimSpace(track.ArtistsString())
	if artist != "" {
		tag.SetArtist(artist)
	}

	if album := firstAlbum(track); album != nil {
		if album.Title != "" {
			tag.SetAlbum(strings.TrimSpace(album.Title))
		}
		if album.Genre != "" {
			tag.SetGenre(strings.TrimSpace(album.Genre))
		}
		if album.TrackPosition.Index > 0 {
			tag.AddTextFrame(tag.CommonID("Track number/Position in set"), id3v2.EncodingUTF8, strconv.Itoa(album.TrackPosition.Index))
		}
	}

	if year := trackYear(track); year != "" {
		tag.SetYear(year)
	}

	trackID := strings.TrimSpace(track.ID.String())
	if trackID != "" {
		tag.DeleteFrames(tag.CommonID("Unique file identifier"))
		tag.AddUFIDFrame(id3v2.UFIDFrame{
			OwnerIdentifier: yandexTrackOwnerIdentifier,
			Identifier:      []byte(trackID),
		})
	}

	if picture, ok := readCoverPicture(coverPath); ok {
		tag.DeleteFrames(tag.CommonID("Attached picture"))
		tag.AddAttachedPicture(picture)
	}

	return tag.Save()
}

func firstAlbum(track model.Track) *model.Album {
	if len(track.Albums) == 0 {
		return nil
	}
	return &track.Albums[0]
}

func trackYear(track model.Track) string {
	if album := firstAlbum(track); album != nil {
		if album.Year > 0 {
			return strconv.Itoa(album.Year)
		}
		if year := yearFromDate(album.ReleaseDate); year != "" {
			return year
		}
	}

	if track.MetaData.Year > 0 {
		return strconv.Itoa(track.MetaData.Year)
	}

	return ""
}

func yearFromDate(date string) string {
	date = strings.TrimSpace(date)
	if len(date) < 4 {
		return ""
	}

	year := date[:4]
	if _, err := strconv.Atoi(year); err != nil {
		return ""
	}
	return year
}

func readCoverPicture(coverPath string) (id3v2.PictureFrame, bool) {
	if strings.TrimSpace(coverPath) == "" {
		return id3v2.PictureFrame{}, false
	}

	picture, err := os.ReadFile(coverPath)
	if err != nil || len(picture) == 0 {
		return id3v2.PictureFrame{}, false
	}

	mimeType := http.DetectContentType(picture)
	if !strings.HasPrefix(mimeType, "image/") {
		return id3v2.PictureFrame{}, false
	}

	return id3v2.PictureFrame{
		Encoding:    id3v2.EncodingUTF8,
		MimeType:    mimeType,
		PictureType: id3v2.PTFrontCover,
		Description: "Cover",
		Picture:     picture,
	}, true
}
