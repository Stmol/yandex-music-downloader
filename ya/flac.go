package ya

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"ya-music/ya/model"

	"github.com/go-flac/flacpicture"
	"github.com/go-flac/flacvorbis"
	flac "github.com/go-flac/go-flac"
)

func writeFLACTags(filename string, track model.Track, coverPath string) error {
	file, err := flac.ParseFile(filename)
	if err != nil {
		return err
	}

	comments := flacvorbis.New()
	addFLACComment(comments, "TITLE", strings.TrimSpace(track.FullTitle()))
	addFLACComment(comments, "ARTIST", strings.TrimSpace(track.ArtistsString()))

	if album := firstAlbum(track); album != nil {
		addFLACComment(comments, "ALBUM", strings.TrimSpace(album.Title))
		addFLACComment(comments, "ALBUMARTIST", strings.TrimSpace(track.ArtistsString()))
		addFLACComment(comments, "GENRE", strings.TrimSpace(album.Genre))
		if album.TrackPosition.Index > 0 {
			addFLACComment(comments, "TRACKNUMBER", strconv.Itoa(album.TrackPosition.Index))
		}
		if album.TrackPosition.Volume > 0 {
			addFLACComment(comments, "DISCNUMBER", strconv.Itoa(album.TrackPosition.Volume))
		}
	}

	addFLACComment(comments, "DATE", trackYear(track))
	if trackID := strings.TrimSpace(track.ID.String()); trackID != "" {
		addFLACComment(comments, "YANDEX_TRACK_ID", trackID)
	}
	if trackURL := yandexTrackURL(track); trackURL != "" {
		addFLACComment(comments, "COMMENT", trackURL)
	}

	vorbisBlock := comments.Marshal()
	metadata := make([]*flac.MetaDataBlock, 0, len(file.Meta)+2)
	for _, block := range file.Meta {
		if block.Type == flac.VorbisComment || block.Type == flac.Picture {
			continue
		}
		metadata = append(metadata, block)
	}
	metadata = append(metadata, &vorbisBlock)

	if pictureBlock, ok := readFLACCoverPicture(coverPath); ok {
		metadata = append(metadata, pictureBlock)
	}

	file.Meta = metadata
	return file.Save(filename)
}

func addFLACComment(comments *flacvorbis.MetaDataBlockVorbisComment, key string, value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	_ = comments.Add(key, value)
}

func readFLACCoverPicture(coverPath string) (*flac.MetaDataBlock, bool) {
	if strings.TrimSpace(coverPath) == "" {
		return nil, false
	}

	picture, err := os.ReadFile(coverPath)
	if err != nil || len(picture) == 0 {
		return nil, false
	}

	mimeType := http.DetectContentType(picture)
	if !strings.HasPrefix(mimeType, "image/") {
		return nil, false
	}

	flacPicture, err := flacpicture.NewFromImageData(flacpicture.PictureTypeFrontCover, "Cover", picture, mimeType)
	if err != nil {
		return nil, false
	}

	block := flacPicture.Marshal()
	return &block, true
}

func yandexTrackURL(track model.Track) string {
	trackID := strings.TrimSpace(track.ID.String())
	if trackID == "" {
		return ""
	}
	if album := firstAlbum(track); album != nil {
		albumID := strings.TrimSpace(album.ID.String())
		if albumID != "" {
			return fmt.Sprintf("https://music.yandex.ru/album/%s/track/%s", albumID, trackID)
		}
	}
	return fmt.Sprintf("https://music.yandex.ru/track/%s", trackID)
}
